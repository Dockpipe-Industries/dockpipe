package application

import (
	"context"
	"errors"
	"sync"
	"testing"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure"
)

func TestWorkflowInvocationRejectsBeforeSelection(t *testing.T) {
	previousRoot := repoRootAppFn
	repoRootAppFn = func() (string, error) {
		t.Fatal("rejected workflow reached repository resolution")
		return "", nil
	}
	t.Cleanup(func() { repoRootAppFn = previousRoot })
	denied := errors.New("workflow invocation denied")
	services := runExecutionServices{admission: infrastructure.NativeExecutionAdmission{
		Authorize: func(_ context.Context, binding invocation.Binding) error {
			if binding.Contract != (invocation.ID{Capability: "workflow", Action: "run", Version: "1"}) || len(binding.Digest) != 64 {
				t.Fatal("wrong workflow authority binding")
			}
			return denied
		},
	}}
	if err := executeWorkflowRun([]string{"--workflow", "fixture"}, nil, services); !errors.Is(err, denied) {
		t.Fatalf("workflow denial lost: %v", err)
	}
	if err := executeWorkflowRun([]string{"--workflow", "bad\x00name"}, nil, runExecutionServices{}); err == nil {
		t.Fatal("workflow accepted malformed arguments")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	services.admission.Context = ctx
	if err := executeWorkflowRun(nil, nil, services); !errors.Is(err, context.Canceled) {
		t.Fatalf("workflow cancellation lost: %v", err)
	}
}

func TestWorkflowInvocationCapturesAuthorityForNestedSteps(t *testing.T) {
	denied := errors.New("child denied")
	base := runExecutionServices{
		steps: &stepExecutionServices{
			runHostCommand: func(string, []string) error {
				t.Fatal("denied child reached executor")
				return nil
			},
		},
		runSteps: func(options runStepsOpts) error {
			child := options
			return child.executionServices().runHostStepCommand("", "echo child", []string{"KEY=value"})
		},
		admission: infrastructure.NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error { return denied }},
	}
	bound := base.withNativeAdmission()
	base.admission.Authorize = func(context.Context, invocation.Binding) error { return nil }
	if err := bound.executeSteps(runStepsOpts{}); !errors.Is(err, denied) {
		t.Fatalf("nested child lost invocation authority: %v", err)
	}
}

func TestWorkflowInvocationParallelAuthoritiesDoNotCrossTalk(t *testing.T) {
	firstDenied := errors.New("first invocation denied")
	secondDenied := errors.New("second invocation denied")
	base := &stepExecutionServices{runHostCommand: func(string, []string) error {
		t.Error("denied parallel invocation executed")
		return nil
	}}
	first := base.withNativeAdmission(infrastructure.NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error { return firstDenied }})
	second := base.withNativeAdmission(infrastructure.NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error { return secondDenied }})
	var workers sync.WaitGroup
	for index := 0; index < 8; index++ {
		workers.Add(2)
		go func() {
			defer workers.Done()
			if err := first.runHostStepCommand("", "echo first", nil); !errors.Is(err, firstDenied) {
				t.Errorf("first invocation authority changed: %v", err)
			}
		}()
		go func() {
			defer workers.Done()
			if err := second.runHostStepCommand("", "echo second", nil); !errors.Is(err, secondDenied) {
				t.Errorf("second invocation authority changed: %v", err)
			}
		}()
	}
	workers.Wait()
}
