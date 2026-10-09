//go:build linux || darwin

package remotecmd

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestOnlinePairingPersistsIdentityBeforeApproval(t *testing.T) {
	brokerRoot := filepath.Join(t.TempDir(), "broker")
	operator, err := initialize(brokerRoot, "127.0.0.1:47831")
	if err != nil {
		t.Fatal(err)
	}
	broker, err := remoteio.NewBroker(brokerRoot)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(broker)
	defer server.Close()
	admin, _ := remoteio.NewClient(server.URL, operator.Token)
	defer admin.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := admin.Call(ctx, "/v1/pairing-open", nil, nil); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "worker")
	done := make(chan error, 1)
	go func() { done <- pairOnline(ctx, root, server.URL, "worker", true, 60) }()
	var pending []contract.PairingStatus
	for len(pending) == 0 && ctx.Err() == nil {
		if err := admin.Call(ctx, "/v1/pairings", nil, &pending); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatal("request was not created")
	}
	var saved contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &saved); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(ctx, "/v1/approve", map[string]string{"code": pending[0].Code}, nil); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := pairOnline(ctx, root, server.URL, "worker", true, 60); err != nil {
		t.Fatal(err)
	}
	var retried contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &retried); err != nil {
		t.Fatal(err)
	}
	if retried.Token != saved.Token {
		t.Fatal("retry changed credential")
	}
	if err := pairOnline(ctx, root, server.URL, "worker", true, 600); err == nil {
		t.Fatal("retry widened worker authority")
	}
	if err := pairOnline(ctx, root, server.URL, "other-worker", true, 60); err == nil {
		t.Fatal("retry changed worker identity")
	}
	if err := admin.Call(ctx, "/v1/revoke", map[string]string{"id": "worker"}, nil); err != nil {
		t.Fatal(err)
	}
	go func() { done <- pairOnline(ctx, root, server.URL, "worker", true, 60) }()
	pending = nil
	for len(pending) == 0 && ctx.Err() == nil {
		if err := admin.Call(ctx, "/v1/pairings", nil, &pending); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatal("fresh pairing request was not created")
	}
	var replacement contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &replacement); err != nil {
		t.Fatal(err)
	}
	if replacement.Token == saved.Token {
		t.Fatal("revoked credential was reused")
	}
	if err := admin.Call(ctx, "/v1/approve", map[string]string{"code": pending[0].Code}, nil); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryRejectsChangedPreviewBeforeContactingBroker(t *testing.T) {
	source := t.TempDir()
	workflow := filepath.Join(source, "work", "config.yml")
	if err := os.MkdirAll(filepath.Dir(workflow), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflow, []byte("name: example\nsteps:\n  - id: run\n    kind: host\n    run: echo hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := submitDelivery(context.Background(), filepath.Join(t.TempDir(), "no-broker"), "worker", "job", source,
		"work/config.yml", nil, nil, nil, false, strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "changed since preview") {
		t.Fatalf("unexpected result: %v", err)
	}
}
