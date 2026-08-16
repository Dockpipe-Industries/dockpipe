//go:build !windows

package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func durableFileIdentity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("filesystem identity is unavailable")
	}
	return fmt.Sprintf("unix:%d:%d", uint64(stat.Dev), uint64(stat.Ino)), nil
}

func durableDeviceIdentity(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("filesystem device identity is unavailable")
	}
	return fmt.Sprintf("unix:%d", uint64(stat.Dev)), nil
}

func durableFileInfoIsLinkOrReparse(info os.FileInfo) bool {
	return info == nil || info.Mode()&os.ModeSymlink != 0
}

func makePrivatePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if durableFileInfoIsLinkOrReparse(info) {
		return errors.New("private state path is a filesystem link")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("private state path is not owned by the current user")
	}
	want := os.FileMode(0o600)
	if directory {
		if !info.IsDir() {
			return errors.New("private state directory is not a directory")
		}
		want = 0o700
	} else if !info.Mode().IsRegular() {
		return errors.New("private state file is not a regular file")
	}
	if err := os.Chmod(path, want); err != nil {
		return err
	}
	final, err := os.Lstat(path)
	if err != nil {
		return err
	}
	finalStat, ok := final.Sys().(*syscall.Stat_t)
	if !ok || int(finalStat.Uid) != os.Geteuid() || final.Mode().Perm() != want {
		return errors.New("private state permissions could not be enforced")
	}
	return nil
}

func validatePrivatePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if durableFileInfoIsLinkOrReparse(info) {
		return errors.New("private state path is a filesystem link")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("private state path is not owned by the current user")
	}
	want := os.FileMode(0o600)
	if directory {
		if !info.IsDir() {
			return errors.New("private state directory is not a directory")
		}
		want = 0o700
	} else if !info.Mode().IsRegular() {
		return errors.New("private state file is not a regular file")
	}
	if info.Mode().Perm() != want {
		return fmt.Errorf("private state permissions are %04o, want %04o", info.Mode().Perm(), want)
	}
	return nil
}

func lockPrivateFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlockPrivateFile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func replacePrivateFile(source, destination string) error {
	return os.Rename(source, destination)
}

func syncPrivateDirectory(path string) error {
	dir, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func sameDurablePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
