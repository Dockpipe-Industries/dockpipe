package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHostContextPreCancelledDoesNotBeginRun(t *testing.T) {
	for _, inline := range []bool{false, true} {
		name := "script"
		if inline {
			name = "command"
		}
		t.Run(name, func(t *testing.T) {
			workdir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			env := []string{"DOCKPIPE_WORKDIR=" + workdir}
			var err error
			if inline {
				err = RunHostCommandWithContext(ctx, "must not execute", env, "invalid-shell")
			} else {
				err = RunHostScriptWithContext(ctx, filepath.Join(workdir, "missing"), env, "invalid-shell")
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("pre-cancelled %s result = %v", name, err)
			}
			entries, err := os.ReadDir(workdir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("pre-cancelled call created run state: %v, %v", entries, err)
			}
		})
	}
}

func TestHostContextPreservesPreparedCommandArguments(t *testing.T) {
	original := exec.Command(os.Args[0], "", "space and \"quote\"", "unicode-λ")
	original.Dir = t.TempDir()
	original.Env = []string{"HOST_CONTEXT_INPUT=exact"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	command := hostCommandWithContext(ctx, original)
	if command.Path != original.Path || command.Dir != original.Dir || command.Cancel == nil {
		t.Fatalf("prepared command identity/context changed: %#v", command)
	}
	if len(command.Args) != len(original.Args) || len(command.Env) != len(original.Env) {
		t.Fatal("prepared argument/environment extent changed")
	}
	for index := range original.Args {
		if command.Args[index] != original.Args[index] {
			t.Fatalf("argument %d changed", index)
		}
	}
	if command.Env[0] != original.Env[0] {
		t.Fatal("prepared environment changed")
	}
}
