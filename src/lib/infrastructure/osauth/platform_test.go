package osauth

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestSuspendTrackerIgnoresSamplingNoiseAndDetectsClockReset(t *testing.T) {
	tracker := suspendTracker{}
	if tracker.observe(time.Second) != 0 || tracker.observe(time.Second+time.Millisecond) != 0 {
		t.Fatal("clock sampling noise changed the generation")
	}
	if tracker.observe(5*time.Second) != 1 || tracker.observe(0) != 2 {
		t.Fatal("suspend or clock reset did not change the generation")
	}
}

func TestOSCommandCopiedOutputCannotBypassBound(t *testing.T) {
	output := &boundedOutput{maximum: 4}
	// LimitReader deliberately exercises io.Copy's destination fast-path lookup.
	reader := io.LimitReader(strings.NewReader("12345"), 5)
	if _, err := io.Copy(output, reader); err == nil {
		t.Fatal("copy accepted output larger than its contract")
	}
	if len(output.Bytes()) > 4 {
		t.Fatal("copy retained output beyond its bound")
	}
}

func TestOSCommandOutputIsBounded(t *testing.T) {
	output := &boundedOutput{maximum: 4}
	if _, err := output.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("5")); err == nil || output.String() != "1234" {
		t.Fatal("native output exceeded its contract")
	}
}
