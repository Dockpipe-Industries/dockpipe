package packagebuild

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTarballExtractionSourceRequiresExactArchiveAndContainment(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "authored")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: example\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "dockpipe-workflow-example-1.2.3.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(source, archive, "workflows/example"); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "cache")
	extraction, err := EnsureTarballExtractedCache(archive, cache)
	if err != nil {
		t.Fatal(err)
	}
	derived := filepath.Join(extraction, "workflows", "example")
	check := func(path string, expected bool) {
		t.Helper()
		actual, err := IsTarballExtractionSource(archive, cache, path)
		if err != nil || actual != expected {
			t.Fatalf("source %q: materialized=%v, expected=%v, err=%v", path, actual, expected, err)
		}
	}
	checkError := func(path string) {
		t.Helper()
		if _, err := IsTarballExtractionSource(archive, cache, path); err == nil {
			t.Fatalf("invalid extraction provenance accepted: %s", path)
		}
	}
	check(derived, true)
	check(source, false)
	other := filepath.Join(cache, "different", "workflows", "example")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	checkError(other)
	escape := filepath.Join(extraction, "..", "..", "authored")
	check(escape, false)
	symlinkEscape := filepath.Join(extraction, "symlink-escape")
	if err := os.Symlink(source, symlinkEscape); err != nil {
		t.Fatal(err)
	}
	checkError(symlinkEscape)
	if _, err := IsTarballExtractionSource("", cache, derived); err == nil {
		t.Fatal("orphan materialization was accepted without an installed archive")
	}
	marker := filepath.Join(extraction, extractMarkerFile)
	for _, malformed := range []string{"wrong digest\n", string(make([]byte, 1000))} {
		if err := os.WriteFile(marker, []byte(malformed), 0o644); err != nil {
			t.Fatal(err)
		}
		checkError(derived)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	checkError(derived)
	if _, err := EnsureTarballExtractedCache(archive, cache); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("changed archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkError(derived)
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if _, err := IsTarballExtractionSource(archive, cache, derived); err == nil {
		t.Fatal("missing archive error was suppressed")
	}
}
