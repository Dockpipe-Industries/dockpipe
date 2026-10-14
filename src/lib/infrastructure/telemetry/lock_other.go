//go:build !linux && !darwin && !windows

package telemetry

import (
	"errors"
	"os"
)

func tryLock(*os.File) error { return errors.New("execution history locking unsupported") }
