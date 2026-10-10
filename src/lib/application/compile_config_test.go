package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestCmdPackageCompileAllUsesCanonicalWorkflowRoots(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	for _, dir := range []string{
		filepath.Join(repo, "src", "core", "runtimes"),
		filepath.Join(repo, "vendor", "packages", "resolvers", "fixture", "profile"),
		filepath.Join(repo, "vendor", "packages", "fixture-workflow"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "VERSION"), []byte("0.6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectConfig := `{"schema":1,"compile":{"workflows":["vendor/packages"]},"packages":{"namespace":"fixture"}}`
	if err := os.WriteFile(filepath.Join(repo, domain.DockpipeProjectConfigFileName), []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "vendor", "packages", "resolvers", "fixture", "profile", "env"), []byte("FIXTURE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "vendor", "packages", "fixture-workflow", "config.yml"), []byte("name: fixture-workflow\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdPackageCompileAll([]string{"--workdir", repo, "--force"}); err != nil {
		t.Fatalf("compile all: %v", err)
	}
	for label, pattern := range map[string]string{
		"core":     filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "core", "dockpipe-core-*.tar.gz"),
		"resolver": filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "resolvers", "dockpipe-resolver-fixture-*.tar.gz"),
		"workflow": filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "workflows", "dockpipe-workflow-fixture-workflow-*.tar.gz"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("%s outputs = %v for %s", label, matches, pattern)
		}
	}
}
