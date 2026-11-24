//go:build linux || darwin

package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func prepareDockerExecTerminal(command *exec.Cmd, input *os.File) (func() error, error) {
	descriptor, ok := fdInt(input)
	if !ok {
		return nil, errors.New("container terminal descriptor is unavailable")
	}
	if !term.IsTerminal(descriptor) {
		return func() error { return nil }, nil
	}
	foreground, err := unix.IoctlGetInt(descriptor, unix.TIOCGPGRP)
	if err != nil {
		return nil, fmt.Errorf("read container terminal foreground group: %w", err)
	}
	state, err := term.GetState(descriptor)
	if err != nil {
		return nil, err
	}
	command.SysProcAttr = &syscall.SysProcAttr{Foreground: true, Ctty: descriptor}
	return func() error {
		if err := restoreDockerExecForeground(descriptor, foreground); err != nil {
			return fmt.Errorf("restore container terminal foreground group: %w", err)
		}
		return term.Restore(descriptor, state)
	}, nil
}
