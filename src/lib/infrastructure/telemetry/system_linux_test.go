//go:build linux

package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSystemCountersAndMissingMetrics(t *testing.T) {
	var resources SystemResources
	parseCPU("cpu 100 20 30 400 50 6 7 8 99 88\ncpu0 1\ncpu1 2\n", &resources)
	if resources.CPUTotal == nil || *resources.CPUTotal != 621 || *resources.CPUIdle != 450 || resources.LogicalCPUs != 2 {
		t.Fatalf("guest time must not be counted twice: %+v", resources)
	}
	parseMemory("MemTotal: 100 kB\nMemAvailable: 25 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n", &resources)
	if *resources.MemoryTotal != 102400 || *resources.MemoryAvailable != 25600 || resources.SwapTotal == nil || *resources.SwapTotal != 0 {
		t.Fatal("memory counters were not converted from KiB")
	}
	parseNetwork("Inter-| Receive | Transmit\nlo: 999 0 0 0 0 0 0 0 999 0 0 0 0 0 0 0\neth0: 12 0 0 0 0 0 0 0 34 0 0 0 0 0 0 0\n", &resources)
	if *resources.NetworkReceived != 12 || *resources.NetworkSent != 34 {
		t.Fatal("loopback was included in network counters")
	}
	var invalid SystemResources
	parseCPU("cpu 18446744073709551615 1 0 0", &invalid)
	parseMemory("MemTotal: 18446744073709551615 kB", &invalid)
	parseNetwork("invalid", &invalid)
	if invalid.CPUTotal != nil || invalid.MemoryTotal != nil || invalid.NetworkSent != nil || firstMetric("NaN") != nil {
		t.Fatal("invalid or missing measurements became measured values")
	}
}

func TestSystemCollectionCancellationAndReadBound(t *testing.T) {
	collector := NativeCollector{Root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(collector.Root, "stat"), make([]byte, 256*1024+1), 0600); err != nil {
		t.Fatal(err)
	}
	resources, err := collector.System(context.Background())
	if err != nil || resources.CPUTotal != nil || resources.MemoryTotal != nil || len(resources.Notices) == 0 {
		t.Fatalf("missing/oversized sources: %+v %v", resources, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := collector.System(ctx); err != context.Canceled {
		t.Fatal("system collector lost cancellation", err)
	}
}
