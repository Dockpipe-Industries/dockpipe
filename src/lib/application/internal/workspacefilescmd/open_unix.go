//go:build linux || darwin

package workspacefilescmd

import (
	"os"
	"syscall"
)

func openRequest(name string) (*os.File, error) {
	// Validate the opened descriptor afterward. A FIFO replacement cannot
	// block between an earlier pathname check and the actual open.
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
