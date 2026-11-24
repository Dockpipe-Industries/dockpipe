//go:build darwin

package process

import (
	"errors"
	"strings"

	"golang.org/x/sys/unix"
)

func HostBootIdentity() (string, error) {
	identity, err := unix.Sysctl("kern.bootsessionuuid")
	identity = strings.TrimSpace(identity)
	if err != nil || len(identity) != 36 {
		return "", errors.New("kernel boot-session UUID is unavailable")
	}
	return strings.ToLower(identity), nil
}
