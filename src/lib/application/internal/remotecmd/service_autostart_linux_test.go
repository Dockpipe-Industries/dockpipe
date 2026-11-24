package remotecmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopAutostartDefinitionQuotesArguments(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, `work %f "quote" $PATH \ & 'x'`)
	path, content, arguments, err := desktopWorkerDefinition("relative", home, root, "worker", []string{"/usr/local/bin/dock pipe"}, strings.Repeat("1", 64), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "autostart", "worker.desktop"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if arguments[4] != root || !strings.Contains(content, `Exec="/usr/local/bin/dock pipe" "remote" "service-worker-desktop"`) ||
		!strings.Contains(content, `work %%f \\"quote\\" \\$PATH \\\\ & 'x'`) {
		t.Fatalf("registration did not preserve the expected command arguments:\n%s", content)
	}
	if _, err := desktopWorkerExec([]string{"good\nHidden=true"}); err == nil {
		t.Fatal("embedded desktop entry key passed validation")
	}
}

func TestDesktopAutostartReplacesOnlyPreviousSupervisor(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "worker.desktop")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oldOwner, err := claimDesktopWorker(ctx, root, path, "old")
	if err != nil || oldOwner == nil {
		t.Fatalf("claim original supervisor: %v", err)
	}
	defer oldOwner.Close()
	duplicate, err := claimDesktopWorker(ctx, root, path, "old")
	if duplicate != nil {
		_ = duplicate.Close()
		t.Fatal("duplicate supervisor obtained ownership")
	}
	if err != nil {
		t.Fatal(err)
	}

	oldCtx, cancelOld := context.WithCancel(ctx)
	defer cancelOld()
	go watchDesktopWorkerRegistration(oldCtx, cancelOld, path, "old", func() bool { return false })
	oldDone := make(chan struct{})
	go func() {
		defer close(oldDone)
		<-oldCtx.Done()
		_ = oldOwner.Close()
	}()
	newDefinition := serviceDefinition{path: path, role: "worker", manager: "xdg-autostart", content: "new"}
	if err := publishServiceDefinition(ctx, newDefinition, true); err != nil {
		t.Fatal(err)
	}

	newOwner, err := claimDesktopWorker(ctx, root, path, "new")
	if err != nil || newOwner == nil {
		t.Fatalf("claim new supervisor: %v", err)
	}
	defer newOwner.Close()
	<-oldDone
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
}

func TestDesktopAutostartRestartsAnExitedWorker(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "dockpipe")
	script := `#!/bin/sh
printf '%s\n' "$$" >> "$4/starts"
exit 1
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	handle, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	executableHandle := fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), handle.Fd())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- runDesktopWorkerLoop(ctx, executable, executableHandle, root, 20*time.Millisecond) }()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-result:
			t.Fatalf("supervisor exited before restarting the worker: %v", err)
		case <-ctx.Done():
			t.Fatal("worker was never restarted")
		case <-ticker.C:
			starts, _ := os.ReadFile(filepath.Join(root, "starts"))
			pids := strings.Fields(string(starts))
			if len(pids) >= 2 && pids[0] != pids[1] {
				cancel()
				if err := <-result; err != nil {
					t.Fatal(err)
				}
				return
			}
		}
	}
}
