package infrastructure

import (
	"context"
	"os/exec"
)

// RunHostScriptWithContext carries cancellation into the active host process
// owner, preserving the same admission, registry and cleanup as script execution.
func RunHostScriptWithContext(ctx context.Context, path string, env []string, shell string) error {
	ctx = hostExecutionContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}

	return AdmitHostScript(NativeExecutionAdmission{Context: ctx}, path, env, shell, func(path string, env []string, shell string) error {
		return runHostScriptWithContext(ctx, path, env, shell)
	})
}

// RunHostCommandWithContext retains the caller context through the temporary
// script adapter rather than returning to a contextless script entry point.
func RunHostCommandWithContext(ctx context.Context, command string, env []string, shell string) error {
	ctx = hostExecutionContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}

	return AdmitHostCommand(NativeExecutionAdmission{Context: ctx}, command, env, shell, func(command string, env []string, shell string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return runHostCommandUsing(command, env, shell, func(path string, env []string, shell string) error {
			return runHostScriptWithContext(ctx, path, env, shell)
		})
	})
}

func hostExecutionContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func runHostScriptWithContext(parent context.Context, path string, env []string, shell string) error {
	ctx, stop := hostExecutionSignalContext(hostExecutionContext(parent))
	defer stop()
	if err := ctx.Err(); err != nil {
		return err
	}

	original, commandEnv, err := hostScriptCommand(path, env, shell)
	if err != nil {
		return err
	}
	command := hostCommandWithContext(ctx, original)
	return runContextHostScriptProcess(ctx, path, command, commandEnv)
}

// Interpreter selection and encoded arguments remain owned by hostScriptCommand.
// Rebind that prepared command before pipes or process state exist so exec owns
// a genuine context watcher without repeating shell/argument construction.
func hostCommandWithContext(ctx context.Context, original *exec.Cmd) *exec.Cmd {
	command := exec.CommandContext(ctx, original.Path)
	command.Path = original.Path
	command.Args = append([]string(nil), original.Args...)
	command.Env = original.Env
	command.Dir = original.Dir
	command.SysProcAttr = original.SysProcAttr
	command.ExtraFiles = original.ExtraFiles
	command.Err = original.Err
	return command
}
