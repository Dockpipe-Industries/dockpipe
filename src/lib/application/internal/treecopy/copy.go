// Package treecopy owns byte-copying rules shared by application source materializers.
package treecopy

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Copy copies a directory tree using DockPipe's normalized authored-file modes.
func Copy(src, dst string) error {
	return copyFiltered(src, dst, nil)
}

// CopyCore copies an authored core tree while excluding generated Python cache entries.
func CopyCore(src, dst string) error {
	return copyFiltered(src, dst, isGeneratedPythonCacheEntry)
}

// CopyExcludingTopLevel copies a core source tree while skipping selected immediate children.
func CopyExcludingTopLevel(src, dst string, exclude map[string]bool) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if exclude[name] || isGeneratedPythonCacheEntry(entry) {
			continue
		}
		from := filepath.Join(src, name)
		to := filepath.Join(dst, name)
		if entry.IsDir() {
			if err := CopyCore(from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	return os.WriteFile(dst, data, 0o644)
}

func isGeneratedPythonCacheEntry(entry fs.DirEntry) bool {
	if entry.IsDir() {
		return entry.Name() == "__pycache__"
	}
	return strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo")
}

func copyFiltered(src, dst string, skip func(fs.DirEntry) bool) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if skip != nil && skip(entry) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
