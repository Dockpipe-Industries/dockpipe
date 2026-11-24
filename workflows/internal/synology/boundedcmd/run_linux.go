//go:build linux

// Package boundedcmd owns process output budgets and process-group cleanup.
package boundedcmd

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Failure struct {
	Reason   string
	ExitCode *int
}

func (failure *Failure) Error() string {
	return "bounded process failed"
}

// stdout and stderr share one byte budget. Stderr is never retained or returned.
type helperOutput struct {
	mutex     sync.Mutex
	remaining int
	stdout    strings.Builder
	exhausted bool
}

type helperWriter struct {
	output *helperOutput
	retain bool
}

func (writer helperWriter) Write(data []byte) (int, error) {
	writer.output.mutex.Lock()
	defer writer.output.mutex.Unlock()
	if len(data) > writer.output.remaining {
		writer.output.exhausted = true
		return 0, errors.New("helper output limit")
	}
	writer.output.remaining -= len(data)
	if writer.retain {
		writer.output.stdout.Write(data)
	}
	return len(data), nil
}

// Run owns the caller's command group and complete aggregate output budget.
// Command selection stays at each fixed vendor-helper or qualification boundary.
// Failures contain only fixed reasons and a normal exit code, never raw errors.
func Run(command *exec.Cmd, maximumOutput int) (string, *Failure) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var cancelled atomic.Bool
	command.Cancel = func() error {
		cancelled.Store(true)
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	command.WaitDelay = 100 * time.Millisecond
	defer func() {
		if command.Process != nil {
			// Also release descendants when the helper exits before its pipes do.
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	}()
	output := &helperOutput{remaining: maximumOutput}
	command.Stdout = helperWriter{output: output, retain: true}
	command.Stderr = helperWriter{output: output}
	runError := command.Run()
	if output.exhausted {
		return "", &Failure{Reason: "output_limit"}
	}
	if cancelled.Load() {
		return "", &Failure{Reason: "cancelled"}
	}
	if runError != nil {
		if command.Process == nil {
			return "", &Failure{Reason: "start_failed"}
		}
		var exitError *exec.ExitError
		if errors.As(runError, &exitError) && exitError.Exited() {
			exitCode := exitError.ExitCode()
			return "", &Failure{Reason: "exit_nonzero", ExitCode: &exitCode}
		}
		return "", &Failure{Reason: "process_failed"}
	}
	return output.stdout.String(), nil
}
