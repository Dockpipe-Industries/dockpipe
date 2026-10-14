package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"dockpipe/src/lib/infrastructure/telemetry"
)

func TestContainerMetricsMissingAndMalformedAreNotZero(t *testing.T) {
	metrics, err := parseContainerMetrics("12.5%|1.5MiB / 8GiB\n")
	if err != nil || metrics.CPUPercent == nil || *metrics.CPUPercent != 12.5 || metrics.MemoryBytes == nil || *metrics.MemoryBytes != 1572864 {
		t.Fatalf("%+v %v", metrics, err)
	}
	for _, invalid := range []string{"", "--|--", "NaN%|0B / 0B", "0%|NaN B / 0B", "1%|999999999999999999999GiB / 0B"} {
		if _, err := parseContainerMetrics(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
}

func TestContainerMetricsRefuseUnassociatedAndHistoricalNames(t *testing.T) {
	original := execCommandContextFn
	defer func() { execCommandContextFn = original }()
	execCommandContextFn = func(context.Context, string, ...string) *exec.Cmd { t.Fatal("unassociated Docker command"); return nil }
	snapshot := telemetry.Snapshot{Executions: []telemetry.Execution{{ID: "run", Container: "past-name", State: "succeeded"}}}
	CollectExecutionContainer(context.Background(), &snapshot, "arbitrary")
	CollectExecutionContainer(context.Background(), &snapshot, "past-name")
}

func TestContainerMetricsUseBoundedExistingTransport(t *testing.T) {
	original := execCommandContextFn
	defer func() { execCommandContextFn = original }()
	t.Setenv("DOCKPIPE_TELEMETRY_STATS_FIXTURE", "1")
	execCommandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) != 6 || args[0] != "stats" || args[1] != "--no-stream" || args[4] != "--" || args[5] != "fixture-container" {
			t.Fatal(args)
		}
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestContainerStatsHelper$")
	}
	snapshot := telemetry.Snapshot{Executions: []telemetry.Execution{{ID: "run", Container: "fixture-container", State: "running"}}}
	CollectExecutionContainer(context.Background(), &snapshot, "fixture-container")
	metrics := snapshot.Executions[0].ContainerMetrics
	if len(snapshot.Notices) != 0 || metrics == nil || metrics.CPUPercent == nil || *metrics.CPUPercent != 0 {
		t.Fatalf("%+v", snapshot)
	}
}

func TestContainerStatsHelper(t *testing.T) {
	if os.Getenv("DOCKPIPE_TELEMETRY_STATS_FIXTURE") != "1" {
		return
	}
	fmt.Print("0%|2MiB / 8GiB\n")
	os.Exit(0)
}
