package application

import (
	"context"
	"fmt"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func runWorkflowControl(options *runStepsOpts, dockerEnvironment map[string]string) error {
	execute := options.executionServices().runControl
	if execute == nil {
		execute = infrastructure.RunWorkflowControl
	}
	return execute(infrastructure.WorkflowControlRunOptions{
		Workdir: runStepsWorkdir(options), ArtifactWorkdir: options.controlArtifactRoot(), WorkflowRoot: options.wfRoot, Admission: options.services.admission, Workflow: options.wf, Environment: options.envMap,
		Context: options.services.admission.ExecutionContext(),
		Invoke: func(ctx context.Context, id string, arguments map[string]string) error {
			index := -1
			for candidate, step := range options.wf.Steps {
				if step.ID == id {
					index = candidate
					break
				}
			}
			if index < 0 {
				return fmt.Errorf("control requested an undeclared step")
			}
			for name, value := range arguments {
				if options.locked[name] && options.envMap[name] != value {
					return fmt.Errorf("control argument %s conflicts with a locked caller binding", name)
				}
			}
			// Parameters are invocation-local. Preserve merged dotenv outputs,
			// but restore only these explicit argument bindings after the call.
			previous := map[string]string{}
			present := map[string]bool{}
			previousDocker := map[string]string{}
			presentDocker := map[string]bool{}
			for name, value := range arguments {
				previous[name], present[name] = options.envMap[name]
				previousDocker[name], presentDocker[name] = dockerEnvironment[name]
				options.envMap[name] = value
				dockerEnvironment[name] = value
			}
			defer func() {
				for name := range arguments {
					if present[name] {
						options.envMap[name] = previous[name]
					} else {
						delete(options.envMap, name)
					}
					if presentDocker[name] {
						dockerEnvironment[name] = previousDocker[name]
					} else {
						delete(dockerEnvironment, name)
					}
				}
			}()
			// Existing step vars may not silently replace a typed argument.
			workflowCopy := *options.wf
			workflowCopy.Steps = append([]domain.Step(nil), options.wf.Steps...)
			step := workflowCopy.Steps[index]
			variables := map[string]string{}
			for name, value := range step.Vars {
				variables[name] = value
			}
			for name, value := range arguments {
				variables[name] = value
			}
			step.Vars = variables
			workflowCopy.Steps[index] = step
			invocation := *options
			invocation.wf = &workflowCopy
			invocation.services = options.executionServices().withHostExecutionContext(ctx)
			return runBlockingStep(&invocation, index, len(workflowCopy.Steps), dockerEnvironment)
		},
	})
}

func (options *runStepsOpts) controlArtifactRoot() string {
	if options.controlArtifactWorkdir != "" {
		return options.controlArtifactWorkdir
	}
	return options.projectRoot
}
