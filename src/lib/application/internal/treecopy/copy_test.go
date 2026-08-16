package treecopy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyExcludingTopLevelOmitsSelectedAndPythonCache(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	for path, value := range map[string]string{
		"keep/value.txt":           "kept",
		"resolvers/value.txt":      "excluded",
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
	if err := CopyExcludingTopLevel(src, dst, map[string]bool{"resolvers": true}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "keep", "value.txt")); err != nil || string(data) != "kept" {
		t.Fatalf("kept file = %q, err=%v", data, err)
	}
	for _, path := range []string{"resolvers", "__pycache__", filepath.Join("nested", "__pycache__")} {
		if _, err := os.Stat(filepath.Join(dst, path)); !os.IsNotExist(err) {
			t.Fatalf("excluded path %q exists, err=%v", path, err)
		}
	}
}
