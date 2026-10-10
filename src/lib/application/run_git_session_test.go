package application

import (
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func TestCheckpointWorkflowGitSessionManualAndInvalidModes(t *testing.T) {
	session := &infrastructure.GitSession{SessionID: "session-test"}
	wf := &domain.Workflow{
		Workspace: domain.WorkflowWorkspaceConfig{
			Lifecycle: domain.WorkflowWorkspaceLifecycleConfig{Checkpoint: "manual"},
		},
	}
	if err := checkpointWorkflowGitSession(session, wf); err != nil {
		t.Fatalf("manual checkpoint: %v", err)
	}

	wf.Workspace.Lifecycle.Checkpoint = "invalid"
	err := checkpointWorkflowGitSession(session, wf)
	if err == nil || !strings.Contains(err.Error(), "workspace.lifecycle.checkpoint must be manual, auto, or step") {
		t.Fatalf("invalid checkpoint mode error = %v", err)
	}
}
