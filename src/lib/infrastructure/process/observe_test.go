package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestObserverSeesStartFailureAndCompletion(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("containment unsupported")
	}
	command := exec.CommandContext(context.Background(), filepath.Join(t.TempDir(), "missing"))
	called := false
	err := RunObserved(command, func(observed *exec.Cmd) func(error) {
		if observed.Process != nil {
			t.Fatal("start failure has a process")
		}
		return func(err error) {
			called = true
			if err == nil {
				t.Error("lost start error")
			}
		}
	})
	if err == nil || !called {
		t.Fatal("missing failure lifecycle")
	}
}

func TestObserverCancellationKeepsSupervision(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("containment unsupported")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestObserverChild$")
	command.Env = append(os.Environ(), "DOCKPIPE_OBSERVER_CHILD=1")
	// Keep the child's read blocked until supervision cancels it. A nil Stdin
	// supplies EOF, allowing the child to exit successfully before cancellation.
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	finished := false
	err = RunObserved(command, func(observed *exec.Cmd) func(error) {
		if observed.Process == nil {
			t.Fatal("missing PID after start")
		}
		cancel()
		return func(err error) {
			finished = true
			if err == nil {
				t.Error("lost cancellation")
			}
		}
	})
	if err == nil || !finished {
		t.Fatal("missing cancellation lifecycle")
	}
}

func TestObserverChild(t *testing.T) {
	if os.Getenv("DOCKPIPE_OBSERVER_CHILD") != "1" {
		return
	}
	// The parent cancels immediately after Start; the pipe blocks without timing.
	_, _ = os.Stdin.Read(make([]byte, 1))
	os.Exit(0)
}
