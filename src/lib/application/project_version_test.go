package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/engineversion"
)

func TestRunRejectsProjectVersionBeforeWorkflowResolution(t *testing.T) {
	withRunSeams(t)
	previous := engineversion.Current()
	engineversion.Configure("1.2.4")
	t.Cleanup(func() { engineversion.Configure(previous) })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte(`{"dockpipe_version":"1.2.3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// A project VERSION file cannot impersonate the running executable.
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRootAppFn = func() (string, error) {
		t.Fatal("version mismatch reached workflow resolution")
		return root, nil
	}
	runStepsAppFn = func(runStepsOpts) error {
		t.Fatal("version mismatch executed workflow steps")
		return nil
	}
	err := Run([]string{"--workdir", root, "--workflow", "example"}, nil)
	if err == nil || !strings.Contains(err.Error(), "requires Dockpipe 1.2.3 exactly; running 1.2.4") {
		t.Fatalf("expected exact version failure, got %v", err)
	}
	if err := checkCLIProjectVersion([]string{"package", "build", "source", "--workdir", root}); err == nil {
		t.Fatal("package build bypassed the repository version")
	}
	for _, args := range [][]string{
		{"--workdir", root, "--help"},
		{"--workdir", root, "--force", "--help"},
		{"package", "--help", "--workdir", root},
		{"install", "core", "--workdir", root},
		{"--version"},
	} {
		if err := checkCLIProjectVersion(args); err != nil {
			t.Errorf("informational/bootstrap command rejected: %v", err)
		}
	}
	if err := checkCLIProjectVersion([]string{"--workdir", root, "--workflow", "example", "--", "--help"}); err == nil {
		t.Fatal("payload help bypassed repository version")
	}
	engineversion.Configure("1.2.3")
	if err := checkCLIProjectVersion([]string{"--workdir", root, "--workflow", "example"}); err != nil {
		t.Fatalf("matching version rejected: %v", err)
	}
	configPath := filepath.Join(root, "workflow.yml")
	config := "name: pinned\ndocker_preflight: false\nsteps:\n  - kind: host\n    run: script.sh\n"
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRootAppFn = func() (string, error) { return root, nil }
	called := false
	runStepsAppFn = func(runStepsOpts) error {
		called = true
		return nil
	}
	if err := Run([]string{"--workdir", root, "--workflow-file", configPath}, nil); err != nil {
		t.Fatalf("matching version did not reach workflow: %v", err)
	}
	if !called {
		t.Fatal("matching version did not execute workflow steps")
	}
}

func TestProjectMinimumAdmissionUsesSelectedWorkdir(t *testing.T) {
	withRunSeams(t)
	previous := engineversion.Current()
	t.Cleanup(func() { engineversion.Configure(previous) })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte(`{"min_dockpipe_version":"0.6.11"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)
	repoRootAppFn = func() (string, error) {
		t.Fatal("minimum version failure reached workflow resolution")
		return root, nil
	}
	engineversion.Configure("0.6.10")
	if err := Run([]string{"--workdir", child, "--workflow", "example"}, nil); err == nil || !strings.Contains(err.Error(), "or newer") {
		t.Fatalf("expected minimum admission failure, got %v", err)
	}
	for _, version := range []string{"0.6.10", "0.6.11", "0.6.33", "dev"} {
		engineversion.Configure(version)
		for _, args := range [][]string{
			{"--workdir", child, "--workflow", "example"},
			{"package", "build", "source", "--workdir=" + child},
			{"--workflow", "example"},
		} {
			err := checkCLIProjectVersion(args)
			allowed := version == "0.6.11" || version == "0.6.33"
			if (err == nil) != allowed {
				t.Errorf("version %s args %v allowed=%v: %v", version, args, allowed, err)
			}
		}
		if err := checkCLIProjectVersion([]string{"install", "core", "--workdir", child}); err != nil {
			t.Errorf("repair installation rejected: %v", err)
		}
	}
}
