package infrastructure

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type controlledCheckpointTestState struct {
	repo        string
	session     *GitSession
	request     ControlledCheckpointRequest
	requestPath string
	receiptPath string
	parent      string
}

func prepareControlledCheckpointTest(t *testing.T) *controlledCheckpointTestState {
	t.Helper()
	repo := initGitSessionTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitRun(repo, "add", "--", "other.txt"); err != nil {
		t.Fatal(err)
	}
	if err := gitRun(repo, "commit", "-m", "add second fixture file"); err != nil {
		t.Fatal(err)
	}
	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID: "checkpoint-workspace", SourceDir: repo, Mode: "managed", BranchPrefix: "ai",
		SessionID: "controlled-checkpoint", Checkpoint: "manual", Publish: "none",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	t.Cleanup(func() { gitRemoveWorktree(t, repo, session.Storage.Workspace) })
	parent, err := GitRevParse(session.Storage.Workspace, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	postimage := []byte("reviewed checkpoint\n")
	if err := os.WriteFile(filepath.Join(session.Storage.Workspace, "README.md"), postimage, 0o644); err != nil {
		t.Fatal(err)
	}
	request, err := FinalizeControlledCheckpointRequest(ControlledCheckpointRequest{
		ContractVersion: ControlledCheckpointRequestContract,
		RequestID:       "checkpoint-request-015", AuthorizationFingerprint: controlledCheckpointBytesSHA256([]byte("approval")),
		SessionID: session.SessionID, WorkspaceID: session.WorkspaceID, ExpectedBranch: session.Repo.SessionRef,
		ExpectedParent: strings.TrimSpace(parent), CheckpointScope: ControlledCheckpointScope,
		Message: "checkpoint(runtime): reviewed remote patch", Paths: []string{"README.md"},
		Postimages: []ControlledCheckpointPostimage{{Path: "README.md", SHA256: controlledCheckpointBytesSHA256(postimage), Bytes: int64(len(postimage))}},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := t.TempDir()
	requestPath := filepath.Join(artifactRoot, "checkpoint-request.json")
	if err := WriteControlledCheckpointRequest(requestPath, request); err != nil {
		t.Fatal(err)
	}
	return &controlledCheckpointTestState{
		repo: repo, session: session, request: request, requestPath: requestPath,
		receiptPath: filepath.Join(artifactRoot, "checkpoint-receipt.json"), parent: strings.TrimSpace(parent),
	}
}

func TestControlledCheckpointCreatesExactCommitAndIsIdempotent(t *testing.T) {
	state := prepareControlledCheckpointTest(t)
	result, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath)
	if err != nil {
		t.Fatalf("CheckpointSessionFromRequest: %v", err)
	}
	if result.Idempotent || result.Recovered || result.Receipt.Parent != state.parent || result.Receipt.Commit == state.parent {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !reflect.DeepEqual(result.Receipt.Paths, []string{"README.md"}) || result.Receipt.Actions.Push || result.Receipt.Actions.Publication || result.Receipt.Actions.Sync || result.Receipt.Actions.Merge {
		t.Fatalf("receipt widened checkpoint authority: %+v", result.Receipt)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")); got != "README.md" {
		t.Fatalf("checkpoint paths = %q", got)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-parse", "HEAD^")); got != state.parent {
		t.Fatalf("checkpoint parent = %q", got)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "status", "--porcelain")); got != "" {
		t.Fatalf("workspace is dirty after checkpoint: %q", got)
	}
	second, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath)
	if err != nil {
		t.Fatalf("idempotent checkpoint: %v", err)
	}
	if !second.Idempotent || second.Receipt.Commit != result.Receipt.Commit {
		t.Fatalf("idempotent result = %+v", second)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-list", "--count", state.parent+"..HEAD")); got != "1" {
		t.Fatalf("checkpoint created %s commits", got)
	}
}

func TestControlledCheckpointRejectsUnrelatedAndStaleWorkspaceState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *controlledCheckpointTestState)
		want   string
	}{
		{name: "unrelated modified", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := os.WriteFile(filepath.Join(state.session.Storage.Workspace, "other.txt"), []byte("modified\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_change_set_mismatch:"},
		{name: "unrelated untracked", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := os.WriteFile(filepath.Join(state.session.Storage.Workspace, "untracked.txt"), []byte("untracked\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_change_set_mismatch:"},
		{name: "unrelated deleted", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := os.Remove(filepath.Join(state.session.Storage.Workspace, "other.txt")); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_change_set_mismatch:"},
		{name: "unrelated renamed", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := os.Rename(filepath.Join(state.session.Storage.Workspace, "other.txt"), filepath.Join(state.session.Storage.Workspace, "renamed.txt")); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_change_set_mismatch:"},
		{name: "stale accepted path", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := os.WriteFile(filepath.Join(state.session.Storage.Workspace, "README.md"), []byte("stale\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_postimage_mismatch:"},
		{name: "deleted accepted path", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := os.Remove(filepath.Join(state.session.Storage.Workspace, "README.md")); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_change_set_mismatch:"},
		{name: "preexisting staged path", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			if err := gitRun(state.session.Storage.Workspace, "add", "--", "README.md"); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_staged_changes_present:"},
		{name: "wrong parent", mutate: func(t *testing.T, state *controlledCheckpointTestState) {
			state.request.ExpectedParent = strings.Repeat("f", 40)
			updated, err := FinalizeControlledCheckpointRequest(state.request)
			if err != nil {
				t.Fatal(err)
			}
			state.request = updated
			if err := os.Remove(state.requestPath); err != nil {
				t.Fatal(err)
			}
			if err := WriteControlledCheckpointRequest(state.requestPath, updated); err != nil {
				t.Fatal(err)
			}
		}, want: "checkpoint_recovery_ambiguous:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := prepareControlledCheckpointTest(t)
			test.mutate(t, state)
			_, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath)
			if err == nil || !strings.HasPrefix(err.Error(), test.want) {
				t.Fatalf("error = %v, want %s", err, test.want)
			}
			if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-parse", "HEAD")); got != state.parent {
				t.Fatalf("HEAD changed after rejection: %s", got)
			}
			if _, statErr := os.Lstat(state.receiptPath); !os.IsNotExist(statErr) {
				t.Fatalf("receipt exists after rejection: %v", statErr)
			}
		})
	}
}

func TestControlledCheckpointRejectsSessionMismatchAndEscapingRequest(t *testing.T) {
	state := prepareControlledCheckpointTest(t)
	mismatched := *state.session
	mismatched.WorkspaceID = "wrong-workspace"
	if _, err := CheckpointSessionFromRequest(&mismatched, state.requestPath, state.receiptPath); err == nil || !strings.HasPrefix(err.Error(), "checkpoint_session_mismatch:") {
		t.Fatalf("workspace mismatch error = %v", err)
	}
	escaping := state.request
	escaping.Paths = []string{"../README.md"}
	escaping.Postimages[0].Path = "../README.md"
	if _, err := FinalizeControlledCheckpointRequest(escaping); err == nil || !strings.HasPrefix(err.Error(), "checkpoint_request_invalid:") {
		t.Fatalf("escaping request error = %v", err)
	}
}

func TestControlledCheckpointRejectsNonRegularAndLinkedPaths(t *testing.T) {
	t.Run("non-regular accepted path", func(t *testing.T) {
		state := prepareControlledCheckpointTest(t)
		path := filepath.Join(state.session.Storage.Workspace, "README.md")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath); err == nil || !strings.HasPrefix(err.Error(), "checkpoint_change_set_mismatch:") {
			t.Fatalf("non-regular path error = %v", err)
		}
	})

	t.Run("linked accepted path", func(t *testing.T) {
		state := prepareControlledCheckpointTest(t)
		path := filepath.Join(state.session.Storage.Workspace, "README.md")
		target := filepath.Join(t.TempDir(), "target.txt")
		if err := os.WriteFile(target, []byte("reviewed checkpoint\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath); err == nil {
			t.Fatal("linked accepted path was checkpointed")
		}
	})
}

func TestControlledCheckpointPrecommitFailureRestoresIndex(t *testing.T) {
	state := prepareControlledCheckpointTest(t)
	hooks := controlledCheckpointHooks{
		BeforeCommit:  func() error { return errors.New("forced precommit failure") },
		Commit:        func(workspace, message string) error { return gitRun(workspace, "commit", "-m", message) },
		WriteMetadata: writeControlledCheckpointMetadata,
		WriteReceipt: func(path string, receipt *ControlledCheckpointReceipt) error {
			return writeControlledCheckpointJSON(path, receipt, false)
		},
	}
	_, err := checkpointSessionControlled(state.session, &state.request, state.receiptPath, hooks)
	if err == nil || !strings.HasPrefix(err.Error(), "checkpoint_precommit_failed:") {
		t.Fatalf("error = %v", err)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "diff", "--cached", "--name-only")); got != "" {
		t.Fatalf("index changed after precommit failure: %q", got)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-parse", "HEAD")); got != state.parent {
		t.Fatalf("HEAD changed after precommit failure: %s", got)
	}
}

func TestControlledCheckpointMetadataFailureRecoversExactCommit(t *testing.T) {
	state := prepareControlledCheckpointTest(t)
	hooks := controlledCheckpointHooks{
		BeforeCommit:  func() error { return nil },
		Commit:        func(workspace, message string) error { return gitRun(workspace, "commit", "-m", message) },
		WriteMetadata: func(*GitSession, *GitCheckpoint) error { return errors.New("forced metadata failure") },
		WriteReceipt: func(path string, receipt *ControlledCheckpointReceipt) error {
			return writeControlledCheckpointJSON(path, receipt, false)
		},
	}
	_, err := checkpointSessionControlled(state.session, &state.request, state.receiptPath, hooks)
	if err == nil || !strings.HasPrefix(err.Error(), "checkpoint_metadata_failed:") {
		t.Fatalf("error = %v", err)
	}
	commit := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-parse", "HEAD"))
	if commit == state.parent {
		t.Fatal("metadata failure did not preserve the created commit for recovery")
	}
	if _, statErr := os.Lstat(state.receiptPath); !os.IsNotExist(statErr) {
		t.Fatalf("receipt exists after metadata failure: %v", statErr)
	}
	recovered, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath)
	if err != nil {
		t.Fatalf("recover checkpoint: %v", err)
	}
	if !recovered.Recovered || recovered.Receipt.Commit != commit {
		t.Fatalf("recovery result = %+v", recovered)
	}
}

func TestControlledCheckpointCommitAndReceiptFailuresAreDistinct(t *testing.T) {
	t.Run("commit failure restores index", func(t *testing.T) {
		state := prepareControlledCheckpointTest(t)
		hooks := controlledCheckpointHooks{
			BeforeCommit:  func() error { return nil },
			Commit:        func(string, string) error { return errors.New("forced commit failure") },
			WriteMetadata: writeControlledCheckpointMetadata,
			WriteReceipt: func(path string, receipt *ControlledCheckpointReceipt) error {
				return writeControlledCheckpointJSON(path, receipt, false)
			},
		}
		_, err := checkpointSessionControlled(state.session, &state.request, state.receiptPath, hooks)
		if err == nil || !strings.HasPrefix(err.Error(), "checkpoint_commit_failed:") {
			t.Fatalf("commit failure error = %v", err)
		}
		if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "diff", "--cached", "--name-only")); got != "" {
			t.Fatalf("index changed after commit failure: %q", got)
		}
		if got := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-parse", "HEAD")); got != state.parent {
			t.Fatalf("HEAD changed after commit failure: %s", got)
		}
	})

	t.Run("receipt failure recovers commit", func(t *testing.T) {
		state := prepareControlledCheckpointTest(t)
		hooks := controlledCheckpointHooks{
			BeforeCommit:  func() error { return nil },
			Commit:        func(workspace, message string) error { return gitRun(workspace, "commit", "-m", message) },
			WriteMetadata: writeControlledCheckpointMetadata,
			WriteReceipt:  func(string, *ControlledCheckpointReceipt) error { return errors.New("forced receipt failure") },
		}
		_, err := checkpointSessionControlled(state.session, &state.request, state.receiptPath, hooks)
		if err == nil || !strings.HasPrefix(err.Error(), "checkpoint_receipt_failed:") {
			t.Fatalf("receipt failure error = %v", err)
		}
		commit := strings.TrimSpace(mustGitOutput(t, state.session.Storage.Workspace, "rev-parse", "HEAD"))
		result, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath)
		if err != nil || !result.Recovered || result.Receipt.Commit != commit {
			t.Fatalf("receipt recovery = %+v, %v", result, err)
		}
	})
}

func TestControlledCheckpointRejectsTamperedReceipt(t *testing.T) {
	state := prepareControlledCheckpointTest(t)
	result, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(state.receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), result.Receipt.Commit, strings.Repeat("f", 40), 1))
	if err := os.WriteFile(state.receiptPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckpointSessionFromRequest(state.session, state.requestPath, state.receiptPath); err == nil || !strings.HasPrefix(err.Error(), "checkpoint_receipt_invalid:") {
		t.Fatalf("tampered receipt error = %v", err)
	}
}
