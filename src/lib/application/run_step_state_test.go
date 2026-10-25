package application

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func TestRunStepsCaptureServicesAcrossChildAndFinally(t *testing.T) {
	withRunStepsSeams(t)
	originalLoad := loadWorkflowAppFn
	t.Cleanup(func() { loadWorkflowAppFn = originalLoad })
	root := t.TempDir()
	childRoot := filepath.Join(root, "workflows", "child")
	if err := os.MkdirAll(childRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte(`{"compile":{"workflows":["workflows"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	child := "name: child\nnamespace: fixture\nsteps:\n  - kind: host\n    cmd: child\n"
	if err := os.WriteFile(filepath.Join(childRoot, "config.yml"), []byte(child), 0o600); err != nil {
		t.Fatal(err)
	}
	var commands []string
	runHostCommandFn = func(command string, env []string) error {
		commands = append(commands, command)
		if command == "start" {
			// Defaults changed after entry cannot change an active invocation,
			// including the loader used for a later nested workflow.
			runHostCommandFn = func(string, []string) error {
				return errors.New("new command default leaked into active run")
			}
			loadWorkflowAppFn = func(string) (*domain.Workflow, error) {
				return nil, errors.New("new loader default leaked into active run")
			}
		}
		return nil
	}
	options := baseRunStepsOpts()
	options.repoRoot = root
	options.projectRoot = root
	options.wfRoot = root
	options.opts.Workdir = root
	options.envMap["DOCKPIPE_WORKDIR"] = root
	options.wf = &domain.Workflow{
		Name: "parent",
		Steps: []domain.Step{
			{Kind: "host", Cmd: "start"},
			{WorkflowName: "child", Package: "fixture"},
		},
		Finally: []domain.Step{{Kind: "host", Cmd: "finally"}},
	}
	if err := runSteps(options); err != nil {
		t.Fatal(err)
	}
	if want := []string{"start", "child", "finally"}; !reflect.DeepEqual(commands, want) {
		t.Fatalf("executed %v, want %v", commands, want)
	}
}

func TestRunBlockingStepCapturesServicesBeforeSetup(t *testing.T) {
	withRunStepsSeams(t)
	root := t.TempDir()
	setup := filepath.Join(root, "setup.sh")
	if err := os.WriteFile(setup, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	runHostCommandFn = func(string, []string) error {
		called = true
		return nil
	}
	runHostScriptFn = func(string, []string) error {
		runHostCommandFn = func(string, []string) error {
			return errors.New("setup changed the active command adapter")
		}
		return nil
	}
	options := baseRunStepsOpts()
	options.wfRoot = root
	options.repoRoot = root
	options.projectRoot = root
	options.opts.Workdir = root
	options.wf.Steps = []domain.Step{{Kind: "host", Run: []string{setup}, Cmd: "run"}}
	if err := runBlockingStep(&options, 0, 1, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("captured command adapter did not execute")
	}
}

func TestRunBlockingStepRefreshesOutputPathAfterSourcedSetup(t *testing.T) {
	withRunStepsSeams(t)
	root := t.TempDir()
	setup := filepath.Join(root, "setup.sh")
	outputRoot := filepath.Join(root, "dynamic-output")
	if err := os.WriteFile(setup, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceHostScriptFn = func(string, []string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_OUTPUT_ROOT": outputRoot, "HOST_ONLY": "private"}, nil
	}
	runContainerFn = func(options infrastructure.RunOpts, args []string) (int, error) {
		containerEnv := domain.EnvSliceToMap(options.ExtraEnv)
		if _, leaked := containerEnv["HOST_ONLY"]; leaked {
			t.Fatal("host environment widened into container environment")
		}
		if err := os.MkdirAll(outputRoot, 0o755); err != nil {
			return 0, err
		}
		return 0, os.WriteFile(filepath.Join(outputRoot, "result.env"), []byte("RESULT=ready\n"), 0o600)
	}
	options := baseRunStepsOpts()
	options.wfRoot = root
	options.repoRoot = root
	options.projectRoot = root
	options.opts.Workdir = root
	options.envMap["DOCKPIPE_WORKDIR"] = root
	options.wf.Steps = []domain.Step{{Run: []string{setup}, Cmd: "execute", Isolate: "fixture:latest", Outputs: "result.env"}}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&options, 0, 1, dockerEnv); err != nil {
		t.Fatal(err)
	}
	if options.envMap["RESULT"] != "ready" || dockerEnv["RESULT"] != "ready" {
		t.Fatalf("dynamic output path did not merge: host=%v container=%v", options.envMap, dockerEnv)
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "result.env")); !os.IsNotExist(err) {
		t.Fatalf("merged output not consumed: %v", err)
	}
}

func TestRunBlockingStepLoadsOrdinaryProfileAfterSetup(t *testing.T) {
	for _, existingProfile := range []bool{false, true} {
		name := "created_by_setup"
		if existingProfile {
			name = "rewritten_by_setup"
		}
		t.Run(name, func(t *testing.T) {
			withRunStepsSeams(t)
			root := t.TempDir()
			for _, script := range []string{"setup.sh", "before.sh", "after.sh"} {
				if err := os.WriteFile(filepath.Join(root, script), []byte("#!/bin/sh\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			profile := func(script string) string {
				return "DOCKPIPE_RESOLVER_HOST_ISOLATE=" + script + "\nDOCKPIPE_RESOLVER_HOST_REQUIRES_DOCKER=false\n"
			}
			if existingProfile {
				writeTestCoreResolver(t, root, "dynamic", profile("before.sh"))
			}
			sourceHostScriptFn = func(string, []string) (map[string]string, error) {
				writeTestCoreResolver(t, root, "dynamic", profile("after.sh"))
				return nil, nil
			}
			var executedScript string
			runHostScriptFn = func(path string, _ []string) error {
				executedScript = filepath.Base(path)
				return nil
			}
			options := baseRunStepsOpts()
			options.repoRoot = root
			options.projectRoot = root
			options.wfRoot = root
			options.opts.Workdir = root
			options.envMap["DOCKPIPE_WORKDIR"] = root
			options.wf.Steps = []domain.Step{{Resolver: "dynamic", Run: []string{"setup.sh"}, Cmd: "execute"}}
			if err := runBlockingStep(&options, 0, 1, map[string]string{}); err != nil {
				t.Fatal(err)
			}
			if executedScript != "after.sh" {
				t.Fatalf("executed %q, want profile from setup", executedScript)
			}
		})
	}
}
