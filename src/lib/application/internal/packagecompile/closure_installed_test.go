package packagecompile

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestCompileClosureRetainsInstalledUserArchive(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	producer := t.TempDir()
	consumer := t.TempDir()
	source := t.TempDir()
	for _, root := range []string{producer, consumer} {
		if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, contents := range map[string]string{
		"package.yml": "schema: 1\nkind: workflow\nname: installed-proof\nversion: 1.0.0\nnamespace: example-owner\n",
		"config.yml":  "name: installed-proof\nvault: none\nsteps:\n  - kind: host\n    cmd: echo installed-proof\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entry, err := compileWorkflowEntry(producer, source, "", false)
	if err != nil {
		t.Fatal(err)
	}
	archive, member, ok := infrastructure.SplitTarWorkflowURI(entry)
	if !ok {
		t.Fatalf("compiler returned no archive: %q", entry)
	}
	store, err := infrastructure.GlobalPackagesWorkflowsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(store, filepath.Base(archive))
	if err := os.Rename(archive, installed); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := compileClosureEntryForWorkflow(consumer, "installed-proof", false)
	if err != nil {
		t.Fatal(err)
	}
	if selected != infrastructure.FormatTarWorkflowURI(installed, member) {
		t.Fatalf("closure changed the selected installed archive: %q", selected)
	}
	after, err := os.ReadFile(installed)
	if err != nil || string(after) != string(before) {
		t.Fatalf("installed archive changed: %v", err)
	}
}
