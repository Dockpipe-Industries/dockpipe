//go:build linux

package infrastructure

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

func restoreDockerExecForeground(descriptor, foreground int) error {
	// tcsetpgrp from the background must block SIGTTOU. Restrict the mask to
	// this OS thread so the session cannot replace another owner's handlers.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var blocked, previous unix.Sigset_t
	bitsPerWord := uint(unsafe.Sizeof(blocked.Val[0]) * 8)
	signal := uint(unix.SIGTTOU - 1)
	blocked.Val[signal/bitsPerWord] |= 1 << (signal % bitsPerWord)
	if err := unix.PthreadSigmask(unix.SIG_BLOCK, &blocked, &previous); err != nil {
		return err
	}
	err := unix.IoctlSetPointerInt(descriptor, unix.TIOCSPGRP, foreground)
	return errors.Join(err, unix.PthreadSigmask(unix.SIG_SETMASK, &previous, nil))
}
