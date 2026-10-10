//go:build linux || darwin

package remote

import (
	"errors"
	"os"
	"syscall"
)

func Lock(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("another remote process owns this state directory")
	}
	return func() { _ = file.Close() }, nil
}
