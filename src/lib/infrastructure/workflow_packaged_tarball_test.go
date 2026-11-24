package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestPackagedWorkflowSelectorsResolveCompiledArchives(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	manifest := "schema: 1\nkind: workflow\nname: example\nversion: 1.0.0\nnamespace: example-owner\n"
	project := writeNamespaceArchiveFixture(t, "", "example-owner", manifest)
	for _, selector := range []struct {
		name     string
		resolve  func(string, string, string, string) (string, error)
		identity string
	}{
		{name: "nested namespace", resolve: ResolvePackagedWorkflowConfigPath, identity: "example-owner"},
		{name: "top-level package", resolve: ResolvePackageWorkflowConfigPath, identity: "example"},
	} {
		t.Run(selector.name, func(t *testing.T) {
			locator, err := selector.resolve(project, project, "example", selector.identity)
			if err != nil || !strings.HasPrefix(locator, "tar://") {
				t.Fatalf("compiled workflow was not selected: %q, %v", locator, err)
			}
			if _, err := LoadWorkflow(locator); err != nil {
				t.Fatalf("selected workflow cannot load: %v", err)
			}
			if locator, err := selector.resolve(project, project, "example", "other-owner"); err == nil {
				t.Fatalf("wrong selector admitted %q", locator)
			}
		})
	}
}

func TestPackagedNamespaceSearchContinuesAfterNewerDifferentOwner(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	manifest := "schema: 1\nkind: workflow\nname: example\nversion: 1.0.0\n"
	project := writeNamespaceArchiveFixture(t, "", "expected-owner", manifest)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: example\nnamespace: different-owner\nsteps: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := PackagesWorkflowsDir(project)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(store, "dockpipe-workflow-example-2.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, "workflows/example"); err != nil {
		t.Fatal(err)
	}
	locator, err := ResolvePackagedWorkflowConfigPath(project, project, "example", "expected-owner")
	if err != nil || !strings.Contains(locator, "example-1.0.0.tar.gz##") {
		t.Fatalf("different owner hid the matching archive: %q, %v", locator, err)
	}
}

func TestPackageArchiveSelectorRejectsUnboundManifest(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	for _, manifest := range []string{
		"",
		"schema: 1\nkind: workflow\nname: other\nversion: 1.0.0\n",
		"schema: 1\nkind: workflow\nname: example\nversion: 2.0.0\n",
	} {
		project := writeNamespaceArchiveFixture(t, "", "example-owner", manifest)
		if locator, err := ResolvePackageWorkflowConfigPath(project, project, "example", "example"); err == nil {
			t.Fatalf("unbound archive manifest admitted %q", locator)
		}
	}
}
