package remote

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
)

// Native OS snapshots take measurable time. Keep the existing four verification
// slots occupied so a renderer's simultaneous CONNECT burst is reproducible.
type slowNetworkAdmissionPlatform struct {
	*securityFixturePlatform
	delay time.Duration
}

func (platform *slowNetworkAdmissionPlatform) Snapshot(ctx context.Context) (osauth.Snapshot, error) {
	timer := time.NewTimer(platform.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return osauth.Snapshot{}, ctx.Err()
	case <-timer.C:
		return platform.securityFixturePlatform.Snapshot(ctx)
	}
}

type networkAdmissionStatusTransport struct {
	http.RoundTripper
	statuses chan int
}

func (transport networkAdmissionStatusTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.RoundTripper.RoundTrip(request)
	if err == nil && request.URL.Path == "/v1/network-connect" {
		transport.statuses <- response.StatusCode
	}
	return response, err
}

func TestNetworkProxyConcurrentConnectAdmission(t *testing.T) {
	broker, operator, worker, catalog := networkFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, server, password, err := StartNetworkProxy(ctx, operator, catalog.Target)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	defer server.Close()
	go server.Serve(listener)
	broker.security.platform = &slowNetworkAdmissionPlatform{securityFixturePlatform: broker.security.platform.(*securityFixturePlatform), delay: 100 * time.Millisecond}
	statuses := make(chan int, 16)
	operator.HTTP.Transport = networkAdmissionStatusTransport{RoundTripper: operator.HTTP.Transport, statuses: statuses}
	origin, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	go func() {
		for {
			connection, err := origin.Accept()
			if err != nil {
				return
			}
			go func() { defer connection.Close(); _, _ = io.Copy(connection, connection) }()
		}
	}()
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
					go func(ticket contract.NetworkTicket) { defer streams.Done(); _ = worker.ServeNetworkTCP(ctx, ticket) }(ticket)
				}
			}
		}
	}()
	defer func() { cancel(); <-workerDone; streams.Wait() }()
	const connections = 8
	start := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	results := make(chan int, connections)
	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("dockpipe:"+password))
	for index := 0; index < connections; index++ {
		go func() {
			<-start
			connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			if err != nil {
				results <- 0
				return
			}
			defer connection.Close()
			_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
			_, err = fmt.Fprintf(connection, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: %s\r\n\r\n", origin.Addr().String(), origin.Addr().String(), authorization)
			if err != nil {
				results <- 0
				return
			}
			reader := bufio.NewReader(connection)
			response, err := http.ReadResponse(reader, nil)
			if err != nil {
				results <- 0
				return
			}
			if response.StatusCode == http.StatusOK {
				_, err = connection.Write([]byte("probe"))
				if err == nil {
					_, err = io.ReadFull(reader, make([]byte, 5))
				}
				if err != nil {
					results <- 0
					return
				}
			}
			results <- response.StatusCode
			<-release
		}()
	}
	close(start)
	successes, failures := 0, 0
	for count := 0; count < connections; count++ {
		select {
		case status := <-results:
			if status == http.StatusOK {
				successes++
			} else {
				failures++
				t.Logf("local CONNECT returned %d", status)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("concurrent CONNECT fixture exceeded its deadline")
		}
	}
	close(statuses)
	for status := range statuses {
		if status != http.StatusOK {
			t.Logf("operator network-connect admission returned HTTP %d", status)
		}
	}
	if failures != 0 {
		t.Fatalf("renderer CONNECT burst: %d succeeded, %d failed within the eight-stream budget", successes, failures)
	}
}
