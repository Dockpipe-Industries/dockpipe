//go:build linux

package infrastructure

import (
	"bytes"
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

// This test qualifies an explicitly selected newly built CLI, never an implicit
// PATH executable. The helper-only and PTY tests do not require that artifact.
func TestHostSignalCLIWorkflowFinally(t *testing.T) {
	binary := os.Getenv("DOCKPIPE_TEST_SIGNAL_CLI")
	if binary == "" {
		t.Skip("set DOCKPIPE_TEST_SIGNAL_CLI to the absolute candidate CLI for the workflow signal witness")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("candidate CLI must be an absolute path")
	}
	directory := t.TempDir()
	childPath := filepath.Join(directory, "child")
	finallyPath := filepath.Join(directory, "finally")
	afterPath := filepath.Join(directory, "after")
	config := `name: host-signal-contract
vault: none
docker_preflight: false
steps:
  - id: foreground
    kind: host
    run: hold.sh
  - id: after
    kind: host
    run: after.sh
finally:
  - id: cleanup
    kind: host
    run: cleanup.sh
`
	files := map[string]string{
		"config.yml": config,
		"hold.sh":    "#!/bin/bash\ntrap 'exit 130' INT TERM\n" + bashSingleQuoted(os.Args[0]) + " -test.run=^TestHostSignalGrandchildLinux$\nexit \"$?\"\n",
		"cleanup.sh": "#!/bin/bash\nprintf complete > " + bashSingleQuoted(finallyPath) + "\n",
		"after.sh":   "#!/bin/bash\nprintf unexpected > " + bashSingleQuoted(afterPath) + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(binary, "--workflow-file", filepath.Join(directory, "config.yml"), "--workdir", directory, "--no-vault")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "DOCKPIPE_") && !strings.HasPrefix(entry, "XDG_STATE_HOME=") && !strings.HasPrefix(entry, "XDG_CACHE_HOME=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env,
		"DOCKPIPE_SIGNAL_TEST_CHILD="+childPath,
		"XDG_STATE_HOME="+filepath.Join(directory, "state"),
		"XDG_CACHE_HOME="+filepath.Join(directory, "cache"))
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	completed := false
	defer func() {
		if !completed {
			_ = command.Process.Kill()
			select {
			case <-done:
			case <-time.After(6 * time.Second):
				t.Error("candidate CLI did not settle after test disposal")
			}
		}
	}()
	child := waitHostSignalFile(t, childPath)
	pid, err := strconv.Atoi(string(child))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid foreground child identity: %q", child)
	}
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	original, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		completed = true
		if err == nil {
			t.Fatalf("interrupted workflow falsely succeeded: %s", output.String())
		}
	case <-time.After(6 * time.Second):
		t.Fatal("candidate CLI did not finish its interrupted workflow")
	}
	if time.Since(started) >= 2*time.Second {
		t.Fatalf("foreground work outlived graceful signal budget: %s", output.String())
	}
	cleanup, err := os.ReadFile(finallyPath)
	if err != nil || string(cleanup) != "complete" {
		t.Fatalf("workflow finally did not run: %q, %v; %s", cleanup, err, output.String())
	}
	if _, err := os.Stat(afterPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workflow continued after interrupted host step: %v", err)
	}
	if current, err := os.ReadFile(statusPath); err == nil && sameHostSignalProcess(original, current) {
		t.Fatal("interrupted workflow left its foreground descendant alive or unreaped")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
