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

func TestForSourceOwnership(t *testing.T) {
	root := t.TempDir()
	owner := filepath.Join(root, "owner")
	child := filepath.Join(owner, "nested", "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest := func(directory, version string) {
		t.Helper()
		content := "schema: 1\nname: example\nkind: bundle\nversion: " + version + "\n"
		if err := os.WriteFile(filepath.Join(directory, "package.yml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(workdir, source, expected string) {
		t.Helper()
		actual, err := ForSource(workdir, source, "9.0.0")
		if err != nil || actual != expected {
			t.Fatalf("ForSource = %q, %v; want %q", actual, err, expected)
		}
	}
	check(root, child, "9.0.0")
	writeManifest(owner, "1.2.3")
	check(root, child, "1.2.3")
	check(child, child, "9.0.0")       // Never inherit outside the workdir.
	check(t.TempDir(), child, "9.0.0") // External source: own manifest only.
	writeManifest(child, "2.0.0-rc.1")
	check(root, child, "2.0.0-rc.1")
	check(t.TempDir(), child, "2.0.0-rc.1")
	writeManifest(child, "not-semver")
	if _, err := ForSource(root, child, "9.0.0"); err == nil {
		t.Fatal("invalid owning manifest silently fell back")
	}
}
