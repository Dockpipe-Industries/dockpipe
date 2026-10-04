//go:build !linux && !darwin

package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestWorkerRejectsUnsupportedPlatformBeforePollingOrExecution(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checkout := t.TempDir()
	executable := remoteTestExecutable(t, checkout, "success")
	config := contract.WorkerConfig{
		Schema:   contract.Version,
		Endpoint: server.URL,
		Node:     "unsupported",
		Token:    strings.Repeat("a", 64),
		Profiles: map[string]contract.Profile{
			"bench": {Workdir: checkout, Workflow: "bench", TimeoutSeconds: 1},
		},
	}
	root := filepath.Join(t.TempDir(), "worker")
	err := RunWorker(context.Background(), root, executable, config)
	if err == nil || err.Error() != "remote nodes currently require Linux or macOS" {
		t.Fatalf("expected unsupported-platform rejection, got %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("unsupported worker contacted the broker")
	}
	if _, err := os.Stat(filepath.Join(checkout, "count")); !os.IsNotExist(err) {
		t.Fatalf("unsupported worker executed a workflow: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported worker wrote state: %v, %v", entries, err)
	}
}
