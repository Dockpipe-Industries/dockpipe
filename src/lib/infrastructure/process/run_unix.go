//go:build linux || darwin

// Package process owns the lifetime of command trees used for bounded operations.
package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Run requires a command created with exec.CommandContext. Descendants share a
// process group and are stopped on cancellation and when the command finishes.
func Run(command *exec.Cmd) error {
	if command.Cancel == nil {
		return errors.New("process tree requires a context-bound command")
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	if command.Env == nil {
		command.Env = os.Environ()
	}
	command.Env = append(command.Env, "DOCKPIPE_PROCESS_TREE=unix-group")
	command.SysProcAttr.Setpgid = true
	command.SysProcAttr.Pgid = 0
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	defer func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }()
	return command.Wait()
}

func inheritedTree() bool {
	return os.Getenv("DOCKPIPE_PROCESS_TREE") == "unix-group"
}
