package dockpipe

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBundledAuthoredCompatibility(t *testing.T) {
	// Independent membership oracle: the old directory walk's visible authored
	// files, discovered from version control rather than the generated manifest.
	out, err := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "VERSION" && name != "assets/entrypoint.sh" && !strings.HasPrefix(name, "src/core/") && !strings.HasPrefix(name, "packages/") && !strings.HasPrefix(name, "workflows/") {
			continue
		}
		hidden := false
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
				hidden = true
			}
		}
		if hidden {
			continue
		}
		nested := false
		for dir := filepath.Dir(name); dir != "."; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				nested = true
			}
		}
		if nested {
			continue
		}
		expected[name] = true
		want, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := BundledFS.ReadFile(name)
		if err != nil || !bytes.Equal(want, got) {
			t.Fatalf("authored asset changed or missing: %s: %v", name, err)
		}
	}
	var count int
	err = fs.WalkDir(BundledFS, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		if !expected[name] {
			prefix := "packages/dorkpipe/resolvers/dorkpipe/assets/tooling/bin/linux/"
			if name != prefix+"dockpipe" && name != prefix+"dorkpipe" && name != prefix+"mcpd" {
				t.Errorf("undeclared bundled input: %s", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("preserved %d authored files; %d total bundled files", len(expected), count)
	if err := fstest.TestFS(BundledFS, "VERSION", "assets/entrypoint.sh"); err != nil {
		t.Fatal(err)
	}
}
