//go:build linux

package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSourceHostScriptWithContextCancelsInheritedWriterAfterRootExitLinux(t *testing.T) {
	if _, err := resolveBashExeForEnv(os.Environ()); err != nil {
		t.Skipf("actual Bash is required for sourced-script custody qualification: %v", err)
	}

	if !hostHasNoTerminal() {
		t.Skip("post-root group custody qualification requires the existing terminal-free private-group topology")
	}

	workdir := t.TempDir()
	script := filepath.Join(workdir, "source.sh")
	body := "printf '%s' \"$$\" > leader\n" +
		"(\n" +
		"    printf ready > ready\n" +
		"    for ((attempt = 0; attempt < 80; attempt++)); do\n" +
		"        if [ -e release ]; then break; fi\n" +
		"        sleep 0.05\n" +
		"    done\n" +
		"    printf unexpected > after\n" +
		") &\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	environment := append(os.Environ(), "DOCKPIPE_WORKDIR="+workdir, "DOCKPIPE_STEP_CWD="+workdir,
		"DOCKPIPE_HOST_SCRIPT_SPINNER=0", "DOCKPIPE_OBSERVABILITY=off", "DOCKPIPE_SKIP_HOST_CLEANUP=1")
	result := make(chan error, 1)
	completed := make(chan struct{})
	go func() {
		defer close(completed)
		_, err := SourceHostScriptWithContext(ctx, script, environment)
		result <- err
	}()
	t.Cleanup(func() {
		cancel()
		if err := os.WriteFile(filepath.Join(workdir, "release"), []byte("test cleanup"), 0600); err != nil {
			t.Errorf("independent fixture release: %v", err)
		}
		select {
		case <-completed:
		case <-time.After(6 * time.Second):
			t.Error("source inherited output did not join after independent fixture release")
		}
	})

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		leader, leaderErr := os.ReadFile(filepath.Join(workdir, "leader"))
		ready, readyErr := os.ReadFile(filepath.Join(workdir, "ready"))
		pid, parseErr := strconv.Atoi(string(leader))
		if leaderErr == nil && parseErr == nil && pid > 0 && readyErr == nil && string(ready) == "ready" {
			// The direct child is genuinely exited but unreaped. Reading this
			// observation never authorizes signaling a numeric PID or group.
			status, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
			endName := strings.LastIndex(string(status), ")")
			if err == nil && endName >= 0 && strings.HasPrefix(string(status[endName+1:]), " Z ") {
				break
			}
		}
		select {
		case err := <-result:
			t.Fatalf("source completed before an exited root and live inherited writer were observed: %v", err)
		case <-ctx.Done():
			t.Fatal("source never exposed actual post-root inherited-output custody")
		case <-ticker.C:
		}
	}

	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("post-root source cancellation lost its outcome: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("post-root inherited output escaped active source cancellation")
	}
	if _, err := os.Stat(filepath.Join(workdir, "after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inherited source writer survived cancellation to mutate follow-on state: %v", err)
	}
	entries, err := os.ReadDir(HostRunsDir(workdir))
	if err != nil || len(entries) != 0 {
		t.Fatalf("source retained host registry after post-root cancellation: %v, %v", entries, err)
	}
}

func TestSourceHostScriptContextObservationFailureAbortsCaptureLinux(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	setRunHostProcAttrs(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()

	denied := syscall.EPERM
	observations := 0
	aborted := false
	completed := false
	signals := make(chan os.Signal)
	err := waitHostProcessGroupWithCompletion(command, signals, func(int) error {
		observations++
		return denied
	}, func() { completed = true }, func() { aborted = true })
	if !errors.Is(err, denied) || observations != 1 || !aborted || completed {
		t.Fatalf("denied observation claimed capture completion or retried custody: %v, %d, %v, %v", err, observations, aborted, completed)
	}
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 0 {
		t.Fatalf("safe direct-child fallback did not reap the real successful root: %v", command.ProcessState)
	}
}
