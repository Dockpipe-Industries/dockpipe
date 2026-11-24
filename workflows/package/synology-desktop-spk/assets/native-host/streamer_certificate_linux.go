//go:build linux

package main

import (
	"errors"
	"os"
	"syscall"
)

// Older desktop installations created their public certificate as 0644. Keep
// the existing certificate and key, but establish the gateway's private-file
// contract during post-install migration, before its service starts.
func normalizeStreamerCertificate(path string) error {
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		// A fresh desktop creates its certificate under umask 0077 at startup.
		return nil
	}
	if err != nil {
		return errors.New("stream_certificate_migration_unavailable")
	}
	file := os.NewFile(uintptr(descriptor), "streamer certificate migration")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("stream_certificate_migration_invalid")
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(metadata.Uid) != os.Geteuid() {
		return errors.New("stream_certificate_migration_owner_invalid")
	}
	permissions := info.Mode().Perm()
	if permissions != 0600 && permissions != 0644 {
		return errors.New("stream_certificate_migration_permissions_invalid")
	}
	if permissions == 0600 {
		return nil
	}
	// Change the opened inode, not a path that could have been replaced after
	// validation. This does not read, regenerate or rewrite certificate bytes.
	if file.Chmod(0600) != nil {
		return errors.New("stream_certificate_migration_failed")
	}
	return nil
}
