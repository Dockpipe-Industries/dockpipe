package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func TestParallelOutputPathKeysRespectPlatformCasing(t *testing.T) {
	upper := filepath.Join("workflow", "RESULT.env")
	lower := filepath.Join("workflow", "result.env")
	if parallelOutputPathKey(upper, "windows") != parallelOutputPathKey(lower, "windows") {
		t.Fatal("Windows output aliases have different keys")
	}
	if parallelOutputPathKey(upper, "linux") == parallelOutputPathKey(lower, "linux") {
		t.Fatal("case-sensitive output paths were conflated")
	}
	aliased := "workflow" + string(filepath.Separator) + "." + string(filepath.Separator) + "result.env"
	if parallelOutputPathKey(aliased, "linux") != parallelOutputPathKey(lower, "linux") {
		t.Fatal("lexical output aliases have different keys")
	}
}

func TestParallelBatchMergesOutputsFromEachStepScope(t *testing.T) {
	withRunStepsSeams(t)
	root := t.TempDir()
	artifacts := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	o.projectRoot = root
	o.repoRoot = root
	o.opts.Workdir = root
	o.envMap["DOCKPIPE_WORKDIR"] = root
	o.envMap["DOCKPIPE_SOURCE_ROOT"] = root
	o.envMap["DOCKPIPE_ARTIFACT_ROOT"] = artifacts
	o.wf.Steps = []domain.Step{
		{Kind: "host", Cmd: "REPO", Outputs: "result.env", Scopes: domain.StepScopes{Artifacts: "repo"}},
		{Kind: "host", Cmd: "ARTIFACT", Outputs: "result.env", Scopes: domain.StepScopes{Artifacts: "artifacts"}},
	}
	runHostCommandFn = func(command string, env []string) error {
		values := domain.EnvSliceToMap(env)
		output := filepath.Join(values["DOCKPIPE_OUTPUT_ROOT"], "result.env")
		return os.WriteFile(output, []byte(command+"=merged\n"), 0o600)
	}
	dockerEnv := map[string]string{}
	if err := runParallelBatch(&o, 0, 2, 2, dockerEnv); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"REPO", "ARTIFACT"} {
		if o.envMap[key] != "merged" || dockerEnv[key] != "merged" {
			t.Errorf("%s outputs did not reach the next step: env=%q docker=%q", key, o.envMap[key], dockerEnv[key])
		}
	}
	for _, directory := range []string{root, artifacts} {
		if _, err := os.Stat(filepath.Join(directory, "result.env")); !os.IsNotExist(err) {
			t.Errorf("merged output was not consumed: %s, %v", directory, err)
		}
	}
}

func TestParallelBatchRejectsResolvedOutputAliasesBeforeExecution(t *testing.T) {
	withRunStepsSeams(t)
	root := t.TempDir()
	o := baseRunStepsOpts()
	o.projectRoot = root
	o.repoRoot = root
	o.opts.Workdir = root
	o.envMap["DOCKPIPE_WORKDIR"] = root
	o.wf.Steps = []domain.Step{
		{Kind: "host", Cmd: "first", Outputs: "result.env", Scopes: domain.StepScopes{Artifacts: "repo"}},
		{Kind: "host", Cmd: "second", Outputs: filepath.Join(root, "result.env")},
	}
	runHostCommandFn = func(string, []string) error {
		t.Error("conflicting steps executed")
		return nil
	}
	if err := runParallelBatch(&o, 0, 2, 2, map[string]string{}); err == nil || !strings.Contains(err.Error(), "duplicate outputs path") {
		t.Fatalf("resolved output conflict was not rejected: %v", err)
	}
}

func TestParallelHostBuiltinsRunWithWorkerLocalEnvironment(t *testing.T) {
	withRunStepsSeams(t)
	root := t.TempDir()
	o := baseRunStepsOpts()
	o.projectRoot = root
	o.repoRoot = root
	o.opts.Workdir = root
	o.envMap["DOCKPIPE_WORKDIR"] = root
	o.wf.Compose = domain.WorkflowComposeConfig{
		File: "compose.yml",
		Exports: map[string]string{
			"COMPOSE_ENDPOINT": "http://fixture.invalid",
		},
	}
	o.wf.Steps = []domain.Step{
		{Kind: "host", HostBuiltin: "compose_up", Cmd: "first", Outputs: "first.env", Vars: map[string]string{"WORKER": "first"}},
		{Kind: "host", HostBuiltin: "compose_ps", Cmd: "second", Outputs: "second.env", Vars: map[string]string{"WORKER": "second"}},
	}
	seen := make(chan string, 2)
	composeLifecycleFn = func(opts infrastructure.ComposeLifecycleOpts) error {
		values := domain.EnvSliceToMap(opts.Env)
		seen <- opts.Action + ":" + values["WORKER"]
		return nil
	}
	runHostCommandFn = func(command string, env []string) error {
		values := domain.EnvSliceToMap(env)
		if values["WORKER"] != command || values["COMPOSE_ENDPOINT"] != "http://fixture.invalid" {
			return fmt.Errorf("host command missed its builtin environment: %s", command)
		}
		return nil
	}
	if err := runParallelBatch(&o, 0, 2, 2, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	close(seen)
	actions := map[string]bool{}
	for action := range seen {
		actions[action] = true
	}
	if !actions["up:first"] || !actions["ps:second"] || len(actions) != 2 {
		t.Fatalf("async host builtins were not executed: %v", actions)
	}
	if _, exists := o.envMap["COMPOSE_ENDPOINT"]; exists {
		t.Fatal("worker builtin mutated the shared environment")
	}
	if _, exists := o.envMap["WORKER"]; exists {
		t.Fatal("worker variables leaked into the shared environment")
	}
}

func TestParallelHostBuiltinFailureStopsItsCommand(t *testing.T) {
	withRunStepsSeams(t)
	root := t.TempDir()
	o := baseRunStepsOpts()
	o.projectRoot = root
	o.repoRoot = root
	o.opts.Workdir = root
	o.envMap["DOCKPIPE_WORKDIR"] = root
	o.wf.Compose.File = "compose.yml"
	o.wf.Steps = []domain.Step{{Kind: "host", HostBuiltin: "compose_up", Cmd: "must-not-run"}}
	failure := errors.New("fixture compose failure")
	composeLifecycleFn = func(infrastructure.ComposeLifecycleOpts) error { return failure }
	runHostCommandFn = func(string, []string) error {
		t.Error("command ran after its builtin failed")
		return nil
	}
	if err := runParallelBatch(&o, 0, 1, 1, map[string]string{}); !errors.Is(err, failure) {
		t.Fatalf("builtin failure was not preserved: %v", err)
	}
}
