package pipelang

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Export current inputs, never pass results, for the bounded shared-linking
// experiment. Each invocation requires a fresh, private destination. Normal
// conformance execution and native-cache identity remain unchanged.
func exportGeneratedSharedExperiment(t *testing.T, root, source, binary, key string) {
	t.Helper()
	info, err := os.Lstat(root)
	if !filepath.IsAbs(root) || err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || key == "" {
		t.Fatal("shared experiment export requires an absolute private directory and retained artifact key")
	}
	target := filepath.Join(root, key)
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			return os.Mkdir(filepath.Join(target, rel), 0700)
		}
		if !entry.Type().IsRegular() {
			return &os.PathError{Op: "export", Path: path, Err: fs.ErrInvalid}
		}
		if strings.HasSuffix(rel, ".test") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, rel), data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := generatedFileDigest(binary)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]string{"key": key, "baseline": binary, "sha256": digest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "experiment.json"), metadata, 0600); err != nil {
		t.Fatal(err)
	}
}
