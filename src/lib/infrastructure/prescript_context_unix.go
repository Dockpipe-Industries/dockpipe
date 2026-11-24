//go:build !windows

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	ownedprocess "dockpipe/src/lib/infrastructure/process"
)

func hostExecutionSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(parent)
}

func runContextHostScriptProcess(ctx context.Context, path string, command *exec.Cmd, env []string) error {
	return runHostScriptProcessObserved(path, command, env, runHostScriptWithContextSignals, func(actual *exec.Cmd) error {
		return actual.Cancel()
	}, false, ctx.Err)
}

// Cancellation becomes a signal request consumed by the existing waiter. Only
// that waiter may signal a held Linux group; exec's watcher never signals an
// unprotected numeric group ID after the root may have been reaped.
func runHostScriptWithContextSignals(command *exec.Cmd, observer ownedprocess.HostStartedObserver) error {
	return runHostScriptWithContextSignalsUsing(command, observer, waitContextHostScriptSignals)
}

// A private Linux group retains its leader through captured EOF. Other host
// topologies preserve normal execution, but cancellation aborts private readers
// and reports that descendant/output closure could not be established.
func runContextSourceHostScriptProcess(ctx context.Context, path string, command *exec.Cmd, env []string, output io.Writer) error {
	run := func(actual *exec.Cmd, observer ownedprocess.HostStartedObserver, joinOutput, abortOutput func()) error {
		if sourceHostUsesPrivateGroup(actual) {
			return runHostScriptWithContextSignalsUsing(actual, observer, func(command *exec.Cmd, signals <-chan os.Signal) error {
				return waitContextSourceHostScriptSignals(command, signals, joinOutput, abortOutput)
			})
		}

		stop := make(chan struct{})
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			select {
			case <-ctx.Done():
				abortOutput()
			case <-stop:
			}
		}()
		outcome := runHostScriptWithContextSignalsUsing(actual, observer, waitContextHostScriptSignals)
		close(stop)
		<-joined
		if ctx.Err() != nil {
			return errors.Join(outcome, fmt.Errorf("source cancellation did not establish descendant/output closure for this host topology"))
		}

		return outcome
	}
	return runHostScriptProcessObservedOutput(path, command, env, run, func(actual *exec.Cmd) error {
		return actual.Cancel()
	}, true, ctx.Err, hostScriptProcessIO{stdout: output, stderr: output, preserveCommandEnvironment: true})
}

func runHostScriptWithContextSignalsUsing(command *exec.Cmd, observer ownedprocess.HostStartedObserver, wait func(*exec.Cmd, <-chan os.Signal) error) error {
	cancellation := make(chan struct{})
	requestCancellation := sync.OnceFunc(func() { close(cancellation) })
	command.Cancel = func() error {
		requestCancellation()
		return nil
	}

	received := make(chan os.Signal, 8)
	signal.Notify(received, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(received)
	forward := make(chan os.Signal, 8)
	stop := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		cancelled := cancellation
		for {
			var next os.Signal
			select {
			case <-stop:
				return
			case next = <-received:
			case <-cancelled:
				next = syscall.SIGKILL
				cancelled = nil
			}

			select {
			case forward <- next:
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		close(stop)
		<-joined
	}()

	if err := command.Start(); err != nil {
		return errors.Join(err, observer(command)(err))
	}
	finish := observer(command)
	err := wait(command, forward)
	return errors.Join(err, finish(err))
}
