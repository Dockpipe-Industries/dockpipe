// Package telemetry observes executions without owning their supervision.
package telemetry

import (
	"context"
	"dockpipe/src/lib/infrastructure/operationrecord"
	"time"
)

const (
	Schema = "dockpipe.execution_snapshot.v1"
	// The explorer is a recent-activity view, not an audit archive.
	MaxExecutions    = 256
	MaxProcesses     = 4096
	MaxSnapshotBytes = 1024 * 1024
)

// ProcessIdentity includes a kernel start token; a PID alone never proves identity.
type ProcessIdentity struct {
	PID        int    `json:"pid"`
	StartToken string `json:"start_token,omitempty"`
}

type Process struct {
	ProcessIdentity
	ParentPID   int      `json:"parent_pid"`
	Name        string   `json:"name"`
	State       string   `json:"state"`
	Machine     string   `json:"machine"`
	MemoryBytes *uint64  `json:"memory_bytes"`
	CPUSeconds  *float64 `json:"cpu_seconds"`
}

// Resource is an explicit association registered by the owning runtime.
type Resource struct {
	ID      string          `json:"id"`
	Runtime string          `json:"runtime"`
	Process ProcessIdentity `json:"process"`
	Metrics *Process        `json:"metrics,omitempty"`
	State   string          `json:"state"`
}

type ContainerMetrics struct {
	CPUPercent  *float64 `json:"cpu_percent"`
	MemoryBytes *uint64  `json:"memory_bytes"`
}

// Execution deliberately excludes command arguments, environment and raw output.
type Execution struct {
	OperationEvents  []operationrecord.OperationSummary `json:"operation_events,omitempty"`
	LogNotice        string                             `json:"log_notice,omitempty"`
	ContainerMetrics *ContainerMetrics                  `json:"container_metrics,omitempty"`
	Resource         *Resource                          `json:"resource,omitempty"`
	ID               string                             `json:"id"`
	ParentID         string                             `json:"parent_id,omitempty"`
	Workflow         string                             `json:"workflow,omitempty"`
	WorkflowRun      string                             `json:"workflow_run,omitempty"`
	Task             string                             `json:"task,omitempty"`
	Machine          string                             `json:"machine"`
	Runtime          string                             `json:"runtime"`
	Process          ProcessIdentity                    `json:"process"`
	ParentPID        int                                `json:"parent_pid"`
	State            string                             `json:"state"`
	StartedAt        time.Time                          `json:"started_at"`
	FinishedAt       *time.Time                         `json:"finished_at,omitempty"`
	DurationMS       *int64                             `json:"duration_ms,omitempty"`
	ExitCode         *int                               `json:"exit_code,omitempty"`
	Reason           string                             `json:"reason,omitempty"`
	Container        string                             `json:"container,omitempty"`
	Log              string                             `json:"log,omitempty"`
	Metrics          *Process                           `json:"metrics,omitempty"`
}

type Snapshot struct {
	System     *SystemResources `json:"system,omitempty"`
	Schema     string           `json:"schema"`
	Machine    string           `json:"machine"`
	CapturedAt time.Time        `json:"captured_at"`
	Executions []Execution      `json:"executions"`
	Processes  []Process        `json:"processes"`
	Notices    []string         `json:"notices"`
	Truncated  bool             `json:"truncated"`
}

// Collector implementations report unsupported or inaccessible data explicitly.
// Metrics are optional; nil is never converted into a measured zero.
type Collector interface {
	Inspect(context.Context, int) (Process, error)
	Discover(context.Context) ([]Process, bool, error)
}

func SameProcess(left, right ProcessIdentity) bool {
	return left.PID > 0 && left.PID == right.PID && left.StartToken != "" && left.StartToken == right.StartToken
}
