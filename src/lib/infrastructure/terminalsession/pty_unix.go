//go:build linux || darwin

package terminalsession

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type unixTerminal struct {
	master    *os.File
	command   *exec.Cmd
	session   int
	stopOnce  sync.Once
	stopError error
}

func startTerminal(ctx context.Context, profile Profile, size Size) (terminalBackend, error) {
	master, slave, err := openPTY()
	if err != nil {
		return nil, err
	}
	defer slave.Close()
	terminal := &unixTerminal{master: master}
	if err = terminal.Resize(size); err != nil {
		master.Close()
		return nil, err
	}
	// Cancellation relies on the Go poller interrupting reads/writes at Close.
	// Fail before spawn if the device cannot provide deadline-aware I/O.
	if err = master.SetReadDeadline(time.Time{}); err != nil {
		master.Close()
		return nil, err
	}
	command := exec.CommandContext(ctx, profile.Executable, profile.Args...)
	command.Dir = profile.Dir
	command.Env = append([]string{}, profile.Env...)
	command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err = command.Start(); err != nil {
		master.Close()
		return nil, err
	}
	terminal.command = command
	terminal.session = command.Process.Pid
	return terminal, nil
}

func (terminal *unixTerminal) Read(data []byte) (int, error) {
	return terminal.master.Read(data)
}
func (terminal *unixTerminal) Write(data []byte) (int, error) {
	return terminal.master.Write(data)
}
func (terminal *unixTerminal) SetWriteDeadline(deadline time.Time) error {
	return terminal.master.SetWriteDeadline(deadline)
}
func (terminal *unixTerminal) Close() error {
	return terminal.master.Close()
}

func (terminal *unixTerminal) Resize(size Size) error {
	connection, err := terminal.master.SyscallConn()
	if err != nil {
		return err
	}
	var resizeError error
	err = connection.Control(func(descriptor uintptr) {
		resizeError = unix.IoctlSetWinsize(int(descriptor), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(size.Rows), Col: uint16(size.Columns)})
	})
	return errors.Join(err, resizeError)
}

func (terminal *unixTerminal) Wait() (int, error) {
	err := terminal.command.Wait()
	if terminal.command.ProcessState == nil {
		return -1, err
	}
	return terminal.command.ProcessState.ExitCode(), err
}

// Interactive shells move jobs into separate process groups. Killing just the
// shell group misses them. Inspect only process identity/session membership,
// never argv or environment; verify birth identity again before every signal.
// Escaped sessions (setsid/daemonization) are deliberately outside this boundary.
func (terminal *unixTerminal) Stop() error {
	terminal.stopOnce.Do(func() {
		members, err := sessionMembers(terminal.session)
		terminal.stopError = err
		// A failed/bounded enumeration must still stop the owned shell. Go's
		// Process handle guards a completed process against PID reuse.
		defer terminal.command.Process.Kill()
		deadline := time.Now().Add(250 * time.Millisecond)
		for {
			remaining := false
			for _, member := range members {
				current, inspectError := inspectProcess(member.pid)
				if errors.Is(inspectError, os.ErrNotExist) || errors.Is(inspectError, unix.ESRCH) {
					continue
				}
				if inspectError != nil {
					terminal.stopError = errors.Join(terminal.stopError, ErrCleanupIncomplete)
					continue
				}
				if current.birth != member.birth || current.session != terminal.session || current.group != member.group || current.zombie {
					continue
				}
				remaining = true
				if member.group > 1 && member.group != unix.Getpgrp() {
					if err := unix.Kill(-member.group, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
						terminal.stopError = errors.Join(terminal.stopError, ErrCleanupIncomplete)
					}
				}
			}
			if !remaining {
				// A job could have forked into another group during the first
				// snapshot. Reinspect before claiming the session is drained.
				latest, err := sessionMembers(terminal.session)
				if err != nil {
					terminal.stopError = errors.Join(terminal.stopError, err)
					break
				}
				members = members[:0]
				for _, member := range latest {
					if !member.zombie {
						members = append(members, member)
					}
				}
				if len(members) == 0 {
					break
				}
			}
			if !time.Now().Before(deadline) {
				terminal.stopError = errors.Join(terminal.stopError, ErrCleanupIncomplete)
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
	return terminal.stopError
}

type processIdentity struct {
	pid     int
	group   int
	session int
	birth   string
	zombie  bool
}

// Same bounds as desktop cleanup: avoid unbounded host enumeration and leave an
// explicit uncertainty receipt when complete cleanup cannot be established.
const maxHostProcesses = 16384
const maxSessionMembers = 1024
