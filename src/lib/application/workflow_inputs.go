package application

import (
	"dockpipe/src/lib/application/internal/workflowinputs"
	"dockpipe/src/lib/domain"
)

func resolveWorkflowInputsEnv(wf *domain.Workflow, configPath, projectRoot string, env map[string]string) (map[string]string, error) {
	return workflowinputs.ResolveWorkflow(wf, configPath, projectRoot, env)
}

func resolveStepInputsEnv(wf *domain.Workflow, configPath, projectRoot string, step domain.Step, env map[string]string) (map[string]string, error) {
	return workflowinputs.ResolveStep(wf, configPath, projectRoot, step, env)
}

func resolveWorkflowInputBindingsEnv(wf *domain.Workflow, configPath, projectRoot string, bindings map[string]domain.InputBinding, env map[string]string) (map[string]string, error) {
	return workflowinputs.ResolveBindings(wf, configPath, projectRoot, bindings, env)
}
