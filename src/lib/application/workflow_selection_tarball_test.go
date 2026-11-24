package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func compileSelectionWorkflow(t *testing.T, project string) string {
	t.Helper()
	source := t.TempDir()
	config := "name: locator-fixture\nnamespace: fixture-owner\ndocker_preflight: false\nsteps:\n  - kind: host\n    run: report.sh\n"
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "report.sh"), []byte("selected archive asset"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", project, "--from", source}); err != nil {
		t.Fatal(err)
	}
	store, err := infrastructure.PackagesWorkflowsDir(project)
	if err != nil {
		t.Fatal(err)
	}
	archives, err := filepath.Glob(filepath.Join(store, "dockpipe-workflow-locator-fixture-*.tar.gz"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("expected one compiled fixture archive, got %v, %v", archives, err)
	}
	return "tar://" + archives[0] + "##workflows/locator-fixture/config.yml"
}

func TestWorkflowSelectionExplicitCompiledLocatorPreservesArchiveIdentity(t *testing.T) {
	project := t.TempDir()
	locator := compileSelectionWorkflow(t, project)
	var loaded string
	selection, err := selectWorkflow(workflowSelectionRequest{
		effectiveWorkdir: project,
		workflowFile:     locator,
	}, func(config string) (*domain.Workflow, error) {
		loaded = config
		return infrastructure.LoadWorkflow(config)
	})
	if err != nil {
		t.Fatal(err)
	}
	if loaded != locator || selection.config != locator || !selection.stepsMode {
		t.Fatalf("explicit archive identity changed: loader=%q, selection=%+v", loaded, selection)
	}
	asset := filepath.Join(selection.root, "report.sh")
	content, err := os.ReadFile(asset)
	if err != nil || string(content) != "selected archive asset" {
		t.Fatalf("asset did not resolve beside extracted selected config: %q, %v", content, err)
	}
}

func TestRunExplicitCompiledWorkflowLocatorReachesSelectedAssets(t *testing.T) {
	withRunSeams(t)
	project := t.TempDir()
	locator := compileSelectionWorkflow(t, project)
	repoRootAppFn = func() (string, error) { return t.TempDir(), nil }
	called := false
	runStepsAppFn = func(options runStepsOpts) error {
		called = true
		asset := infrastructure.ResolveWorkflowScript("report.sh", options.wfRoot, options.repoRoot, options.projectRoot)
		content, err := os.ReadFile(asset)
		if err != nil || string(content) != "selected archive asset" {
			t.Fatalf("CLI selected another workflow's assets: %q, %v", content, err)
		}
		return nil
	}
	if err := Run([]string{"--workdir", project, "--workflow-file", locator}, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("explicit compiled locator did not reach execution")
	}
}

func TestWorkflowSelectionRejectsUnsafeExplicitArchiveMemberBeforeLoading(t *testing.T) {
	for _, member := range []string{"../config.yml", "/config.yml", "C:/config.yml", "workflows/../config.yml", `workflows\fixture\config.yml`, ""} {
		t.Run(member, func(t *testing.T) {
			_, err := selectWorkflow(workflowSelectionRequest{
				effectiveWorkdir: t.TempDir(),
				workflowFile:     "tar:///missing/fixture.tar.gz##" + member,
			}, func(string) (*domain.Workflow, error) {
				t.Fatal("unsafe member reached workflow loading")
				return nil, nil
			})
			if err == nil {
				t.Fatal("unsafe member was accepted")
			}
		})
	}
}
