package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectDisposableRemovalTreeReportsLogicalBytes(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(root, "a"):      "abc",
		filepath.Join(root, "b", "c"): "12345",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := InspectDisposableRemovalTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.LogicalBytes != 8 || summary.Files != 2 {
		t.Fatalf("summary = %#v want 8 bytes and 2 files", summary)
	}
}

func TestValidateDisposableRemovalPathRejectsFilesystemSubstitution(t *testing.T) {
	boundary := t.TempDir()
	target := filepath.Join(boundary, "bin", ".dockpipe")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	boundary = filepath.Clean(boundary)
	target = filepath.Clean(target)
	err := validateDisposableRemovalPath(boundary, target, func(path string) (string, error) {
		if filepath.Clean(path) == target {
			return "substituted-filesystem", nil
		}
		return "checkout-filesystem", nil
	})
	if err == nil || !strings.Contains(err.Error(), "filesystem boundary") {
		t.Fatalf("filesystem substitution was accepted: %v", err)
	}
}

func TestValidateDisposableRemovalPathAllowsMissingTrailingTarget(t *testing.T) {
	boundary := t.TempDir()
	target := filepath.Join(boundary, "bin", ".dockpipe")
	if err := ValidateDisposableRemovalPath(boundary, target); err != nil {
		t.Fatalf("missing disposable target: %v", err)
	}
}
