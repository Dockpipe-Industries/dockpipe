//go:build linux || darwin

package remote

import (
	"os"
	"strconv"
	"syscall"
)

func terminalTestProcess(data []byte) (*os.Process, error) {
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return nil, err
	}
	return os.FindProcess(pid)
}

func terminalTestAlive(process *os.Process) bool {
	return process.Signal(syscall.Signal(0)) == nil
}
