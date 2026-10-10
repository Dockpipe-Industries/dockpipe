package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowCompileRootsCachedUsesCanonicalConfig(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	root := filepath.Join(repo, "vendor", "dockpipe-pkgs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"schema":1,"compile":{"workflows":["vendor/dockpipe-pkgs"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	got := WorkflowCompileRootsCached(repo)
	if len(got) != 1 || got[0] != root {
		t.Fatalf("workflow roots = %v, want [%s]", got, root)
	}
}
