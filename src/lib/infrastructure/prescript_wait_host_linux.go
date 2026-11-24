//go:build linux

package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func waitHostScriptWithSignalForward(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid || cmd.SysProcAttr.Pgid != 0 {
		return waitCommandWithSignalForward(cmd)
	}
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(signals)
	return waitHostProcessGroup(cmd, signals)
}

// cmd is the sole owner of this direct child's wait/reap lifecycle. Observe
// exit without reaping: even a zombie leader reserves its PID, so the group ID
// cannot be reused while a forwarding syscall is in flight. Join forwarding
// before cmd.Wait releases that identity. Docker and non-group commands use
// os.Process.Signal instead and never reach this negative-PID signaling path.
func waitHostProcessGroup(cmd *exec.Cmd, signals <-chan os.Signal) error {
	return waitHostProcessGroupWithObservation(cmd, signals, observeHostProcessExit)
}

type hostSignalForwardingResult struct {
	err     error
	pending os.Signal
}

// The observer is a local dependency, not a mutable global syscall seam. It
// observes but never reaps; exec.Cmd remains the only owner of the actual Wait.
func waitHostProcessGroupWithObservation(cmd *exec.Cmd, signals <-chan os.Signal, observe func(int) error) error {
	return waitHostProcessGroupWithCompletion(cmd, signals, observe, nil, nil)
}

// Source output remains supervised while WNOWAIT reserves the exited leader's
// identity. A denied observation aborts private readers before direct-child wait.
func waitHostProcessGroupWithCompletion(cmd *exec.Cmd, signals <-chan os.Signal, observe func(int) error, completeOutput, abortOutput func()) error {
	stop := make(chan struct{})
	joined := make(chan hostSignalForwardingResult, 1)
	go func() {
		var outcome hostSignalForwardingResult
		defer func() { joined <- outcome }()
		for {
			select {
			case <-stop:
				return
			case received, open := <-signals:
				if !open {
					return
				}
				outcome.pending = received
				// Preserve a received-but-unforwarded signal for a possible safe
				// PID-only handoff when observation fails.
				select {
				case <-stop:
					return
				default:
				}
				value, ok := received.(syscall.Signal)
				if !ok {
					outcome.pending = nil
					continue
				}
				err := unix.Kill(-cmd.Process.Pid, value)
				outcome.pending = nil
				if err != nil && !errors.Is(err, unix.ESRCH) && outcome.err == nil {
					outcome.err = fmt.Errorf("forward host job signal: %w", err)
				}
			}
		}
	}()
	observeErr := observe(cmd.Process.Pid)
	if observeErr == nil && completeOutput != nil {
		completeOutput()
	}
	close(stop)
	forwarding := <-joined
	if observeErr != nil {
		if abortOutput != nil {
			abortOutput()
		}
		// A policy can deny P_PID/WNOWAIT while permitting Go's P_PIDFD/WEXITED
		// wait. Continue cancellable direct-child waiting with the same queued
		// signals; never issue another negative-PID signal after this handoff.
		var pendingErr error
		if forwarding.pending != nil {
			pendingErr = cmd.Process.Signal(forwarding.pending)
			if errors.Is(pendingErr, os.ErrProcessDone) || errors.Is(pendingErr, syscall.ESRCH) {
				pendingErr = nil
			}
		}
		waitErr := waitCommandWithSignals(cmd, signals)
		return errors.Join(fmt.Errorf("observe host job exit before reap: %w", observeErr), forwarding.err, pendingErr, waitErr)
	}
	waitErr := cmd.Wait()
	if forwarding.err == nil {
		return waitErr
	}
	return errors.Join(forwarding.err, waitErr)
}

func observeHostProcessExit(pid int) error {
	var information unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, pid, &information, unix.WEXITED|unix.WNOWAIT, nil)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
