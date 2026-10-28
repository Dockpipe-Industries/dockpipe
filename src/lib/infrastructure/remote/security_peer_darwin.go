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
	var credentials *unix.Xucred
	var pid int
	var socketError error
	if err := raw.Control(func(fd uintptr) {
		credentials, socketError = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if socketError == nil {
			pid, socketError = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		}
	}); err != nil {
		return 0, 0, err
	}
	if socketError != nil || credentials == nil || pid <= 0 {
		return 0, 0, errors.New("cannot verify remote security IPC peer")
	}
	return int(credentials.Uid), pid, nil
}
