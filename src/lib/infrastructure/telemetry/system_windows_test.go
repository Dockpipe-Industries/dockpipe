//go:build windows

package telemetry

import (
	"context"
	"testing"
)

// This runs on the native Windows CI runner; cross-compilation alone does not
// qualify the native ABI or prove that boot identity survives CLI invocations.
func TestNativeWindowsSystemCounters(t *testing.T) {
	collector := NativeCollector{}
	first, err := collector.System(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := collector.System(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.BootID == "" || second.BootID != first.BootID {
		t.Fatalf("kernel boot identity unavailable or unstable: %q %q; notices=%v", first.BootID, second.BootID, second.Notices)
	}
	if second.LogicalCPUs <= 0 || second.MemoryTotal == nil || second.MemoryAvailable == nil || second.DiskTotal == nil || second.DiskAvailable == nil || second.NetworkReceived == nil || second.NetworkSent == nil || second.UptimeSeconds == nil {
		t.Fatalf("native Windows counters unavailable: %+v", second)
	}
	if *second.MemoryAvailable > *second.MemoryTotal || *second.DiskAvailable > *second.DiskTotal || *second.UptimeSeconds < *first.UptimeSeconds {
		t.Fatalf("invalid native counter bounds: %+v", second)
	}
	groups, _, _ := systemProcessorGroupCount.Call()
	if groups == 1 && second.LogicalCPUs <= 64 {
		if first.CPUTotal == nil || second.CPUTotal == nil || second.CPUIdle == nil || *second.CPUIdle > *second.CPUTotal || *second.CPUTotal < *first.CPUTotal {
			t.Fatalf("invalid native CPU counters: %+v", second)
		}
	} else if second.CPUTotal != nil || second.CPUIdle != nil {
		t.Fatal("one processor group was incorrectly presented as the whole system")
	}
	if second.SwapTotal != nil || second.SwapFree != nil || second.Load1 != nil {
		t.Fatal("unsupported native metrics were reported as measured")
	}
}
