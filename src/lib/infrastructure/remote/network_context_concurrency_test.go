//go:build linux || darwin

package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
)

func TestNetworkConcurrentContextsAndPulseShareGlobalAuthority(t *testing.T) {
	broker, original, worker, catalog := networkFixture(t)
	directory, err := os.MkdirTemp("", "dp-context-")
	if err != nil {
		t.Fatal(err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	platform := &slowNetworkAdmissionPlatform{securityFixturePlatform: &securityFixturePlatform{directory: directory, snapshot: osauth.Snapshot{LockKnown: true, SleepKnown: true}}, delay: 80 * time.Millisecond}
	stop, err := broker.startOperatorSecurityAt(context.Background(), platform, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err = broker.security.Unlock(ctx, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, "isolated-context")
	}))
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	workerDone := make(chan struct{})
	var streams sync.WaitGroup
	go func() {
		defer close(workerDone)
		ticker := time.NewTicker(10 * time.Millisecond)
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
					streams.Add(1)
					go func(ticket contract.NetworkTicket) {
						defer streams.Done()
						_ = worker.ServeNetworkTCP(ctx, ticket, contract.NetworkPermissions{Enabled: true})
					}(ticket)
				}
			}
		}
	}()
	defer func() { cancel(); <-workerDone; streams.Wait() }()
	start := make(chan struct{})
	results := make(chan error, 28)
	var operators []*Client
	for contextIndex := 0; contextIndex < 3; contextIndex++ {
		operator, err := NewClient(original.Endpoint, original.Token)
		if err != nil {
			t.Fatal(err)
		}
		defer operator.HTTP.CloseIdleConnections()
		operator.protectLocalOperator(broker.state.Identity.PublicKey, func(ctx context.Context, request SecurityRequest) (SecurityResponse, error) {
			return callSecurityAt(ctx, directory, request)
		})
		operators = append(operators, operator)
		listener, server, password, err := StartNetworkProxy(ctx, operator, catalog.Target)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		defer server.Close()
		go server.Serve(listener)
		proxyURL, _ := url.Parse(NetworkProxyEndpoint(listener))
		proxyURL.User = url.UserPassword("dockpipe", password)
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		for requestIndex := 0; requestIndex < 8; requestIndex++ {
			go func() {
				<-start
				response, err := client.Get(origin.URL)
				if err == nil {
					payload, readErr := io.ReadAll(io.LimitReader(response.Body, 1024))
					response.Body.Close()
					if readErr != nil {
						err = readErr
					} else if response.StatusCode != http.StatusOK || string(payload) != "isolated-context" {
						err = fmt.Errorf("unexpected isolated context response status %d", response.StatusCode)
					}
				}
				results <- err
			}()
		}
	}
	for index := 0; index < 4; index++ {
		go func(operator *Client) {
			<-start
			results <- operator.Call(ctx, "/v1/network-pulse", catalog.Target, nil)
		}(operators[index%len(operators)])
	}
	close(start)
	for count := 0; count < 28; count++ {
		select {
		case err := <-results:
			if err != nil {
				t.Error("independent context or pulse failed fresh bounded admission", err)
			}
		case <-ctx.Done():
			t.Fatal("independent contexts/pulse exceeded their bounded deadline")
		}
	}
	platform.mutex.Lock()
	authentications := platform.authCalls
	platform.mutex.Unlock()
	if authentications != 1 {
		t.Fatal("burst automatically authenticated instead of using the original explicit unlock")
	}
}
