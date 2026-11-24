package telemetry

import (
	"context"
	"errors"
	"math"
	"testing"
)

type darwinSystemFixture struct {
	bootFailure                 error
	failure                     error
	free, inactive, page, total uint64
	cancel                      context.CancelFunc
	calls                       int
}

func (source *darwinSystemFixture) boot() (string, float64, error) {
	source.calls++
	if source.cancel != nil {
		source.cancel()
	}
	if source.bootFailure != nil {
		return "", 0, source.bootFailure
	}
	return "native-boot-identity", 1234.5, source.failure
}
func (source *darwinSystemFixture) cpu() (uint32, [4]uint32, error) {
	source.calls++
	return 12, [4]uint32{10, 20, 80, 5}, source.failure
}
func (source *darwinSystemFixture) memory() (uint64, uint64, uint64, uint64, error) {
	source.calls++
	return source.total, source.free, source.inactive, source.page, source.failure
}
func (source *darwinSystemFixture) swap() (uint64, uint64, error) {
	source.calls++
	return 0, 0, source.failure // A genuinely unused swap pool remains measured zero.
}
func (source *darwinSystemFixture) disk() (uint64, uint64, error) {
	source.calls++
	return 1024, 256, source.failure
}
func (source *darwinSystemFixture) network(context.Context) (uint64, uint64, error) {
	source.calls++
	return 123456789, 987654321, source.failure
}
func (source *darwinSystemFixture) load() (float64, error) {
	source.calls++
	return 1.25, source.failure
}

func TestDarwinSystemCounterSemantics(t *testing.T) {
	source := &darwinSystemFixture{total: 100 * 16384, free: 3, inactive: 7, page: 16384}
	result, err := collectDarwinSystem(context.Background(), source)
	if err != nil || len(result.Notices) != 0 || source.calls != 7 {
		t.Fatalf("unexpected collection: %#v, %v, calls=%d", result, err, source.calls)
	}
	if result.CPUTotal == nil || *result.CPUTotal != 115 || result.CPUIdle == nil || *result.CPUIdle != 80 || result.LogicalCPUs != 12 {
		t.Fatal("native cumulative CPU ticks were replaced by another unit")
	}
	if result.MemoryAvailable == nil || *result.MemoryAvailable != 10*16384 {
		t.Fatal("memory pages were double-counted or not converted to bytes")
	}
	if result.NetworkReceived == nil || *result.NetworkReceived != 123456789 || result.NetworkSent == nil || *result.NetworkSent != 987654321 {
		t.Fatal("native network byte counters changed")
	}
	if result.SwapTotal == nil || *result.SwapTotal != 0 || result.SwapFree == nil || *result.SwapFree != 0 {
		t.Fatal("real zero swap counters were confused with unavailable data")
	}
	if result.BootID != "native-boot-identity" || result.UptimeSeconds == nil || *result.UptimeSeconds != 1234.5 || result.Load1 == nil || *result.Load1 != 1.25 {
		t.Fatal("native identity and gauges were lost")
	}
}

func TestDarwinSystemPartialFailuresRemainNull(t *testing.T) {
	source := &darwinSystemFixture{failure: errors.New("native failure")}
	result, err := collectDarwinSystem(context.Background(), source)
	if err != nil || len(result.Notices) != 7 {
		t.Fatalf("partial failures must be reported per source: %#v %v", result, err)
	}
	if result.CPUTotal != nil || result.CPUIdle != nil || result.MemoryTotal != nil || result.MemoryAvailable != nil || result.SwapTotal != nil || result.SwapFree != nil || result.DiskTotal != nil || result.DiskAvailable != nil || result.NetworkReceived != nil || result.NetworkSent != nil || result.UptimeSeconds != nil || result.Load1 != nil || result.BootID != "" {
		t.Fatal("unavailable counters became synthetic zero measurements")
	}
}

func TestDarwinSystemMemoryOverflowAndInconsistency(t *testing.T) {
	for name, source := range map[string]darwinSystemFixture{
		"page count overflow": {total: math.MaxUint64, free: math.MaxUint64, inactive: 1, page: 1},
		"byte overflow":       {total: math.MaxUint64, free: math.MaxUint64, page: 16384},
		"above total":         {total: 10, free: 1, page: 16384},
		"zero page":           {total: 100, free: 1},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := collectDarwinSystem(context.Background(), &source)
			if err != nil || result.MemoryTotal != nil || result.MemoryAvailable != nil || len(result.Notices) == 0 {
				t.Fatalf("invalid memory was published: %#v %v", result, err)
			}
			if result.CPUTotal == nil || result.DiskTotal == nil {
				t.Fatal("one invalid source suppressed independent measurements")
			}
		})
	}
}

func TestDarwinSystemCancellationStopsNativeSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := &darwinSystemFixture{}
	if _, err := collectDarwinSystem(ctx, source); err != context.Canceled || source.calls != 0 {
		t.Fatal("cancelled collection accessed native sources")
	}
	ctx, cancel = context.WithCancel(context.Background())
	source = &darwinSystemFixture{cancel: cancel}
	if _, err := collectDarwinSystem(ctx, source); err != context.Canceled || source.calls != 1 {
		t.Fatal("cancellation did not stop subsequent native sources")
	}
}

func TestDarwinSystemRejectsNonfiniteGauges(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1} {
		if validDarwinGauge(value) {
			t.Fatal("invalid native gauge accepted")
		}
	}
}

func TestDarwinSystemBootFailureKeepsIndependentMeasurements(t *testing.T) {
	source := &darwinSystemFixture{
		bootFailure: errors.New("boot identity unavailable"),
		total:       100 * 16384, free: 3, inactive: 7, page: 16384,
	}
	result, err := collectDarwinSystem(context.Background(), source)
	if err != nil || result.BootID != "" || result.UptimeSeconds != nil || len(result.Notices) != 1 {
		t.Fatalf("boot failure must remain explicit: %#v %v", result, err)
	}
	if result.MemoryAvailable == nil || *result.MemoryAvailable != 10*16384 || result.SwapTotal == nil || result.DiskTotal == nil || result.Load1 == nil {
		t.Fatal("boot failure suppressed independent native capacity gauges")
	}
	// These remain truthful cumulative measurements, not synthesized rates.
	// SystemResourceOverview requires a nonempty matching boot identity before
	// deriving either CPU percentages or network rates from two snapshots.
	if result.CPUTotal == nil || *result.CPUTotal != 115 || result.CPUIdle == nil || *result.CPUIdle != 80 || result.NetworkReceived == nil || *result.NetworkReceived != 123456789 {
		t.Fatal("boot failure converted raw native counters into fake measurements")
	}
}
