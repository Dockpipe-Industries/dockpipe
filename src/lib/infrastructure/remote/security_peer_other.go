//go:build !linux && !darwin

package remote

import (
	"errors"
	"net"
)

func securityPeer(connection *net.UnixConn) (int, int, error) {
	return 0, 0, errors.New("OS-authenticated remote security IPC is unavailable on this platform")
}
