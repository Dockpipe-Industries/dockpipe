//go:build !linux && !darwin

package remote

import "errors"

func lockContainerTerminalStore(string) (func(), error) {
	return nil, errors.New("remote nodes currently require Linux or macOS")
}
