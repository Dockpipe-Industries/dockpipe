//go:build linux || darwin

package containerscmd

import (
	"os"
	"syscall"
)

func openRequest(name string) (*os.File, error) {
	// A FIFO replacement must not block before validating the opened descriptor.
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
