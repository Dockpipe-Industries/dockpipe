package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitWorkspaceStatus(t *testing.T, root string) *WorkspaceGitSnapshot {
	t.Helper()
	result, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func gitWorkspaceAction(t *testing.T, root string, request WorkspaceGitRequest) *WorkspaceGitSnapshot {
	t.Helper()
	request.Workdir = root
	request.Snapshot = gitWorkspaceStatus(t, root).Snapshot
	result, err := RunWorkspaceGit(workspaceGitTestContext(t), request)
	if err != nil {
		t.Fatalf("%s: %v", request.Action, err)
	}
	return result
}

func gitWorkspaceCommit(t *testing.T, root, path, content, subject string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "add", "--", path)
	mustGitOutput(t, root, "commit", "-m", subject)
	return strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD"))
}

func TestWorkspaceGitOrderedCherryPickAndReview(t *testing.T) {
	root := initGitSessionTestRepo(t)
	original := gitWorkspaceStatus(t, root)
	mustGitOutput(t, root, "switch", "-c", "source")
	first := gitWorkspaceCommit(t, root, "first.txt", "first\n", "First selected")
	second := gitWorkspaceCommit(t, root, "second.txt", "second\n", "Second selected")
	mustGitOutput(t, root, "switch", original.Branch)
	result := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "cherry-pick", Commits: []string{second, first}, Confirm: true})
	if result.Operation != nil || len(result.Changes) != 0 {
		t.Fatalf("unexpected remaining state: %+v", result)
	}
	subjects := strings.FieldsFunc(strings.TrimSpace(mustGitOutput(t, root, "log", "-2", "--format=%s")), func(char rune) bool { return char == '\n' })
	if len(subjects) != 2 || subjects[0] != "First selected" || subjects[1] != "Second selected" {
		t.Fatalf("selection order not preserved: %v", subjects)
	}
	review := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "commit-review", Commit: first})
	if review.CommitReview == nil || review.CommitReview.Commit.ID != first || !strings.Contains(review.CommitReview.Patch, "+first") {
		t.Fatalf("missing commit review: %+v", review.CommitReview)
	}
}

func workspaceGitConflictFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := initGitSessionTestRepo(t)
	original := gitWorkspaceStatus(t, root)
	mustGitOutput(t, root, "switch", "-c", "source")
	conflict := gitWorkspaceCommit(t, root, "README.md", "source side\n", "Conflict commit")
	next := gitWorkspaceCommit(t, root, "after.txt", "after\n", "After conflict")
	mustGitOutput(t, root, "switch", original.Branch)
	gitWorkspaceCommit(t, root, "README.md", "destination side\n", "Destination change")
	return root, conflict, next
}

func TestWorkspaceGitConflictResolutionContinuesNativeQueue(t *testing.T) {
	root, conflict, next := workspaceGitConflictFixture(t)
	stopped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "cherry-pick", Commits: []string{conflict, next}, Confirm: true})
	if stopped.Operation == nil || stopped.Operation.Kind != "cherry-pick" || stopped.Operation.Current != conflict || len(stopped.Operation.Conflicts) != 1 || stopped.Operation.CanContinue || !stopped.Operation.CanAbort || !stopped.Operation.CanSkip {
		t.Fatalf("missing stopped operation: %+v", stopped.Operation)
	}
	if len(stopped.Operation.Remaining) != 2 || stopped.Operation.Remaining[0] != conflict || stopped.Operation.Remaining[1] != next {
		t.Fatalf("native sequencer queue did not preserve full selected identities: %v", stopped.Operation.Remaining)
	}
	for _, request := range []WorkspaceGitRequest{
		{Action: "operation-continue"}, {Action: "commit", Message: "forbidden", Paths: []string{"README.md"}},
		{Action: "stage", Paths: []string{"README.md"}}, {Action: "resolve-stage", Paths: []string{"README.md"}, Confirm: true},
		{Action: "fetch"}, {Action: "branch-create", Name: "blocked"},
	} {
		request.Workdir, request.Snapshot = root, stopped.Snapshot
		if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
			t.Fatalf("unsafe %s accepted", request.Action)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("resolved combination\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := WorkspaceGitRequest{Workdir: root, Action: "resolve-stage", Snapshot: stopped.Snapshot, Paths: []string{"README.md"}, Confirm: true}
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), stale); err == nil {
		t.Fatal("stale resolution accepted")
	}
	resolved := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "resolve-stage", Paths: []string{"README.md"}, Confirm: true})
	if resolved.Operation == nil || !resolved.Operation.CanContinue || len(resolved.Operation.Conflicts) != 0 {
		t.Fatalf("resolution not staged: %+v", resolved.Operation)
	}
	completed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-continue"})
	if completed.Operation != nil || len(completed.Changes) != 0 {
		t.Fatalf("queue not finished: %+v", completed.Operation)
	}
	if got := mustGitOutput(t, root, "show", "HEAD:after.txt"); got != "after\n" {
		t.Fatalf("remaining commit not applied: %q", got)
	}
}

