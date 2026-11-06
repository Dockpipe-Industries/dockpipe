package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/telemetry"
)

func TestMachineSnapshotUsesActualMachineIdentityAndAllowlistedCollectors(t *testing.T) {
	systemCalls, dockerCalls := 0, 0
	snapshot, err := readMachineSnapshot(context.Background(), "selected-host", true, true,
		func(ctx context.Context, directory string, system, resources bool) (telemetry.Snapshot, error) {
			systemCalls++
			if directory != "" || !system || !resources {
				t.Fatal("machine inspection must not query project execution history")
			}
			return telemetry.Snapshot{Schema: telemetry.Schema, Machine: "local",
				Processes: []telemetry.Process{{Name: "host-process", Machine: "local"}}}, nil
		},
		func(context.Context) telemetry.DockerSnapshot {
			dockerCalls++
			return telemetry.DockerSnapshot{Available: false, Notices: []string{"Permission denied."}}
		})
	if err != nil || systemCalls != 1 || dockerCalls != 1 {
		t.Fatal("collectors did not complete", err, systemCalls, dockerCalls)
	}
	if snapshot.Node != "selected-host" || snapshot.Execution.Machine != snapshot.Node ||
		snapshot.Execution.Processes[0].Machine != snapshot.Node {
		t.Fatal("snapshot presented a different machine")
	}
	if snapshot.State != "live" || snapshot.Docker.Available || len(snapshot.Docker.Notices) == 0 {
		t.Fatal("Docker permission failure must remain explicitly unavailable")
	}
}

func TestMachineSnapshotCancellationPreventsFurtherCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, err := readMachineSnapshot(ctx, "host", true, true,
		func(context.Context, string, bool, bool) (telemetry.Snapshot, error) {
			cancel()
			return telemetry.Snapshot{}, context.Canceled
		},
		func(context.Context) telemetry.DockerSnapshot {
			t.Fatal("Docker collection ran after cancellation")
			return telemetry.DockerSnapshot{}
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation was not propagated", err)
	}
}

func TestMachineSnapshotCategoryFailureDoesNotExposeUnderlyingDiagnostic(t *testing.T) {
	snapshot, err := readMachineSnapshot(context.Background(), "host", true, false,
		func(context.Context, string, bool, bool) (telemetry.Snapshot, error) {
			return telemetry.Snapshot{}, errors.New("private filesystem diagnostic")
		},
		func(context.Context) telemetry.DockerSnapshot {
			t.Fatal("unrequested Docker collection ran")
			return telemetry.DockerSnapshot{}
		})
	if err != nil || snapshot.Execution != nil || len(snapshot.Notices) == 0 {
		t.Fatal("category failure was not reported", err)
	}
	data, _ := json.Marshal(snapshot)
	if strings.Contains(string(data), "private filesystem diagnostic") {
		t.Fatal("raw collector diagnostic escaped the inspection boundary")
	}
}

func TestMachineSnapshotBoundsProcessAndDockerResponses(t *testing.T) {
	snapshot := telemetry.MachineSnapshot{Schema: telemetry.MachineSchema, State: "live",
		Execution: &telemetry.Snapshot{}, Docker: &telemetry.DockerSnapshot{Available: true,
			Containers: []telemetry.DockerContainer{{Names: strings.Repeat("x", telemetry.MaxMachineSnapshotBytes)}}}}
	for range telemetry.MaxProcesses {
		snapshot.Execution.Processes = append(snapshot.Execution.Processes,
			telemetry.Process{Name: strings.Repeat("p", 512)})
	}
	if err := FitMachineSnapshot(&snapshot); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(snapshot)
	if len(data) > telemetry.MaxMachineSnapshotBytes || !snapshot.Execution.Truncated || snapshot.Docker.Available {
		t.Fatal("response was not bounded with explicit availability")
	}
	snapshot.Notices = []string{strings.Repeat("x", telemetry.MaxMachineSnapshotBytes)}
	if err := FitMachineSnapshot(&snapshot); err == nil {
		t.Fatal("unreducible oversized metadata was accepted")
	}
}
