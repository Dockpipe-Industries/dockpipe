package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/domain/workflow"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	ownedprocess "dockpipe/src/lib/infrastructure/process"
)

type WorkflowControlRunOptions struct {
	ArtifactWorkdir string
	WorkflowRoot    string
	Admission       NativeExecutionAdmission
	Workdir         string
	Workflow        *workflow.Workflow
	Environment     map[string]string
	Context         context.Context
	Invoke          func(context.Context, string, map[string]string) error
}

type controlFrameEvent struct {
	payload []byte
	err     error
}

// RunWorkflowControl executes only a verified prebuilt artifact. Its child has
// no inherited environment or compiler dependency. Compute deadlines pause while
// the ordinary step dispatcher owns a requested effect and its lifetime.
func RunWorkflowControl(options WorkflowControlRunOptions) error {
	if err := workflow.ValidateLoadedWorkflow(options.Workflow); err != nil {
		return err
	}
	artifactWorkdir := options.ArtifactWorkdir
	if artifactWorkdir == "" {
		artifactWorkdir = options.Workdir
	}
	binary, manifest, err := loadControlArtifact(artifactWorkdir, options.Workflow, options.WorkflowRoot)
	if err != nil {
		return err
	}
	input, err := encodeControlInputs(options.Workflow.Control, options.Environment)
	if err != nil {
		return err
	}
	target, identity := nativeinvocation.Local()
	definition := invocation.Definition{ID: invocation.ID{Capability: "workflow.control", Action: "run", Version: "1"}, Target: target, RequireAuthorization: options.Admission.Authorize != nil}
	contract, err := invocation.New(definition, func(context.Context, struct{}) (invocation.Prepared[struct{}], error) {
		prepared := invocation.Prepared[struct{}]{Execute: func(ctx context.Context) (struct{}, error) {
			options.Context = ctx
			return struct{}{}, runVerifiedWorkflowControl(options, binary, manifest, input)
		}}
		if options.Admission.Authorize != nil {
			prepared.Digest = NativeExecutionDigest([]string{manifest.DefinitionSHA256, manifest.BinarySHA256}, []string{string(input.Bytes())})
		}
		return prepared, nil
	}, options.Admission.Authorize)
	if err != nil {
		return err
	}
	ctx := options.Context
	if ctx == nil {
		ctx = options.Admission.ExecutionContext()
	}
	_, err = contract.Invoke(ctx, invocation.Call{Contract: definition.ID, Target: target, Identity: identity}, struct{}{})
	return err
}

