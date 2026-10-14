//go:build !linux

package telemetry

import (
	"context"
	"errors"
)

type NativeCollector struct{ Root string }

func (NativeCollector) Inspect(ctx context.Context, pid int) (Process, error) {
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	return Process{}, errors.New("native process metrics unavailable on this platform")
}

func (NativeCollector) Discover(ctx context.Context) ([]Process, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return nil, false, errors.New("system process discovery unavailable on this platform")
}
