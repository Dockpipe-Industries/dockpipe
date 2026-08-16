package packageversion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthoredReadsTrimmedVersion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(" 1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Authored(root); got != "1.2.3" {
		t.Fatalf("Authored() = %q", got)
	}
}

func TestAuthoredFallsBackToDefault(t *testing.T) {
	if got := Authored(t.TempDir()); got != Default {
		t.Fatalf("Authored() = %q, want %q", got, Default)
	}
}
