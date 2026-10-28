package osauth

import "time"

// Samples are taken before authority is used. A resumed process therefore sees
// a changed generation before issuing another request, even if sleep signals
// were missed. This does not claim to send broker revocation before suspension.
type suspendTracker struct {
	initialized bool
	previous    time.Duration
	generation  uint64
}

func (tracker *suspendTracker) observe(suspended time.Duration) uint64 {
	if tracker.initialized && (suspended-tracker.previous > 100*time.Millisecond || suspended < tracker.previous-100*time.Millisecond) {
		tracker.generation++
	}
	tracker.previous = suspended
	tracker.initialized = true
	return tracker.generation
}
