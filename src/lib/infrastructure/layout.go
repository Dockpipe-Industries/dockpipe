package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultUserWorkflowsDirRel is the default directory (under repo root) for named workflows in normal projects.
// WorkflowsRootDir also supports an explicit directory override.
const DefaultUserWorkflowsDirRel = "workflows"

// workflowsDirRelProcess is set by the CLI for the current process (--workflows-dir); cleared after the command.
var workflowsDirRelProcess string

// SetWorkflowsDirForProcess sets a repo-relative or absolute workflows directory for this process (empty clears).
// Persist across commands with DOCKPIPE_WORKFLOWS_DIR instead.
func SetWorkflowsDirForProcess(rel string) {
	workflowsDirRelProcess = strings.TrimSpace(rel)
	if workflowsDirRelProcess != "" {
		workflowsDirRelProcess = filepath.Clean(workflowsDirRelProcess)
	}
}

func effectiveWorkflowsDirRel() string {
	if workflowsDirRelProcess != "" {
		return workflowsDirRelProcess
	}
	v := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOWS_DIR"))
	if v != "" {
		return filepath.Clean(v)
	}
	return ""
}

// WorkflowsRootDir returns the project's workflow directory, honoring the process
// override and DOCKPIPE_WORKFLOWS_DIR before the default workflows/ directory.
func WorkflowsRootDir(repoRoot string) string {
	rel := effectiveWorkflowsDirRel()
	if rel == "" {
		rel = DefaultUserWorkflowsDirRel
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(repoRoot, rel)
}
