package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func writeNamespaceArchiveFixture(t *testing.T, filter, configNamespace, manifest string) string {
	t.Helper()
	project := t.TempDir()
	// Anchor even an unfiltered fixture so a workflow-owned TMPDIR cannot
	// inherit its parent project's namespace filter.
	projectConfig := `{"schema":1}`
	if filter != "" {
		projectConfig = `{"schema":1,"packages":{"namespace":"` + filter + `"}}`
	}
	if err := os.WriteFile(filepath.Join(project, "dockpipe.config.json"), []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	config := "name: example\nsteps: []\n"
	if configNamespace != "" {
		config += "namespace: " + configNamespace + "\n"
	}
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(source, "package.yml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := PackagesWorkflowsDir(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(store, "dockpipe-workflow-example-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, "workflows/example"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestWorkflowTarballUsesCompiledManifestNamespaceDefault(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	manifest := "schema: 1\nkind: workflow\nname: example\nversion: 1.0.0\nnamespace: example-owner\n"
	project := writeNamespaceArchiveFixture(t, "example-owner", "", manifest)
	uri, err := tryResolveWorkflowTarballURI(project, project, "example")
	if err != nil || uri == "" {
		t.Fatalf("source-hidden compiled default namespace was not admitted: %q, %v", uri, err)
	}
	workflow, err := LoadWorkflow(uri)
	if err != nil || workflow.Name != "example" {
		t.Fatalf("admitted archive did not load its workflow: %#v, %v", workflow, err)
	}
}

func TestWorkflowTarballManifestNamespaceAdmissionRejectsInvalidIdentity(t *testing.T) {
	valid := "schema: 1\nkind: workflow\nname: example\nversion: 1.0.0\nnamespace: example-owner\n"
	for _, test := range []struct {
		name     string
		manifest string
		filter   string
	}{
		{name: "wrong-filter", manifest: valid, filter: "other-owner"},
		{name: "missing-manifest", filter: "example-owner"},
		{name: "missing-namespace", manifest: strings.ReplaceAll(valid, "namespace: example-owner\n", ""), filter: "example-owner"},
		{name: "wrong-name", manifest: strings.ReplaceAll(valid, "name: example", "name: other"), filter: "example-owner"},
		{name: "wrong-kind", manifest: strings.ReplaceAll(valid, "kind: workflow", "kind: resolver"), filter: "example-owner"},
		{name: "wrong-version", manifest: strings.ReplaceAll(valid, "version: 1.0.0", "version: 2.0.0"), filter: "example-owner"},
		{name: "wrong-schema", manifest: strings.ReplaceAll(valid, "schema: 1", "schema: 2"), filter: "example-owner"},
		{name: "invalid-namespace", manifest: strings.ReplaceAll(valid, "example-owner", "../example-owner"), filter: "example-owner"},
		{name: "malformed-yaml", manifest: "namespace: [\n", filter: "example-owner"},
		{name: "duplicate-namespace", manifest: valid + "namespace: other-owner\n", filter: "example-owner"},
		{name: "oversized-manifest", manifest: valid + "description: " + strings.Repeat("x", maxCompatibilityMetadataBytes), filter: "example-owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
			project := writeNamespaceArchiveFixture(t, test.filter, "", test.manifest)
			uri, err := tryResolveWorkflowTarballURI(project, project, "example")
			if err != nil {
				t.Fatal(err)
			}
			if uri != "" {
				t.Fatalf("invalid manifest/filter must not admit archive: %s", uri)
			}
		})
	}
}

func TestWorkflowTarballPreservesExplicitConfigNamespacePrecedence(t *testing.T) {
	// The explicit config namespace remains authoritative; a conflicting
	// manifest must not cause admission under the manifest's namespace.
	manifest := "schema: 1\nkind: workflow\nname: example\nversion: 1.0.0\nnamespace: manifest-owner\n"
	for _, test := range []struct {
		filter   string
		admitted bool
	}{
		{filter: "config-owner", admitted: true},
		{filter: "manifest-owner", admitted: false},
		{filter: "other-owner", admitted: false},
	} {
		t.Run(test.filter, func(t *testing.T) {
			t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
			project := writeNamespaceArchiveFixture(t, test.filter, "config-owner", manifest)
			uri, err := tryResolveWorkflowTarballURI(project, project, "example")
			if err != nil || (uri != "") != test.admitted {
				t.Fatalf("explicit namespace precedence changed: %q, %v", uri, err)
			}
		})
	}
}

func TestWorkflowTarballPreservesUnfilteredLegacyArchiveAdmission(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	project := writeNamespaceArchiveFixture(t, "", "", "")
	uri, err := tryResolveWorkflowTarballURI(project, project, "example")
	if err != nil || uri == "" {
		t.Fatalf("legacy unfiltered archive admission changed: %q, %v", uri, err)
	}
}
