//go:build darwin

package infrastructure

import (
	"errors"
	"runtime"
	"syscall"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

func restoreDockerExecForeground(descriptor, foreground int) error {
	// Darwin's sigset_t is a 32-bit mask. x/sys does not export pthread_sigmask
	// here; use the system libc entrypoint without changing process handlers.
	library, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return err
	}
	defer purego.Dlclose(library)
	symbol, err := purego.Dlsym(library, "pthread_sigmask")
	if err != nil {
		return err
	}
	var mask func(int32, *uint32, *uint32) int32
	purego.RegisterFunc(&mask, symbol)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const block, setMask = 1, 3
	blocked := uint32(1) << uint(unix.SIGTTOU-1)
	var previous uint32
	if code := mask(block, &blocked, &previous); code != 0 {
		return syscall.Errno(code)
	}
	err = unix.IoctlSetPointerInt(descriptor, unix.TIOCSPGRP, foreground)
	if code := mask(setMask, &previous, nil); code != 0 {
		return errors.Join(err, syscall.Errno(code))
	}
	return err
}
