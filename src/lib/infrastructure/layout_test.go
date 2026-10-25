package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowsRootDirIgnoresLegacyLayouts(t *testing.T) {
	t.Setenv("DOCKPIPE_WORKFLOWS_DIR", "")
	root := t.TempDir()
	for _, path := range []string{"src/core/runtimes", "src/core/workflows", "bundle/core", "bundle/workflows"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := WorkflowsRootDir(root), filepath.Join(root, "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
}

func TestWorkflowsRootDirOverridePrecedence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_WORKFLOWS_DIR", "environment-flows")
	if got, want := WorkflowsRootDir(root), filepath.Join(root, "environment-flows"); got != want {
		t.Fatalf("environment override = %q, want %q", got, want)
	}
	SetWorkflowsDirForProcess("process-flows")
	t.Cleanup(func() { SetWorkflowsDirForProcess("") })
	if got, want := WorkflowsRootDir(root), filepath.Join(root, "process-flows"); got != want {
		t.Fatalf("process override = %q, want %q", got, want)
	}
	absolute := t.TempDir()
	SetWorkflowsDirForProcess(absolute)
	if got := WorkflowsRootDir(root); got != absolute {
		t.Fatalf("absolute override = %q, want %q", got, absolute)
	}
}
