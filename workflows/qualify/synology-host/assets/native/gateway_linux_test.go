//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func proxyReportRequest() *http.Request {
	request := httptest.NewRequest("GET", "http://nas.example:5001/report?SynoToken=fixture-token", nil)
	request.RemoteAddr = "127.0.0.1:45000"
	request.Header.Set("Cookie", "id=fixture-cookie")
	request.Header.Set("User-Agent", "qualification-fixture")
	request.Header.Set("Origin", "https://nas.example:5001")
	request.Header.Set("X-Dockpipe-Remote-Addr", "192.0.2.10")
	request.Header.Set("X-Dockpipe-Server-Addr", "192.0.2.20")
	request.Header.Set("X-Dockpipe-Server-Port", "5001")
	request.Header.Set("X-Dockpipe-Scheme", "https")
	return request
}

func TestProxyReportUsesExplicitCGIEnvironment(t *testing.T) {
	request := proxyReportRequest()
	request.Header.Set("X-DSM-User", "root")
	request.Header.Set("X-DSM-UID", "0")
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	handler := authenticationGateway(func(ctx context.Context, environment []string) (string, error) {
		actual := make(map[string]string)
		for _, entry := range environment {
			key, value, _ := strings.Cut(entry, "=")
			actual[key] = value
		}
		for key, expected := range map[string]string{
			"REMOTE_ADDR": "192.0.2.10", "SERVER_ADDR": "192.0.2.20",
			"SERVER_PORT": "5001", "SERVER_NAME": "nas.example",
			"HTTP_HOST": "nas.example:5001", "HTTP_COOKIE": "id=fixture-cookie",
			"HTTP_USER_AGENT": "qualification-fixture", "HTTPS": "on",
			"GATEWAY_INTERFACE": "CGI/1.1", "SERVER_PROTOCOL": "HTTP/1.1",
			"REQUEST_METHOD": "GET", "SCRIPT_NAME": reportScriptName,
			"REQUEST_URI":  reportScriptName + "?SynoToken=fixture-token",
			"QUERY_STRING": "SynoToken=fixture-token",
		} {
			if actual[key] != expected {
				t.Fatalf("%s: got %q, want %q", key, actual[key], expected)
			}
		}
		if strings.Contains(strings.Join(environment, "\n"), "root") || actual["HTTP_X_FORWARDED_FOR"] != "" {
			t.Fatal("untrusted identity or generic proxy claims reached helper")
		}
		if _, found := ctx.Deadline(); !found {
			t.Fatal("report authentication has no deadline")
		}
		return "dsm-user", nil
	}, func(context.Context) serviceResult { return serviceResult{Available: true} })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var result authenticationResponse
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.AuthenticatedUser != "dsm-user" || !result.Service.Available {
		t.Fatal("proxy report did not preserve authenticated identity")
	}
	if strings.Contains(response.Body.String(), "fixture-cookie") || strings.Contains(response.Body.String(), "fixture-token") {
		t.Fatal("report leaked credentials")
	}
}

func TestProxyReportRejectsInvalidBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*http.Request)
		want   int
	}{
		{name: "remote peer", mutate: func(r *http.Request) { r.RemoteAddr = "192.0.2.2:42000" }, want: 403},
		{name: "missing proxy metadata", mutate: func(r *http.Request) { r.Header.Del("X-Dockpipe-Remote-Addr") }, want: 403},
		{name: "invalid forwarded address", mutate: func(r *http.Request) { r.Header.Set("X-Dockpipe-Remote-Addr", "root") }, want: 403},
		{name: "invalid server address", mutate: func(r *http.Request) { r.Header.Set("X-Dockpipe-Server-Addr", "not-an-ip") }, want: 403},
		{name: "ambiguous proxy metadata", mutate: func(r *http.Request) { r.Header.Add("X-Dockpipe-Server-Port", "443") }, want: 403},
		{name: "invalid port", mutate: func(r *http.Request) { r.Header.Set("X-Dockpipe-Server-Port", "65536") }, want: 403},
		{name: "insecure scheme", mutate: func(r *http.Request) { r.Header.Set("X-Dockpipe-Scheme", "http") }, want: 403},
		{name: "wrong origin", mutate: func(r *http.Request) { r.Header.Set("Origin", "https://other.example") }, want: 403},
		{name: "invalid host", mutate: func(r *http.Request) { r.Host = "nas.example/forged" }, want: 403},
		{name: "invalid host port", mutate: func(r *http.Request) { r.Host = "nas.example:65536" }, want: 403},
		{name: "oversized cookie", mutate: func(r *http.Request) { r.Header.Set("Cookie", strings.Repeat("x", (16<<10)+1)) }, want: 403},
		{name: "no cookie", mutate: func(r *http.Request) { r.Header.Del("Cookie") }, want: 401},
		{name: "wrong route", mutate: func(r *http.Request) { r.URL.Path = "/v1/status" }, want: 404},
		{name: "write method", mutate: func(r *http.Request) { r.Method = "POST" }, want: 405},
		{name: "request body", mutate: func(r *http.Request) { r.ContentLength = 1 }, want: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := proxyReportRequest()
			test.mutate(request)
			handler := authenticationGateway(func(context.Context, []string) (string, error) {
				t.Fatal("invalid request reached helper")
				return "", nil
			}, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want || strings.Contains(response.Body.String(), "identity") {
				t.Fatalf("invalid request returned %d with %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProxyReportFailedAuthenticationHasNoIdentity(t *testing.T) {
	handler := authenticationGateway(func(context.Context, []string) (string, error) {
		return "", errUnauthenticated
	}, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, proxyReportRequest())
	if response.Code != 401 || strings.Contains(response.Body.String(), "identity") {
		t.Fatal("failed DSM authentication exposed identity")
	}
}

func TestHTTPReportServiceLifecycle(t *testing.T) {
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("service intentionally refuses root")
	}
	directory := t.TempDir()
	_ = os.Chmod(directory, 0700)
	socket := filepath.Join(directory, "probe.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- serveWithAuthentication(ctx, socket, authenticationListenAddress) }()
	deadline := time.Now().Add(2 * time.Second)
	client := &http.Client{Timeout: time.Second}
	for {
		select {
		case err := <-completed:
			t.Fatalf("report service exited before readiness: %v", err)
		default:
		}
		response, err := client.Get("http://" + authenticationListenAddress + "/report")
		if err == nil {
			response.Body.Close()
			if response.StatusCode != 403 {
				t.Fatalf("unforwarded local request returned %d", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("report listener was not ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !querySocket(ctx, socket).Available {
		t.Fatal("private status listener unavailable alongside report listener")
	}
	cancel()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("report service failed to stop both listeners")
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("report service left private socket behind")
	}
	connection, err := net.DialTimeout("tcp4", authenticationListenAddress, 100*time.Millisecond)
	if err == nil {
		connection.Close()
		t.Fatal("report listener survived shutdown")
	}
}

func TestReportListenerFailureReleasesPrivateSocket(t *testing.T) {
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("service intentionally refuses root")
	}
	occupied, err := net.Listen("tcp4", authenticationListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	directory := t.TempDir()
	_ = os.Chmod(directory, 0700)
	socket := filepath.Join(directory, "probe.sock")
	if err := serveWithAuthentication(context.Background(), socket, authenticationListenAddress); err == nil {
		t.Fatal("occupied report port was accepted")
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial startup failure left private socket behind")
	}
}

func TestReportListenerAcceptsOnlyFixedLoopbackAddress(t *testing.T) {
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("service intentionally refuses root")
	}
	directory := t.TempDir()
	_ = os.Chmod(directory, 0700)
	socket := filepath.Join(directory, "probe.sock")
	for _, address := range []string{"0.0.0.0:38140", "127.0.0.1:0", "127.0.0.1:38141", "[::1]:38140"} {
		if err := serveWithAuthentication(context.Background(), socket, address); err == nil {
			t.Fatalf("accepted unexpected report listener %s", address)
		}
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid listener configuration created a private socket")
	}
}

func TestProxyReportAcceptsMaximumCookieBudget(t *testing.T) {
	request := proxyReportRequest()
	request.Header.Set("Cookie", strings.Repeat("x", 16<<10))
	handler := authenticationGateway(func(context.Context, []string) (string, error) {
		return "dsm-user", nil
	}, func(context.Context) serviceResult { return serviceResult{} })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("maximum-size cookie rejected: %d", response.Code)
	}
}

func TestSaturatedReportConnectionsDoNotRequireAnotherServiceSlot(t *testing.T) {
	enteredAuthentication := make(chan struct{}, maximumConnections)
	releaseAuthentication := make(chan struct{})
	handler := authenticationGateway(func(ctx context.Context, _ []string) (string, error) {
		enteredAuthentication <- struct{}{}
		select {
		case <-releaseAuthentication:
			return "dsm-user", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}, inProcessServiceStatus)
	server := httptest.NewUnstartedServer(handler)
	server.Config = qualificationServer(handler)
	server.Listener = &loopbackListener{Listener: server.Listener, slots: make(chan struct{}, maximumConnections)}
	server.Start()
	defer server.Close()
	results := make(chan error, maximumConnections)
	for index := 0; index < maximumConnections; index++ {
		go func() {
			request := proxyReportRequest()
			request.URL.Scheme = "http"
			request.URL.Host = server.Listener.Addr().String()
			request.RequestURI = ""
			response, err := server.Client().Do(request)
			if err != nil {
				results <- err
				return
			}
			defer response.Body.Close()
			var report authenticationResponse
			if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&report) != nil || !report.Service.Available {
				results <- errors.New("saturated report did not include service status")
				return
			}
			results <- nil
		}()
	}
	for index := 0; index < maximumConnections; index++ {
		select {
		case <-enteredAuthentication:
		case <-time.After(time.Second):
			close(releaseAuthentication)
			t.Fatal("report connections did not fill their shared connection budget")
		}
	}
	close(releaseAuthentication)
	for index := 0; index < maximumConnections; index++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("saturated reports blocked waiting for another connection slot")
		}
	}
}
