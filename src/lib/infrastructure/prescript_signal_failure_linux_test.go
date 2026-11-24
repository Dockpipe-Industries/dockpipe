//go:build linux

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// A direct child may legally join its parent's group in the same session.
// File gates coordinate only this private fixture, without global syscall hooks.
func TestHostSignalEscapedChildLinux(t *testing.T) {
	directory := os.Getenv("DOCKPIPE_SIGNAL_TEST_ESCAPE")
	if directory == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(directory, "initial"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	waitHostSignalFile(t, filepath.Join(directory, "leave"))
	parentGroup, err := unix.Getpgid(os.Getppid())
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Setpgid(0, parentGroup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "escaped"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(4 * time.Second)
	os.Exit(84)
}

func TestHostSignalFailureOwnerLinux(t *testing.T) {
	mode := os.Getenv("DOCKPIPE_SIGNAL_TEST_FAILURE")
	if mode == "" {
		return
	}
	directory := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestHostSignalEscapedChildLinux$")
	child.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_ESCAPE="+directory)
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	waitStarted := false
	reaped := false
	defer func() {
		_ = child.Process.Kill()
		if !waitStarted {
			_ = child.Wait()
		} else if !reaped {
			select {
			case <-result:
			case <-time.After(2 * time.Second):
				t.Error("owned error-path child did not settle")
			}
		}
	}()
	waitHostSignalFile(t, filepath.Join(directory, "initial"))

	var member *exec.Cmd
	if mode == "startup-retained-group" {
		member = exec.Command("sleep", "4")
		member.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: child.Process.Pid, Pdeathsig: syscall.SIGKILL}
		if err := member.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = member.Process.Kill()
			_ = member.Wait()
		}()
	}
	// Same parent group as the escaped leader; negative signaling of this
	// group would hit both this control and the owner, so it is never used.
	foreign := exec.Command("sleep", "4")
	foreign.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = foreign.Process.Kill()
		_ = foreign.Wait()
	}()
	if err := os.WriteFile(filepath.Join(directory, "leave"), []byte("go"), 0600); err != nil {
		t.Fatal(err)
	}
	waitHostSignalFile(t, filepath.Join(directory, "escaped"))
	group, err := unix.Getpgid(child.Process.Pid)
	if err != nil || group != unix.Getpgrp() {
		t.Fatalf("direct child did not leave its original group: %d, %v", group, err)
	}

	started := time.Now()
	switch mode {
	case "startup-empty-group", "startup-retained-group":
		if err := terminateRunHostProcess(child); err != nil {
			t.Fatal(err)
		}
		waitStarted = true
		err = child.Wait()
		reaped = true
		var exit *exec.ExitError
		if !errors.As(err, &exit) || !exit.Sys().(syscall.WaitStatus).Signaled() || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Fatalf("startup disposal did not kill escaped direct child: %v", err)
		}
		if member != nil {
			var information unix.Siginfo
			if err := unix.Waitid(unix.P_PID, member.Process.Pid, &information, unix.WEXITED|unix.WNOWAIT, nil); err != nil {
				t.Fatal(err)
			}
			// The defer owns this member's sole actual Wait.
		}
	case "waitid-denied":
		signals := make(chan os.Signal, 8)
		observed := make(chan struct{})
		waitStarted = true
		go func() {
			result <- waitHostProcessGroupWithObservation(child, signals, func(pid int) error {
				if pid != child.Process.Pid {
					return syscall.EINVAL
				}
				close(observed)
				return syscall.EPERM
			})
		}()
		<-observed
		// Exercise cancellation during and after handoff. The escaped child
		// cannot be reached through its old group, so only PID forwarding can
		// finish this witness. Delivery is bounded and stops at the one Wait.
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
	awaiting:
		for {
			select {
			case err = <-result:
				reaped = true
				break awaiting
			case <-ticker.C:
				select {
				case signals <- syscall.SIGTERM:
				default:
				}
			case <-deadline.C:
				t.Fatal("waitid-denied child lost cancellable direct-child waiting")
			}
		}
		if !errors.Is(err, syscall.EPERM) {
			t.Fatalf("observation failure was lost: %v", err)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
			t.Fatalf("safe fallback did not preserve signaled child outcome: %v", err)
		}
	default:
		t.Fatalf("unknown error-path mode: %s", mode)
	}
	if time.Since(started) >= 2*time.Second {
		t.Fatal("owned child outlived error-path disposal/cancellation bound")
	}
	var information unix.Siginfo
	if err := unix.Waitid(unix.P_PID, foreign.Process.Pid, &information, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil); err != nil || information.Signo != 0 {
		t.Fatalf("unrelated control child was disturbed: %v", err)
	}
}

func TestHostSignalOwnedErrorPaths(t *testing.T) {
	for _, mode := range []string{"startup-empty-group", "startup-retained-group", "waitid-denied"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
			defer cancel()
			owner := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostSignalFailureOwnerLinux$")
			owner.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_FAILURE="+mode)
			owner.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			var output bytes.Buffer
			owner.Stdout, owner.Stderr = &output, &output
			if err := owner.Run(); err != nil {
				t.Fatalf("%s helper failed: %v; %s", mode, err, output.String())
			}
		})
	}
}
