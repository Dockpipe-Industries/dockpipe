//go:build !linux && !darwin && !windows

package dependencylifecycle

import (
	"errors"
	"os"
)

func openReceipt(path string) (*os.File, error) {
	return nil, errors.New("secure dependency receipts are unsupported on this platform")
}
