package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/infrastructure"
)

func TestWorkflowSelectionExecutesExactCompiledSourceAndStepPolicy(t *testing.T) {
	repo := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repo)
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	project := t.TempDir()
	source := filepath.Join(project, "workflows", "source-choice")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(project, "dockpipe.config.json"): "{}\n",
		filepath.Join(source, "package.yml"):           "schema: 1\nkind: workflow\nname: package-family\nversion: 1.2.3\n",
		filepath.Join(source, "relative.sh"):           "echo compiled-source-fixture\n",
		filepath.Join(source, "config.yml"): `name: selected-workflow
runtime: dockerimage
resolver: workflow-tool
isolate: debian:fixture
security:
  network:
    mode: offline
  filesystem:
    root: readonly
steps:
  - id: run
    resolver: step-tool
    isolate: debian:step-fixture
    cmd: bash relative.sh
    security:
      process:
        pid_limit: 32
        resources:
          memory: 64m
`,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A same-named workflow in another store must not replace the selected source.
	decoy := filepath.Join(t.TempDir(), "workflows", "source-choice")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decoy, "config.yml"), []byte("name: decoy\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", filepath.Dir(filepath.Dir(decoy)))
	request := workflowSelectionRequest{
		repoRoot: repo, effectiveWorkdir: project, workflowName: "source-choice", compileDepsWanted: true,
	}
	var compiledModificationTime int64
	for attempt := range 2 {
		selection, err := selectWorkflow(request, infrastructure.LoadWorkflow)
		if err != nil {
			t.Fatal(err)
		}
		archive, member, ok := infrastructure.SplitTarWorkflowURI(selection.config)
		if !ok || !strings.HasSuffix(archive, "dockpipe-workflow-selected-workflow-1.2.3.tar.gz") || member != "workflows/selected-workflow/config.yml" {
			t.Fatalf("attempt %d lost the exact compiled entry: %+v", attempt, selection)
		}
		information, err := os.Stat(archive)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			compiledModificationTime = information.ModTime().UnixNano()
		} else if information.ModTime().UnixNano() != compiledModificationTime {
			t.Fatal("second selection rebuilt the archive instead of exercising the cache-hit handoff")
		}
		if selection.workflow.Name != "selected-workflow" || selection.root == source {
			t.Fatalf("attempt %d executed source or decoy instead of compiled truth: %+v", attempt, selection)
		}
		asset, err := os.ReadFile(filepath.Join(selection.root, "relative.sh"))
		if err != nil || string(asset) != "echo compiled-source-fixture\n" {
			t.Fatalf("compiled relative asset: %q, %v", asset, err)
		}
		options := infrastructure.RunOpts{}
		manifest, err := runtimepolicy.ApplyCompiledRuntimePolicyForStep(&options, selection.workflow,
			selection.config, selection.root, selection.workflow.Steps[0], "run")
		if err != nil {
			t.Fatal(err)
		}
		if manifest == nil || manifest.ResolverProfile != "step-tool" || manifest.Image.Ref != "debian:step-fixture" {
			t.Fatalf("compiled custom image/resolver selection lost: %+v", manifest)
		}
		if options.NetworkMode != "none" || !options.ReadOnlyRootFS || options.PIDLimit != 32 || options.MemoryLimit != "64m" {
			t.Fatalf("compiled policy was not applied: %+v", options)
		}
	}
}

func TestUncompiledDeclaredPolicyIsRefusedBeforeContainerEffects(t *testing.T) {
	for _, selection := range []string{"file", "no-compile"} {
		t.Run(selection, func(t *testing.T) {
			withRunSeams(t)
			root := t.TempDir()
			workflow := filepath.Join(root, "workflows", "declared-policy")
			if err := os.MkdirAll(workflow, 0o755); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(workflow, "config.yml")
			if err := os.WriteFile(config, []byte("name: declared-policy\nruntime: dockerimage\nisolate: debian:fixture\nsecurity:\n  network:\n    mode: offline\nsteps: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			repoRootAppFn = func() (string, error) { return root, nil }
			loadWorkflowAppFn = infrastructure.LoadWorkflow
			templateBuildAppFn = func(string, string) (string, string, string, bool, error) {
				return "", "", "", false, nil
			}
			dockerBuildAppFn = func(string, string, string) error {
				t.Fatal("missing policy reached image build")
				return nil
			}
			dockerPullAppFn = func(string) error {
				t.Fatal("missing policy reached image pull")
				return nil
			}
			runContainerAppFn = func(infrastructure.RunOpts, []string) (int, error) {
				t.Fatal("missing policy reached container spawn")
				return 0, nil
			}
			arguments := []string{"--workdir", root, "--no-vault"}
			if selection == "file" {
				arguments = append(arguments, "--workflow-file", config)
			} else {
				arguments = append(arguments, "--workflow", "declared-policy", "--no-compile-deps")
			}
			arguments = append(arguments, "--", "true")
			err := Run(arguments, nil)
			if err == nil || !strings.Contains(err.Error(), "no compiled runtime manifest") {
				t.Fatalf("uncompiled declared policy was not refused: %v", err)
			}
		})
	}
}
