package application

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func TestWorkflowControlDispatchPreservesArtifactRootAndFinally(t *testing.T) {
	definition, err := domain.ParseWorkflowYAML([]byte("name: test\ncontrol:\n  schema: 1\n  kind: host\n  source: |\n    try steps.test.Run();\nsteps:\n  - id: test\n    kind: host\n    cmd: body\nfinally:\n  - id: cleanup\n    kind: host\n    cmd: cleanup\n"))
	if err != nil {
		t.Fatal(err)
	}
	original := t.TempDir()
	workspace := t.TempDir()
	services := snapshotStepExecutionServices()
	var commands []string
	failed := errors.New("body failure")
	services.runHostCommand = func(command string, _ []string) error {
		commands = append(commands, command)
		if command == "body" {
			return failed
		}
		return nil
	}
	services.runHostCommandContext = func(_ context.Context, command string, environment []string, _ string) error {
		return services.runHostCommand(command, environment)
	}
	services.runControl = func(options infrastructure.WorkflowControlRunOptions) error {
		if options.ArtifactWorkdir != original || options.Workdir != workspace {
			t.Fatalf("lost cache/cwd distinction: %+v", options)
		}
		return options.Invoke(context.Background(), "test", nil)
	}
	options := runStepsOpts{wf: definition, projectRoot: workspace, controlArtifactWorkdir: original,
		opts: &CliOpts{Workdir: workspace}, envMap: map[string]string{}, locked: map[string]bool{}, services: services}
	err = runSteps(options)
	if !errors.Is(err, failed) || !reflect.DeepEqual(commands, []string{"body", "cleanup"}) {
		t.Fatalf("lost failure/finally: %v %v", commands, err)
	}
}

func TestWorkflowControlCancellationReachesActiveStepWithoutCancellingFinally(t *testing.T) {
	definition, err := domain.ParseWorkflowYAML([]byte(`name: test
control:
  schema: 1
  kind: host
  source: |
    try steps.test.Run();
steps:
  - id: test
    kind: host
    cmd: body
finally:
  - id: cleanup
    kind: host
    cmd: cleanup
`))
	if err != nil {
		t.Fatal(err)
	}
	parent, cancelParent := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelParent()

	services := snapshotStepExecutionServices()
	services.admission.Context = parent
	started := make(chan struct{})
	var commands []string
	services.runHostCommandContext = func(ctx context.Context, command string, _ []string, _ string) error {
		commands = append(commands, command)
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	services.runHostCommand = func(command string, _ []string) error {
		if command != "cleanup" || services.admission.ExecutionContext().Err() != nil {
			t.Fatalf("finally lost its independent parent context: %s", command)
		}
		commands = append(commands, command)
		return nil
	}
	services.runControl = func(options infrastructure.WorkflowControlRunOptions) error {
		ctx, cancel := context.WithCancel(options.Context)
		defer cancel()
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			select {
			case <-started:
				cancel()
			case <-parent.Done():
			}
		}()

		err := options.Invoke(ctx, "test", nil)
		cancel()
		<-joined
		return err
	}

	root := t.TempDir()
	options := runStepsOpts{wf: definition, projectRoot: root,
		opts: &CliOpts{Workdir: root}, envMap: map[string]string{}, locked: map[string]bool{}, services: services}
	err = runSteps(options)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(commands, []string{"body", "cleanup"}) {
		t.Fatalf("active cancellation/finally failed: %v %v", commands, err)
	}
	if parent.Err() != nil || services.admission.Context != parent {
		t.Fatal("callback cancellation mutated the parent invocation")
	}
}
