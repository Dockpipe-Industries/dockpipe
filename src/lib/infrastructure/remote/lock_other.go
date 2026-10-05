//go:build !linux && !darwin

package remote

import "errors"

func Lock(path string) (func(), error) {
	return nil, errors.New("remote nodes currently require Linux or macOS")
}
