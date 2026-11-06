//go:build linux

package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/localhttp"
)

func TestRealWebSocketUpgradeRetainsSlotsUntilLeaseExpiry(t *testing.T) {
	manager, authorized := authorizedDesktop(t)
	tracker := &streamTracker{connections: make(map[*trackedConnection]string)}
	const maximumConnections = 64
	upstreamClosed := make(chan struct{}, maximumConnections)
	var upstreamMutex sync.Mutex
	var upstreamWorkers sync.WaitGroup
	var upstreamConnections []net.Conn
	upstreamClosing := false
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamWorkers.Add(1)
		defer upstreamWorkers.Done()
		username, password, ok := request.BasicAuth()
		if !ok || username != "fixture-stream-user" || password != "fixture-stream-password" {
			t.Error("upgrade lost the streamer's internal authentication")
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Cookie") != "" || request.Header.Get("X-Syno-Token") != "" {
			t.Error("browser credentials reached the upgraded upstream")
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		connection, buffer, err := writer.(http.Hijacker).Hijack()
		if err != nil {
			t.Error("upstream upgrade failed")
			return
		}
		defer connection.Close()
		upstreamMutex.Lock()
		if upstreamClosing {
			upstreamMutex.Unlock()
			return
		}
		upstreamConnections = append(upstreamConnections, connection)
		upstreamMutex.Unlock()
		digest := sha1.Sum([]byte(request.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		accept := base64.StdEncoding.EncodeToString(digest[:])
		_, err = buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n")
		if err != nil || buffer.Flush() != nil {
			t.Error("upstream upgrade response failed")
			return
		}
		// A real unmasked server text frame proves bytes crossed the hijacked
		// connection rather than stopping at a synthetic 101 response.
		if _, err := connection.Write([]byte{0x81, 0x01, 'x'}); err != nil {
			t.Error("upstream frame write failed")
			return
		}
		_, _ = io.Copy(io.Discard, buffer)
		select {
		case upstreamClosed <- struct{}{}:
		default:
		}
	}))
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw})
	pinned, err := pinnedStreamerTLS(certificate)
	if err != nil {
		upstream.Close()
		upstreamWorkers.Wait()
		t.Fatal(err)
	}
	transport := &http.Transport{
		Proxy: nil, TLSClientConfig: pinned, MaxConnsPerHost: maximumConnections,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", upstream.Listener.Addr().String())
		},
	}
	frontend := httptest.NewUnstartedServer(desktopProxy(manager, tracker, transport, "fixture-stream-user", "fixture-stream-password"))
	slots := make(chan struct{}, maximumConnections)
	frontend.Listener = &localhttp.Listener{Listener: frontend.Listener, Slots: slots}
	frontend.Config.ReadHeaderTimeout = time.Second
	frontend.Config.SetKeepAlivesEnabled(false)
	frontend.Config.ErrorLog = log.New(io.Discard, "", 0)
	frontend.StartTLS()
	roots := x509.NewCertPool()
	roots.AddCert(frontend.Certificate())
	var clients []*tls.Conn
	t.Cleanup(func() {
		tracker.closeInvalid(func(string) bool { return false })
		for _, connection := range clients {
			_ = connection.Close()
		}
		upstreamMutex.Lock()
		upstreamClosing = true
		connections := append([]net.Conn(nil), upstreamConnections...)
		upstreamMutex.Unlock()
		for _, connection := range connections {
			_ = connection.Close()
		}
		frontend.Close()
		transport.CloseIdleConnections()
		upstream.Close()
		upstreamWorkers.Wait()
	})
	dial := func() (*tls.Conn, error) {
		connection, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", frontend.Listener.Addr().String(),
			&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "127.0.0.1"})
		if err == nil {
			clients = append(clients, connection)
		}
		return connection, err
	}
	readers := make([]*bufio.Reader, 0, maximumConnections)
	for index := 0; index < maximumConnections; index++ {
		connection, err := dial()
		if err != nil {
			t.Fatal("gateway connection failed before reaching its configured bound", err)
		}
		_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
		request := authorized.Clone(context.Background())
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		if request.Write(connection) != nil {
			t.Fatal("gateway upgrade request failed")
		}
		reader := bufio.NewReader(connection)
		response, err := http.ReadResponse(reader, request)
		if err != nil || response.StatusCode != http.StatusSwitchingProtocols {
			t.Fatal("gateway did not perform an actual HTTP upgrade", err)
		}
		frame := make([]byte, 3)
		if _, err := io.ReadFull(reader, frame); err != nil || string(frame) != string([]byte{0x81, 0x01, 'x'}) {
			t.Fatal("upgraded gateway did not relay the upstream frame", err)
		}
		_ = connection.SetDeadline(time.Time{})
		readers = append(readers, reader)
	}
	tracker.mutex.Lock()
	tracked := len(tracker.connections)
	tracker.mutex.Unlock()
	if len(slots) != maximumConnections || tracked != maximumConnections {
		t.Fatalf("hijacking released the listener bound: slots=%d tracked=%d", len(slots), tracked)
	}
	if connection, err := dial(); err == nil {
		_ = connection.Close()
		t.Fatal("a sixty-fifth upgraded client bypassed the listener bound")
	}

	manager.mutex.Lock()
	expired := manager.now().Add(leaseLifetime + time.Second)
	manager.now = func() time.Time { return expired }
	manager.mutex.Unlock()
	tracker.closeInvalid(manager.gateAlive)
	for index, reader := range readers {
		_ = clients[index].SetReadDeadline(time.Now().Add(time.Second))
		_, err := reader.ReadByte()
		if err == nil {
			t.Fatal("expired upgrade still returned application data")
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("expired upgrade remained open until the client deadline")
		}
	}
	for index := 0; index < maximumConnections; index++ {
		select {
		case <-upstreamClosed:
		case <-time.After(2 * time.Second):
			t.Fatal("lease expiry left an upstream tunnel open")
		}
	}
	tracker.mutex.Lock()
	remaining := len(tracker.connections)
	tracker.mutex.Unlock()
	if remaining != 0 || len(slots) != 0 {
		t.Fatalf("expired upgrades retained resources: tracked=%d slots=%d", remaining, len(slots))
	}
	connection, err := dial()
	if err != nil {
		t.Fatal("closed upgrades did not release listener capacity", err)
	}
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	request := authorized.Clone(context.Background())
	request.Header.Del("Connection")
	request.Header.Del("Upgrade")
	if request.Write(connection) != nil {
		t.Fatal("post-expiry authorization request failed")
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), request)
	if err != nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatal("expired browser regained access after listener capacity was released", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	_ = connection.Close()
	if strings.Contains(response.Header.Get("WWW-Authenticate"), "Basic") {
		t.Fatal("expired session received a streamer authentication prompt")
	}
}
