package remote

import (
	"context"
	"errors"
	"time"

	"dockpipe/src/lib/infrastructure/osauth"
)

type DesktopSessionSnapshot interface {
	Snapshot(context.Context) (osauth.Snapshot, error)
}

// DesktopSessionFence reuses the local OS authentication adapter's read-only
// lock/suspend signals. It never authenticates or replaces operator idle policy.
// Unknown lock/sleep state and a resumed lifetime permanently end this fence.
type DesktopSessionFence struct {
	platform    DesktopSessionSnapshot
	generation  uint64
	initialized bool
	failed      bool
}

func NewDesktopSessionFence(platform DesktopSessionSnapshot) *DesktopSessionFence {
	return &DesktopSessionFence{platform: platform}
}

func (fence *DesktopSessionFence) Check(parent context.Context) error {
	if fence == nil || fence.platform == nil || fence.failed {
		return errors.New("native desktop local OS session is unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	snapshot, err := fence.platform.Snapshot(ctx)
	if err != nil || ctx.Err() != nil || !snapshot.LockKnown || snapshot.Locked || !snapshot.SleepKnown ||
		fence.initialized && snapshot.SleepGeneration != fence.generation {
		fence.failed = true
		return errors.New("native desktop local OS lock or sleep state is unknown, locked or changed")
	}
	fence.generation, fence.initialized = snapshot.SleepGeneration, true
	return nil
}
