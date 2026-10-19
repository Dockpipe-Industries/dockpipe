package telemetry

import (
	"context"
	"errors"
	"math"
	"testing"
)

type windowsSystemFixture struct {
	boot                         string
	logical                      uint32
	kernel, user, idle           uint64
	memoryTotal, memoryAvailable uint64
	diskTotal, diskAvailable     uint64
	received, sent               uint64
	seconds                      float64
	failure                      string
	calls                        []string
	cancel                       context.CancelFunc
}

func (fixture *windowsSystemFixture) called(name string) error {
	fixture.calls = append(fixture.calls, name)
	if fixture.cancel != nil {
		fixture.cancel()
	}
	if fixture.failure == name {
		return errors.New("native source failed")
	}
	return nil
}

func (fixture *windowsSystemFixture) bootID() (string, error) {
	return fixture.boot, fixture.called("boot")
}
func (fixture *windowsSystemFixture) cpu() (uint32, uint64, uint64, uint64, error) {
	return fixture.logical, fixture.kernel, fixture.user, fixture.idle, fixture.called("cpu")
}
func (fixture *windowsSystemFixture) memory() (uint64, uint64, error) {
	return fixture.memoryTotal, fixture.memoryAvailable, fixture.called("memory")
}
func (fixture *windowsSystemFixture) disk() (uint64, uint64, error) {
	return fixture.diskTotal, fixture.diskAvailable, fixture.called("disk")
}
func (fixture *windowsSystemFixture) network(context.Context) (uint64, uint64, error) {
	return fixture.received, fixture.sent, fixture.called("network")
}
func (fixture *windowsSystemFixture) uptime() (float64, error) {
	return fixture.seconds, fixture.called("uptime")
}

func validWindowsSystemFixture() *windowsSystemFixture {
	return &windowsSystemFixture{
		boot: "kernel-boot-guid", logical: 8, kernel: 700, user: 300, idle: 400,
		memoryTotal: 16384, memoryAvailable: 8192, diskTotal: 1000, diskAvailable: 250,
		received: 120, sent: 80, seconds: 123.456,
	}
}

func TestWindowsSystemCounterSemantics(t *testing.T) {
	first, err := collectWindowsSystem(context.Background(), validWindowsSystemFixture())
	if err != nil {
		t.Fatal(err)
	}
	if first.BootID != "kernel-boot-guid" || first.LogicalCPUs != 8 || *first.CPUTotal != 1000 || *first.CPUIdle != 400 {
		t.Fatalf("idle must already be included in kernel time: %+v", first)
	}
	if *first.MemoryTotal != 16384 || *first.MemoryAvailable != 8192 || *first.DiskTotal != 1000 || *first.DiskAvailable != 250 || *first.NetworkReceived != 120 || *first.NetworkSent != 80 || *first.UptimeSeconds != 123.456 {
		t.Fatalf("native units changed: %+v", first)
	}
	if first.SwapTotal != nil || first.SwapFree != nil || first.Load1 != nil || len(first.Notices) != 1 {
		t.Fatal("commit limits must not become swap, nor missing load become zero")
	}
	// Independent sources model separate short-lived CLI collectors. They must
	// use the kernel identity rather than deriving a fresh timestamp per sample.
	nextSource := validWindowsSystemFixture()
	nextSource.kernel += 200
	nextSource.received += 50
	next, _ := collectWindowsSystem(context.Background(), nextSource)
	if next.BootID != first.BootID || *next.CPUTotal-*first.CPUTotal != 200 || *next.NetworkReceived-*first.NetworkReceived != 50 {
		t.Fatal("independent samples lost cumulative counters or boot identity")
	}
	nextSource.boot = "next-kernel-boot-guid"
	nextSource.kernel, nextSource.user, nextSource.idle = 5, 3, 2
	rebooted, _ := collectWindowsSystem(context.Background(), nextSource)
	if rebooted.BootID == first.BootID || *rebooted.CPUTotal != 8 {
		t.Fatal("reboot identity/counter reset was hidden")
	}
}

