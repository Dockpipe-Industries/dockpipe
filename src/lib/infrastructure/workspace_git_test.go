package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func workspaceGitTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestWorkspaceGitCommitsOnlySelectedAndPreservesOtherStagedFiles(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	for name, content := range map[string]string{"README.md": "unrelated staged change\n", "script with space.sh": "echo ready\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGitOutput(t, root, "add", "README.md")
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit", Snapshot: before.Snapshot,
		Paths: []string{"script with space.sh"}, Message: "Add script"})
	if err != nil {
		t.Fatal(err)
	}
	if after.Head == before.Head {
		t.Fatal("commit did not advance HEAD")
	}
	if got := mustGitOutput(t, root, "show", "HEAD:README.md"); strings.TrimSpace(got) != "hello" {
		t.Fatalf("unselected staged change was committed: %q", got)
	}
	if got := mustGitOutput(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(got) != "README.md" {
		t.Fatalf("unselected index entry changed: %q", got)
	}
}

func TestWorkspaceGitRejectsStaleReviewAndDirtyPull(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "pull", Snapshot: before.Snapshot}); err == nil {
		t.Fatal("dirty pull accepted")
	}
	if err := os.WriteFile(path, []byte("changed after review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit", Snapshot: before.Snapshot,
		Paths: []string{"README.md"}, Message: "stale"}); err == nil || !strings.Contains(err.Error(), "changed since review") {
		t.Fatalf("stale commit = %v", err)
	}
	if got := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD")); got != before.Head {
		t.Fatal("rejected request changed HEAD")
	}
}

func TestWorkspaceGitPushAndFastForwardPull(t *testing.T) {
	root := initGitSessionTestRepo(t)
	remote := t.TempDir()
	mustGitOutput(t, remote, "init", "--bare")
	mustGitOutput(t, root, "remote", "add", "origin", remote)
	ctx := workspaceGitTestContext(t)
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "push", Snapshot: before.Snapshot})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/heads/"+before.Branch)); got != before.Head {
		t.Fatal("push did not publish the reviewed branch")
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "pull", Snapshot: after.Snapshot}); err != nil {
		t.Fatalf("clean fast-forward pull: %v", err)
	}
}

func TestWorkspaceGitCancellationAndOutputBound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: t.TempDir(), Action: "status"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	var buffer workspaceGitBuffer
	if _, err := buffer.Write(make([]byte, workspaceGitBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Write([]byte("overflow")); err == nil || buffer.Len() != workspaceGitBytes {
		t.Fatal("output budget was not enforced")
	}
}

func TestWorkspaceGitFailedCommitPreservesSelectedDraft(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture hook requires a POSIX shell")
	}
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit", Snapshot: before.Snapshot,
		Paths: []string{"README.md"}, Message: "Rejected by hook"})
	if err == nil || !strings.Contains(err.Error(), "may leave selected files staged") {
		t.Fatalf("failed commit outcome = %v", err)
	}
	after, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil || after.Head != before.Head || len(after.Changes) != 1 || after.Changes[0].Status != "M " {
		t.Fatalf("failed commit did not preserve the staged draft: %+v, %v", after, err)
	}
}

func TestWorkspaceGitBinaryReviewBindsContentWithoutRenderingBytes(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	path := filepath.Join(root, "image.bin")
	if err := os.WriteFile(path, []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil || !strings.Contains(before.Diff, "Binary file: 4 bytes, SHA256") || strings.ContainsRune(before.Diff, '\x00') {
		t.Fatalf("binary review = %+v, %v", before, err)
	}
	if err := os.WriteFile(path, []byte{0, 1, 2, 4}, 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil || before.Snapshot == after.Snapshot {
		t.Fatalf("binary content change did not invalidate review: %v", err)
	}
}

func TestWorkspaceGitDiffIsExactReadOnlyAndSnapshotBound(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	name := "script [literal] space.py"
	if err := os.WriteFile(filepath.Join(root, name), []byte("print('selected')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("unrelated changes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	request := WorkspaceGitRequest{Workdir: root, Action: "diff", Snapshot: before.Snapshot, Paths: []string{name}}
	selected, err := RunWorkspaceGit(ctx, request)
	if err != nil || selected.Diff != "print('selected')\n" || selected.Snapshot != before.Snapshot {
		t.Fatalf("selected untracked diff: %+v, %v", selected, err)
	}
	request.Paths = []string{"README.md"}
	selected, err = RunWorkspaceGit(ctx, request)
	if err != nil || !strings.Contains(selected.Diff, "+unrelated changes") || strings.Contains(selected.Diff, "selected") {
		t.Fatalf("selected tracked diff: %+v, %v", selected, err)
	}
	request.Paths = []string{"../outside"}
	if _, err := RunWorkspaceGit(ctx, request); err == nil {
		t.Fatal("accepted a path outside reviewed changes")
	}
	request.Paths = []string{name}
	if err := os.WriteFile(filepath.Join(root, name), []byte("newer content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunWorkspaceGit(ctx, request); err == nil || !strings.Contains(err.Error(), "changed since review") {
		t.Fatalf("stale diff accepted: %v", err)
	}
	if got := mustGitOutput(t, root, "diff", "--cached", "--name-only"); got != "" {
		t.Fatal("diff modified the index")
	}
}
