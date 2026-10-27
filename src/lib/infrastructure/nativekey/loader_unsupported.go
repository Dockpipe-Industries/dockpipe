//go:build (!linux && !darwin) || (!amd64 && !arm64)

package nativekey

import "errors"

func loadLibrary() (*library, error) {
	return nil, errors.New("native security keys are supported on Linux and macOS amd64/arm64; this platform has not been qualified")
}
