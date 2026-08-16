package clonecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestRunRejectsPackageThatDisallowsCloning(t *testing.T) {
	dir := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: paid\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\nname: paid\nversion: 1.0.0\nkind: workflow\nrequires_capabilities: [workflow.paid]\nallow_clone: false\ndistribution: binary\n"
	if err := os.WriteFile(filepath.Join(source, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, filepath.Join(store, "dockpipe-workflow-paid-1.0.0.tar.gz"), "workflows/paid"); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"paid", "--workdir", dir, "--to", filepath.Join(dir, "workflows", "paid")})
	if err == nil || !strings.Contains(err.Error(), "does not allow cloning") {
		t.Fatalf("expected clone denied, got %v", err)
	}
}
