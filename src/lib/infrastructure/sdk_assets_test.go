package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellSDKWithoutCoreTree(t *testing.T) {
	root := t.TempDir()
	path, err := ShellSDKPath(root)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(content), "dockpipe_sdk_refresh()") {
		t.Fatalf("shell helper = %q, %v", content, err)
	}
	repeated, err := ShellSDKPath(root)
	if err != nil || repeated != path {
		t.Fatalf("repeated path = %q, %v; want %q", repeated, err, path)
	}
	if _, err := os.Stat(filepath.Join(root, "templates")); !os.IsNotExist(err) {
		t.Fatal("SDK materialized a template tree")
	}
	if err := os.WriteFile(path, []byte("corrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ShellSDKPath(root); err == nil {
		t.Fatal("corrupted cached helper accepted")
	}
}
