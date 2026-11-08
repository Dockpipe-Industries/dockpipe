package remotecmd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestBrokerReadinessWaitsForAuthorityAndHTTP(t *testing.T) {
	var healthCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Error("readiness attempted a non-health request")
		}
		if healthCalls.Add(1) == 1 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"schema":"dockpipe.remote/v1"}`))
	}))
	defer server.Close()
	client, err := remoteio.NewClient(server.URL, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	authorityCalls := 0
	security := func(ctx context.Context, request remoteio.SecurityRequest) (remoteio.SecurityResponse, error) {
		if request.Action != "status" {
			t.Fatal("readiness attempted authentication or mutation")
		}
		authorityCalls++
		if authorityCalls == 1 {
			return remoteio.SecurityResponse{}, errors.New("authority is starting")
		}
		return remoteio.SecurityResponse{Status: &remoteio.SecurityStatus{AuthorityAvailable: true, Locked: true}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := waitForBrokerReadiness(ctx, client, security); err != nil {
		t.Fatal(err)
	}
	if authorityCalls < 3 || healthCalls.Load() < 2 {
		t.Fatal("service was reported ready before both listeners were available")
	}
}

func TestBrokerReadinessHonorsCancellation(t *testing.T) {
	client, err := remoteio.NewClient("http://127.0.0.1:1", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	security := func(context.Context, remoteio.SecurityRequest) (remoteio.SecurityResponse, error) {
		return remoteio.SecurityResponse{}, errors.New("still starting")
	}
	if err := waitForBrokerReadiness(ctx, client, security); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readiness did not propagate its deadline: %v", err)
	}
}
