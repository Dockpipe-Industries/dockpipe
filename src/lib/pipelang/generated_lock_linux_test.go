//go:build linux

package pipelang

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// The lock covers lookup, repair, publication and execution. In particular, a
// stale miss must never quarantine an artifact another worker just published.
// Kernel-owned locks are released if a contained worker exits or is stopped.
func lockGeneratedArtifact(root, key string) (func(), error) {
	dir := filepath.Join(root, ".locks")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, key), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { file.Close() }, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			file.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			file.Close()
			return nil, fmt.Errorf("compiled artifact lock exceeded 30 seconds")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
