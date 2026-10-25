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

func TestRunSessionCompletionOrdersEffectsAndStopsOnFailure(t *testing.T) {
	for _, failedStage := range []string{"", "after", "finalize", "commit"} {
		t.Run("failure="+failedStage, func(t *testing.T) {
			root := t.TempDir()
			after := filepath.Join(root, "after.sh")
			if err := os.WriteFile(after, []byte("#!/bin/sh\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			expectedFailure := errors.New("effect failed")
			var events []string
			effect := func(stage string) error {
				events = append(events, stage)
				if stage == failedStage {
					return expectedFailure
				}
				return nil
			}
			session := newRunSession([]string{"RESULT=old"}, runExecutionServices{
				hostScript: func(_ string, environment []string) error {
					if value := domain.EnvSliceToMap(environment)["RESULT"]; value != "current" {
						t.Fatalf("after hook received stale result %q", value)
					}
					return effect("after")
				},
				finalize: func(*infrastructure.GitSession, *domain.Workflow) error {
					return effect("finalize")
				},
				commit: func(string, string, string, bool) error {
					return effect("commit")
				},
			})
			session.env["RESULT"] = "current"
			err := session.complete(runCompletion{repoRoot: root, afterScripts: []string{after}}, root, true)
			want := []string{"after", "finalize", "commit"}
			switch failedStage {
			case "after":
				want = want[:1]
			case "finalize":
				want = want[:2]
			}
			if !reflect.DeepEqual(events, want) {
				t.Fatalf("effects=%v want=%v", events, want)
			}
			if failedStage == "" && err != nil {
				t.Fatal(err)
			}
			if failedStage != "" && !errors.Is(err, expectedFailure) {
				t.Fatalf("lost failure: %v", err)
			}
		})
	}
}

func TestRunResolverAfterHookReceivesMergedStepOutputs(t *testing.T) {
	withRunSeams(t)
	withRunStepsSeams(t)
	root := writeAdmissionFixture(t, "")
	for name, contents := range map[string]string{
		"config.yml":        "name: caller\nresolver: delegate\nstrategy: report\n",
		"strategies/report": "DOCKPIPE_STRATEGY_AFTER=scripts/report.sh\n",
		"scripts/report.sh": "#!/bin/sh\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repoRootAppFn = func() (string, error) { return root, nil }
	runStepsAppFn = runSteps
	runHostCommandFn = func(_ string, environment []string) error {
		values := domain.EnvSliceToMap(environment)
		directory := values["DOCKPIPE_OUTPUT_ROOT"]
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(directory, domain.DefaultOutputsEnvRel), []byte("CHILD_RESULT=ready\n"), 0o600)
	}
	afterCalled := false
	runHostScriptAppFn = func(_ string, environment []string) error {
		afterCalled = true
		if got := domain.EnvSliceToMap(environment)["CHILD_RESULT"]; got != "ready" {
			t.Fatalf("after hook missed child result: %q", got)
		}
		return nil
	}
	if err := Run([]string{"--workflow-file", filepath.Join(root, "config.yml"), "--workdir", root, "--"}, nil); err != nil {
		t.Fatal(err)
	}
	if !afterCalled {
		t.Fatal("after hook did not run")
	}
}