func TestWorkspaceGitConflictSkipAbortAndChosenSide(t *testing.T) {
	for _, action := range []string{"operation-abort", "operation-skip", "resolve-theirs", "resolve-ours"} {
		t.Run(action, func(t *testing.T) {
			root, conflict, next := workspaceGitConflictFixture(t)
			before := gitWorkspaceStatus(t, root)
			gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "cherry-pick", Commits: []string{conflict, next}, Confirm: true})
			request := WorkspaceGitRequest{Action: action, Confirm: true}
			if strings.HasPrefix(action, "resolve-") {
				request.Paths = []string{"README.md"}
			}
			result := gitWorkspaceAction(t, root, request)
			if action == "operation-abort" && (result.Operation != nil || result.Head != before.Head || len(result.Changes) != 0) {
				t.Fatal("abort did not restore pre-operation state")
			}
			if action == "operation-skip" && (result.Operation != nil || !strings.Contains(mustGitOutput(t, root, "log", "-1", "--format=%s"), "After conflict")) {
				t.Fatal("skip did not resume next commit")
			}
			if action == "resolve-theirs" {
				if result.Operation == nil || !result.Operation.CanContinue {
					t.Fatal("chosen side not staged")
				}
				if got := mustGitOutput(t, root, "show", ":README.md"); got != "source side\n" {
					t.Fatalf("wrong side: %q", got)
				}
				gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-continue"})
			}
			if action == "resolve-ours" {
				gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-skip", Confirm: true})
			}
		})
	}
}

func TestWorkspaceGitHistorySearchAllRefsAndPages(t *testing.T) {
	root := initGitSessionTestRepo(t)
	initial := gitWorkspaceStatus(t, root)
	mustGitOutput(t, root, "switch", "-c", "history-source")
	old := gitWorkspaceCommit(t, root, "old.txt", "old\n", "Prefix needle old\n\nUnique body phrase")
	for index := 0; index < 4; index++ {
		gitWorkspaceCommit(t, root, "recent.txt", strings.Repeat("x", index+1), "Recent unrelated")
	}
	mustGitOutput(t, root, "switch", initial.Branch)
	for _, request := range []WorkspaceGitRequest{
		{Action: "history", Ref: "all", Query: "unique BODY phrase", Match: "keyword", Limit: 1},
		{Action: "history", Ref: "all", Query: "prefix NEEDLE", Match: "prefix", Limit: 1},
		{Action: "history", Ref: "history-source", Query: old[:10], Match: "prefix", Limit: 1},
	} {
		result := gitWorkspaceAction(t, root, request)
		if result.History == nil || len(result.History.Commits) != 1 || result.History.Commits[0].ID != old {
			t.Fatalf("older/ref search failed: %+v", result.History)
		}
	}
	first := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "history", Ref: "all", Limit: 2})
	next := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "history", Ref: "all", Limit: 2, Offset: 2})
	if !first.History.HasMore || first.History.Commits[0].ID == next.History.Commits[0].ID {
		t.Fatal("history paging did not advance")
	}
	for _, request := range []WorkspaceGitRequest{
		{Action: "history", Limit: 201}, {Action: "history", Offset: -1}, {Action: "history", Query: strings.Repeat("x", 257)}, {Action: "history", Ref: "--all"}, {Action: "commit-review", Commit: old[:8]},
	} {
		request.Workdir = root
		if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
			t.Fatalf("malformed history accepted: %+v", request)
		}
	}
}

