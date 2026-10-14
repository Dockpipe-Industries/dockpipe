package remote

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/telemetry"
)

const MonitoringSchema = "dockpipe.machine_snapshot.v1"
const MaxMonitoringBytes = 1 << 20
const MonitoringFreshness = 30 * time.Second

// MonitoringPermission is worker-local opt-in, independent of workflow delivery.
// The broker cannot enable it, request paths, or execute diagnostic commands.
type MonitoringPermission struct {
	System bool `json:"system"`
	Docker bool `json:"docker"`
}

type MachineSnapshot struct {
	Schema     string                    `json:"schema"`
	Node       string                    `json:"node"`
	ReceivedAt time.Time                 `json:"received_at"`
	State      string                    `json:"state"`
	Execution  *telemetry.Snapshot       `json:"execution,omitempty"`
	Docker     *telemetry.DockerSnapshot `json:"docker,omitempty"`
}

// RunMonitoring belongs to the worker lifetime and operates independently of
// its job loop. Each collection/publication is bounded; failures never cancel jobs.
func RunMonitoring(ctx context.Context, root, node string, client *Client) {
	for ctx.Err() == nil {
		var permission MonitoringPermission
		err := ReadPrivate(filepath.Join(root, "monitoring.json"), &permission)
		if err == nil {
			sampleContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			snapshot := collectMachine(sampleContext, permission)
			cancel()
			if ctx.Err() != nil {
				return
			}
			snapshot.Node = node
			if fitMachineSnapshot(&snapshot) == nil {
				publishContext, stop := context.WithTimeout(ctx, 4*time.Second)
				// The next sample replaces this one. Never retry an old sample.
				_ = client.Call(publishContext, "/v1/monitor-publish", snapshot, nil)
				stop()
			}
		}
		// Missing or invalid local permission fails closed; old data expires.
		if !pause(ctx, 4*time.Second) {
			return
		}
	}
}

func collectMachine(ctx context.Context, permission MonitoringPermission) MachineSnapshot {
	result := MachineSnapshot{Schema: MonitoringSchema, State: "disabled"}
	if permission.System {
		snapshot, err := infrastructure.ReadExecutionSnapshot(ctx, "", true, true)
		if err == nil {
			result.Execution = &snapshot
		}
	}
	if permission.Docker && ctx.Err() == nil {
		snapshot := infrastructure.CollectDockerInventory(ctx)
		result.Docker = &snapshot
	}
	if permission.System || permission.Docker {
		result.State = "live"
	}
	return result
}

func fitMachineSnapshot(snapshot *MachineSnapshot) error {
	for {
		data, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if len(data) <= MaxMonitoringBytes {
			return nil
		}
		if snapshot.Execution != nil && len(snapshot.Execution.Processes) > 0 {
			snapshot.Execution.Processes = snapshot.Execution.Processes[:len(snapshot.Execution.Processes)/2]
			snapshot.Execution.Truncated = true
			continue
		}
		if snapshot.Docker != nil && snapshot.Docker.Available {
			snapshot.Docker = &telemetry.DockerSnapshot{Notices: []string{"Docker inventory exceeds the 1 MiB remote snapshot limit."}}
			continue
		}
		return errors.New("machine snapshot exceeds 1 MiB")
	}
}

func (b *Broker) publishMonitoring(node string, raw []byte) (any, error) {
	var snapshot MachineSnapshot
	if len(raw) > MaxMonitoringBytes || Decode(raw, &snapshot) != nil || snapshot.Schema != MonitoringSchema || snapshot.Node != node {
		return nil, errors.New("invalid machine snapshot")
	}
	if snapshot.State != "live" && snapshot.State != "disabled" {
		return nil, errors.New("invalid monitoring state")
	}
	if snapshot.Execution != nil {
		execution := snapshot.Execution
		if execution.Schema != telemetry.Schema || len(execution.Processes) > telemetry.MaxProcesses || len(execution.Executions) != 0 {
			return nil, errors.New("invalid process inventory")
		}
		execution.Machine = node
		for index := range execution.Processes {
			execution.Processes[index].Machine = node
		}
	}
	if snapshot.Docker != nil {
		docker := snapshot.Docker
		if len(docker.Containers) > telemetry.MaxDockerObjects || len(docker.Networks) > telemetry.MaxDockerObjects || len(docker.Volumes) > telemetry.MaxDockerObjects || len(docker.Topology) > telemetry.MaxDockerObjects {
			return nil, errors.New("invalid Docker inventory")
		}
	}
	if snapshot.State == "disabled" {
		snapshot.Execution, snapshot.Docker = nil, nil
	}
	snapshot.ReceivedAt = time.Now().UTC()
	if err := fitMachineSnapshot(&snapshot); err != nil {
		return nil, err
	}
	if b.monitoring == nil {
		b.monitoring = map[string]MachineSnapshot{}
	}
	// Enrollments already have a 64-node limit; retain one <=1 MiB sample/node.
	b.monitoring[node] = snapshot
	return map[string]bool{"accepted": true}, nil
}

func (b *Broker) observeMachine(raw []byte) (any, error) {
	var request struct {
		Node string `json:"node"`
	}
	if err := Decode(raw, &request); err != nil {
		return nil, errors.New("invalid monitoring request")
	}
	enrollment, ok := b.state.Nodes[request.Node]
	if !ok || enrollment.Revoked || enrollment.TokenHash == "" {
		return nil, errors.New("machine is not paired")
	}
	snapshot, ok := b.monitoring[request.Node]
	if !ok {
		return MachineSnapshot{Schema: MonitoringSchema, Node: request.Node, State: "unavailable"}, nil
	}
	if time.Since(snapshot.ReceivedAt) > MonitoringFreshness {
		// Do not deliver cached metrics as current or retain stale private data.
		delete(b.monitoring, request.Node)
		return MachineSnapshot{Schema: MonitoringSchema, Node: request.Node, ReceivedAt: snapshot.ReceivedAt, State: "stale"}, nil
	}
	return snapshot, nil
}
