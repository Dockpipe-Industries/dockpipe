package nativehost

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func validSocketPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && len(path) <= 107
}

func peer(conn *net.UnixConn) (*unix.Ucred, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var credential *unix.Ucred
	var credentialErr error
	err = raw.Control(func(fd uintptr) {
		credential, credentialErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}
	return credential, credentialErr
}

func listenSocket(path string) (*net.UnixListener, error) {
	if !validSocketPath(path) {
		return nil, errors.New("invalid native host socket path")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(parent)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0022 != 0 {
		return nil, errors.New("native host socket directory must be owned and not writable by others")
	}
	if existing, err := os.Lstat(path); err == nil {
		owner, ok := existing.Sys().(*syscall.Stat_t)
		if !ok || owner.Uid != uint32(os.Geteuid()) || existing.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("native host socket path occupied")
		}
		connection, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
		if dialErr == nil {
			connection.Close()
			return nil, errors.New("native host socket already listening")
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) && !errors.Is(dialErr, syscall.ENOENT) {
			return nil, errors.New("native host socket liveness is uncertain")
		}
		current, err := os.Lstat(path)
		if os.IsNotExist(err) {
			// Nothing remains to remove.
		} else if err != nil || !os.SameFile(existing, current) {
			return nil, errors.New("native host socket changed during admission")
		} else if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0666); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}
