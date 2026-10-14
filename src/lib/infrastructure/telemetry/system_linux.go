//go:build linux

package telemetry

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// System reads bounded kernel counters. Missing individual sources remain null.
// The disk figures describe / in the collector's mount namespace.
func (collector NativeCollector) System(ctx context.Context) (SystemResources, error) {
	result := SystemResources{Notices: []string{}}
	read := func(name string) string {
		if ctx.Err() != nil {
			return ""
		}
		file, err := os.Open(filepath.Join(collector.root(), name))
		if err != nil {
			result.Notices = append(result.Notices, name+" unavailable")
			return ""
		}
		defer file.Close()
		// Covers large CPU/interface inventories without unbounded proc reads.
		data, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
		if err != nil || len(data) > 256*1024 {
			result.Notices = append(result.Notices, name+" unavailable or exceeds 256 KiB")
			return ""
		}
		return string(data)
	}
	result.BootID = strings.TrimSpace(read("sys/kernel/random/boot_id"))
	parseCPU(read("stat"), &result)
	parseMemory(read("meminfo"), &result)
	parseNetwork(read("net/dev"), &result)
	result.UptimeSeconds = firstMetric(read("uptime"))
	result.Load1 = firstMetric(read("loadavg"))
	if err := ctx.Err(); err != nil {
		return result, err
	}
	var disk unix.Statfs_t
	if err := unix.Statfs("/", &disk); err == nil && disk.Bsize > 0 && disk.Blocks <= math.MaxUint64/uint64(disk.Bsize) && disk.Bavail <= disk.Blocks {
		total := disk.Blocks * uint64(disk.Bsize)
		available := disk.Bavail * uint64(disk.Bsize)
		result.DiskTotal, result.DiskAvailable = &total, &available
	} else {
		result.Notices = append(result.Notices, "Root filesystem capacity unavailable")
	}
	return result, ctx.Err()
}

func parseCPU(data string, result *SystemResources) {
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(fields[0], "cpu") && fields[0] != "cpu" {
			if _, err := strconv.Atoi(strings.TrimPrefix(fields[0], "cpu")); err == nil {
				result.LogicalCPUs++
			}
		}
		if fields[0] != "cpu" || len(fields) < 5 {
			continue
		}
		var total, idle uint64
		valid := true
		// guest and guest_nice are already included in user/nice.
		for index := 1; index < len(fields) && index <= 8; index++ {
			value, err := strconv.ParseUint(fields[index], 10, 64)
			if err != nil || value > math.MaxUint64-total {
				valid = false
				break
			}
			total += value
			if index == 4 || index == 5 {
				idle += value
			}
		}
		if valid {
			result.CPUTotal, result.CPUIdle = &total, &idle
		}
	}
}

func parseMemory(data string, result *SystemResources) {
	targets := map[string]**uint64{"MemTotal:": &result.MemoryTotal, "MemAvailable:": &result.MemoryAvailable,
		"SwapTotal:": &result.SwapTotal, "SwapFree:": &result.SwapFree}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "kB" {
			continue
		}
		if target, ok := targets[fields[0]]; ok {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil && value <= math.MaxUint64/1024 {
				value *= 1024
				*target = &value
			}
		}
	}
}

func parseNetwork(data string, result *SystemResources) {
	if !strings.Contains(data, "Receive") || !strings.Contains(data, "Transmit") {
		return
	}
	var received, sent uint64
	for _, line := range strings.Split(data, "\n") {
		name, values, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "lo" {
			continue
		}
		fields := strings.Fields(values)
		if len(fields) != 16 {
			return
		}
		rx, rxErr := strconv.ParseUint(fields[0], 10, 64)
		tx, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr != nil || txErr != nil || rx > math.MaxUint64-received || tx > math.MaxUint64-sent {
			return
		}
		received += rx
		sent += tx
	}
	result.NetworkReceived, result.NetworkSent = &received, &sent
}

func firstMetric(data string) *float64 {
	fields := strings.Fields(data)
	if len(fields) == 0 {
		return nil
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return nil
	}
	return &value
}