func TestWorkspaceGitBranchesStashesAndStaleMetadata(t *testing.T) {
	root := initGitSessionTestRepo(t)
	original := gitWorkspaceStatus(t, root)
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-create", Name: "topic"})
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-switch", Name: "topic"})
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("stash me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stashed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", Message: "Saved example", IncludeUntracked: true})
	if len(stashed.Changes) != 0 {
		t.Fatalf("stash-save left reviewed changes in the working tree: %+v", stashed.Changes)
	}
	listed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	if len(listed.Stashes) != 1 {
		t.Fatalf("stash list: %+v", listed.Stashes)
	}
	id := listed.Stashes[0].ID
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-apply", Stash: id})
	if content, err := os.ReadFile(filepath.Join(root, "new.txt")); err != nil || string(content) != "untracked\n" {
		t.Fatal("stash did not restore untracked file")
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", Message: "Second", IncludeUntracked: true})
	before := gitWorkspaceStatus(t, root)
	request := WorkspaceGitRequest{Workdir: root, Action: "stash-drop", Snapshot: before.Snapshot, Stash: id}
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
		t.Fatal("stash drop without confirmation")
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-drop", Stash: id, Confirm: true})
	request.Action, request.Name, request.Confirm = "branch-create", "stale", true
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
		t.Fatal("non-top stash change did not invalidate snapshot")
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-switch", Name: original.Branch})
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-delete", Name: "topic", Confirm: true})
}

func TestWorkspaceGitStashIncludesUntrackedWithoutWeakeningCleanGuard(t *testing.T) {
	root := initGitSessionTestRepo(t)
	// The command wrapper enables literal mode for file actions. Stash must
	// disable it for its own internal root pathspec, even when inherited.
	t.Setenv("GIT_LITERAL_PATHSPECS", "1")
	directory := filepath.Join(root, "nested")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	untracked := filepath.Join(directory, "new [file].txt")
	if err := os.WriteFile(untracked, []byte("saved untracked content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", IncludeUntracked: true})
	if len(saved.Changes) != 0 {
		t.Fatalf("stash did not remove saved untracked files: %+v", saved.Changes)
	}
	if _, err := os.Stat(untracked); !os.IsNotExist(err) {
		t.Fatalf("saved untracked file still exists: %v", err)
	}
	listed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	if len(listed.Stashes) != 1 {
		t.Fatalf("stash list: %+v", listed.Stashes)
	}
	// A new user change after stashing must still block apply/pop, rather than
	// relaxing the clean-tree guard to work around incomplete stash cleanup.
	dirty := filepath.Join(root, "README.md")
	if err := os.WriteFile(dirty, []byte("new user change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"stash-apply", "stash-pop"} {
		current := gitWorkspaceStatus(t, root)
		_, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{
			Workdir: root, Action: action, Snapshot: current.Snapshot, Stash: listed.Stashes[0].ID,
		})
		if err == nil || !strings.Contains(err.Error(), "commit or stash local changes") {
			t.Fatalf("%s accepted a dirty working tree: %v", action, err)
		}
	}
	content, err := os.ReadFile(dirty)
	if err != nil || string(content) != "new user change\n" {
		t.Fatal("rejected stash operation changed the user's file")
	}
	after := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	if len(after.Stashes) != 1 || after.Stashes[0].ID != listed.Stashes[0].ID {
		t.Fatal("rejected stash operation removed the saved stash")
	}
}

