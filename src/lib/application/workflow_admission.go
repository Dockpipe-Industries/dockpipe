package application

import (
	"fmt"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/hostdependencies"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// preparedWorkflow binds admission to the workflow that will actually execute.
// Setup scripts cannot change the selected config between admission and execution.
type preparedWorkflow struct {
	workflow    *domain.Workflow
	name        string
	root        string
	config      string
	workdir     string
	projectRoot string
}

func preparePackageWorkflow(o *runStepsOpts, step domain.Step, index int) (*preparedWorkflow, error) {
	name := strings.TrimSpace(step.WorkflowName)
	namespace := strings.TrimSpace(step.Package)
	if name == "" {
		return nil, fmt.Errorf("step %s: packaged workflow step requires workflow: <name>", step.DisplayName(index))
	}
	if namespace == "" {
		return nil, fmt.Errorf("step %s: packaged workflow step requires package: <namespace> (must match nested workflow namespace:)", step.DisplayName(index))
	}
	workdir := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	config, err := infrastructure.ResolvePackagedWorkflowConfigPath(o.repoRoot, workdir, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("step %s: %w", step.DisplayName(index), err)
	}
	workflow, err := o.executionServices().loadWorkflow(config)
	if err != nil {
		return nil, fmt.Errorf("step %s: package workflow: %w", step.DisplayName(index), err)
	}
	if len(workflow.Steps) == 0 {
		return nil, fmt.Errorf("step %s: packaged workflow %q has no steps", step.DisplayName(index), name)
	}
	prepared := &preparedWorkflow{
		workflow:    workflow,
		name:        name,
		root:        filepath.Dir(config),
		config:      config,
		workdir:     workdir,
		projectRoot: o.projectRoot,
	}
	if err := prepared.admit(o.opts != nil && o.opts.ApproveSystemChanges); err != nil {
		return nil, fmt.Errorf("step %s: %w", step.DisplayName(index), err)
	}
	return prepared, nil
}

func prepareResolverWorkflow(load func(string) (*domain.Workflow, error), name, repoRoot string, opts *CliOpts) (*preparedWorkflow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("resolver workflow name is empty")
	}
	workdir := effectiveWorkdirForWorkflowOpts(opts)
	projectRoot := workdir
	if absolute, err := filepath.Abs(workdir); err == nil {
		projectRoot = absolute
	}
	config, err := infrastructure.ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir(repoRoot, workdir, name)
	if err != nil {
		return nil, fmt.Errorf("resolver workflow %q: %w", name, err)
	}
	workflow, err := load(config)
	if err != nil {
		return nil, fmt.Errorf("parse resolver workflow %q: %w", name, err)
	}
	if len(workflow.Steps) == 0 {
		return nil, fmt.Errorf("resolver workflow %q has no steps", name)
	}
	prepared := &preparedWorkflow{
		workflow:    workflow,
		name:        name,
		root:        filepath.Dir(config),
		config:      config,
		workdir:     workdir,
		projectRoot: projectRoot,
	}
	if err := prepared.admit(opts != nil && opts.ApproveSystemChanges); err != nil {
		return nil, fmt.Errorf("resolver workflow %q: %w", name, err)
	}
	return prepared, nil
}

func (workflow *preparedWorkflow) admit(approveSystemChanges bool) error {
	if err := domain.ValidateLoadedWorkflow(workflow.workflow); err != nil {
		return err
	}
	return hostDependencyService().Check(hostdependencies.Request{
		Workflow:             workflow.workflow,
		WorkflowRoot:         workflow.root,
		Config:               workflow.config,
		Workdir:              workflow.workdir,
		ApproveSystemChanges: approveSystemChanges,
	})
}
