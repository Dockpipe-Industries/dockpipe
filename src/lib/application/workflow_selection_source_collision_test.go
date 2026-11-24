package application

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/infrastructure"
)

func TestWorkflowSelectionDoesNotReuseAnotherSourcesCompiledArchive(t *testing.T) {
	repo := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repo)
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", t.TempDir())
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "dockpipe.config.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Both sources predate the first compilation. A different source must not
	// qualify for the existing archive solely because its timestamps are older.
	oldTime := time.Now().Add(-time.Hour)
	for index, name := range []string{"first-source", "second-source"} {
		source := filepath.Join(project, "workflows", name)
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		config := fmt.Sprintf(`name: shared-workflow
runtime: dockerimage
isolate: debian:fixture
security:
  network:
    mode: offline
steps:
  - id: run
    cmd: echo %s; bash relative.sh
    security:
      process:
        resources:
          memory: %dm
`, name, (index+1)*64)
		files := map[string]string{
			"config.yml":  config,
			"package.yml": "schema: 1\nkind: workflow\nname: shared-family\nversion: 1.2.3\n",
			"relative.sh": "echo " + name + "-asset\n",
		}
		for filename, contents := range files {
			path := filepath.Join(source, filename)
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(source, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	selectSource := func(name string) workflowSelection {
		t.Helper()
		selection, err := selectWorkflow(workflowSelectionRequest{
			repoRoot: repo, effectiveWorkdir: project, workflowName: name, compileDepsWanted: true,
		}, infrastructure.LoadWorkflow)
		if err != nil {
			t.Fatal(err)
		}
		return selection
	}

	first := selectSource("first-source")
	if first.workflow.Steps[0].Cmd != "echo first-source; bash relative.sh" {
		t.Fatalf("first selection did not compile its source: %+v", first.workflow.Steps)
	}
	second := selectSource("second-source")
	if second.workflow.Steps[0].Cmd != "echo second-source; bash relative.sh" {
		t.Errorf("second source executed another source's cached command: %q", second.workflow.Steps[0].Cmd)
	}
	asset, err := os.ReadFile(filepath.Join(second.root, "relative.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if string(asset) != "echo second-source-asset\n" {
		t.Errorf("second source used another source's cached asset: %q", asset)
	}
	options := infrastructure.RunOpts{}
	_, err = runtimepolicy.ApplyCompiledRuntimePolicyForStep(&options, second.workflow,
		second.config, second.root, second.workflow.Steps[0], "run")
	if err != nil {
		t.Fatal(err)
	}
	if options.MemoryLimit != "128m" {
		t.Errorf("second source used another source's cached policy: memory=%q", options.MemoryLimit)
	}
}
