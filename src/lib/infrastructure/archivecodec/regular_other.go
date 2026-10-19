//go:build !linux && !darwin && !windows

package archivecodec

import (
	"errors"
	"os"
)

func openRegular(path string) (*os.File, error) {
	return nil, errors.New("secure external archive decoder files are unsupported on this platform")
}
