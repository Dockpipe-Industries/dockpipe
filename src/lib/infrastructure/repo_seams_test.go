package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoRootUsesProjectContextWithoutMaterialization(t *testing.T) {
	t.Setenv("DOCKPIPE_REPO_ROOT", "")
	root := t.TempDir()
	cache := filepath.Join(t.TempDir(), "unused-cache")
	t.Setenv("DOCKPIPE_BUNDLED_CACHE", cache)
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte(`{"schema":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(root, "nested")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(subdir)
	got, err := RepoRoot()
	if err != nil || got != root {
		t.Fatalf("RepoRoot = %q, %v; want %q", got, err, root)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("project resolution created bundle cache: %v", err)
	}
}

func TestRepoRootExplicitOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	got, err := RepoRoot()
	if err != nil || got != root {
		t.Fatalf("RepoRoot = %q, %v; want %q", got, err, root)
	}
}
