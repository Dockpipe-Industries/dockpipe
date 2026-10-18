package infrastructure

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceGitReviewSeparatesIndexAndWorkingTree(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("staged draft\n")
	mustGitOutput(t, root, "add", "README.md")
	write("working draft\n")
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "review-status"})
	if err != nil {
		t.Fatal(err)
	}
	for comparison, expected := range map[string][]string{
		"staged":   {"-hello", "+staged draft"},
		"unstaged": {"-staged draft", "+working draft"},
		"combined": {"-hello", "+working draft"},
	} {
		reply, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: snapshot.Snapshot, Paths: []string{"README.md"}, Comparison: comparison})
		if err != nil {
			t.Fatal(err)
		}
		if reply.Review.Revision != snapshot.Changes[0].Revision {
			t.Fatal("comparison revision mismatch")
		}
		for _, line := range expected {
			if !strings.Contains(reply.Review.Patch, line) {
				t.Fatalf("%s: %s", comparison, reply.Review.Patch)
			}
		}
	}
	committed, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit-index", Snapshot: snapshot.Snapshot, Confirm: true, Message: "Commit index only"})
	if err != nil {
		t.Fatal(err)
	}
	if committed.Head == snapshot.Head || strings.TrimSpace(mustGitOutput(t, root, "show", "HEAD:README.md")) != "staged draft" {
		t.Fatal("did not commit exact index")
	}
	content, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if string(content) != "working draft\n" {
		t.Fatal("commit overwrote unstaged draft")
	}
}

func TestWorkspaceGitReviewUnbornAndStageLifecycle(t *testing.T) {
	root := t.TempDir()
	mustGitOutput(t, root, "init")
	ctx := workspaceGitTestContext(t)
	name := "new [literal]\nfile.txt"
	if err := os.WriteFile(filepath.Join(root, name), []byte("first\nsecond"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "review-status"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Head != "" || len(snapshot.Changes) != 1 {
		t.Fatal("unborn snapshot")
	}
	review, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: snapshot.Snapshot, Paths: []string{name}, Comparison: "unstaged"})
	if err != nil || !strings.Contains(review.Review.BeforeLabel, "absent") || !strings.Contains(review.Review.Patch, "+second\n\\ No newline") {
		t.Fatalf("untracked: %+v, %v", review, err)
	}
	staged, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "stage", Snapshot: snapshot.Snapshot, Paths: []string{name}})
	if err != nil {
		t.Fatal(err)
	}
	review, err = RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: staged.Snapshot, Paths: []string{name}, Comparison: "staged"})
	if err != nil || !strings.Contains(review.Review.Patch, "+first") {
		t.Fatalf("staged unborn: %v", err)
	}
	unstaged, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "unstage", Snapshot: staged.Snapshot, Paths: []string{name}})
	if err != nil || unstaged.Changes[0].Status != "??" {
		t.Fatalf("unstage unborn: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, name)); err != nil {
		t.Fatal("unstage deleted working file")
	}
}

