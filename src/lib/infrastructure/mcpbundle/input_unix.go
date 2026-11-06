//go:build linux || darwin

package mcpbundle

import (
	"fmt"
	"os"
	"syscall"
)

func openRegular(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("MCPB input must be a regular file: %s", path)
	}
	return file, nil
}
