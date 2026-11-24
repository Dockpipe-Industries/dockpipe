package application

import (
	"context"

	"dockpipe/src/lib/infrastructure"
)

// A controller callback borrows its cancellation context without changing the
// parent invocation. Core finally therefore retains its own execution context.
func (services *stepExecutionServices) withHostExecutionContext(ctx context.Context) *stepExecutionServices {
	owned := *services
	admission := services.admission
	admission.Context = ctx
	owned.admission = admission

	if services.sourceHostScriptContext != nil {
		source := services.sourceHostScriptContext
		owned.sourceHostScript = func(path string, environment []string) (map[string]string, error) {
			return infrastructure.AdmitSourceHostScript(admission, path, environment,
				func(path string, environment []string) (map[string]string, error) {
					return source(ctx, path, environment)
				})
		}
	}

	if services.runHostScriptContext != nil {
		runScript := services.runHostScriptContext
		execute := func(path string, environment []string, shell string) error {
			return runScript(ctx, path, environment, shell)
		}
		owned.runHostScript = func(path string, environment []string) error {
			return infrastructure.AdmitHostScript(admission, path, environment, "", execute)
		}
		owned.runHostScriptWithShell = func(path string, environment []string, shell string) error {
			return infrastructure.AdmitHostScript(admission, path, environment, shell, execute)
		}
	}

	if services.runHostCommandContext != nil {
		runCommand := services.runHostCommandContext
		execute := func(command string, environment []string, shell string) error {
			return runCommand(ctx, command, environment, shell)
		}
		owned.runHostCommand = func(command string, environment []string) error {
			return infrastructure.AdmitHostCommand(admission, command, environment, "", execute)
		}
		owned.runHostCommandWithShell = func(command string, environment []string, shell string) error {
			return infrastructure.AdmitHostCommand(admission, command, environment, shell, execute)
		}
	}

	return &owned
}
