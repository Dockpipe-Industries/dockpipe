// Package osauth owns operating-system authentication and local activity signals.
// It never accepts authentication results or activity timestamps from a client.
package osauth

import (
	"context"
	"time"
)

// Snapshot reports independently available signals. Unknown signals must not be
// interpreted as activity or as evidence that the computer is unlocked.
type Snapshot struct {
	Idle            time.Duration
	IdleKnown       bool
	Locked          bool
	LockKnown       bool
	SleepGeneration uint64
	SleepKnown      bool
	Detail          string
}

// Platform is owned by the local authorization authority, not its clients.
// peerPID must come from authenticated kernel IPC credentials.
type Platform interface {
	Authenticate(ctx context.Context, peerPID int) error
	Snapshot(ctx context.Context) (Snapshot, error)
	SecurityDirectory() (string, error)
}

// SuspendClock provides a final admission check without starting a process or
// waiting for desktop services. The generation shares the platform Snapshot's
// sleep tracker. Implementations use only bounded native clock reads.
type SuspendClock interface {
	SuspendGeneration() (uint64, error)
}
