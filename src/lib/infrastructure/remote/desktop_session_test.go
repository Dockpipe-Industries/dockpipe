package remote

import (
	"context"
	"errors"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/osauth"
)

type desktopSessionSnapshotFixture struct {
	snapshot osauth.Snapshot
	err      error
}

func (fixture *desktopSessionSnapshotFixture) Snapshot(ctx context.Context) (osauth.Snapshot, error) {
	deadline, exists := ctx.Deadline()
	if !exists || time.Until(deadline) > 2*time.Second {
		return osauth.Snapshot{}, errors.New("snapshot call did not carry a bounded deadline")
	}
	return fixture.snapshot, fixture.err
}

func TestDesktopSessionFenceRejectsUnknownLockAndResumedLifetimes(t *testing.T) {
	for _, scenario := range []string{"locked", "lock-unknown", "sleep-unknown", "resume", "error"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := &desktopSessionSnapshotFixture{snapshot: osauth.Snapshot{LockKnown: true, SleepKnown: true}}
			fence := NewDesktopSessionFence(fixture)
			// Idle is intentionally unknown. This fence does not replace or weaken
			// the independently configured operator inactivity policy.
			if err := fence.Check(context.Background()); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "locked":
				fixture.snapshot.Locked = true
			case "lock-unknown":
				fixture.snapshot.LockKnown = false
			case "sleep-unknown":
				fixture.snapshot.SleepKnown = false
			case "resume":
				fixture.snapshot.SleepGeneration++
			case "error":
				fixture.err = errors.New("fixture unavailable")
			}
			if fence.Check(context.Background()) == nil {
				t.Fatal("unsafe local OS session retained desktop authority")
			}
			fixture.snapshot, fixture.err = osauth.Snapshot{LockKnown: true, SleepKnown: true}, nil
			if fence.Check(context.Background()) == nil {
				t.Fatal("ended desktop lifetime silently resumed after unlock")
			}
		})
	}
}
