package infrastructure

import (
	"fmt"
	"path/filepath"
	"strings"
)

// WorkflowCompileSource preserves the selected config identity alongside its
// materialized directory. An installed archive is not authored source to rebuild.
type WorkflowCompileSource struct {
	Directory string
	Config    string
}

func ResolveWorkflowCompileSource(repoRoot, projectRoot, workflowName string) (WorkflowCompileSource, error) {
	workflowName = strings.TrimSpace(workflowName)
	if workflowName == "" {
		return WorkflowCompileSource{}, fmt.Errorf("workflow name is empty")
	}
	config := OnDiskWorkflowConfigPath(projectRoot, workflowName)
	if config == "" {
		var err error
		config, err = ResolveWorkflowConfigPathWithWorkdir(repoRoot, projectRoot, workflowName)
		if err != nil {
			return WorkflowCompileSource{}, err
		}
	}
	onDiskConfig, err := WorkflowConfigOnDiskPath(projectRoot, config)
	if err != nil {
		return WorkflowCompileSource{}, err
	}
	return WorkflowCompileSource{Directory: filepath.Dir(onDiskConfig), Config: config}, nil
}
