package pipelang

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var generatedProfileOrigin = time.Now()
var generatedProfileSequence atomic.Uint64

// Process-relative monotonic spans must never be subtracted across processes or
// boots. The containing unit supplies UTC/boot provenance and wall accounting.
func generatedPhase(t *testing.T, phase string) func() {
	if os.Getenv("PIPELANG_PERFORMANCE_PROFILE") != "1" {
		return func() {}
	}
	start := time.Since(generatedProfileOrigin).Nanoseconds()
	id := generatedProfileSequence.Add(1)
	return func() {
		end := time.Since(generatedProfileOrigin).Nanoseconds()
		t.Logf("generated_span phase=%s clock=go-%d id=%d parent=%q start_ns=%d end_ns=%d elapsed_ns=%d", phase, os.Getpid(), id, t.Name(), start, end, end-start)
	}
}
