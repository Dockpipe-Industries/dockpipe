//go:build linux

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/endpointtls"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func authorizedDesktop(t *testing.T) (*leaseManager, *http.Request) {
	t.Helper()
	manager := fixtureLeases(t)
	_, _, err := manager.prepare(fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	request := fixtureRequest(http.MethodGet, "")
	cookie, err := manager.renew(request, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	attachCookie(request, cookie)
	request.URL.Path = desktopPrefix
	return manager, request
}

func TestDesktopGateStripsCredentialsAndPinsUpstream(t *testing.T) {
	manager, request := authorizedDesktop(t)
	request.Header.Set("Authorization", "Basic browser-credential-must-not-pass")
	request.Header.Set("X-Forwarded-Host", "untrusted.example")
	request.Header.Set("Referer", "https://nas.example/?private-session")
	calls := 0
	transport := roundTripFunc(func(upstream *http.Request) (*http.Response, error) {
		calls++
		if upstream.URL.Scheme != "https" || upstream.URL.Host != "127.0.0.1:38139" || upstream.URL.Path != desktopPrefix {
			t.Error("upstream target was not fixed")
		}
		for _, key := range []string{"Cookie", "X-Syno-Token", "X-Dockpipe-Remote-Addr", "X-Forwarded-Host", "Referer"} {
			if upstream.Header.Get(key) != "" {
				t.Error("DSM/browser metadata reached the GUI streamer")
			}
		}
		user, password, ok := upstream.BasicAuth()
		if !ok || user != "stream-user" || password != "stream-password" {
			t.Error("streamer did not receive its own internal authentication")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": []string{"id=must-not-override-dsm"}, "Www-Authenticate": []string{"Basic realm=must-not-prompt"}}, Body: io.NopCloser(strings.NewReader("desktop"))}, nil
	})
	tracker := &streamTracker{connections: make(map[*trackedConnection]string)}
	handler := desktopProxy(manager, tracker, transport, "stream-user", "stream-password")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 200 || calls != 1 || recorder.Header().Get("Set-Cookie") != "" || recorder.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("desktop proxy authentication boundary failed")
	}
	request.Header.Set("Cookie", "id=different-session")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 401 || calls != 1 {
		t.Fatal("foreign browser session reached streamer")
	}
}

func TestDesktopGateRejectsForeignOriginAndSessionQuery(t *testing.T) {
	for _, scenario := range []string{"origin", "dsm-query", "path-traversal", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			manager, request := authorizedDesktop(t)
			switch scenario {
			case "origin":
				request.Header.Set("Origin", "https://other.example")
			case "dsm-query":
				request.URL.RawQuery = "SynoToken=must-not-forward"
			case "path-traversal":
				request.URL.Path = desktopPrefix + "../outside"
			case "expired":
				manager.revoke()
			}
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("rejected request reached upstream")
				return nil, context.Canceled
			})
			handler := desktopProxy(manager, &streamTracker{connections: make(map[*trackedConnection]string)}, transport, "user", "password")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code < 400 {
				t.Fatal("unsafe desktop request was accepted")
			}
		})
	}
}

func TestStreamerPinRejectsDifferentCertificateBeforeCredentials(t *testing.T) {
	root := t.TempDir()
	if os.Chmod(root, 0755) != nil || endpointtls.Initialize(root) != nil {
		t.Fatal("TLS fixture initialization")
	}
	_, certificatePath, _ := endpointtls.Paths(root, false)
	certificate, err := os.ReadFile(certificatePath)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := pinnedStreamerTLS(certificate)
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := endpointtls.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	foreignRoot := t.TempDir()
	if os.Chmod(foreignRoot, 0755) != nil || endpointtls.Initialize(foreignRoot) != nil {
		t.Fatal("foreign TLS fixture")
	}
	foreign, err := endpointtls.Load(foreignRoot)
	if err != nil {
		t.Fatal(err)
	}
	var current atomic.Pointer[tls.Certificate]
	trustedLeaf := trusted.Certificates[0]
	trustedLeaf.Certificate = trustedLeaf.Certificate[:1]
	foreignLeaf := foreign.Certificates[0]
	foreignLeaf.Certificate = foreignLeaf.Certificate[:1]
	current.Store(&trustedLeaf)
	var received atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received.Add(1)
		writer.WriteHeader(204)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*current.Load()}}, nil
	}}
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSClientConfig: pinned}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("trusted streamer certificate rejected: %v", err)
	}
	response.Body.Close()
	current.Store(&foreignLeaf)
	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	request.SetBasicAuth("private-stream-user", "private-stream-password")
	if response, err := client.Do(request); err == nil {
		response.Body.Close()
		t.Fatal("forged streamer accepted")
	}
	if received.Load() != 1 {
		t.Fatal("stream credentials reached the forged listener")
	}
	if err := pinned.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{}}); err == nil {
		t.Fatal("missing streamer certificate accepted")
	}
}

func TestTrackedWebSocketClosesWhenItsLeaseExpires(t *testing.T) {
	manager, request := authorizedDesktop(t)
	token, allowed := manager.gate(request)
	if !allowed {
		t.Fatal("fixture browser lease")
	}
	peer, trackedSide := net.Pipe()
	defer peer.Close()
	tracker := &streamTracker{connections: make(map[*trackedConnection]string)}
	tracked := &trackedConnection{Conn: trackedSide}
	tracked.release = func() {
		tracker.mutex.Lock()
		delete(tracker.connections, tracked)
		tracker.mutex.Unlock()
	}
	tracker.connections[tracked] = token
	manager.revoke()
	tracker.closeInvalid(manager.gateAlive)
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 1)); err != io.EOF {
		t.Fatal("expired browser websocket remained open")
	}
	tracker.mutex.Lock()
	remaining := len(tracker.connections)
	tracker.mutex.Unlock()
	if remaining != 0 {
		t.Fatal("closed websocket retained tracking state")
	}
}
