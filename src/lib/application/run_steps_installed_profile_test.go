package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func installIsolationFixture(t *testing.T, project, kind, name string, files map[string]string) {
	t.Helper()
	source := t.TempDir()
	for name, body := range files {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := infrastructure.PackagesRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(store, kind+"s")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "dockpipe-"+kind+"-"+name+"-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, kind+"s/"+name); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
}

func TestRunNamedInstalledWorkflowLoadsSourceHiddenStepResolver(t *testing.T) {
	withRunSeams(t)
	project, core := t.TempDir(), t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	repoRootAppFn = func() (string, error) { return core, nil }
	installIsolationFixture(t, project, "resolver", "installed-tool", map[string]string{
		"profile": "DOCKPIPE_RESOLVER_CMD=installed-tool\nDOCKPIPE_RESOLVER_CONTAINER_AUTH_DIR=/tool/auth\n",
	})
	installIsolationFixture(t, project, "workflow", "installed-job", map[string]string{
		"config.yml": "name: installed-job\nnamespace: dockpipeproject\ndocker_preflight: false\nsteps:\n  - id: execute\n    resolver: installed-tool\n    isolate: fixture-image\n",
	})
	called := false
	runStepsAppFn = func(options runStepsOpts) error {
		called = true
		if options.repoRoot != core || options.projectRoot != project {
			t.Fatalf("test must exercise separate core/project roots: %#v", options)
		}
		step := options.wf.Steps[0]
		assignments, _, _, err := loadStepResolver(&options, step, 0)
		if err != nil {
			return err
		}
		if assignments.Cmd != "installed-tool" || assignments.ContainerAuthDir != "/tool/auth" {
			t.Fatalf("installed profile assignments lost: %#v", assignments)
		}
		_, _, _, environment, err := loadStepResolverWithProfileEnv(&options, step, 0)
		if err != nil {
			return err
		}
		resolved, err := infrastructure.ResolveResolverFilePath(project, "installed-tool")
		if err != nil {
			return err
		}
		if environment["DOCKPIPE_RESOLVER_PACKAGE_ROOT"] != filepath.Dir(resolved) {
			t.Fatalf("profile root is not its installed archive: %#v", environment)
		}
		return nil
	}
	if err := Run([]string{"--workdir", project, "--workflow", "installed-job"}, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("named installed workflow never reached pre-container profile loading")
	}
}

func TestStepResolverPreservesMergedProjectPrecedence(t *testing.T) {
	core, project := t.TempDir(), t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	writeTestCoreResolver(t, core, "example", "DOCKPIPE_RESOLVER_CMD=core\nDOCKPIPE_RESOLVER_AUTH_DIR=.core-auth\n")
	installIsolationFixture(t, project, "resolver", "example", map[string]string{
		"profile": "DOCKPIPE_RESOLVER_CMD=project\n",
	})
	options := runStepsOpts{repoRoot: core, projectRoot: project, wf: &domain.Workflow{}}
	step := domain.Step{ID: "execute", Resolver: "example"}
	assignments, _, _, err := loadStepResolver(&options, step, 0)
	if err != nil || assignments.Cmd != "project" || assignments.AuthDir != ".core-auth" {
		t.Fatalf("project override/base fields not merged: %#v, %v", assignments, err)
	}
	assignments, _, _, environment, err := loadStepResolverWithProfileEnv(&options, step, 0)
	if err != nil || assignments.Cmd != "project" || environment["DOCKPIPE_RESOLVER_AUTH_DIR"] != ".core-auth" {
		t.Fatalf("profile-env loader differs from merged semantics: %#v, %v", environment, err)
	}
}

func TestStepResolverPreservesMissingAndConflictingProfileFailures(t *testing.T) {
	for _, body := range []string{"", "DOCKPIPE_RESOLVER_WORKFLOW=child\nDOCKPIPE_RESOLVER_HOST_ISOLATE=local.sh\n"} {
		t.Run(body, func(t *testing.T) {
			core, project := t.TempDir(), t.TempDir()
			t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
			if body != "" {
				installIsolationFixture(t, project, "resolver", "example", map[string]string{"profile": body})
			}
			options := runStepsOpts{repoRoot: core, projectRoot: project, wf: &domain.Workflow{}}
			step := domain.Step{ID: "execute", Resolver: "example"}
			_, _, _, err := loadStepResolver(&options, step, 0)
			_, _, _, _, envErr := loadStepResolverWithProfileEnv(&options, step, 0)
			expected := "isolation profile"
			if body != "" {
				expected = "set only one"
			}
			if err == nil || envErr == nil || !strings.Contains(err.Error(), expected) || !strings.Contains(envErr.Error(), expected) {
				t.Fatalf("profile failure must remain explicit in both loaders: %v; %v", err, envErr)
			}
		})
	}
}
