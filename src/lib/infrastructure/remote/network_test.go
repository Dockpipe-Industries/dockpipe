package remote

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"github.com/coder/websocket"
)

func networkFixture(t *testing.T) (*Broker, *Client, *Client, contract.NetworkCatalog) {
	t.Helper()
	broker, operator, worker, _, _, _, features := featureFixture(t)
	catalog := contract.NetworkCatalog{Schema: contract.NetworkVersion, Target: features.Target, Enabled: true}
	if err := worker.Call(context.Background(), "/v1/network-worker", catalog, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(broker.CloseNetworkRelays)
	return broker, operator, worker, catalog
}

func networkTicketFixture(t *testing.T) (*Broker, *Client, *Client, contract.NetworkCatalog, contract.NetworkTicket, contract.NetworkTicket) {
	t.Helper()
	broker, operator, worker, catalog := networkFixture(t)
	request := contract.NetworkConnect{Schema: contract.NetworkVersion, Target: catalog.Target, Address: "localhost:80"}
	var viewerTicket contract.NetworkTicket
	if err := operator.Call(context.Background(), "/v1/network-connect", request, &viewerTicket); err != nil {
		t.Fatal(err)
	}
	var workerTickets []contract.NetworkTicket
	if err := worker.Call(context.Background(), "/v1/network-worker", catalog, &workerTickets); err != nil || len(workerTickets) != 1 {
		t.Fatalf("worker assignment: %v %+v", err, workerTickets)
	}
	return broker, operator, worker, catalog, viewerTicket, workerTickets[0]
}

func TestNetworkAuthenticationAndExactTargets(t *testing.T) {
	broker, operator, worker, catalog := networkFixture(t)
	ctx := context.Background()
	catalogs, err := operator.NetworkCatalog(ctx)
	if err != nil || len(catalogs) != 1 {
		t.Fatal(catalogs, err)
	}
	for _, scenario := range []string{"generation", "fingerprint", "node", "port", "disabled", "stale", "revoked", "locked"} {
		t.Run(scenario, func(t *testing.T) {
			request := contract.NetworkConnect{Schema: contract.NetworkVersion, Target: catalog.Target, Address: "localhost:80"}
			broker.mutex.Lock()
			original := broker.networks.catalogs[catalog.Target.Node]
			enrollment := broker.state.Nodes[catalog.Target.Node]
			switch scenario {
			case "generation":
				request.Target.Generation = Hash("other")
			case "fingerprint":
				request.Target.Fingerprint = Hash("other")
			case "node":
				request.Target.Node = "other"
			case "port":
				request.Address = "localhost:0"
			case "disabled":
				changed := original
				changed.Enabled = false
				broker.networks.catalogs[catalog.Target.Node] = changed
			case "stale":
				changed := original
				changed.ReceivedAt = time.Now().Add(-10 * time.Second)
				broker.networks.catalogs[catalog.Target.Node] = changed
			case "revoked":
				changed := enrollment
				changed.Revoked = true
				broker.state.Nodes[catalog.Target.Node] = changed
			case "locked":
				broker.security.Lock()
			}
			broker.mutex.Unlock()
			if err := operator.Call(ctx, "/v1/network-connect", request, nil); err == nil {
				t.Fatal("unauthorized network request admitted")
			}
			broker.mutex.Lock()
			broker.networks.catalogs[catalog.Target.Node] = original
			broker.state.Nodes[catalog.Target.Node] = enrollment
			broker.mutex.Unlock()
		})
	}
	legacy, _ := NewClient(worker.Endpoint, worker.Token)
	defer legacy.HTTP.CloseIdleConnections()
	if legacy.Call(ctx, "/v1/network-worker", catalog, nil) == nil {
		t.Fatal("bearer-only worker accepted")
	}
}

func TestNetworkRelayDuplexTicketReplayAndRevocation(t *testing.T) {
	for _, reason := range []string{"lock", "revoke", "generation", "stale", "text", "oversize"} {
		t.Run(reason, func(t *testing.T) {
			broker, operator, worker, catalog, viewerTicket, workerTicket := networkTicketFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			viewer, err := operator.attachNetwork(ctx, viewerTicket)
			if err != nil {
				t.Fatal(err)
			}
			defer viewer.CloseNow()
			peer, err := worker.attachNetwork(ctx, workerTicket)
			if err != nil {
				t.Fatal(err)
			}
			defer peer.CloseNow()
			if _, err := operator.attachNetwork(ctx, viewerTicket); err == nil {
				t.Fatal("ticket replay admitted")
			}
			if err := viewer.Write(ctx, websocket.MessageBinary, []byte("private TCP")); err != nil {
				t.Fatal(err)
			}
			_, payload, err := peer.Read(ctx)
			if err != nil || string(payload) != "private TCP" {
				t.Fatal(string(payload), err)
			}
			broker.mutex.Lock()
			switch reason {
			case "lock":
				broker.security.Lock()
			case "revoke":
				entry := broker.state.Nodes[catalog.Target.Node]
				entry.Revoked = true
				broker.state.Nodes[catalog.Target.Node] = entry
			case "generation":
				entry := broker.networks.catalogs[catalog.Target.Node]
				entry.Target.Generation = Hash("restart")
				broker.networks.catalogs[catalog.Target.Node] = entry
			case "stale":
				entry := broker.networks.catalogs[catalog.Target.Node]
				entry.ReceivedAt = time.Now().Add(-10 * time.Second)
				broker.networks.catalogs[catalog.Target.Node] = entry
			}
			broker.mutex.Unlock()
			if reason == "text" {
				_ = viewer.Write(ctx, websocket.MessageText, []byte("invalid"))
			}
			if reason == "oversize" {
				_ = viewer.Write(ctx, websocket.MessageBinary, make([]byte, contract.MaxNetworkFrame+1))
			}
			if _, _, err = peer.Read(ctx); err == nil {
				t.Fatal("revoked or invalid stream stayed usable")
			}
			if ctx.Err() != nil {
				t.Fatal("revocation did not close stream promptly")
			}
		})
	}
}

func TestNetworkProxyAuthenticationAndWorkerLocalhost(t *testing.T) {
	_, operator, worker, catalog := networkFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("proxy credential leaked to origin")
		}
		_, _ = w.Write([]byte("worker localhost"))
	}))
	defer origin.Close()
	target := strings.TrimPrefix(origin.URL, "http://")
	permission := contract.NetworkPermissions{
		Schema:              contract.NetworkVersion,
		Enabled:             true,
		LoopbackDestination: target,
	}
	if err := permission.Validate(); err != nil {
		t.Fatal(err)
	}
	listener, server, password, err := StartNetworkProxy(ctx, operator, catalog.Target)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	defer listener.Close()
	go server.Serve(listener)
	proxyURL, _ := url.Parse(NetworkProxyEndpoint(listener))
	unauth := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: time.Second}
	response, err := unauth.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 407 {
		t.Fatal("unauthenticated HTTP forwarded")
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(connection, "CONNECT localhost:443 HTTP/1.1\r\nHost: localhost:443\r\n\r\n")
	denied, err := http.ReadResponse(bufio.NewReader(connection), nil)
	connection.Close()
	if err != nil || denied.StatusCode != 407 {
		t.Fatal("unauthenticated CONNECT forwarded", err)
	}
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var tickets []contract.NetworkTicket
				if worker.Call(ctx, "/v1/network-worker", catalog, &tickets) != nil {
					return
				}
				for _, ticket := range tickets {
					go func(ticket contract.NetworkTicket) { _ = worker.ServeNetworkTCP(ctx, ticket, permission) }(ticket)
				}
			}
		}
	}()
	proxyURL.User = url.UserPassword("dockpipe", password)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err = client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(payload) != "worker localhost" {
		t.Fatal(string(payload), err)
	}
	// CONNECT carries raw bytes, preserving origin TLS and certificate validation.
	raw, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	credential := base64.StdEncoding.EncodeToString([]byte("dockpipe:" + password))
	_, _ = io.WriteString(raw, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\nProxy-Authorization: Basic "+credential+"\r\n\r\n")
	reader := bufio.NewReader(raw)
	connected, err := http.ReadResponse(reader, nil)
	if err != nil || connected.StatusCode != 200 {
		t.Fatal("CONNECT failed", err)
	}
	_, _ = io.WriteString(raw, "GET / HTTP/1.1\r\nHost: "+target+"\r\nConnection: close\r\n\r\n")
	tunneled, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = io.ReadAll(tunneled.Body)
	tunneled.Body.Close()
	if err != nil || string(payload) != "worker localhost" {
		t.Fatal("CONNECT payload changed", string(payload), err)
	}
	restrictedOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("worker dialed a port outside the grant")
		w.WriteHeader(http.StatusOK)
	}))
	defer restrictedOrigin.Close()
	rejected, err := client.Get(restrictedOrigin.URL)
	if err != nil {
		t.Fatal(err)
	}
	rejected.Body.Close()
	if rejected.StatusCode != http.StatusBadGateway {
		t.Fatalf("restricted destination returned %d", rejected.StatusCode)
	}

	cancel()
	<-workerDone
}