func TestWorkspaceGitRejectsUnsafeCherryPickInputs(t *testing.T) {
	root := initGitSessionTestRepo(t)
	current := gitWorkspaceStatus(t, root)
	for _, request := range []WorkspaceGitRequest{
		{Action: "cherry-pick", Commits: []string{current.Head}},
		{Action: "cherry-pick", Commits: []string{current.Head[:8]}, Confirm: true},
		{Action: "cherry-pick", Commits: []string{current.Head, current.Head}, Confirm: true},
		{Action: "cherry-pick", Commits: make([]string, 129), Confirm: true},
		{Action: "branch-create", Name: "--bad"},
		{Action: "branch-switch", Name: "../outside"},
	} {
		request.Workdir, request.Snapshot = root, current.Snapshot
		if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
			t.Fatalf("unsafe input accepted: %+v", request)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty := gitWorkspaceStatus(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{Workdir: root, Action: "cherry-pick", Snapshot: dirty.Snapshot, Commits: []string{current.Head}, Confirm: true}); err == nil {
		t.Fatal("dirty cherry-pick accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunWorkspaceGit(cancelled, WorkspaceGitRequest{Workdir: root, Action: "status"}); err == nil {
		t.Fatal("cancelled operation accepted")
	}
}

func TestWorkspaceGitMergeRevertAndMergeCommitPolicy(t *testing.T) {
	root := initGitSessionTestRepo(t)
	original := gitWorkspaceStatus(t, root)
	mustGitOutput(t, root, "switch", "-c", "merge-source")
	source := gitWorkspaceCommit(t, root, "feature.txt", "feature\n", "Merge feature")
	mustGitOutput(t, root, "switch", original.Branch)
	gitWorkspaceCommit(t, root, "other.txt", "other\n", "Destination unrelated")
	merged := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "merge", Commit: source, Confirm: true})
	if merged.Operation != nil || len(merged.Changes) != 0 {
		t.Fatalf("merge did not complete: %+v", merged)
	}
	for _, action := range []string{"cherry-pick", "revert"} {
		request := WorkspaceGitRequest{Workdir: root, Action: action, Snapshot: merged.Snapshot, Commit: merged.Head, Commits: []string{merged.Head}, Confirm: true}
		if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "mainline") {
			t.Fatalf("mergecommit %s policy: %v", action, err)
		}
	}
	reverted := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "revert", Commit: source, Confirm: true})
	if reverted.Operation != nil || len(reverted.Changes) != 0 {
		t.Fatalf("revert did not complete: %+v", reverted)
	}
	if _, err := os.Stat(filepath.Join(root, "feature.txt")); !os.IsNotExist(err) {
		t.Fatal("revert did not remove selected feature")
	}
}

func TestWorkspaceGitMergeAndRevertConflictRecovery(t *testing.T) {
	t.Run("merge", func(t *testing.T) {
		root, conflict, _ := workspaceGitConflictFixture(t)
		before := gitWorkspaceStatus(t, root)
		stopped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "merge", Commit: conflict, Confirm: true})
		if stopped.Operation == nil || stopped.Operation.Kind != "merge" || stopped.Operation.CanSkip || stopped.Operation.CanContinue {
			t.Fatalf("merge operation state: %+v", stopped.Operation)
		}
		gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "resolve-theirs", Paths: []string{"README.md"}, Confirm: true})
		finished := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-continue"})
		if finished.Operation != nil || finished.Head == before.Head {
			t.Fatal("merge continue failed")
		}
	})
	t.Run("revert", func(t *testing.T) {
		root := initGitSessionTestRepo(t)
		selected := gitWorkspaceCommit(t, root, "README.md", "selected\n", "Selected change")
		original := gitWorkspaceCommit(t, root, "README.md", "later different\n", "Later change")
		stopped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "revert", Commit: selected, Confirm: true})
		if stopped.Operation == nil || stopped.Operation.Kind != "revert" {
			t.Fatalf("revert operation state: %+v", stopped.Operation)
		}
		aborted := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-abort", Confirm: true})
		if aborted.Operation != nil || aborted.Head != original {
			t.Fatal("revert abort changed original state")
		}
	})
}

func TestWorkspaceGitResolveDeletedConflict(t *testing.T) {
	root := initGitSessionTestRepo(t)
	original := gitWorkspaceStatus(t, root)
	mustGitOutput(t, root, "switch", "-c", "deletion-source")
	mustGitOutput(t, root, "rm", "README.md")
	mustGitOutput(t, root, "commit", "-m", "Delete readme")
	deleted := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD"))
	mustGitOutput(t, root, "switch", original.Branch)
	gitWorkspaceCommit(t, root, "README.md", "changed destination\n", "Modify readme")
	stopped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "cherry-pick", Commits: []string{deleted}, Confirm: true})
	if stopped.Operation == nil || len(stopped.Operation.Conflicts) != 1 {
		t.Fatalf("expected modify/delete conflict: %+v", stopped.Operation)
	}
	resolved := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "resolve-delete", Paths: []string{"README.md"}, Confirm: true})
	if !resolved.Operation.CanContinue {
		t.Fatal("deletion not staged")
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-continue"})
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatal("resolved deletion was not committed")
	}
}

