package application

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestRunWorkflowAfterHookReceivesMergedStepOutputs(t *testing.T) {
	withRunSeams(t)
	withRunStepsSeams(t)
	root := t.TempDir()
	for name, content := range map[string]string{
		"strategies/report": "DOCKPIPE_STRATEGY_AFTER=scripts/report.sh\n",
		"scripts/report.sh": "#!/bin/sh\n",
		"config.yml":        "name: results\nstrategy: report\nsteps:\n  - kind: host\n    cmd: emit\n    outputs: result.env\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repoRootAppFn = func() (string, error) { return root, nil }
	runStepsAppFn = runSteps
	runHostCommandFn = func(command string, env []string) error {
		if command != "emit" {
			return fmt.Errorf("unexpected host command %q", command)
		}
		values := domain.EnvSliceToMap(env)
		directory := values["DOCKPIPE_OUTPUT_ROOT"]
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(directory, "result.env"), []byte("STEP_RESULT=ready\n"), 0o600)
	}
	afterCalled := false
	runHostScriptAppFn = func(path string, env []string) error {
		afterCalled = true
		if filepath.Base(path) != "report.sh" {
			return fmt.Errorf("unexpected after hook %q", path)
		}
		values := domain.EnvSliceToMap(env)
		if values["STEP_RESULT"] != "ready" {
			return fmt.Errorf("after hook missed merged step outputs: STEP_RESULT=%q", values["STEP_RESULT"])
		}
		return nil
	}
	if err := Run([]string{"--workflow-file", filepath.Join(root, "config.yml"), "--workdir", root}, nil); err != nil {
		t.Fatal(err)
	}
	if !afterCalled {
		t.Fatal("noncommit strategy after hook did not run")
	}
}