func TestNetworkNodeCapacityAndCancellation(t *testing.T) {
	broker, operator, _, catalog := networkFixture(t)
	ctx := context.Background()
	request := contract.NetworkConnect{Schema: contract.NetworkVersion, Target: catalog.Target, Address: "localhost:80"}
	for count := 0; count < contract.MaxNetworkNodeStreams; count++ {
		if err := operator.Call(ctx, "/v1/network-connect", request, nil); err != nil {
			t.Fatalf("stream %d: %v", count, err)
		}
	}
	if operator.Call(ctx, "/v1/network-connect", request, nil) == nil {
		t.Fatal("per-node capacity exceeded")
	}
	broker.CloseNetworkRelays()
	if err := operator.Call(ctx, "/v1/network-connect", request, nil); err != nil {
		t.Fatal("cancelled capacity was not reclaimed", err)
	}
	left, peer := net.Pipe()
	right, other := net.Pipe()
	defer peer.Close()
	defer other.Close()
	copyContext, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- copyNetwork(copyContext, left, right) }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation became success")
		}
	case <-time.After(time.Second):
		t.Fatal("TCP pumps failed to drain")
	}
}

func TestNetworkProxyWebSocketUpgrade(t *testing.T) {
	_, operator, worker, catalog := networkFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		socket, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer socket.CloseNow()
		kind, payload, err := socket.Read(ctx)
		if err == nil {
			_ = socket.Write(ctx, kind, payload)
		}
	}))
	defer origin.Close()
	listener, server, password, err := StartNetworkProxy(ctx, operator, catalog.Target)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	defer listener.Close()
	go server.Serve(listener)
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var tickets []contract.NetworkTicket
				if worker.Call(ctx, "/v1/network-worker", catalog, &tickets) != nil {
					return
				}
				for _, ticket := range tickets {
					go func(ticket contract.NetworkTicket) {
						_ = worker.ServeNetworkTCP(ctx, ticket, contract.NetworkPermissions{Enabled: true})
					}(ticket)
				}
			}
		}
	}()
	proxyURL, _ := url.Parse(NetworkProxyEndpoint(listener))
	proxyURL.User = url.UserPassword("dockpipe", password)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	socket, _, err := websocket.Dial(ctx, strings.Replace(origin.URL, "http://", "ws://", 1), &websocket.DialOptions{HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal("proxied WebSocket upgrade failed", err)
	}
	defer socket.CloseNow()
	if err = socket.Write(ctx, websocket.MessageText, []byte("worker websocket")); err != nil {
		t.Fatal(err)
	}
	_, payload, err := socket.Read(ctx)
	if err != nil || string(payload) != "worker websocket" {
		t.Fatal(string(payload), err)
	}
}