func runVerifiedWorkflowControl(options WorkflowControlRunOptions, binary string, manifest workflowControlManifest, input *controlWireWriter) error {
	parent := options.Context
	if parent == nil {
		parent = context.Background()
	}
	signals, stop := signal.NotifyContext(parent, workflowControlSignals()...)
	defer stop()
	ctx, cancel := context.WithCancel(signals)
	defer cancel()
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer inputRead.Close()
	defer inputWrite.Close()
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer outputRead.Close()
	defer outputWrite.Close()
	command := exec.CommandContext(ctx, binary)
	command.Dir = options.Workdir
	command.Env = []string{"LANG=C.UTF-8"}
	if manifest.Sanitizers {
		command.Env = append(command.Env, "ASAN_OPTIONS=detect_leaks=1:halt_on_error=1", "UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1")
	}
	command.Stdin = inputRead
	command.Stdout = outputWrite
	var diagnostics controlBuildOutput
	command.Stderr = &diagnostics
	var closeOnce sync.Once
	closeChildEnds := func() {
		closeOnce.Do(func() {
			// Release the parent's copies once the start attempt has finished.
			_ = inputRead.Close()
			_ = outputWrite.Close()
		})
	}
	exited := make(chan error, 1)
	go func() {
		exited <- ownedprocess.RunObserved(command, func(*exec.Cmd) func(error) { closeChildEnds(); return nil })
	}()
	events := make(chan controlFrameEvent, 1)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			payload, err := readControlFrame(outputRead)
			select {
			case events <- controlFrameEvent{payload, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	// Every exit joins both the process owner and protocol reader. Finally must
	// never overlap an unresolved controller process or callback.
	var processErr error
	defer func() {
		cancel()
		// Closing the pipes unblocks I/O. Abort paths may have closed them already.
		_ = inputWrite.Close()
		_ = outputRead.Close()
		if exited != nil {
			<-exited
		}
		<-readerDone
	}()
	seconds := options.Workflow.Control.Limits.ComputeTimeoutSeconds
	if seconds == 0 {
		seconds = 5
	}
	timeout := time.Duration(seconds) * time.Second
	write := func(frame *controlWireWriter) error {
		done := make(chan error, 1)
		go func() { done <- frame.send(inputWrite) }()
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			_ = inputWrite.Close() // Unblock the write and preserve the cancellation result.
			<-done
			return ctx.Err()
		case <-timer.C:
			cancel()
			_ = inputWrite.Close() // Unblock the write and preserve the timeout result.
			<-done
			return fmt.Errorf("workflow control compute timeout during protocol write")
		}
	}
	next := func() (controlFrameEvent, error) {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case event := <-events:
			return event, nil
		case <-ctx.Done():
			return controlFrameEvent{}, ctx.Err()
		case <-timer.C:
			return controlFrameEvent{}, fmt.Errorf("workflow control compute timeout")
		}
	}
	if err := write(input); err != nil {
		return err
	}
	maximumCalls := options.Workflow.Control.Limits.MaxCalls
	if maximumCalls == 0 {
		maximumCalls = 128
	}
	var lastStepError error
	stepErrors := map[string]error{}
	for calls := 0; ; {
		event, err := next()
		if err != nil {
			return err
		}
		if event.err != nil {
			interrupted := signals.Err()
			cancel()
			processErr = <-exited
			exited = nil
			if interrupted != nil {
				return interrupted
			}
			if processErr != nil {
				return fmt.Errorf("workflow control process failed before DONE: %w: %s", processErr, diagnostics.String())
			}
			return fmt.Errorf("workflow control ended before DONE: %w", event.err)
		}
		message, err := decodeControlMessage(event.payload)
		if err != nil {
			return err
		}
		if message.kind == 3 {
			end, err := next()
			if err != nil {
				return err
			}
			if end.err != io.EOF {
				return fmt.Errorf("workflow control emitted data after DONE")
			}
			select {
			case processErr = <-exited:
				exited = nil
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(timeout):
				return fmt.Errorf("workflow control failed to exit after DONE")
			}
			if processErr != nil {
				return fmt.Errorf("workflow control process failed: %w: %s", processErr, diagnostics.String())
			}
			if message.outcome == 0 {
				return nil
			}
			if message.outcome == 1 {
				return fmt.Errorf("workflow control cancelled: %w", context.Canceled)
			}
			if original := stepErrors[message.diagnostic]; original != nil {
				return original
			}
			return fmt.Errorf("workflow control failed: %s", message.diagnostic)
		}
		if message.sequence != uint64(calls+1) || calls >= maximumCalls {
			return fmt.Errorf("workflow control call sequence or invocation budget exceeded")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		arguments, err := controlStepArguments(options.Workflow, message.step, message.arguments)
		if err != nil {
			return err
		}
		calls++
		fmt.Fprintf(os.Stderr, "[dockpipe] Control call %d: %s\n", calls, message.step)
		lastStepError = options.Invoke(ctx, message.step, arguments)
		if lastStepError != nil {
			stepErrors[controlErrorText(lastStepError)] = lastStepError
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		reply := &controlWireWriter{}
		reply.byte(2)
		reply.number(message.sequence, 8)
		outcome := byte(0)
		if lastStepError != nil {
			outcome = 2
		}
		if errors.Is(lastStepError, context.Canceled) {
			outcome = 1
		}
		reply.byte(outcome)
		reply.text(controlErrorText(lastStepError))
		if err := write(reply); err != nil {
			return err
		}
	}
}

func controlStepArguments(definition *workflow.Workflow, id string, supplied map[string]string) (map[string]string, error) {
	var selected *workflow.Step
	for index := range definition.Steps {
		if definition.Steps[index].ID == id {
			selected = &definition.Steps[index]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("control requested an undeclared step")
	}
	var parameters []workflow.ControlParameter
	if selected.Control != nil {
		parameters = selected.Control.Parameters
	}
	if len(supplied) != len(parameters) {
		return nil, fmt.Errorf("control argument set differs from step %s", id)
	}
	environment := map[string]string{}
	for _, parameter := range parameters {
		value, exists := supplied[parameter.Name]
		if !exists {
			return nil, fmt.Errorf("control step %s is missing a declared argument", id)
		}
		switch parameter.Type {
		case "bool":
			if value != "true" && value != "false" {
				return nil, fmt.Errorf("control step %s has an invalid boolean", id)
			}
		case "int":
			number, err := strconv.ParseInt(value, 10, 64)
			if err != nil || strconv.FormatInt(number, 10) != value {
				return nil, fmt.Errorf("control step %s has an invalid integer", id)
			}
		}
		environment[parameter.Env] = value
	}
	return environment, nil
}
