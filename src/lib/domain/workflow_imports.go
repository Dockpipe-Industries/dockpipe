package domain

import "dockpipe/src/lib/domain/workflow"

func ParseWorkflowFromDisk(data []byte, baseDir string, readFile func(string) ([]byte, error)) (*Workflow, error) {
	return workflow.ParseWorkflowFromDisk(data, baseDir, readFile)
}
