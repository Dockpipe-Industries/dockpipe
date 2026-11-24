package telemetry

import (
	"context"
	"math"
)

// Keep counter validation independent of native calls so partial failure,
// cancellation and arithmetic can be verified on every development platform.
type darwinSystemSource interface {
	boot() (identity string, uptime float64, err error)
	cpu() (logical uint32, ticks [4]uint32, err error)
	memory() (total, freePages, inactivePages, pageSize uint64, err error)
	swap() (total, available uint64, err error)
	disk() (total, available uint64, err error)
	network(context.Context) (received, sent uint64, err error)
	load() (float64, error)
}

func collectDarwinSystem(ctx context.Context, source darwinSystemSource) (SystemResources, error) {
	result := SystemResources{Notices: []string{}}
	steps := []func(){
		func() {
			identity, uptime, err := source.boot()
			if err != nil || identity == "" || !validDarwinGauge(uptime) {
				result.Notices = append(result.Notices, "Darwin boot identity or uptime unavailable; rate baselines cannot be compared")
				return
			}
			result.BootID, result.UptimeSeconds = identity, &uptime
		},
		func() {
			logical, ticks, err := source.cpu()
			if err != nil || logical == 0 {
				result.Notices = append(result.Notices, "Darwin cumulative CPU counters unavailable")
				return
			}
			// HOST_CPU_LOAD_INFO reports USER, SYSTEM, IDLE, NICE ticks. These
			// are cumulative raw counters, not a percentage or elapsed wall time.
			// Kernel 32-bit counter wraps produce a sample decrease; consumers
			// must reset/skip that interval, just as for any counter reset.
			var total uint64
			for _, value := range ticks {
				total += uint64(value)
			}
			idle := uint64(ticks[2])
			result.LogicalCPUs = int(logical)
			result.CPUTotal, result.CPUIdle = &total, &idle
		},
		func() {
			total, free, inactive, page, err := source.memory()
			if err != nil || total == 0 || page == 0 || free > math.MaxUint64-inactive || free+inactive > math.MaxUint64/page {
				result.Notices = append(result.Notices, "Darwin physical memory counters unavailable")
				return
			}
			// Conservative reclaimable-memory estimate: free plus inactive
			// pages. Apple already includes speculative pages in free_count;
			// adding speculative/purgeable pages again can double-count them.
			available := (free + inactive) * page
			if available > total {
				result.Notices = append(result.Notices, "Darwin physical memory counters are inconsistent")
				return
			}
			result.MemoryTotal, result.MemoryAvailable = &total, &available
		},
		func() {
			total, available, err := source.swap()
			if err != nil || available > total {
				result.Notices = append(result.Notices, "Darwin swap counters unavailable")
				return
			}
			result.SwapTotal, result.SwapFree = &total, &available
		},
		func() {
			total, available, err := source.disk()
			if err != nil || total == 0 || available > total {
				result.Notices = append(result.Notices, "Darwin root-volume capacity unavailable")
				return
			}
			result.DiskTotal, result.DiskAvailable = &total, &available
		},
		func() {
			received, sent, err := source.network(ctx)
			if err != nil {
				result.Notices = append(result.Notices, "Darwin network counters unavailable or exceed their inspection budget")
				return
			}
			result.NetworkReceived, result.NetworkSent = &received, &sent
		},
		func() {
			load, err := source.load()
			if err != nil || !validDarwinGauge(load) {
				result.Notices = append(result.Notices, "Darwin load average unavailable")
				return
			}
			result.Load1 = &load
		},
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		step()
	}
	return result, ctx.Err()
}

func validDarwinGauge(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