func TestNetworkLeaseRenewalAndFailedTicketConsumption(t *testing.T) {
	broker, operator, _, catalog, viewerTicket, _ := networkTicketFixture(t)
	ctx := context.Background()
	broker.mutex.Lock()
	stream := broker.networks.streams[viewerTicket.ID]
	original := stream.admission.deadline
	stream.admission.deadline = time.Now().Add(time.Second)
	broker.mutex.Unlock()
	if err := operator.Call(ctx, "/v1/network-pulse", catalog.Target, nil); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	renewed := stream.admission.deadline
	broker.mutex.Unlock()
	if renewed.Before(original.Add(-time.Second)) {
		t.Fatal("same original OS admission was not renewed")
	}
	// A pulse cannot revive an expired stream, even before its watchdog runs.
	broker.mutex.Lock()
	stream.admission.deadline = time.Now().Add(-time.Second)
	expired := stream.admission.deadline
	broker.mutex.Unlock()
	if err := operator.Call(ctx, "/v1/network-pulse", catalog.Target, nil); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	after := stream.admission.deadline
	broker.mutex.Unlock()
	if !after.Equal(expired) {
		t.Fatal("expired original stream authority was revived")
	}
	attacker := *operator
	attacker.Token = Hash("foreign-bearer")
	if socket, err := attacker.attachNetwork(ctx, viewerTicket); err == nil {
		socket.CloseNow()
		t.Fatal("foreign bearer attached")
	}
	if socket, err := operator.attachNetwork(ctx, viewerTicket); err == nil {
		socket.CloseNow()
		t.Fatal("failed attachment did not consume ticket")
	}

}
