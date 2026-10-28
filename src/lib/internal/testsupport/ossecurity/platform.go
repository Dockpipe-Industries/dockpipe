// Package ossecurity provides an explicit native-authentication fixture for
// integration tests. Production entrypoints always construct osauth.New().
package ossecurity

import (
	"context"

	"dockpipe/src/lib/infrastructure/osauth"
)

type Platform struct {
	Directory string
}

func (platform Platform) Authenticate(ctx context.Context, pid int) error {
	return ctx.Err()
}

func (platform Platform) Snapshot(ctx context.Context) (osauth.Snapshot, error) {
	return osauth.Snapshot{IdleKnown: true, LockKnown: true, SleepKnown: true}, ctx.Err()
}

func (platform Platform) SecurityDirectory() (string, error) {
	return platform.Directory, nil
}

func (platform Platform) SuspendGeneration() (uint64, error) {
	return 0, nil
}