func TestWorkspaceGitOperationMetadataInvalidatesSnapshot(t *testing.T) {
	root, conflict, next := workspaceGitConflictFixture(t)
	stopped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "cherry-pick", Commits: []string{conflict, next}, Confirm: true})
	gitDirectory := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "--absolute-git-dir"))
	todo := filepath.Join(gitDirectory, "sequencer", "todo")
	if err := os.WriteFile(todo, []byte("pick "+conflict+" Changed description\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{Workdir: root, Action: "operation-abort", Snapshot: stopped.Snapshot, Confirm: true}); err == nil {
		t.Fatal("stale sequencer action accepted")
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "operation-abort", Confirm: true})
}

func TestWorkspaceGitStashSelectorDisambiguatesIdenticalObjects(t *testing.T) {
	root := initGitSessionTestRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("stash copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", Message: "Original"})
	listed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	id := listed.Stashes[0].ID
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("different stash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", Message: "Different"})
	mustGitOutput(t, root, "stash", "store", "-m", "Duplicate", id)
	before := gitWorkspaceStatus(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{Workdir: root, Action: "stash-drop", Snapshot: before.Snapshot, Stash: id, Confirm: true}); err == nil {
		t.Fatal("ambiguous duplicate stash was dropped")
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-drop", Stash: id, StashSelector: "stash@{2}", Confirm: true})
	remaining := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	if len(remaining.Stashes) != 2 || !strings.Contains(remaining.Stashes[0].Subject, "Duplicate") {
		t.Fatalf("wrong duplicate stash removed: %+v", remaining.Stashes)
	}
}

func TestWorkspaceGitPreservesIgnoredFilesDuringBranchAndCommitActions(t *testing.T) {
	for _, action := range []string{"branch-switch", "merge", "cherry-pick"} {
		t.Run(action, func(t *testing.T) {
			root := initGitSessionTestRepo(t)
			original := gitWorkspaceStatus(t, root)
			mustGitOutput(t, root, "switch", "-c", "ignored-source")
			selected := gitWorkspaceCommit(t, root, "local-build.txt", "tracked source\n", "Source file")
			mustGitOutput(t, root, "switch", original.Branch)
			gitDirectory := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "--absolute-git-dir"))
			if err := os.WriteFile(filepath.Join(gitDirectory, "info", "exclude"), []byte("local-build.txt\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "local-build.txt"), []byte("precious ignored file\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			before := gitWorkspaceStatus(t, root)
			request := WorkspaceGitRequest{Workdir: root, Action: action, Name: "ignored-source", Commit: selected, Commits: []string{selected}, Confirm: true, Snapshot: before.Snapshot}
			if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
				t.Fatalf("%s overwrote ignored file", action)
			}
			content, err := os.ReadFile(filepath.Join(root, "local-build.txt"))
			if err != nil || string(content) != "precious ignored file\n" {
				t.Fatalf("ignored content lost: %q, %v", content, err)
			}
		})
	}
}

func TestWorkspaceGitCommitReviewReportsLargePatchExplicitly(t *testing.T) {
	root := initGitSessionTestRepo(t)
	id := gitWorkspaceCommit(t, root, "large.txt", strings.Repeat("large focused review line\n", 100000), "Large patch")
	result := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "commit-review", Commit: id})
	if result.CommitReview == nil || !result.CommitReview.Limited || result.CommitReview.Notice == "" || result.CommitReview.Patch != "" {
		t.Fatalf("large patch mislabeled as ordinary review: %+v", result.CommitReview)
	}
}

