package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

// ExecutionHistoryDir shares the existing project run store; standalone callers
// without a project use the existing per-user data location.
func ExecutionHistoryDir(workdir string) (string, error) {
	if workdir != "" {
		absolute, err := filepath.Abs(workdir)
		if err != nil {
			return "", err
		}
		return filepath.Join(HostRunsDir(absolute), "executions"), nil
	}
	root, err := GlobalDockpipeDataDir()
	return filepath.Join(root, "runs", "executions"), err
}

// NewWorkflowRunID uses the same random ID source as the host-run registry.
func NewWorkflowRunID() (string, error) { return randomRunID() }

type ExecutionObservation struct {
	recorder *telemetry.Recorder
	record   telemetry.Execution
	store    telemetry.Store
	enabled  bool
}

// ObserveExecution records metadata only; supervision remains with the caller.
// Observation failures never change the execution's exit status.
func ObserveExecution(id string, command *exec.Cmd, env []string, workdir, runtimeName, container string) *ExecutionObservation {
	observation := &ExecutionObservation{}
	if envGet(env, "DOCKPIPE_OBSERVABILITY") == "off" {
		return observation
	}
	directory, err := ExecutionHistoryDir(workdir)
	if err != nil {
		return observation
	}
	if id == "" {
		id, err = randomRunID()
		if err != nil {
			return observation
		}
	}
	observation.store = telemetry.Store{Directory: directory}
	observation.recorder, err = telemetry.NewRecorder(2, observation.store.RecordContext)
	if err != nil {
		return observation
	}
	observation.enabled = true
	task := envGet(env, "DOCKPIPE_STEP_ID")
	if task == "" {
		task = envGet(env, "DOCKPIPE_STEP_INDEX")
	}
	observation.record = telemetry.Execution{
		ID: id, ParentID: envGet(env, "DOCKPIPE_PARENT_EXECUTION_ID"),
		Workflow: envGet(env, "DOCKPIPE_WORKFLOW_NAME"), WorkflowRun: envGet(env, "DOCKPIPE_WORKFLOW_RUN_ID"),
		Task: task, Machine: "local", Runtime: runtimeName, Container: container,
		ParentPID: os.Getpid(), State: "running", StartedAt: time.Now().UTC(),
		Log: envGet(env, EnvDockpipeEventLog),
	}
	if command.Process != nil {
		observation.record.Process.PID = command.Process.Pid
		if process, err := (telemetry.NativeCollector{}).Inspect(context.Background(), command.Process.Pid); err == nil {
			observation.record.Process = process.ProcessIdentity
		}
	}
	observation.persist()
	return observation
}

func (observation *ExecutionObservation) persist() {
	if !observation.enabled {
		return
	}
	if !observation.recorder.Submit(observation.record) {
		// Never include arbitrary filesystem error strings, command text or env.
		fmt.Fprintln(os.Stderr, "[dockpipe] Execution history unavailable; execution continues.")
	}
}

func (observation *ExecutionObservation) Finish(command *exec.Cmd, err error) {
	if !observation.enabled {
		return
	}
	finished := time.Now().UTC()
	duration := finished.Sub(observation.record.StartedAt).Milliseconds()
	observation.record.FinishedAt = &finished
	observation.record.DurationMS = &duration
	observation.record.State = "succeeded"
	if command.ProcessState != nil {
		code := command.ProcessState.ExitCode()
		observation.record.ExitCode = &code
	}
	if err != nil {
		observation.record.State = "failed"
		observation.record.Reason = "process_failed"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			observation.record.State = "cancelled"
			observation.record.Reason = "context_cancelled"
		} else if command.ProcessState == nil {
			observation.record.Reason = "start_failed"
		} else if command.ProcessState.ExitCode() < 0 {
			observation.record.State = "terminated"
			observation.record.Reason = "process_signaled"
		}
	}
	observation.persist()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := observation.recorder.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "[dockpipe] Execution history incomplete; execution result is unchanged.")
	}
}

