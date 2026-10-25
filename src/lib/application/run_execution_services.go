package application

import (
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// runExecutionServices captures execution effects at the invocation boundary.
// Legacy package seams remain composition adapters for compatibility tests;
// running sessions and their nested steps use their captured dependencies.
type runExecutionServices struct {
	sourceScript func(string, []string) (map[string]string, error)
	hostScript   func(string, []string) error
	container    func(infrastructure.RunOpts, []string) (int, error)
	buildImage   func(string, string, string) error
	runSteps     func(runStepsOpts) error
	steps        *stepExecutionServices
	finalize     func(*infrastructure.GitSession, *domain.Workflow) error
	commit       func(string, string, string, bool) error
}

func snapshotRunExecutionServices() runExecutionServices {
	return runExecutionServices{
		sourceScript: sourceHostScriptAppFn,
		hostScript:   runHostScriptAppFn,
		container:    runContainerAppFn,
		buildImage:   dockerBuildAppFn,
		runSteps:     runStepsAppFn,
		steps:        snapshotStepExecutionServices(),
		finalize:     finalizeWorkflowGitSession,
		commit:       runHostCommit,
	}
}

func (services runExecutionServices) executeSteps(options runStepsOpts) error {
	options.services = services.steps
	return services.runSteps(options)
}
