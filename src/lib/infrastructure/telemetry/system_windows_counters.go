package telemetry

import (
	"context"
	"errors"
	"math"
)

// The collection policy is platform independent so failures and counter
// invariants can be exercised without replacing process-global native functions.
type windowsSystemSource interface {
	bootID() (string, error)
	cpu() (logical uint32, kernel, user, idle uint64, err error)
	memory() (total, available uint64, err error)
	disk() (total, available uint64, err error)
	network(context.Context) (received, sent uint64, err error)
	uptime() (seconds float64, err error)
}

func collectWindowsSystem(ctx context.Context, source windowsSystemSource) (SystemResources, error) {
	result := SystemResources{Notices: []string{}}
	steps := []func(){
		func() {
			boot, err := source.bootID()
			if err != nil || boot == "" {
				result.Notices = append(result.Notices, "Windows boot identity unavailable; rate baselines cannot be compared")
				return
			}
			result.BootID = boot
		},
		func() {
			logical, kernel, user, idle, err := source.cpu()
			result.LogicalCPUs = int(logical)
			// GetSystemTimes covers only one processor group on machines with
			// more than 64 logical CPUs. Do not present it as a machine total.
			if err != nil || logical == 0 || logical > 64 || kernel < idle || user > math.MaxUint64-kernel {
				result.Notices = append(result.Notices, "Windows whole-system CPU counters unavailable (requires a single processor group with at most 64 logical CPUs)")
				return
			}
			total := kernel + user // Kernel time already includes idle time.
			result.CPUTotal, result.CPUIdle = &total, &idle
		},
		func() {
			total, available, err := source.memory()
			if err != nil || total == 0 || available > total {
				result.Notices = append(result.Notices, "Windows physical memory counters unavailable")
				return
			}
			result.MemoryTotal, result.MemoryAvailable = &total, &available
		},
		func() {
			total, available, err := source.disk()
			if err != nil || total == 0 || available > total {
				result.Notices = append(result.Notices, "Windows system-volume capacity unavailable")
				return
			}
			result.DiskTotal, result.DiskAvailable = &total, &available
		},
		func() {
			received, sent, err := source.network(ctx)
			if err != nil {
				result.Notices = append(result.Notices, "Windows network counters unavailable")
				return
			}
			result.NetworkReceived, result.NetworkSent = &received, &sent
		},
		func() {
			seconds, err := source.uptime()
			if err != nil || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
				result.Notices = append(result.Notices, "Windows uptime unavailable")
				return
			}
			result.UptimeSeconds = &seconds
		},
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		step()
	}
	// GlobalMemoryStatusEx page-file values describe commit limits, not swap.
	result.Notices = append(result.Notices, "Windows swap usage and Unix load average are not collected")
	return result, ctx.Err()
}

func addWindowsNetworkCounters(received, sent *uint64, nextReceived, nextSent uint64) error {
	if nextReceived > math.MaxUint64-*received || nextSent > math.MaxUint64-*sent {
		return errors.New("network counters overflow")
	}
	*received += nextReceived
	*sent += nextSent
	return nil
}
