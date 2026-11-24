package application

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func boolPtr(v bool) *bool { return &v }

// TestValidateParallelOutputPaths rejects duplicate outputs paths within one async group.
func TestValidateParallelOutputPaths(t *testing.T) {
	paths := []string{"a.env", "b.env"}
	if err := validateParallelOutputPaths(paths, 0); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	paths[1] = "a.env"
	if err := validateParallelOutputPaths(paths, 0); err == nil || !strings.Contains(err.Error(), "duplicate outputs path") {
		t.Fatalf("expected duplicate outputs path error, got %v", err)
	}
}

// TestValidateParallelNoHostCommit forbids bundled commit-worktree in parallel async steps.
func TestValidateParallelNoHostCommit(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf: &domain.Workflow{
			Steps: []domain.Step{
				{Action: "scripts/commit-worktree.sh", Blocking: boolPtr(false)},
			},
		},
	}
	err := validateParallelNoHostCommit(o, 0, 1)
	if err == nil || !strings.Contains(err.Error(), "cannot run inside an async group") {
		t.Fatalf("expected host-commit-in-parallel error, got %v", err)
	}

	o.wf.Steps[0].Action = "scripts/print-summary.sh"
	if err := validateParallelNoHostCommit(o, 0, 1); err != nil {
		t.Fatalf("expected non-commit action to pass, got %v", err)
	}
}

// TestParseStepArgv splits shell command lines for container argv and treats blank as nil argv.
func TestParseStepArgv(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
		wantErr bool
	}{
		{name: "simple", command: "echo hello", want: []string{"echo", "hello"}},
		{name: "blank", command: "   "},
		{name: "quoted spaces", command: `tool "two words" 'three words'`, want: []string{"tool", "two words", "three words"}},
		{name: "empty argument", command: `tool ""`, want: []string{"tool", ""}},
		{name: "hash after empty quotes", command: `tool ""#literal`, want: []string{"tool", "#literal"}},
		{name: "literal environment syntax", command: `tool '$HOME' "$HOME"`, want: []string{"tool", "$HOME", "$HOME"}},
		{name: "quoted redirect digits", command: `tool "2">file`, want: []string{"tool", "2"}},
		{name: "unclosed quote", command: `tool "unfinished`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			argv, err := parseStepArgv(test.command)
			if (err != nil) != test.wantErr {
				t.Fatalf("parse error = %v, want error %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(argv, test.want) {
				t.Fatalf("argv = %#v, want %#v", argv, test.want)
			}
		})
	}
}

// TestApplyStepEnvOverridesRespectsLocks applies step vars to unlocked keys only (CLI-locked keys unchanged).
func TestApplyStepEnvOverridesRespectsLocks(t *testing.T) {
	o := &runStepsOpts{
		envMap: map[string]string{"LOCKED": "old", "FREE": "old"},
		locked: map[string]bool{"LOCKED": true},
	}
	dockerEnv := map[string]string{"LOCKED": "old", "FREE": "old"}
	step := domain.Step{
		Vars: map[string]string{
			"LOCKED": "new",
			"FREE":   "new",
		},
	}
	if err := applyStepEnvOverrides(o, step, 0, o.envMap, dockerEnv); err != nil {
		t.Fatal(err)
	}

	if got := o.envMap["LOCKED"]; got != "old" {
		t.Fatalf("locked key mutated: %q", got)
	}
	if got := o.envMap["FREE"]; got != "new" {
		t.Fatalf("free key not updated: %q", got)
	}
	if got := dockerEnv["LOCKED"]; got != "old" {
		t.Fatalf("docker locked key mutated: %q", got)
	}
	if got := dockerEnv["FREE"]; got != "new" {
		t.Fatalf("docker free key not updated: %q", got)
	}
}

func TestApplyContainerPathEnvMapsDockpipeBin(t *testing.T) {
	workHost := filepath.Join(string(filepath.Separator), "repo")
	dockpipeBin := filepath.Join(workHost, "src", "bin", "dockpipe")
	env := map[string]string{
		"DOCKPIPE_WORKDIR": workHost,
		"DOCKPIPE_BIN":     dockpipeBin,
		"PATH":             "/usr/local/bin:/usr/bin:/bin",
	}

	applyContainerPathEnv(env, workHost, "")

	if got, want := env["DOCKPIPE_BIN"], "/work/src/bin/dockpipe"; got != want {
		t.Fatalf("DOCKPIPE_BIN = %q want %q", got, want)
	}
	if got, want := env["PATH"], "/usr/local/bin:/usr/bin:/bin"; got != want {
		t.Fatalf("PATH = %q want unchanged %q", got, want)
	}
}

func TestBuildStepContainerPassesDockpipeBinAsContainerPath(t *testing.T) {
	workHost := filepath.Join(string(filepath.Separator), "repo")
	o := &runStepsOpts{
		wf:       &domain.Workflow{},
		wfRoot:   filepath.Join(workHost, "workflows", "ci", "test"),
		repoRoot: workHost,
		envMap: map[string]string{
			"DOCKPIPE_WORKDIR": workHost,
			"DOCKPIPE_BIN":     filepath.Join(workHost, "src", "bin", "dockpipe"),
		},
		opts: &CliOpts{},
	}
	step := domain.Step{Isolate: "alpine", Cmd: "dockpipe scope artifacts demo"}

	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, o.envMap, map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := domain.EnvSliceToMap(runOpts.ExtraEnv)
	if got, want := env["DOCKPIPE_BIN"], "/work/src/bin/dockpipe"; got != want {
		t.Fatalf("DOCKPIPE_BIN = %q want %q", got, want)
	}
	if _, ok := env["PATH"]; ok {
		t.Fatalf("PATH should not be injected into container env: %#v", env)
	}
}
