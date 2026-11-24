//go:build windows

package infrastructure

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// TryLockPrivateState holds a nonblocking exclusive lock on a private
// state file. The Windows handle prevents replacement while the lock is held.
func TryLockPrivateState(root, name string) (func(), error) {
	return lockPrivateState(root, name, windows.LOCKFILE_FAIL_IMMEDIATELY)
}

// LockPrivateState serializes bounded state updates with the same private file
// checks and ownership as TryLockPrivateState.
func LockPrivateState(root, name string) (func(), error) {
	return lockPrivateState(root, name, 0)
}

func lockPrivateState(root, name string, options uint32) (func(), error) {
	path, err := JoinStatePath(root, name)
	if err != nil {
		return nil, err
	}
	if filepath.Dir(path) != filepath.Clean(root) {
		return nil, errors.New("lock name must be a single component")
	}
	if err := ValidatePrivatePath(root, true); err != nil {
		return nil, err
	}

	// Establish the existing private-state ACL when the lock file is new.
	created, err := createPrivateFile(path, os.O_RDWR)
	if err == nil {
		if err := created.Close(); err != nil {
			return nil, err
		}
	} else if !os.IsExist(err) {
		return nil, err
	}

	nativePath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(nativePath, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open lock", Path: path, Err: err}
	}

	retained := false
	defer func() {
		if !retained {
			_ = windows.CloseHandle(handle)
		}
	}()
	var metadata windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &metadata); err != nil {
		return nil, err
	}
	if metadata.NumberOfLinks != 1 ||
		metadata.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return nil, errors.New("private state lock must be one unlinked regular file")
	}
	if err := ValidatePrivatePath(path, false); err != nil {
		return nil, err
	}

	flags := windows.LOCKFILE_EXCLUSIVE_LOCK | options
	if err := windows.LockFileEx(handle, flags, 0, 1, 0, &windows.Overlapped{}); err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, errors.New("another remote process owns this state directory")
		}
		return nil, err
	}

	// os.File makes disposal idempotent even if a caller releases twice.
	file := os.NewFile(uintptr(handle), path)
	retained = true
	return func() { _ = file.Close() }, nil
}
