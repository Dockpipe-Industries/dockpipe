package application

import (
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestHostStepObservationRetainsAdmittedProjectWithoutWorkdirFlag(t *testing.T) {
	for _, cwd := range []string{"repo", "artifacts"} {
		t.Run(cwd, func(t *testing.T) {
			project := t.TempDir()
			o := baseRunStepsOpts()
			o.opts.Workdir = ""
			o.projectRoot = project
			o.wf.Name = "scope-proof"
			delete(o.envMap, "DOCKPIPE_WORKDIR")
			environment := map[string]string{"DOCKPIPE_WORKFLOW_NAME": "scope-proof"}
			if err := applyStepCWDEnv(&o, domain.Step{Kind: "host", CWD: cwd}, environment, map[string]string{}); err != nil {
				t.Fatal(err)
			}
			if environment["DOCKPIPE_WORKDIR"] != project {
				t.Fatalf("host registry would lose admitted project: %#v", environment)
			}
			expectedCWD := project
			if cwd == "artifacts" {
				expectedCWD = filepath.Join(project, "bin", ".dockpipe", "workflows", "scope-proof", "artifacts")
			}
			if environment["DOCKPIPE_STEP_CWD"] != expectedCWD {
				t.Fatalf("step cwd changed while binding observation root: %#v", environment)
			}
		})
	}
}
