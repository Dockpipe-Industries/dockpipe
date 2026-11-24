//go:build linux

package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHostInlineContextCancelsActiveProcessAndCleansRegistryLinux(t *testing.T) {
	workdir := t.TempDir()
	cwd := filepath.Join(workdir, "step")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	env := append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_STEP_CWD="+cwd,
		"DOCKPIPE_HOST_SCRIPT_SPINNER=0",
		"DOCKPIPE_OBSERVABILITY=off",
		"DOCKPIPE_SKIP_HOST_CLEANUP=1",
	)
	result := make(chan error, 1)
	go func() {
		result <- RunHostCommandWithContext(ctx, "printf ready > ready; sleep 10; printf unexpected > after", env, "bash")
	}()

	ready := filepath.Join(cwd, "ready")
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if value, err := os.ReadFile(ready); err == nil && string(value) == "ready" {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("host command returned before readiness: %v", err)
		case <-ctx.Done():
			t.Fatal("host command did not reach its actual working directory")
		case <-ticker.C:
		}
	}

	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active context cancellation was lost: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context did not reach the active host process owner")
	}
	if _, err := os.Stat(filepath.Join(cwd, "after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("post-cancellation command ran: %v", err)
	}
	entries, err := os.ReadDir(HostRunsDir(workdir))
	if err != nil || len(entries) != 0 {
		t.Fatalf("completed host custody retained registry artifacts: %v, %v", entries, err)
	}
}
