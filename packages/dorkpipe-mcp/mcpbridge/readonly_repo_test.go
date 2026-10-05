package mcpbridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoReadFileKeepsSupportedPathForms(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_MCP_REPO_ROOT", root)
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"note.txt", path, "/work/note.txt"} {
		got, err := repoReadFile(input, 100)
		if err != nil || got != "inside" {
			t.Fatalf("read %q = %q, %v", input, got, err)
		}
	}
	if _, err := repoReadFile("../outside.txt", 100); err == nil {
		t.Fatal("accepted traversal outside the repository")
	}
}

func TestRepoReadAndSearchRejectEscapingSymlinks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_MCP_REPO_ROOT", root)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"link.txt", "linked-dir/secret.txt"} {
		if got, err := repoReadFile(input, 100); err == nil {
			t.Fatalf("read escaped through %q: %q", input, got)
		}
	}
	matches, err := repoSearchText("outside-secret", 100)
	if err != nil || len(matches) != 0 {
		t.Fatalf("search exposed external content: %v, %v", matches, err)
	}
}

func TestRepoSearchReadEnforcesByteLimit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_MCP_REPO_ROOT", root)
	path := filepath.Join(root, "large.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 11)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRepoFile(path, 10); err == nil {
		t.Fatal("accepted file exceeding the read limit")
	}
}
