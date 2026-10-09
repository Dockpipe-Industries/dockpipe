package packagecompile

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestWorkflowBatchPrunesByDeclaredIdentity(t *testing.T) {
	root := t.TempDir()
	sources := filepath.Join(root, "sources")
	for _, fixture := range []struct{ folder, name string }{
		{"first/shared", "example.first"},
		{"second/shared", "example.second"},
	} {
		directory := filepath.Join(sources, fixture.folder)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "config.yml"), []byte("name: "+fixture.name+"\nsteps: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination, err := infrastructure.PackagesWorkflowsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(destination, "dockpipe-workflow-removed-0.0.0.tar.gz")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := cmdPackageCompileWorkflowsBatch([]string{"--workdir", root, "--from", sources, "--prune-stale"}); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"example.first", "example.second"} {
			archive := filepath.Join(destination, "dockpipe-workflow-"+name+"-0.0.0.tar.gz")
			if _, err := packagebuild.ReadFileFromTarGz(archive, "workflows/"+name+"/config.yml"); err != nil {
				t.Fatalf("declared workflow lost on attempt %d: %v", attempt, err)
			}
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Fatalf("stale archive was not pruned: %v", err)
		}
	}
}
