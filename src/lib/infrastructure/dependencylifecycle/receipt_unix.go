//go:build linux || darwin

package dependencylifecycle

import (
	"fmt"
	"os"
	"syscall"
)

func openReceipt(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("dependency receipt must be a regular file: %s", path)
	}
	return file, nil
}
