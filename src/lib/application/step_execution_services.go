package application

import (
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// stepExecutionServices captures execution adapters for one runner invocation.
// Nested workflows and finally steps inherit it; global seams are only defaults
// for compatibility callers and tests, not the running invocation's dependencies.
type stepExecutionServices struct {
	dockerBuild             func(string, string, string) error
	dockerImageExists       func(string) (bool, error)
	dockerPull              func(string) error
	composeLifecycle        func(infrastructure.ComposeLifecycleOpts) error
	runContainer            func(infrastructure.RunOpts, []string) (int, error)
	sourceHostScript        func(string, []string) (map[string]string, error)
	runHostScript           func(string, []string) error
	runHostCommand          func(string, []string) error
	runHostScriptWithShell  func(string, []string, string) error
	runHostCommandWithShell func(string, []string, string) error
	stat                    func(string) (os.FileInfo, error)
	getwd                   func() (string, error)
	loadWorkflow            func(string) (*domain.Workflow, error)
}

func snapshotStepExecutionServices() *stepExecutionServices {
	return &stepExecutionServices{
		dockerBuild:             dockerBuildFn,
		dockerImageExists:       dockerImageExistsFn,
		dockerPull:              dockerPullFn,
		composeLifecycle:        composeLifecycleFn,
		runContainer:            runContainerFn,
		sourceHostScript:        sourceHostScriptFn,
		runHostScript:           runHostScriptFn,
		runHostCommand:          runHostCommandFn,
		runHostScriptWithShell:  runHostScriptWithShellFn,
		runHostCommandWithShell: runHostCommandWithShellFn,
		stat:                    osStatFn,
		getwd:                   getwdFn,
		loadWorkflow:            loadWorkflowAppFn,
	}
}

func (options *runStepsOpts) executionServices() *stepExecutionServices {
	if options.services != nil {
		return options.services
	}
	return snapshotStepExecutionServices()
}

// withExecutionServices also admits direct helper callers without mutating their
// options. Normal runner calls already carry their invocation's service instance.
func (options *runStepsOpts) withExecutionServices() *runStepsOpts {
	if options.services != nil {
		return options
	}
	invocation := *options
	invocation.services = snapshotStepExecutionServices()
	return &invocation
}

func (services *stepExecutionServices) runHostStepScript(shell, path string, env []string) error {
	if strings.TrimSpace(shell) == "" {
		return services.runHostScript(path, env)
	}
	return services.runHostScriptWithShell(path, env, shell)
}

func (services *stepExecutionServices) runHostStepCommand(shell, command string, env []string) error {
	if strings.TrimSpace(shell) == "" {
		return services.runHostCommand(command, env)
	}
	return services.runHostCommandWithShell(command, env, shell)
}

func (services *stepExecutionServices) workingDirectory() string {
	directory, _ := services.getwd()
	return directory
}

func (services *stepExecutionServices) ensureRegistryImage(manifest *domain.CompiledRuntimeManifest) (string, error) {
	return ensureCompiledRegistryImage(manifest.Image, manifest.Security.Network, services.dockerImageExists, services.dockerPull)
}

func (services *stepExecutionServices) skipDockerBuild(stateWorkdir, repoRoot, config, workflowRoot, stepID, policyFingerprint, image, buildDirectory, buildContext string) (bool, string, error) {
	return maybeSkipDockerBuildForArtifact(stateWorkdir, repoRoot, config, workflowRoot, stepID, policyFingerprint, image, buildDirectory, buildContext, services.dockerImageExists)
}
