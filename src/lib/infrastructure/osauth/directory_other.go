//go:build !linux && !darwin

package osauth

import "errors"

func securityDirectory() (string, error) {
	return "", errors.New("OS-owned remote security storage is unavailable on this platform")
}
