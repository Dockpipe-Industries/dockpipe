package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

// ReadMachineSnapshot reports only the machine executing this process. Choosing
// and authenticating a remote transport belongs to the caller.
func ReadMachineSnapshot(ctx context.Context, system, docker bool) (telemetry.MachineSnapshot, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return telemetry.MachineSnapshot{}, err
	}
	return readMachineSnapshot(ctx, hostname, system, docker, ReadExecutionSnapshot, CollectDockerInventory)
}

func readMachineSnapshot(ctx context.Context, hostname string, system, docker bool,
	readSystem func(context.Context, string, bool, bool) (telemetry.Snapshot, error),
	readDocker func(context.Context) telemetry.DockerSnapshot,
) (telemetry.MachineSnapshot, error) {
	snapshot := telemetry.MachineSnapshot{
		Schema: telemetry.MachineSchema, Node: hostname, State: "disabled",
		ReceivedAt: time.Now().UTC(),
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	if system {
		execution, err := readSystem(ctx, "", true, true)
		if err != nil {
			if ctx.Err() != nil {
				return snapshot, ctx.Err()
			}
			snapshot.Notices = append(snapshot.Notices, "System and process monitoring is unavailable on this machine.")
		} else {
			execution.Machine = hostname
			for index := range execution.Processes {
				execution.Processes[index].Machine = hostname
			}
			snapshot.Execution = &execution
		}
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	if docker {
		inventory := readDocker(ctx)
		snapshot.Docker = &inventory
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	if system || docker {
		snapshot.State = "live"
	}
	if err := FitMachineSnapshot(&snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

// FitMachineSnapshot retains the existing monitoring contract: oversized process
// lists are explicitly truncated, and oversized Docker inventory is unavailable.
func FitMachineSnapshot(snapshot *telemetry.MachineSnapshot) error {
	for {
		data, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if len(data) < telemetry.MaxMachineSnapshotBytes {
			return nil
		}
		if snapshot.Execution != nil && len(snapshot.Execution.Processes) > 0 {
			snapshot.Execution.Processes = snapshot.Execution.Processes[:len(snapshot.Execution.Processes)/2]
			snapshot.Execution.Truncated = true
			continue
		}
		if snapshot.Docker != nil && snapshot.Docker.Available {
			snapshot.Docker = &telemetry.DockerSnapshot{
				Notices: []string{"Docker inventory exceeds the 1 MiB machine snapshot limit."},
			}
			continue
		}
		return errors.New("machine snapshot exceeds 1 MiB")
	}
}
