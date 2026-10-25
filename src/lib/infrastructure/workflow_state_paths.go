package infrastructure

import (
	"fmt"
	"path/filepath"
	"strings"
)

// WorkflowStateDir returns the workflow state directory without creating it.
func WorkflowStateDir(workdir, workflowName string) (string, error) {
	stateDir, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "workflows", SanitizeWorkflowStateScope(workflowName)), nil
}

// WorkflowArtifactRoot returns the default workflow artifact directory without creating it.
func WorkflowArtifactRoot(workdir, workflowName string) (string, error) {
	stateDir, err := WorkflowStateDir(workdir, workflowName)
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "artifacts"), nil
}

// CIArtifactDirs resolves an explicit workflow or package binding to its raw and analysis directories.
// It does not create directories or select a binding from process environment.
func CIArtifactDirs(workdir, binding string) (string, string, error) {
	binding = strings.TrimSpace(binding)
	if strings.HasPrefix(binding, "package:") {
		scope := strings.TrimSpace(strings.TrimPrefix(binding, "package:"))
		if scope == "" {
			return "", "", fmt.Errorf("CI artifact package binding requires a package name")
		}
		root, err := PackageRuntimeDir(workdir, scope)
		if err != nil {
			return "", "", err
		}
		return filepath.Join(root, "ci", "raw"), filepath.Join(root, "ci", "analysis"), nil
	}
	workflowName := binding
	if strings.HasPrefix(binding, "workflow:") {
		workflowName = strings.TrimSpace(strings.TrimPrefix(binding, "workflow:"))
	}
	if workflowName == "" {
		return "", "", fmt.Errorf("CI artifact workflow binding requires a workflow name")
	}
	root, err := WorkflowArtifactRoot(workdir, workflowName)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(root, "ci-raw"), filepath.Join(root, "ci-analysis"), nil
}

// SanitizeWorkflowStateScope preserves the historical workflow scope spelling used by run artifacts.
func SanitizeWorkflowStateScope(scope string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(scope) {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}
