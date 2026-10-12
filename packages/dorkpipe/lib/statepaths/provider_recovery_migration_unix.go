//go:build !windows

package statepaths

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func migrationFileIdentity(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("provider recovery filesystem identity is unavailable")
	}
	return fmt.Sprintf("unix:%d:%d", uint64(stat.Dev), uint64(stat.Ino)), nil
}

func migrationDeviceIdentity(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("provider recovery filesystem device identity is unavailable")
	}
	return fmt.Sprintf("unix:%d", uint64(stat.Dev)), nil
}

func migrationMakePrivatePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("provider recovery private path is linked")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("provider recovery private path is not owned by the current user")
	}
	want := os.FileMode(0o600)
	if directory {
		if !info.IsDir() {
			return errors.New("provider recovery private directory is not a directory")
		}
		want = 0o700
	} else if !info.Mode().IsRegular() {
		return errors.New("provider recovery private file is not regular")
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
		return errors.New("provider recovery private permissions could not be enforced")
	}
	return nil
}

func migrationLockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func migrationUnlockFile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func migrationSyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
