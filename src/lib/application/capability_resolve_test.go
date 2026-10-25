package application

import (
	"testing"

	"dockpipe/src/lib/domain"
)

func TestWorkflowNeedsDockerReachableResolved_VMHostDelegateSkipsDockerPreflight(t *testing.T) {
	repo := t.TempDir()
	writeVMHostDelegateFixture(t, repo, "fixture-vm")
	wf := &domain.Workflow{
		Runtime:  "vmimage",
		Resolver: "fixture-vm",
		Steps: []domain.Step{{
			Cmd: "echo hi",
		}},
	}
	if WorkflowNeedsDockerReachableResolved(wf, repo, repo) {
		t.Fatal("expected vmimage host-delegate runtime to skip docker preflight")
	}
}

func TestWorkflowNeedsDockerReachableResolved_DockerimageStillNeedsDocker(t *testing.T) {
	repo := t.TempDir()
	wf := &domain.Workflow{
		Runtime: "dockerimage",
		Steps: []domain.Step{{
			Cmd: "echo hi",
		}},
	}
	if !WorkflowNeedsDockerReachableResolved(wf, repo, repo) {
		t.Fatal("expected dockerimage runtime to keep docker preflight")
	}
}
