//go:build !linux

package telemetry

import "context"

func (NativeCollector) System(ctx context.Context) (SystemResources, error) {
	return SystemResources{Notices: []string{"System resource collection is unavailable on this platform."}}, ctx.Err()
}
