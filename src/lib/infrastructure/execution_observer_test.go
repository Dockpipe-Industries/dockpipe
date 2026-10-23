package infrastructure

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

func TestActiveRuntimeAssociationSurvivesRecentHistoryEviction(t *testing.T) {
	directory := t.TempDir()
	runs := HostRunsDir(directory)
	if err := os.MkdirAll(runs, 0700); err != nil {
		t.Fatal(err)
	}
	identity := telemetry.ProcessIdentity{PID: 42, StartToken: "boot:original"}
	host := HostRunRecord{ID: "a1b2c3d4", PID: identity.PID, ProcessStart: identity.StartToken, Workdir: directory, StartedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	resource := telemetry.Resource{ID: "runtime-resource", Runtime: "fixture", Process: identity}
	for name, value := range map[string]any{"a1b2c3d4.json": host, "a1b2c3d4.resource": resource} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runs, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	recent := make([]telemetry.Execution, telemetry.MaxExecutions)
	for index := range recent {
		recent[index] = telemetry.Execution{ID: "completed", State: "succeeded", StartedAt: time.Now()}
	}
	collector := executionFixtureCollector{process: telemetry.Process{ProcessIdentity: identity}}
	merged, truncated, err := mergeActiveExecutionResources(context.Background(), directory, recent, collector)
	if err != nil || !truncated || len(merged) != telemetry.MaxExecutions || merged[0].ID != host.ID {
		t.Fatalf("active association lost: count=%d truncated=%v err=%v", len(merged), truncated, err)
	}
	collector.process.StartToken = "boot:reused"
	merged, _, err = mergeActiveExecutionResources(context.Background(), directory, recent, collector)
	if err != nil || merged[0].ID == host.ID {
		t.Fatal("reused PID recovered an unrelated execution")
	}
	t.Run("reject linked registry entry", func(t *testing.T) {
		collector.process.StartToken = identity.StartToken
		if err := os.Rename(filepath.Join(runs, "a1b2c3d4.json"), filepath.Join(runs, "original-host-record")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(runs, "original-host-record"), filepath.Join(runs, "a1b2c3d4.json")); err != nil {
			t.Skip("platform does not permit creating test symlinks")
		}
		merged, _, err = mergeActiveExecutionResources(context.Background(), directory, recent, collector)
		if err != nil || merged[0].ID == host.ID {
			t.Fatal("linked registry entry accepted")
		}
	})
}

func TestExecutionLifecycleRapidExitAssociationAndPrivacy(t *testing.T) {
	directory := t.TempDir()
	env := append(os.Environ(), "DOCKPIPE_WORKDIR="+directory, "DOCKPIPE_WORKFLOW_NAME=fixture", "DOCKPIPE_WORKFLOW_RUN_ID=run-a", "DOCKPIPE_STEP_ID=task-b", "DOCKPIPE_SKIP_HOST_CLEANUP=1", "PRIVATE_TOKEN=never-persist-this")
	if err := RunHostCommand("exit 7", env); err == nil {
		t.Fatal("expected exit failure")
	}
	snapshot, err := ReadExecutionSnapshot(context.Background(), directory, false, true)
	if err != nil || len(snapshot.Executions) != 1 {
		t.Fatalf("records: %v %+v", err, snapshot)
	}
	record := snapshot.Executions[0]
	if record.State != "failed" || record.ExitCode == nil || *record.ExitCode != 7 || record.Workflow != "fixture" || record.WorkflowRun != "run-a" || record.Task != "task-b" || record.FinishedAt == nil {
		t.Fatal(record)
	}
	data, _ := json.Marshal(snapshot)
	if strings.Contains(string(data), "never-persist") || strings.Contains(string(data), "exit 7") {
		t.Fatal("private execution data persisted")
	}
}

func TestExecutionDockerAssociationAndStartFailure(t *testing.T) {
	directory := t.TempDir()
	command := exec.Command(filepath.Join(directory, "missing"))
	err := command.Start()
	if err == nil {
		t.Fatal("expected start failure")
	}
	observation := ObserveExecution("docker-test", command, nil, directory, "docker", "exact-container")
	observation.Finish(command, err)
	history, _ := ExecutionHistoryDir(directory)
	records, err := (telemetry.Store{Directory: history}).Read()
	if err != nil || len(records) != 1 {
		t.Fatalf("%v %v", records, err)
	}
	if records[0].Container != "exact-container" || records[0].Reason != "start_failed" {
		t.Fatal(records[0])
	}
}

func TestBeginHostRunReplacesInheritedIdentity(t *testing.T) {
	id, _, env, err := BeginHostRun(t.TempDir(), []string{"DOCKPIPE_RUN_ID=parent", "DOCKPIPE_RUN_FILE=old"})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, "DOCKPIPE_RUN_ID=") {
			count++
		}
	}
	if count != 1 || envGet(env, "DOCKPIPE_RUN_ID") != id {
		t.Fatal(env)
	}
}

func TestExecutionObservabilityCanBeDisabled(t *testing.T) {
	directory := t.TempDir()
	command := exec.Command("not-started")
	observation := ObserveExecution("disabled", command, []string{"DOCKPIPE_OBSERVABILITY=off"}, directory, "host", "")
	observation.Finish(command, nil)
	path, _ := ExecutionHistoryDir(directory)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled observer created history")
	}
}

func TestExecutionUnknownMetricsSerializeAsNull(t *testing.T) {
	data, err := json.Marshal(telemetry.Process{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"memory_bytes":null`) || !strings.Contains(string(data), `"cpu_seconds":null`) {
		t.Fatal(string(data))
	}
}

type executionFixtureCollector struct {
	process telemetry.Process
	err     error
}

func (collector executionFixtureCollector) Inspect(context.Context, int) (telemetry.Process, error) {
	return collector.process, collector.err
}

func (collector executionFixtureCollector) Discover(context.Context) ([]telemetry.Process, bool, error) {
	return nil, false, collector.err
}

func TestExecutionSnapshotNeverAttributesReusedPID(t *testing.T) {
	directory := t.TempDir()
	history, _ := ExecutionHistoryDir(directory)
	record := telemetry.Execution{ID: "fixture", State: "running", Process: telemetry.ProcessIdentity{PID: 42, StartToken: "boot:old"}}
	if err := (telemetry.Store{Directory: history}).Record(record); err != nil {
		t.Fatal(err)
	}
	memory := uint64(1234)
	collector := executionFixtureCollector{process: telemetry.Process{ProcessIdentity: telemetry.ProcessIdentity{PID: 42, StartToken: "boot:new"}, MemoryBytes: &memory}}
	snapshot, err := readExecutionSnapshot(context.Background(), directory, false, true, collector)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Executions[0].State != "unverified" || snapshot.Executions[0].Metrics != nil {
		t.Fatal(snapshot.Executions[0])
	}
	collector.process.StartToken = "boot:old"
	snapshot, err = readExecutionSnapshot(context.Background(), directory, false, true, collector)
	if err != nil || snapshot.Executions[0].Metrics == nil {
		t.Fatalf("matching identity missing metrics: %v", err)
	}
}

func TestExecutionCollectorFailureAndCancellation(t *testing.T) {
	collector := executionFixtureCollector{err: os.ErrPermission}
	snapshot, err := readExecutionSnapshot(context.Background(), t.TempDir(), true, true, collector)
	if err != nil || len(snapshot.Notices) != 1 || len(snapshot.Processes) != 0 {
		t.Fatalf("%+v %v", snapshot, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readExecutionSnapshot(ctx, t.TempDir(), false, false, collector); err != context.Canceled {
		t.Fatal(err)
	}
}
