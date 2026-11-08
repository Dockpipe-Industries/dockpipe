package application

import (
	"context"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

type workflowInvocationInput struct {
	arguments   []string
	environment []string
}

// executeWorkflowRun admits workflow selection before loading files, resolving
// dependencies or starting setup commands. Its digest binds the invocation,
// not the contents of a mutable workflow file. Loaded workflow validation and
// compiled runtime policy remain authoritative inside execution.
func executeWorkflowRun(argv []string, baseEnviron []string, services runExecutionServices) error {
	target, identity := nativeinvocation.Local()
	definition := invocation.Definition{
		ID:     invocation.ID{Capability: "workflow", Action: "run", Version: "1"},
		Target: target, RequireAuthorization: services.admission.Authorize != nil,
	}
	contract, err := invocation.New(definition, func(_ context.Context, input workflowInvocationInput) (invocation.Prepared[struct{}], error) {
		if err := infrastructure.ValidateNativeExecutionStrings(input.arguments); err != nil {
			return invocation.Prepared[struct{}]{}, err
		}
		if err := infrastructure.ValidateNativeExecutionEnvironment(input.environment); err != nil {
			return invocation.Prepared[struct{}]{}, err
		}
		arguments := append([]string(nil), input.arguments...)
		environment := append([]string(nil), input.environment...)
		prepared := invocation.Prepared[struct{}]{Execute: func(context.Context) (struct{}, error) {
			return struct{}{}, executeAdmittedWorkflowRun(arguments, environment, services.withNativeAdmission())
		}}
		if services.admission.Authorize != nil {
			prepared.Digest = infrastructure.NativeExecutionDigest(arguments, environment)
		}
		return prepared, nil
	}, services.admission.Authorize)
	if err != nil {
		return err
	}
	_, err = contract.Invoke(services.admission.ExecutionContext(), invocation.Call{
		Contract: definition.ID, Target: target, Identity: identity,
	}, workflowInvocationInput{argv, baseEnviron})
	return err
}

// withNativeAdmission captures the verified local boundary callback in this
// execution's adapters. Nested workflows, parallel steps and finally steps use
// these same values rather than looking up mutable process-global authority.
func (services runExecutionServices) withNativeAdmission() runExecutionServices {
	admission := services.admission
	if admission.Authorize == nil && admission.Context == nil {
		return services
	}
	source := services.sourceScript
	services.sourceScript = func(path string, env []string) (map[string]string, error) {
		return infrastructure.AdmitSourceHostScript(admission, path, env, source)
	}
	host := services.hostScript
	services.hostScript = func(path string, env []string) error {
		return infrastructure.AdmitHostScript(admission, path, env, "", func(path string, env []string, _ string) error { return host(path, env) })
	}
	container := services.container
	services.container = func(options infrastructure.RunOpts, arguments []string) (int, error) {
		return infrastructure.AdmitContainer(admission, options, arguments, container)
	}
	services.steps = services.steps.withNativeAdmission(admission)
	return services
}

func (services *stepExecutionServices) withNativeAdmission(admission infrastructure.NativeExecutionAdmission) *stepExecutionServices {
	if services == nil {
		services = snapshotStepExecutionServices()
	}
	owned := *services
	source := services.sourceHostScript
	owned.sourceHostScript = func(path string, env []string) (map[string]string, error) {
		return infrastructure.AdmitSourceHostScript(admission, path, env, source)
	}
	host := services.runHostScript
	owned.runHostScript = func(path string, env []string) error {
		return infrastructure.AdmitHostScript(admission, path, env, "", func(path string, env []string, _ string) error { return host(path, env) })
	}
	command := services.runHostCommand
	owned.runHostCommand = func(text string, env []string) error {
		return infrastructure.AdmitHostCommand(admission, text, env, "", func(text string, env []string, _ string) error { return command(text, env) })
	}
	hostWithShell := services.runHostScriptWithShell
	owned.runHostScriptWithShell = func(path string, env []string, shell string) error {
		return infrastructure.AdmitHostScript(admission, path, env, shell, hostWithShell)
	}
	commandWithShell := services.runHostCommandWithShell
	owned.runHostCommandWithShell = func(text string, env []string, shell string) error {
		return infrastructure.AdmitHostCommand(admission, text, env, shell, commandWithShell)
	}
	container := services.runContainer
	owned.runContainer = func(options infrastructure.RunOpts, arguments []string) (int, error) {
		return infrastructure.AdmitContainer(admission, options, arguments, container)
	}
	return &owned
}