func TestWorkspaceGitReviewRenameBinaryLimitsAndDiscard(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	mustGitOutput(t, root, "mv", "README.md", "renamed.md")
	if err := os.WriteFile(filepath.Join(root, "renamed.md"), []byte("hello\nworking addition\n"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Changes[0].OriginalPath != "README.md" {
		t.Fatal("rename source lost")
	}
	staged, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "stage", Snapshot: snapshot.Snapshot, Paths: []string{"renamed.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mustGitOutput(t, root, "show", ":renamed.md"), "working addition") {
		t.Fatal("rename edits not staged")
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit-index", Snapshot: staged.Snapshot, Message: "No confirmation"}); err == nil {
		t.Fatal("unconfirmed commit accepted")
	}
	for name, content := range map[string][]byte{"binary": {0, 1, 2}, "invalid-utf8": {0xff, 0xfe}, "large.txt": []byte(strings.Repeat("x", 1024*1024+1))} {
		if err := os.WriteFile(filepath.Join(root, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err = RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "review-status"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"binary", "invalid-utf8", "large.txt"} {
		reply, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: snapshot.Snapshot, Comparison: "unstaged", Paths: []string{name}})
		if err != nil {
			t.Fatal(err)
		}
		if name == "large.txt" && !reply.Review.Limited {
			t.Fatal("large file was not explicitly limited")
		}
		if name != "large.txt" && !reply.Review.Binary {
			t.Fatal("binary file treated as text")
		}
	}
	mustGitOutput(t, root, "commit", "-m", "Rename fixture")
	if err := os.WriteFile(filepath.Join(root, "renamed.md"), []byte("staged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "add", "renamed.md")
	if err := os.WriteFile(filepath.Join(root, "renamed.md"), []byte("unstaged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err = RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	request := WorkspaceGitRequest{Workdir: root, Action: "discard", Snapshot: snapshot.Snapshot, Paths: []string{"renamed.md"}}
	if _, err := RunWorkspaceGit(ctx, request); err == nil {
		t.Fatal("unconfirmed discard accepted")
	}
	request.Confirm = true
	if _, err := RunWorkspaceGit(ctx, request); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "renamed.md"))
	if string(content) != "staged\n" {
		t.Fatal("discard did not restore the index version")
	}
}

func TestWorkspaceGitReviewConflictAndContentIdentity(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	initial := strings.TrimSpace(mustGitOutput(t, root, "branch", "--show-current"))
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mustGitOutput(t, root, "checkout", "-b", "other")
	write("other\n")
	mustGitOutput(t, root, "commit", "-am", "Other")
	mustGitOutput(t, root, "checkout", initial)
	write("ours\n")
	mustGitOutput(t, root, "commit", "-am", "Ours")
	command := exec.CommandContext(ctx, "git", "-C", root, "merge", "other")
	if err := command.Run(); err == nil {
		t.Fatal("fixture did not conflict")
	}
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: snapshot.Snapshot, Comparison: "unstaged", Paths: []string{"README.md"}})
	if err != nil || !reply.Review.Conflict || reply.Review.Patch != "" {
		t.Fatalf("conflict misrepresented: %v", err)
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "stage", Snapshot: snapshot.Snapshot, Paths: []string{"README.md"}}); err == nil {
		t.Fatal("conflict staged silently")
	}
	path := filepath.Join(root, "README.md")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	write("same timestamps, different content\n")
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	updated, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil || snapshot.Changes[0].Revision == updated.Changes[0].Revision {
		t.Fatal("review revision trusted timestamps")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := RunWorkspaceGit(cancelled, WorkspaceGitRequest{Workdir: root, Action: "review-status"}); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestWorkspaceGitLegacyCommitIncludesRenameDeletion(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	mustGitOutput(t, root, "mv", "README.md", "renamed.md")
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit", Snapshot: snapshot.Snapshot,
		Paths: []string{"renamed.md"}, Message: "Commit complete rename"})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Changes) != 0 || strings.Contains(mustGitOutput(t, root, "ls-tree", "--name-only", "HEAD"), "README.md") {
		t.Fatal("rename deletion was not committed")
	}
}

func TestWorkspaceGitReviewDeletionAndIndependentBinaryVersions(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	path := filepath.Join(root, "README.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: snapshot.Snapshot,
		Paths: []string{"README.md"}, Comparison: "unstaged"})
	if err != nil || !strings.Contains(reply.Review.Patch, "-hello") {
		t.Fatalf("deleted-file comparison: %v", err)
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "stage", Snapshot: snapshot.Snapshot,
		Paths: []string{"README.md"}}); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "reset", "--hard", "HEAD")
	if err := os.WriteFile(path, []byte("text staged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "add", "README.md")
	if err := os.WriteFile(path, []byte{'b', 0, 'n'}, 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err = RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	for _, comparison := range []string{"staged", "unstaged"} {
		reply, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "compare", Snapshot: snapshot.Snapshot,
			Paths: []string{"README.md"}, Comparison: comparison})
		if err != nil {
			t.Fatal(err)
		}
		if reply.Review.Binary != (comparison == "unstaged") {
			t.Fatalf("binary state leaked between versions: %s", comparison)
		}
	}
}

func TestWorkspaceGitLegacyCommitDoesNotIncludeRecreatedRenameSource(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	mustGitOutput(t, root, "mv", "README.md", "renamed.md")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("unselected replacement\n"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "commit", Snapshot: snapshot.Snapshot,
		Paths: []string{"renamed.md"}, Message: "Must not include other file"}); err == nil {
		t.Fatal("unselected recreated source was committed")
	}
	if strings.TrimSpace(mustGitOutput(t, root, "show", "HEAD:README.md")) != "hello" {
		t.Fatal("rejected commit changed HEAD")
	}
}
