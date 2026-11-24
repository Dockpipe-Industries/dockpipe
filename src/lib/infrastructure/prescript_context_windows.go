//go:build windows

package infrastructure

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"

	ownedprocess "dockpipe/src/lib/infrastructure/process"
)

func hostExecutionSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}

func runContextHostScriptProcess(ctx context.Context, path string, command *exec.Cmd, env []string) error {
	run := func(actual *exec.Cmd, observer ownedprocess.HostStartedObserver) error {
		return ownedprocess.RunHostObserved(ctx, actual, observer)
	}
	return runHostScriptProcessObserved(path, command, env, run, func(actual *exec.Cmd) error {
		return actual.Cancel()
	}, true, ctx.Err)
}

func runContextSourceHostScriptProcess(ctx context.Context, path string, command *exec.Cmd, env []string, output io.Writer) error {
	run := func(actual *exec.Cmd, observer ownedprocess.HostStartedObserver, _, _ func()) error {
		return ownedprocess.RunHostObserved(ctx, actual, observer)
	}
	return runHostScriptProcessObservedOutput(path, command, env, run, func(actual *exec.Cmd) error {
		return actual.Cancel()
	}, true, ctx.Err, hostScriptProcessIO{stdout: output, stderr: output, preserveCommandEnvironment: true})
}