func TestWindowsSystemPartialFailuresRemainNull(t *testing.T) {
	for _, failure := range []string{"boot", "cpu", "memory", "disk", "network", "uptime"} {
		t.Run(failure, func(t *testing.T) {
			fixture := validWindowsSystemFixture()
			fixture.failure = failure
			result, err := collectWindowsSystem(context.Background(), fixture)
			if err != nil || len(result.Notices) != 2 || len(fixture.calls) != 6 {
				t.Fatalf("individual failure prevented other metrics: %+v, %v", result, err)
			}
			missing := map[string]bool{
				"boot": result.BootID == "", "cpu": result.CPUTotal == nil && result.CPUIdle == nil,
				"memory":  result.MemoryTotal == nil && result.MemoryAvailable == nil,
				"disk":    result.DiskTotal == nil && result.DiskAvailable == nil,
				"network": result.NetworkReceived == nil && result.NetworkSent == nil,
				"uptime":  result.UptimeSeconds == nil,
			}
			for metric, isMissing := range missing {
				if isMissing != (metric == failure) {
					t.Fatalf("wrong missing metric %s after %s failure", metric, failure)
				}
			}
		})
	}
}

func TestWindowsSystemRejectsInvalidCounters(t *testing.T) {
	cases := []struct {
		name    string
		change  func(*windowsSystemFixture)
		missing func(SystemResources) bool
	}{
		{"CPU overflow", func(f *windowsSystemFixture) { f.kernel = math.MaxUint64 }, func(r SystemResources) bool { return r.CPUTotal == nil }},
		{"idle exceeds kernel", func(f *windowsSystemFixture) { f.idle = 701 }, func(r SystemResources) bool { return r.CPUIdle == nil }},
		{"multiple CPU groups", func(f *windowsSystemFixture) { f.logical = 128 }, func(r SystemResources) bool { return r.CPUTotal == nil && r.LogicalCPUs == 128 }},
		{"no CPUs", func(f *windowsSystemFixture) { f.logical = 0 }, func(r SystemResources) bool { return r.CPUTotal == nil }},
		{"memory bounds", func(f *windowsSystemFixture) { f.memoryAvailable = f.memoryTotal + 1 }, func(r SystemResources) bool { return r.MemoryTotal == nil }},
		{"disk bounds", func(f *windowsSystemFixture) { f.diskAvailable = f.diskTotal + 1 }, func(r SystemResources) bool { return r.DiskTotal == nil }},
		{"invalid uptime", func(f *windowsSystemFixture) { f.seconds = math.NaN() }, func(r SystemResources) bool { return r.UptimeSeconds == nil }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := validWindowsSystemFixture()
			test.change(fixture)
			result, err := collectWindowsSystem(context.Background(), fixture)
			if err != nil || !test.missing(result) || len(result.Notices) != 2 {
				t.Fatalf("invalid source became a measurement: %+v %v", result, err)
			}
		})
	}
}

func TestWindowsSystemCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fixture := validWindowsSystemFixture()
	if _, err := collectWindowsSystem(ctx, fixture); err != context.Canceled || len(fixture.calls) != 0 {
		t.Fatal("cancelled collection called native sources", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	fixture = validWindowsSystemFixture()
	fixture.cancel = cancel
	result, err := collectWindowsSystem(ctx, fixture)
	if err != context.Canceled || len(fixture.calls) != 1 || result.CPUTotal != nil {
		t.Fatal("collection continued after cancellation", err)
	}
}

func TestWindowsNetworkOverflowIsAtomic(t *testing.T) {
	received, sent := uint64(10), uint64(20)
	if err := addWindowsNetworkCounters(&received, &sent, 30, 40); err != nil || received != 40 || sent != 60 {
		t.Fatal("valid network counters rejected")
	}
	if err := addWindowsNetworkCounters(&received, &sent, 1, math.MaxUint64); err == nil || received != 40 || sent != 60 {
		t.Fatal("overflow left partial counters")
	}
}