func TestWorkspaceGitFetchAndRemoteHistory(t *testing.T) {
	root := initGitSessionTestRepo(t)
	remote := initGitSessionTestRepo(t)
	selected := gitWorkspaceCommit(t, remote, "remote.txt", "remote\n", "Remote searchable feature")
	mustGitOutput(t, root, "remote", "add", "origin", remote)
	remoteBranch := gitWorkspaceStatus(t, remote).Branch
	mustGitOutput(t, root, "config", "fetch.prune", "true")
	mustGitOutput(t, root, "config", "fetch.pruneTags", "true")
	original := gitWorkspaceStatus(t, root).Head
	mustGitOutput(t, root, "update-ref", "refs/remotes/origin/retained-orphan", original)
	mustGitOutput(t, root, "tag", "retained-local-tag", original)
	before := gitWorkspaceStatus(t, root)
	after := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "fetch"})
	if got := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "refs/remotes/origin/retained-orphan")); got != original {
		t.Fatal("fetch pruned an existing remote tracking ref despite the non-pruning contract")
	}
	if got := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "refs/tags/retained-local-tag")); got != original {
		t.Fatal("fetch pruned a local tag despite the non-pruning contract")
	}
	if after.Snapshot == before.Snapshot {
		t.Fatal("fetch did not invalidate old reviewed refs")
	}
	history := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "history", Ref: "origin/" + remoteBranch, Query: "searchable", Match: "keyword"})
	if len(history.History.Commits) != 1 || history.History.Commits[0].ID != selected {
		t.Fatalf("remote branch history missing: %+v", history.History)
	}
}

func TestWorkspaceGitSafeBranchDeleteAndStashPop(t *testing.T) {
	root := initGitSessionTestRepo(t)
	original := gitWorkspaceStatus(t, root)
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-create", Name: "unmerged"})
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-switch", Name: "unmerged"})
	selected := gitWorkspaceCommit(t, root, "unmerged.txt", "unmerged\n", "Unmerged commit")
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "branch-switch", Name: original.Branch})
	before := gitWorkspaceStatus(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{Workdir: root, Action: "branch-delete", Name: "unmerged", Snapshot: before.Snapshot, Confirm: true}); err == nil {
		t.Fatal("unmerged branch was force-deleted")
	}
	if got := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "refs/heads/unmerged")); got != selected {
		t.Fatal("rejected branch delete changed ref")
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("pop me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", Message: "Pop example"})
	listed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	popped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-pop", Stash: listed.Stashes[0].ID, StashSelector: listed.Stashes[0].Selector})
	if len(popped.Changes) != 1 {
		t.Fatal("stash pop did not restore working state")
	}
	remaining := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
	if len(remaining.Stashes) != 0 {
		t.Fatal("successful stash pop did not remove selected entry")
	}
}

func TestWorkspaceGitStashConflictsRemainVisibleAndKeepStash(t *testing.T) {
	for _, action := range []string{"stash-apply", "stash-pop"} {
		t.Run(action, func(t *testing.T) {
			root := initGitSessionTestRepo(t)
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("stashed side\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-save", Message: "Conflicting saved work"})
			listed := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
			stash := listed.Stashes[0]
			gitWorkspaceCommit(t, root, "README.md", "current committed side\n", "Change current branch")
			stopped := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: action, Stash: stash.ID, StashSelector: stash.Selector})
			operation := stopped.Operation
			if operation == nil || operation.Kind != "conflicts" || len(operation.Conflicts) != 1 || operation.Current != "" || len(operation.Remaining) != 0 || operation.CanContinue || operation.CanSkip || operation.CanAbort || stopped.Notice == "" {
				t.Fatalf("stash conflict lost its structured recovery state: %+v", operation)
			}
			retained := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
			if len(retained.Stashes) != 1 || retained.Stashes[0].ID != stash.ID {
				t.Fatal("conflicting stash was removed")
			}
			for _, verb := range []string{"operation-continue", "operation-skip", "operation-abort"} {
				request := WorkspaceGitRequest{Workdir: root, Action: verb, Snapshot: stopped.Snapshot, Confirm: true}
				if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
					t.Fatalf("stash conflict incorrectly accepted %s", verb)
				}
			}
			resolved := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "resolve-theirs", Paths: []string{"README.md"}, Confirm: true})
			if resolved.Operation != nil {
				t.Fatalf("resolved stash index still advertises a pending operation: %+v", resolved.Operation)
			}
			if content := mustGitOutput(t, root, "show", ":README.md"); content != "stashed side\n" {
				t.Fatalf("stash resolution did not stage saved work: %q", content)
			}
			after := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "stash-list"})
			if len(after.Stashes) != 1 || after.Stashes[0].ID != stash.ID {
				t.Fatal("resolution removed retained stash")
			}
		})
	}
}
