package infrastructure

import (
	"bytes"
	"context"
	"dockpipe/src/lib/infrastructure/process"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

// boundedTelemetryOutput limits diagnostic subprocess output without retaining
// arguments, environment values or raw error text in the snapshot.
type boundedTelemetryOutput struct{ bytes.Buffer }

func (output *boundedTelemetryOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > 8192 {
		return 0, errors.New("telemetry output limit")
	}
	return output.Buffer.Write(data)
}

// CollectExecutionContainer reuses the configured Docker CLI/transport. Only a
// resource already associated by an execution boundary may be sampled.
func CollectExecutionContainer(ctx context.Context, snapshot *telemetry.Snapshot, container string) {
	for index := range snapshot.Executions {
		record := &snapshot.Executions[index]
		if record.Container != container {
			continue
		}
		// A historical generated name is not a stable resource identity. Detached
		// runs carry Docker's full immutable ID; active runs retain their live owner.
		if record.State != "running" && !isFullContainerID(container) {
			return
		}
		commandName := dockerCommandName()
		command := execCommandContextFn(ctx, commandName, "stats", "--no-stream", "--format", "{{.CPUPerc}}|{{.MemUsage}}", "--", container)
		command.Env = dockerCommandEnv(os.Environ(), commandName)
		output := &boundedTelemetryOutput{}
		command.Stdout = output
		command.Stderr = io.Discard
		command.WaitDelay = 500 * time.Millisecond
		if err := process.Run(command); err != nil {
			snapshot.Notices = append(snapshot.Notices, "Selected Docker resource metrics unavailable.")
			return
		}
		metrics, err := parseContainerMetrics(output.String())
		if err != nil {
			snapshot.Notices = append(snapshot.Notices, "Selected Docker resource metrics unavailable.")
			return
		}
		record.ContainerMetrics = &metrics
		return
	}
}

func isFullContainerID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func parseContainerMetrics(output string) (telemetry.ContainerMetrics, error) {
	cpu, memory, ok := strings.Cut(strings.TrimSpace(output), "|")
	if !ok {
		return telemetry.ContainerMetrics{}, errors.New("invalid Docker metrics")
	}
	percent, err := strconv.ParseFloat(strings.TrimSuffix(cpu, "%"), 64)
	if err != nil || percent < 0 || math.IsNaN(percent) || math.IsInf(percent, 0) {
		return telemetry.ContainerMetrics{}, errors.New("invalid Docker CPU metric")
	}
	used, _, ok := strings.Cut(memory, "/")
	if !ok {
		return telemetry.ContainerMetrics{}, errors.New("invalid Docker memory metric")
	}
	used = strings.TrimSpace(used)
	var memoryBytes *uint64
	for _, unit := range []struct {
		suffix     string
		multiplier float64
	}{
		{"GiB", 1024 * 1024 * 1024}, {"MiB", 1024 * 1024}, {"KiB", 1024}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1},
	} {
		if !strings.HasSuffix(used, unit.suffix) {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(used, unit.suffix)), 64)
		if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) || value*unit.multiplier >= float64(^uint64(0)) {
			break
		}
		measured := uint64(value * unit.multiplier)
		memoryBytes = &measured
		break
	}
	if memoryBytes == nil {
		return telemetry.ContainerMetrics{}, errors.New("invalid Docker memory metric")
	}
	return telemetry.ContainerMetrics{CPUPercent: &percent, MemoryBytes: memoryBytes}, nil
}
