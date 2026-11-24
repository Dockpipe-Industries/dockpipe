package application

import (
	"context"
	"errors"
	"testing"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure"
)

func TestWorkflowControlContextPreservesHostAuthorization(t *testing.T) {
	denied := errors.New("host callback denied")
	parent := context.Background()
	services := &stepExecutionServices{
		admission: infrastructure.NativeExecutionAdmission{
			Context: parent,
			Authorize: func(_ context.Context, binding invocation.Binding) error {
				if binding.Contract.Capability != "command" || len(binding.Digest) != 64 {
					t.Fatal("context-bound callback lost its native authority binding")
				}
				return denied
			},
		},
		runHostCommandContext: func(context.Context, string, []string, string) error {
			t.Fatal("denied context-bound callback reached the executor")
			return nil
		},
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	bound := services.withHostExecutionContext(ctx)
	if err := bound.runHostStepCommand("", "echo value", []string{"KEY=value"}); !errors.Is(err, denied) {
		t.Fatalf("host authorization changed: %v", err)
	}
	cancel()
	if err := bound.runHostStepCommand("", "echo value", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled callback was admitted: %v", err)
	}
	if services.admission.Context != parent {
		t.Fatal("call-local context replaced the parent authority")
	}
}

func TestWorkflowControlSourceContextPreservesAuthorizationAndParent(t *testing.T) {
	denied := errors.New("source callback denied")
	parent := context.Background()
	services := &stepExecutionServices{
		admission: infrastructure.NativeExecutionAdmission{
			Context: parent,
			Authorize: func(_ context.Context, binding invocation.Binding) error {
				if binding.Contract.Capability != "script" || len(binding.Digest) != 64 {
					t.Fatal("source callback lost its authority binding")
				}
				return denied
			},
		},
		sourceHostScriptContext: func(context.Context, string, []string) (map[string]string, error) {
			t.Fatal("denied source callback reached the executor")
			return nil, nil
		},
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	bound := services.withHostExecutionContext(ctx)
	if _, err := bound.sourceHostScript("source.sh", []string{"KEY=value"}); !errors.Is(err, denied) {
		t.Fatalf("source authority changed: %v", err)
	}
	cancel()
	if _, err := bound.sourceHostScript("source.sh", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled source callback was admitted: %v", err)
	}
	if services.admission.Context != parent {
		t.Fatal("source callback mutated the parent execution context")
	}
}

func TestWorkflowControlSourceContextIsCallLocal(t *testing.T) {
	parent := context.Background()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	services := &stepExecutionServices{
		admission: infrastructure.NativeExecutionAdmission{Context: parent},
		sourceHostScript: func(string, []string) (map[string]string, error) {
			return map[string]string{"CLEANUP": "independent"}, nil
		},
		sourceHostScriptContext: func(actual context.Context, _ string, _ []string) (map[string]string, error) {
			if actual != ctx {
				t.Fatal("source executor did not receive its callback context")
			}
			cancel()
			return nil, actual.Err()
		},
	}
	bound := services.withHostExecutionContext(ctx)
	if _, err := bound.sourceHostScript("body.sh", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("source callback cancellation changed: %v", err)
	}
	values, err := services.sourceHostScript("cleanup.sh", nil)
	if err != nil || values["CLEANUP"] != "independent" || services.admission.ExecutionContext().Err() != nil {
		t.Fatalf("source callback cancelled independent cleanup: %v, %v", values, err)
	}
}

func TestSourceContextAdapterPreservesCapturedCustomExecutor(t *testing.T) {
	calls := 0
	custom := func(_ context.Context, path string, environment []string) (map[string]string, error) {
		calls++
		if path != "custom-source.sh" || len(environment) != 1 || environment[0] != "KEY=value" {
			t.Fatal("source context adapter changed captured input")
		}
		return map[string]string{"CUSTOM": "captured"}, nil
	}
	services := &stepExecutionServices{
		sourceHostScript:        sourceHostScriptWithoutContext(custom),
		sourceHostScriptContext: custom,
	}
	bound := services.withHostExecutionContext(context.Background())
	values, err := bound.sourceHostScript("custom-source.sh", []string{"KEY=value"})
	if err != nil || values["CUSTOM"] != "captured" || calls != 1 {
		t.Fatalf("custom source executor was replaced: %d calls, %v, %v", calls, values, err)
	}
}