func ReadExecutionSnapshot(ctx context.Context, workdir string, system, resources bool) (telemetry.Snapshot, error) {
	snapshot, err := readExecutionSnapshot(ctx, workdir, system, resources, telemetry.NativeCollector{})
	if err == nil && resources {
		metrics, collectErr := (telemetry.NativeCollector{}).System(ctx)
		snapshot.System = &metrics
		err = collectErr
	}
	return snapshot, err
}

func readExecutionSnapshot(ctx context.Context, workdir string, system, resources bool, collector telemetry.Collector) (telemetry.Snapshot, error) {
	snapshot := telemetry.Snapshot{Schema: telemetry.Schema, Machine: "local", CapturedAt: time.Now().UTC(),
		Executions: []telemetry.Execution{}, Processes: []telemetry.Process{}, Notices: []string{}}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	if system {
		processes, truncated, err := collector.Discover(ctx)
		if processes != nil {
			snapshot.Processes = processes
			if !resources {
				for index := range snapshot.Processes {
					snapshot.Processes[index].MemoryBytes = nil
					snapshot.Processes[index].CPUSeconds = nil
				}
			}
		}
		snapshot.Truncated = truncated
		if err != nil {
			if ctx.Err() != nil {
				return snapshot, ctx.Err()
			}
			snapshot.Notices = append(snapshot.Notices, "System discovery is unavailable or incomplete on this machine.")
		}
		return snapshot, nil
	}
	directory, err := ExecutionHistoryDir(workdir)
	if err != nil {
		return snapshot, err
	}
	snapshot.Executions, err = (telemetry.Store{Directory: directory}).Read()
	if err != nil {
		return snapshot, fmt.Errorf("execution history unavailable")
	}
	for index := range snapshot.Executions {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		record := &snapshot.Executions[index]
		if record.State != "running" {
			continue
		}
		process, err := collector.Inspect(ctx, record.Process.PID)
		if err != nil {
			record.State = "unverified"
			record.Reason = "process_exited_or_inaccessible"
			continue
		}
		if !telemetry.SameProcess(record.Process, process.ProcessIdentity) {
			record.State = "unverified"
			record.Reason = "process_identity_unavailable_or_changed"
			continue
		}
		if resources {
			record.Metrics = &process
		}
		if resource := readExecutionResource(workdir, record.ID); resource != nil {
			resource.State = "unverified"
			if child, err := collector.Inspect(ctx, resource.Process.PID); err == nil && telemetry.SameProcess(resource.Process, child.ProcessIdentity) {
				resource.State = "running"
				if resources {
					resource.Metrics = &child
				}
			}
			record.Resource = resource
		}
		// Existing runtime-owned sidecars explicitly associate resources; process
		// names and unrelated Docker labels are never used to manufacture ownership.
		if workdir != "" && isValidHostRunID(record.ID) {
			runFile := filepath.Join(HostRunsDir(workdir), record.ID+".json")
			if container := ReadHostRunContainerSidecar(runFile); container != "" {
				record.Container = container
			}
		}
	}
	if len(snapshot.Executions) == telemetry.MaxExecutions {
		snapshot.Truncated = true
		snapshot.Notices = append(snapshot.Notices, "Showing the most recent "+strconv.Itoa(telemetry.MaxExecutions)+" executions.")
	}
	return snapshot, nil
}

// ExecutionMetadataEnv copies only the public association context. It does not
// carry resolver credentials or other execution environment values to telemetry.
func ExecutionMetadataEnv(values map[string]string) []string {
	result := make([]string, 0, 8)
	for _, key := range []string{"DOCKPIPE_OBSERVABILITY", "DOCKPIPE_PARENT_EXECUTION_ID", "DOCKPIPE_WORKFLOW_NAME", "DOCKPIPE_WORKFLOW_RUN_ID", "DOCKPIPE_STEP_ID", "DOCKPIPE_STEP_INDEX", EnvDockpipeEventLog} {
		if value, exists := values[key]; exists {
			result = append(result, key+"="+value)
		}
	}
	return result
}
