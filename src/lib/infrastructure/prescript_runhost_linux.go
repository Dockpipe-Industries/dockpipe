//go:build linux

package infrastructure

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// Terminal jobs keep their existing process-group and shell job-control
// topology. Only a verified terminal-free launch receives a private group.
func setRunHostProcAttrs(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	if hostHasNoTerminal() {
		cmd.SysProcAttr.Setpgid = true
	}
}

func hostHasNoTerminal() bool {
	descriptor, err := unix.Open("/dev/tty", unix.O_RDONLY|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err == nil {
		_ = unix.Close(descriptor)
		return false
	}
	// ENXIO is the kernel's no-controlling-terminal answer. An inaccessible
	// or missing /dev/tty is not evidence of absence: retain existing behavior.
	if !errors.Is(err, unix.ENXIO) {
		return false
	}
	for _, descriptor := range []int{0, 1, 2} {
		_, err := unix.IoctlGetTermios(descriptor, unix.TCGETS)
		if !errors.Is(err, unix.ENOTTY) {
			return false
		}
	}
	return true
}

// Startup failed after spawn, before a waiter could own signal forwarding.
// The direct child is still unreaped, so a configured private group is held.
func terminateRunHostProcess(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid || cmd.SysProcAttr.Pgid != 0 {
		return cmd.Process.Kill()
	}
	groupErr := unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
	if errors.Is(groupErr, unix.ESRCH) {
		groupErr = nil
	}
	// A direct child can leave its initial group without leaving our parentage.
	// Retain that safe handle authority even if old-group signaling succeeded
	// for descendants which stayed behind. Kill is idempotent for an exited child.
	childErr := cmd.Process.Kill()
	if errors.Is(childErr, os.ErrProcessDone) || errors.Is(childErr, syscall.ESRCH) {
		childErr = nil
	}
	return errors.Join(groupErr, childErr)
}
