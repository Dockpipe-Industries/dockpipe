package remote

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

func TestMonitoringAuthorizationFreshnessAndRevocation(t *testing.T) {
	_, broker, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "worker")
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	ctx := context.Background()
	request := map[string]string{"node": "worker"}
	var result MachineSnapshot
	if err := admin.Call(ctx, "/v1/observe", request, &result); err != nil || result.State != "unavailable" {
		t.Fatal("unpublished machine looked live", err)
	}
	snapshot := MachineSnapshot{Schema: MonitoringSchema, Node: "worker", State: "live",
		Execution: &telemetry.Snapshot{Schema: telemetry.Schema, Executions: []telemetry.Execution{}, Processes: []telemetry.Process{}, CapturedAt: time.Now().UTC()}}
	if err := worker.Call(ctx, "/v1/monitor-publish", snapshot, nil); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(ctx, "/v1/observe", request, &result); err != nil || result.State != "live" || result.Execution.Machine != "worker" {
		t.Fatal("published sample unavailable", err)
	}
	if err := worker.Call(ctx, "/v1/observe", request, &result); err == nil {
		t.Fatal("worker read operator telemetry")
	}
	snapshot.Node = "different"
	if err := worker.Call(ctx, "/v1/monitor-publish", snapshot, nil); err == nil {
		t.Fatal("worker impersonated another machine")
	}
	broker.mutex.Lock()
	stale := broker.monitoring["worker"]
	stale.ReceivedAt = time.Now().Add(-MonitoringFreshness - time.Second)
	broker.monitoring["worker"] = stale
	broker.mutex.Unlock()
	result = MachineSnapshot{}
	if err := admin.Call(ctx, "/v1/observe", request, &result); err != nil || result.State != "stale" || result.Execution != nil {
		t.Fatal("stale data returned as live", err)
	}
	snapshot.Node = "worker"
	snapshot.State = "disabled"
	if err := worker.Call(ctx, "/v1/monitor-publish", snapshot, nil); err != nil {
		t.Fatal(err)
	}
	result = MachineSnapshot{}
	if err := admin.Call(ctx, "/v1/observe", request, &result); err != nil || result.State != "disabled" || result.Execution != nil {
		t.Fatal("disabled monitoring retained metrics", err)
	}
	if err := admin.Call(ctx, "/v1/revoke", map[string]string{"id": "worker"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(ctx, "/v1/monitor-publish", snapshot, nil); err == nil {
		t.Fatal("revoked worker published telemetry")
	}
	if err := admin.Call(ctx, "/v1/observe", request, &result); err == nil {
		t.Fatal("revoked machine telemetry remained readable")
	}
}

func TestMonitoringPermissionAndWorkerLifetime(t *testing.T) {
	root, _, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "worker")
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	if err := WritePrivate(filepath.Join(root, "monitoring.json"), MonitoringPermission{System: true}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunMonitoring(ctx, root, "worker", worker)
		close(done)
	}()
	defer cancel()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var snapshot MachineSnapshot
		if err := admin.Call(ctx, "/v1/observe", map[string]string{"node": "worker"}, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Execution != nil {
			if snapshot.Docker != nil || len(snapshot.Execution.Executions) != 0 {
				t.Fatal("system permission shared unrelated data")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker never published its snapshot")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker monitoring leaked past cancellation")
	}
	disabled := collectMachine(context.Background(), MonitoringPermission{})
	if disabled.State != "disabled" || disabled.Execution != nil || disabled.Docker != nil {
		t.Fatal("unapproved collector ran")
	}
}

func TestMonitoringSnapshotByteBudget(t *testing.T) {
	snapshot := MachineSnapshot{Schema: MonitoringSchema, State: "live", Execution: &telemetry.Snapshot{}}
	for range telemetry.MaxProcesses {
		snapshot.Execution.Processes = append(snapshot.Execution.Processes, telemetry.Process{Name: strings.Repeat("x", 512)})
	}
	if err := fitMachineSnapshot(&snapshot); err != nil || !snapshot.Execution.Truncated {
		t.Fatal("oversized process list was not bounded", err)
	}
	data, _ := json.Marshal(snapshot)
	if len(data) > MaxMonitoringBytes {
		t.Fatal("snapshot exceeds byte budget")
	}
}
