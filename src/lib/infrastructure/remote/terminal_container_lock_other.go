//go:build !linux && !darwin && !windows

package remote

import "errors"

func lockContainerTerminalStore(string) (func(), error) {
	return nil, errors.New("terminal receipt storage is unavailable on this platform")
}
