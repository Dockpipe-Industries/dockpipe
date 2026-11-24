package remote

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"github.com/coder/websocket"
)

func (client *Client) NetworkCatalog(ctx context.Context) ([]contract.NetworkCatalog, error) {
	var catalogs []contract.NetworkCatalog
	err := client.Call(ctx, "/v1/network-catalog", map[string]string{"schema": contract.NetworkVersion}, &catalogs)
	if err != nil {
		return nil, err
	}
	if len(catalogs) > 64 {
		return nil, errors.New("network catalog capacity exceeded")
	}
	for _, catalog := range catalogs {
		if catalog.Validate() != nil {
			return nil, errors.New("invalid network catalog response")
		}
	}
	return catalogs, nil
}

func (client *Client) attachNetwork(ctx context.Context, ticket contract.NetworkTicket) (*websocket.Conn, error) {
	if ticket.Request.Validate() != nil || !validMachineNonce(ticket.ID) || !validMachineNonce(ticket.Secret) {
		return nil, errors.New("invalid network ticket")
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+client.Token)
	headers.Set(networkTicketHeader, ticket.Secret)
	httpClient := *client.HTTP
	httpClient.Timeout = 0
	handshake, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	socket, _, err := websocket.Dial(handshake, client.Endpoint+contract.NetworkRelayPath, &websocket.DialOptions{HTTPClient: &httpClient, HTTPHeader: headers, Subprotocols: []string{contract.NetworkProtocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return nil, errors.New("authenticated TCP relay attachment failed")
	}
	if socket.Subprotocol() != contract.NetworkProtocol {
		_ = socket.CloseNow()
		return nil, errors.New("TCP relay protocol changed")
	}
	socket.SetReadLimit(contract.MaxNetworkFrame)
	return socket, nil
}

func (client *Client) networkDial(ctx context.Context, target contract.FeatureTarget, address string) (net.Conn, error) {
	request := contract.NetworkConnect{Schema: contract.NetworkVersion, Target: target, Address: address}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	var ticket contract.NetworkTicket
	if err := client.Call(ctx, "/v1/network-connect", request, &ticket); err != nil {
		return nil, err
	}
	if ticket.Request != request {
		return nil, errors.New("network ticket changed exact target")
	}
	socket, err := client.attachNetwork(ctx, ticket)
	if err != nil {
		return nil, err
	}
	ready, cancel := context.WithTimeout(ctx, 15*time.Second)
	kind, payload, err := socket.Read(ready)
	cancel()
	if err != nil || kind != websocket.MessageBinary || len(payload) != 1 || payload[0] != 1 {
		_ = socket.CloseNow()
		return nil, errors.New("remote TCP destination unavailable")
	}
	return websocket.NetConn(ctx, socket, websocket.MessageBinary), nil
}

// ServeNetworkTCP resolves and opens the destination exclusively on the worker.
// Cancellation closes both sides and both pumps are joined before return.
func (client *Client) ServeNetworkTCP(ctx context.Context, ticket contract.NetworkTicket, permission contract.NetworkPermissions) error {
	if ticket.Request.Validate() != nil {
		return errors.New("invalid network directive")
	}
	socket, err := client.attachNetwork(ctx, ticket)
	if err != nil {
		return err
	}
	defer socket.CloseNow()

	if !permission.AllowsDestination(ticket.Request.Address) {
		return errors.New("worker TCP destination is outside the network grant")
	}

	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", ticket.Request.Address)
	if err != nil {
		return errors.New("worker TCP destination unavailable")
	}
	defer connection.Close()
	ready, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = socket.Write(ready, websocket.MessageBinary, []byte{1})
	cancel()
	if err != nil {
		return err
	}
	relay := websocket.NetConn(ctx, socket, websocket.MessageBinary)
	defer relay.Close()
	return copyNetwork(ctx, connection, relay)
}

func copyNetwork(ctx context.Context, left, right io.ReadWriteCloser) error {
	stop := context.AfterFunc(ctx, func() {
		_ = left.Close()
		_ = right.Close()
	})
	defer stop()
	results := make(chan error, 2)
	pump := func(destination, source io.ReadWriteCloser) {
		buffer := make([]byte, contract.MaxNetworkFrame)
		count, err := io.CopyBuffer(destination, io.LimitReader(source, int64(contract.MaxNetworkBytes)+1), buffer)
		if count > int64(contract.MaxNetworkBytes) {
			err = errors.New("TCP stream transfer budget exceeded")
		}
		_ = left.Close()
		_ = right.Close()
		results <- err
	}
	go pump(left, right)
	go pump(right, left)
	first := <-results
	second := <-results
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if first != nil {
		return first
	}
	return second
}

type NetworkProxy struct {
	client   *Client
	target   contract.FeatureTarget
	password string
	slots    chan struct{}
	context  context.Context
}

// StartNetworkProxy binds a new IPv4 loopback port. Every request requires a
// per-instance random credential; redirects and destinations never dial locally.
func StartNetworkProxy(ctx context.Context, client *Client, target contract.FeatureTarget) (net.Listener, *http.Server, string, error) {
	if target.Validate() != nil {
		return nil, nil, "", errors.New("invalid network proxy target")
	}
	catalogs, err := client.NetworkCatalog(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	found := false
	for _, catalog := range catalogs {
		if catalog.Target == target && catalog.Enabled {
			found = true
		}
	}
	if !found {
		return nil, nil, "", errors.New("exact network target is offline or not granted")
	}
	password, err := Secret()
	if err != nil {
		return nil, nil, "", err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, "", err
	}
	proxy := &NetworkProxy{client: client, target: target, password: password, slots: make(chan struct{}, contract.MaxNetworkNodeStreams), context: ctx}
	server := &http.Server{Handler: proxy, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: contract.NetworkIdle, WriteTimeout: contract.NetworkIdle, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	return newNetworkListener(listener), server, password, nil
}

func (proxy *NetworkProxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("dockpipe:"+proxy.password))
	if len(request.Header.Values("Proxy-Authorization")) != 1 || subtle.ConstantTimeCompare([]byte(request.Header.Get("Proxy-Authorization")), []byte(expected)) != 1 {
		writer.Header().Set("Proxy-Authenticate", `Basic realm="Dockpipe"`)
		http.Error(writer, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	select {
	case proxy.slots <- struct{}{}:
		defer func() { <-proxy.slots }()
	default:
		http.Error(writer, "network connection capacity reached", 503)
		return
	}
	ctx, cancel := context.WithCancel(proxy.context)
	defer cancel()
	stop := context.AfterFunc(request.Context(), cancel)
	defer stop()
	address := request.Host
	if request.Method == http.MethodConnect {
		if contract.ValidateNetworkAddress(address) != nil {
			http.Error(writer, "invalid TCP destination", 400)
			return
		}
		connection, err := proxy.client.networkDial(ctx, proxy.target, address)
		if err != nil {
			http.Error(writer, "remote TCP destination unavailable", 502)
			return
		}
		defer connection.Close()
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			http.Error(writer, "TCP tunneling unavailable", 500)
			return
		}
		local, buffer, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer local.Close()
		if _, err = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			return
		}
		if buffer.Flush() != nil {
			return
		}
		// Retain any bytes read ahead with the CONNECT headers.
		buffered := &bufferedNetworkConn{Conn: local, reader: buffer.Reader}
		_ = copyNetwork(ctx, buffered, connection)
		return
	}
	if request.URL == nil || !request.URL.IsAbs() || request.URL.Scheme != "http" || request.URL.User != nil || request.URL.Host != request.Host {
		http.Error(writer, "absolute HTTP URL required", 400)
		return
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(request.URL.Hostname(), "80")
	}
	if contract.ValidateNetworkAddress(address) != nil {
		http.Error(writer, "invalid HTTP destination", 400)
		return
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxResponseHeaderBytes: 32 << 10, ResponseHeaderTimeout: 30 * time.Second, DialContext: func(dialCtx context.Context, _, destination string) (net.Conn, error) {
		if destination != address {
			return nil, errors.New("HTTP destination changed")
		}
		return proxy.client.networkDial(dialCtx, proxy.target, destination)
	}}
	defer transport.CloseIdleConnections()
	forwarded := request.Clone(ctx)
	forwarded.RequestURI = ""
	forwarded.Header = request.Header.Clone()
	upgrade := strings.EqualFold(forwarded.Header.Get("Upgrade"), "websocket")
	removeNetworkHopHeaders(forwarded.Header)
	if upgrade {
		forwarded.Header.Set("Connection", "Upgrade")
		forwarded.Header.Set("Upgrade", "websocket")
	}
	response, err := transport.RoundTrip(forwarded)
	if err != nil {
		http.Error(writer, "remote HTTP destination unavailable", 502)
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		tunnel, ok := response.Body.(io.ReadWriteCloser)
		hijacker, supported := writer.(http.Hijacker)
		if !upgrade || !ok || !supported || !strings.EqualFold(response.Header.Get("Upgrade"), "websocket") {
			http.Error(writer, "unsupported remote protocol upgrade", http.StatusBadGateway)
			return
		}
		local, buffer, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer local.Close()
		headers := *response
		headers.Body = nil
		if err = headers.Write(buffer); err != nil {
			return
		}
		if buffer.Flush() != nil {
			return
		}
		_ = copyNetwork(ctx, &bufferedNetworkConn{Conn: local, reader: buffer.Reader}, tunnel)
		return
	}
	removeNetworkHopHeaders(response.Header)
	for name, values := range response.Header {
		for _, value := range values {
			writer.Header().Add(name, value)
		}
	}
	writer.WriteHeader(response.StatusCode)
	_, _ = io.CopyBuffer(writer, response.Body, make([]byte, contract.MaxNetworkFrame))
}

type bufferedNetworkConn struct {
	net.Conn
	reader *bufio.Reader
}

func (connection *bufferedNetworkConn) Read(buffer []byte) (int, error) {
	return connection.reader.Read(buffer)
}

func removeNetworkHopHeaders(header http.Header) {
	for _, name := range strings.Split(header.Get("Connection"), ",") {
		header.Del(strings.TrimSpace(name))
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization", "Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}

// Keep URL construction out of callers and preserve the exact loopback address.
func NetworkProxyEndpoint(listener net.Listener) string {
	return (&url.URL{Scheme: "http", Host: listener.Addr().String()}).String()
}
