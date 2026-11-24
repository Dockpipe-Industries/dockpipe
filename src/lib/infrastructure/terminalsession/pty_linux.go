//go:build linux

package terminalsession

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Linux devpts establishes slave ownership when opening ptmx; unlockpt is the
// TIOCSPTLCK ioctl. See Linux pts(4), ioctl_tty(2), and pty(7).
func openPTY() (*os.File, *os.File, error) {
	descriptor, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	master := os.NewFile(uintptr(descriptor), "terminal-master")
	name, err := unix.IoctlGetInt(descriptor, unix.TIOCGPTN)
	if err == nil {
		err = unix.IoctlSetPointerInt(descriptor, unix.TIOCSPTLCK, 0)
	}
	if err != nil {
		return nil, nil, errors.Join(err, master.Close())
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", name), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, errors.Join(err, master.Close())
	}
	return master, slave, nil
}

func inspectProcess(pid int) (processIdentity, error) {
	file, err := os.Open("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return processIdentity{}, err
	}
	defer file.Close()
	// stat contains bounded kernel identity fields; never read cmdline/environ.
	data, err := io.ReadAll(io.LimitReader(file, 4096))
	if err != nil {
		return processIdentity{}, err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return processIdentity{}, ErrCleanupIncomplete
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 {
		return processIdentity{}, ErrCleanupIncomplete
	}
	group, groupError := strconv.Atoi(fields[2])
	session, sessionError := strconv.Atoi(fields[3])
	if groupError != nil || sessionError != nil {
		return processIdentity{}, ErrCleanupIncomplete
	}
	return processIdentity{pid: pid, group: group, session: session, birth: fields[19], zombie: fields[0] == "Z" || fields[0] == "X"}, nil
}

func sessionMembers(session int) ([]processIdentity, error) {
	directory, err := os.Open("/proc")
	if err != nil {
		return nil, ErrCleanupIncomplete
	}
	defer directory.Close()
	deadline := time.Now().Add(50 * time.Millisecond)
	members := make([]processIdentity, 0)
	examined := 0
	for {
		entries, err := directory.ReadDir(128)
		if err != nil && !errors.Is(err, io.EOF) {
			return members, ErrCleanupIncomplete
		}
		for _, entry := range entries {
			pid, parseError := strconv.Atoi(entry.Name())
			if parseError != nil {
				continue
			}
			examined++
			if examined > maxHostProcesses || len(members) >= maxSessionMembers || time.Now().After(deadline) {
				return members, ErrCleanupIncomplete
			}
			identity, inspectError := inspectProcess(pid)
			if inspectError != nil {
				if errors.Is(inspectError, os.ErrNotExist) || errors.Is(inspectError, unix.ESRCH) {
					continue
				}
				// /proc permissions can conceal same-session processes, so do
				// not silently claim complete cleanup in that configuration.
				return members, ErrCleanupIncomplete
			}
			if identity.session == session {
				members = append(members, identity)
			}
		}
		if errors.Is(err, io.EOF) {
			return members, nil
		}
	}
}
