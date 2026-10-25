package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestedWorkflowSearchUsesOnlyDeclaredRoots(t *testing.T) {
	root := t.TempDir()
	declared := filepath.Join(root, "vendor", "workflows")
	for _, base := range []string{filepath.Join(root, "src", "core", "workflows"), declared} {
		path := filepath.Join(base, "namespace", "demo", "config.yml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("name: demo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := nestedWorkflowConfigCandidates(root, "demo", []string{declared})
	want := filepath.Join(declared, "namespace", "demo", "config.yml")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("nested candidates = %v, want only %q", got, want)
	}
}
