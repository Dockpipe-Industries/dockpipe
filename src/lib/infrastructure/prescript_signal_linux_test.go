//go:build linux

package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The foreground child has a finite fallback lifetime even if forwarding fails.
// It does not install a trap or emulate Dockpipe's cancellation behavior.
func TestHostSignalGrandchildLinux(t *testing.T) {
	path := os.Getenv("DOCKPIPE_SIGNAL_TEST_CHILD")
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(81)
	}
	time.Sleep(4 * time.Second)
	os.Exit(82)
}

// Re-execution keeps signal.Notify and SIGINT out of the test runner process.
func TestHostSignalOwnerLinux(t *testing.T) {
	directory := os.Getenv("DOCKPIPE_SIGNAL_TEST_OWNER")
	if directory == "" {
		return
	}
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	script := `trap 'exit 130' INT TERM
trap 'printf finally > "$2"' EXIT
"$1" -test.run=^TestHostSignalGrandchildLinux$
exit "$?"
`
	command := exec.Command("bash", "-c", script, "host-signal-test", os.Args[0], filepath.Join(directory, "finally"))
	command.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_CHILD="+filepath.Join(directory, "child"))
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	setRunHostProcAttrs(command)
	if !command.SysProcAttr.Setpgid {
		t.Fatal("detached owner was not admitted to private host group forwarding")
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	// Emergency disposal goes through the same reserved-identity owner. It is
	// evidence of failure, never accepted as graceful interruption.
	forced := make(chan struct{}, 1)
	timer := time.AfterFunc(3*time.Second, func() {
		forced <- struct{}{}
		signals <- syscall.SIGKILL
	})
	err := waitHostProcessGroup(command, signals)
	timer.Stop()
	select {
	case <-forced:
		t.Fatal("host job required emergency disposal")
	default:
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 130 {
		t.Fatalf("host interruption lost its exit outcome: %v", err)
	}
	value, readErr := os.ReadFile(filepath.Join(directory, "finally"))
	if readErr != nil || string(value) != "finally" {
		t.Fatalf("shell cleanup did not run: %q, %v", value, readErr)
	}
}

func waitHostSignalFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil && len(content) > 0 {
			return content
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("host child did not become ready: %s", path)
	return nil
}

func TestHostSignalReachesForegroundGrandchildAndPreservesForeignGroup(t *testing.T) {
	directory := t.TempDir()
	// This separately owned sibling must never receive a host job signal.
	foreign := exec.Command("sleep", "10")
	foreign.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = foreign.Process.Kill()
		_ = foreign.Wait()
	}()
	owner := exec.Command(os.Args[0], "-test.run=^TestHostSignalOwnerLinux$")
	owner.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_OWNER="+directory)
	owner.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var output bytes.Buffer
	owner.Stdout = &output
	owner.Stderr = &output
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- owner.Wait() }()
	defer func() {
		_ = owner.Process.Kill()
	}()
	childText := waitHostSignalFile(t, filepath.Join(directory, "child"))
	child, err := strconv.Atoi(string(childText))
	if err != nil || child <= 0 {
		t.Fatalf("invalid child identity: %q", childText)
	}
	// Read only an observed owned child's identity. Never signal a recorded PID.
	statusPath := fmt.Sprintf("/proc/%d/stat", child)
	originalStatus, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("host signal helper failed: %v; %s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("host signal helper exceeded its bounded shutdown")
	}
	if err := foreign.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("foreign process group was disturbed: %v", err)
	}
	var foreignStatus unix.Siginfo
	if err := unix.Waitid(unix.P_PID, foreign.Process.Pid, &foreignStatus, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil); err != nil || foreignStatus.Signo != 0 {
		t.Fatalf("foreign process exited during owned-group signaling: %v", err)
	}
	if status, err := os.ReadFile(statusPath); err == nil && sameHostSignalProcess(originalStatus, status) {
		t.Fatal("foreground descendant remained alive or unreaped after shell cleanup")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func sameHostSignalProcess(before, after []byte) bool {
	// /proc stat's comm may contain spaces; starttime is field22, after comm.
	fields := func(value []byte) []string {
		end := strings.LastIndexByte(string(value), ')')
		if end < 0 {
			return nil
		}
		return strings.Fields(string(value[end+1:]))
	}
	left, right := fields(before), fields(after)
	return len(left) > 19 && len(right) > 19 && left[19] == right[19]
}

func TestHostSignalExitJoinsBeforeReap(t *testing.T) {
	// Exercise queued-signal/normal-exit races without signaling the test runner.
	for attempt := 0; attempt < 8; attempt++ {
		command := exec.Command("bash", "-c", "exit 0")
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		signals := make(chan os.Signal, 1)
		signals <- syscall.SIGCONT
		if err := waitHostProcessGroup(command, signals); err != nil {
			t.Fatal(err)
		}
		var information unix.Siginfo
		if err := unix.Waitid(unix.P_PID, command.Process.Pid, &information, unix.WEXITED|unix.WNOHANG, nil); !errors.Is(err, unix.ECHILD) {
			t.Fatalf("direct child was not reaped exactly once: %v", err)
		}
	}
}
