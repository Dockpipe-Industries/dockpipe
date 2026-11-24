//go:build linux || darwin

package remote

import (
	"os"
	"path/filepath"
	"syscall"

	"dockpipe/src/lib/infrastructure"
)

// The terminal and feature workers are separate processes. Serialize the entire
// receipt snapshot with atomic publication, including its private staging file.
// Keep the lock outside the enumerated store so every unexpected entry remains
// an error. Closing the descriptor releases ownership after a worker crash.
func lockContainerTerminalStore(root string) (func(), error) {
	if err := infrastructure.ValidatePrivatePath(root, true); err != nil {
		return nil, err
	}
	path := filepath.Join(root, "container-terminals.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
		}
	}()
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, os.ErrInvalid
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	failed = false
	return func() { _ = file.Close() }, nil
}
