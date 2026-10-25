package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyGitHookPathExplicitSelectionAndCommitIdentity(t *testing.T) {
	workdir := t.TempDir()
	for _, name := range []string{"clone-worktree", "commit-worktree", "export-patch", "print-summary"} {
		path, err := LegacyGitHookPath(workdir, name)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(path) != name+".sh" {
			t.Fatalf("unexpected selected hook: %q", path)
		}
		if got := ResolveWorkflowScript(path, "/unrelated/workflow", workdir, workdir); got != filepath.ToSlash(path) {
			t.Fatalf("absolute selected hook was rewritten: %q", got)
		}
		if name == "commit-worktree" && !IsBundledCommitWorktree(path, workdir) {
			t.Fatal("owned library commit hook was not recognized")
		}
	}

	if _, err := LegacyGitHookPath(workdir, "../commit-worktree"); err == nil {
		t.Fatal("unknown hook selection must be rejected")
	}
	unrelated := filepath.Join(t.TempDir(), "commit-worktree.sh")
	if err := os.WriteFile(unrelated, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if IsBundledCommitWorktree(unrelated, workdir) {
		t.Fatal("an arbitrary matching basename must not become a host commit action")
	}
	projectHook := filepath.Join(workdir, "scripts", "commit-worktree.sh")
	if !IsBundledCommitWorktree(projectHook, workdir) {
		t.Fatal("explicit project script compatibility changed")
	}
	libraryCommit, err := LegacyGitHookPath(workdir, "commit-worktree")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libraryCommit, []byte("#!/bin/sh\n# replaced\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if IsBundledCommitWorktree(libraryCommit, workdir) {
		t.Fatal("modified library hook must not retain host commit identity")
	}
}
