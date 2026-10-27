//go:build (!linux && !darwin) || (linux && !amd64 && !arm64)

package osauth

import (
	"context"
	"errors"
)

type unavailablePlatform struct{}

func New() (Platform, error) { return &unavailablePlatform{}, nil }

func (*unavailablePlatform) Authenticate(ctx context.Context, peerPID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("OS-authenticated remote unlock is unavailable on this platform; native authentication qualification is pending")
}

func (*unavailablePlatform) Snapshot(ctx context.Context) (Snapshot, error) {
	return Snapshot{Detail: "Native remote authentication, idle, lock and sleep integration are unavailable on this platform."}, ctx.Err()
}

func (*unavailablePlatform) SecurityDirectory() (string, error) { return securityDirectory() }
