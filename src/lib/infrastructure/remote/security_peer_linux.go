package remote

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

func securityPeer(connection *net.UnixConn) (int, int, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var credentials *unix.Ucred
	var socketError error
	if err := raw.Control(func(fd uintptr) {
		credentials, socketError = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, 0, err
	}
	if socketError != nil || credentials == nil {
		return 0, 0, errors.New("cannot verify remote security IPC peer")
	}
	return int(credentials.Uid), int(credentials.Pid), nil
}
