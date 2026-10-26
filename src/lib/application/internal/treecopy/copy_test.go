package treecopy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyPreservesAllSourceEntries(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	for path, value := range map[string]string{
		"keep/value.txt":           "kept",
		"resolvers/value.txt":      "resolver",
		"__pycache__/value.pyc":    "cache",
		"nested/__pycache__/x.pyc": "nested-cache",
	} {
		full := filepath.Join(src, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Copy(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "keep", "value.txt")); err != nil || string(data) != "kept" {
		t.Fatalf("kept file = %q, err=%v", data, err)
	}
	for path, expected := range map[string]string{
		"resolvers/value.txt":      "resolver",
		"__pycache__/value.pyc":    "cache",
		"nested/__pycache__/x.pyc": "nested-cache",
	} {
		data, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(path)))
		if err != nil || string(data) != expected {
			t.Fatalf("copied %q = %q, err=%v", path, data, err)
		}
	}
}
