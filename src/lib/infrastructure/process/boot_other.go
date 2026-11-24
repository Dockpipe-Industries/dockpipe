//go:build !linux && !darwin && !windows

package process

import "errors"

func HostBootIdentity() (string, error) {
	return "", errors.New("kernel boot-session identity is unavailable")
}
