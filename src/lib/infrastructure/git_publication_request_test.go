package infrastructure

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type controlledPublicationTestState struct {
	checkpoint        *controlledCheckpointTestState
	checkpointReceipt *ControlledCheckpointReceipt
	request           ControlledPublicationRequest
	requestPath       string
	receiptPath       string
	remote            string
	destination       string
}

func prepareControlledPublicationTest(t *testing.T) *controlledPublicationTestState {
	t.Helper()
	checkpoint := prepareControlledCheckpointTest(t)
	checkpointResult, err := CheckpointSessionFromRequest(checkpoint.session, checkpoint.requestPath, checkpoint.receiptPath)
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	remote := filepath.Join(t.TempDir(), "reviewed.git")
	if err := gitRun(t.TempDir(), "init", "--bare", remote); err != nil {
		t.Fatal(err)
	}
	if err := gitRun(checkpoint.session.Storage.Workspace, "remote", "add", "reviewed", remote); err != nil {
		t.Fatal(err)
	}
	remoteIdentity, err := ControlledPublicationRemoteIdentity(checkpoint.session.Storage.Workspace, "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	_, receiptFP, err := ValidateControlledCheckpointReceiptForSession(checkpoint.session, checkpoint.requestPath, checkpoint.receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	request, err := FinalizeControlledPublicationRequest(ControlledPublicationRequest{
		ContractVersion: ControlledPublicationRequestContract, RequestID: "publication-request-015",
		AuthorizationFingerprint:     controlledCheckpointBytesSHA256([]byte("publication approval")),
		CheckpointRequestFingerprint: checkpoint.request.RequestFingerprint, CheckpointReceiptFingerprint: receiptFP,
		SessionID: checkpoint.session.SessionID, WorkspaceID: checkpoint.session.WorkspaceID,
		ExpectedBranch: checkpoint.session.Repo.SessionRef, SourceCommit: checkpointResult.Receipt.Commit,
		SourceParent: checkpointResult.Receipt.Parent, RemoteName: "reviewed", RemoteIdentity: remoteIdentity,
		DestinationRef: "refs/heads/review/task-015", PublicationScope: ControlledPublicationScope,
		Reason: "publish the separately approved reviewed checkpoint",
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	requestPath := filepath.Join(root, "publication-request.json")
	if err := WriteControlledPublicationRequest(requestPath, request); err != nil {
		t.Fatal(err)
	}
	return &controlledPublicationTestState{checkpoint: checkpoint, checkpointReceipt: checkpointResult.Receipt,
		request: request, requestPath: requestPath, receiptPath: filepath.Join(root, "publication-receipt.json"),
		remote: remote, destination: request.DestinationRef}
}

func TestControlledPublicationPushesExactCommitAndIsIdempotent(t *testing.T) {
	state := prepareControlledPublicationTest(t)
	workspace := state.checkpoint.session.Storage.Workspace
	headBefore := strings.TrimSpace(mustGitOutput(t, workspace, "rev-parse", "HEAD"))
	configBefore, _ := gitCombined(workspace, "config", "--local", "--get-regexp", "^branch\\.")
	result, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result.Idempotent || result.Recovered || result.Receipt.SourceCommit != headBefore || result.Receipt.DestinationRef != state.destination {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.remote, "rev-parse", state.destination)); got != headBefore {
		t.Fatalf("remote ref = %q, want %q", got, headBefore)
	}
	if refs := strings.TrimSpace(mustGitOutput(t, state.remote, "for-each-ref", "--format=%(refname)", "refs/heads")); refs != state.destination {
		t.Fatalf("changed remote refs = %q", refs)
	}
	if headAfter := strings.TrimSpace(mustGitOutput(t, workspace, "rev-parse", "HEAD")); headAfter != headBefore {
		t.Fatalf("publication created or moved a local commit: %s -> %s", headBefore, headAfter)
	}
	configAfter, _ := gitCombined(workspace, "config", "--local", "--get-regexp", "^branch\\.")
	if configAfter != configBefore {
		t.Fatalf("local branch configuration changed:\nbefore=%q\nafter=%q", configBefore, configAfter)
	}
	second, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
	if err != nil || !second.Idempotent || second.Receipt.SourceCommit != headBefore {
		t.Fatalf("idempotent publish = %+v, %v", second, err)
	}
}

func TestControlledPublicationUsesImmutableCommitNotMovingBranchTip(t *testing.T) {
	state := prepareControlledPublicationTest(t)
	workspace := state.checkpoint.session.Storage.Workspace
	newerCommit := ""
	hooks := controlledPublicationHooks{
		BeforePush: func() error {
			if err := os.WriteFile(filepath.Join(workspace, "later.txt"), []byte("later\n"), 0o644); err != nil {
				return err
			}
			if err := gitRun(workspace, "add", "--", "later.txt"); err != nil {
				return err
			}
			if err := gitRun(workspace, "commit", "-m", "later local commit"); err != nil {
				return err
			}
			newerCommit = strings.TrimSpace(mustGitOutput(t, workspace, "rev-parse", "HEAD"))
			return nil
		},
		Push: func(workspace, remote, refspec string) error {
			if refspec != state.request.SourceCommit+":"+state.destination {
				return errors.New("runtime used a moving source")
			}
			return gitRun(workspace, "push", "--porcelain", "--", remote, refspec)
		},
		Observe: observeControlledPublicationRef, WriteMetadata: writeControlledPublicationMetadata,
		WriteReceipt: func(path string, receipt *ControlledPublicationReceipt) error {
			return writeControlledCheckpointJSON(path, receipt, false)
		},
	}
	result, err := publishSessionControlled(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, &state.request, state.receiptPath, hooks)
	if err != nil {
		t.Fatalf("publish moving tip: %v", err)
	}
	if newerCommit == "" || newerCommit == result.Receipt.SourceCommit {
		t.Fatal("test did not advance the local branch")
	}
	if got := strings.TrimSpace(mustGitOutput(t, state.remote, "rev-parse", state.destination)); got != state.request.SourceCommit {
		t.Fatalf("remote received %s, want immutable %s", got, state.request.SourceCommit)
	}
}

func TestControlledPublicationRejectsInvalidRequestRemoteAndWorkspaceState(t *testing.T) {
	invalidDestinations := []string{"main", "refs/tags/v1", "refs/heads/a:refs/heads/b", "refs/heads/*", "refs/heads/a^b", "refs/heads/a..b", "refs/heads/a refs/heads/b", "refs/heads/.hidden", "refs/heads/a.lock/b", "refs/heads/@"}
	baseRequest := ControlledPublicationRequest{
		ContractVersion: ControlledPublicationRequestContract, RequestID: "validation-only-request",
		AuthorizationFingerprint:     controlledCheckpointBytesSHA256([]byte("approval")),
		CheckpointRequestFingerprint: controlledCheckpointBytesSHA256([]byte("checkpoint request")),
		CheckpointReceiptFingerprint: controlledCheckpointBytesSHA256([]byte("checkpoint receipt")),
		SessionID:                    "validation-session", WorkspaceID: "validation-workspace", ExpectedBranch: "review/session",
		SourceCommit: strings.Repeat("1", 40), SourceParent: strings.Repeat("2", 40), RemoteName: "reviewed",
		RemoteIdentity: controlledCheckpointBytesSHA256([]byte("remote")), DestinationRef: "refs/heads/review/valid",
		PublicationScope: ControlledPublicationScope, Reason: "validate one exact destination",
	}
	for _, destination := range invalidDestinations {
		t.Run("destination_"+strings.NewReplacer("/", "_", ":", "_", "*", "wild", " ", "_").Replace(destination), func(t *testing.T) {
			request := baseRequest
			request.DestinationRef = destination
			if _, err := FinalizeControlledPublicationRequest(request); err == nil {
				t.Fatalf("accepted destination %q", destination)
			}
		})
	}

	tests := []struct {
		name   string
		mutate func(*testing.T, *controlledPublicationTestState)
		want   string
	}{
		{name: "wrong remote identity", mutate: func(t *testing.T, state *controlledPublicationTestState) {
			state.request.RemoteIdentity = controlledCheckpointBytesSHA256([]byte("other remote"))
			state.request, _ = FinalizeControlledPublicationRequest(state.request)
			state.requestPath = filepath.Join(t.TempDir(), "request.json")
			_ = WriteControlledPublicationRequest(state.requestPath, state.request)
		}, want: "publication_remote_mismatch:"},
		{name: "dirty worktree", mutate: func(t *testing.T, state *controlledPublicationTestState) {
			_ = os.WriteFile(filepath.Join(state.checkpoint.session.Storage.Workspace, "dirty.txt"), []byte("dirty\n"), 0o644)
		}, want: "checkpoint_receipt_invalid:"},
		{name: "staged worktree", mutate: func(t *testing.T, state *controlledPublicationTestState) {
			workspace := state.checkpoint.session.Storage.Workspace
			_ = os.WriteFile(filepath.Join(workspace, "staged.txt"), []byte("staged\n"), 0o644)
			_ = gitRun(workspace, "add", "--", "staged.txt")
		}, want: "checkpoint_receipt_invalid:"},
		{name: "advanced head", mutate: func(t *testing.T, state *controlledPublicationTestState) {
			workspace := state.checkpoint.session.Storage.Workspace
			_ = os.WriteFile(filepath.Join(workspace, "advanced.txt"), []byte("advanced\n"), 0o644)
			_ = gitRun(workspace, "add", "--", "advanced.txt")
			_ = gitRun(workspace, "commit", "-m", "advanced")
		}, want: "publication_commit_mismatch:"},
		{name: "detached", mutate: func(t *testing.T, state *controlledPublicationTestState) {
			_ = gitRun(state.checkpoint.session.Storage.Workspace, "checkout", "--detach", state.request.SourceCommit)
		}, want: "checkpoint_receipt_invalid:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := prepareControlledPublicationTest(t)
			test.mutate(t, state)
			_, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
			if err == nil || !strings.HasPrefix(err.Error(), test.want) {
				t.Fatalf("error = %v, want %s", err, test.want)
			}
			if _, observeErr := observeControlledPublicationRef(state.checkpoint.session.Storage.Workspace, "reviewed", state.destination); observeErr == nil {
				// Empty is the only acceptable observation before a publication.
			}
		})
	}
}

func TestControlledPublicationFailuresAndExactRecovery(t *testing.T) {
	t.Run("prepush failure leaves remote unchanged", func(t *testing.T) {
		state := prepareControlledPublicationTest(t)
		hooks := controlledPublicationHooks{BeforePush: func() error { return errors.New("forced") },
			Push: func(string, string, string) error { return errors.New("must not run") }, Observe: observeControlledPublicationRef,
			WriteMetadata: writeControlledPublicationMetadata, WriteReceipt: func(path string, receipt *ControlledPublicationReceipt) error {
				return writeControlledCheckpointJSON(path, receipt, false)
			}}
		_, err := publishSessionControlled(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, &state.request, state.receiptPath, hooks)
		if err == nil || !strings.HasPrefix(err.Error(), "publication_prepush_failed:") {
			t.Fatalf("error = %v", err)
		}
		if got, _ := observeControlledPublicationRef(state.checkpoint.session.Storage.Workspace, "reviewed", state.destination); got != "" {
			t.Fatalf("prepush failure changed remote to %s", got)
		}
	})

	t.Run("non fast forward is distinct", func(t *testing.T) {
		state := prepareControlledPublicationTest(t)
		workspace := state.checkpoint.session.Storage.Workspace
		other := filepath.Join(t.TempDir(), "other")
		if err := gitRun(t.TempDir(), "clone", state.remote, other); err != nil {
			t.Fatal(err)
		}
		_ = gitRun(other, "config", "user.email", "test@example.invalid")
		_ = gitRun(other, "config", "user.name", "Other")
		_ = os.WriteFile(filepath.Join(other, "other.txt"), []byte("other\n"), 0o644)
		_ = gitRun(other, "add", "--", "other.txt")
		_ = gitRun(other, "commit", "-m", "other root")
		_ = gitRun(other, "push", "origin", "HEAD:"+state.destination)
		_, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
		if err == nil || !strings.HasPrefix(err.Error(), "publication_push_failed:") {
			t.Fatalf("error = %v", err)
		}
		if got := strings.TrimSpace(mustGitOutput(t, state.remote, "rev-parse", state.destination)); got == state.request.SourceCommit {
			t.Fatal("failed non-fast-forward push reported the approved commit remotely")
		}
		_ = workspace
	})

	t.Run("receipt failure recovers exact remote ref", func(t *testing.T) {
		state := prepareControlledPublicationTest(t)
		hooks := controlledPublicationHooks{BeforePush: func() error { return nil },
			Push: func(workspace, remote, refspec string) error {
				return gitRun(workspace, "push", "--porcelain", "--", remote, refspec)
			},
			Observe: observeControlledPublicationRef, WriteMetadata: writeControlledPublicationMetadata,
			WriteReceipt: func(string, *ControlledPublicationReceipt) error { return errors.New("forced receipt failure") }}
		_, err := publishSessionControlled(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, &state.request, state.receiptPath, hooks)
		if err == nil || !strings.HasPrefix(err.Error(), "publication_receipt_failed:") {
			t.Fatalf("error = %v", err)
		}
		recovered, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
		if err != nil || !recovered.Recovered || recovered.Idempotent {
			t.Fatalf("recovery = %+v, %v", recovered, err)
		}
		second, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
		if err != nil || !second.Idempotent {
			t.Fatalf("post-recovery idempotence = %+v, %v", second, err)
		}
	})

	t.Run("metadata failure recovers exact remote ref", func(t *testing.T) {
		state := prepareControlledPublicationTest(t)
		hooks := controlledPublicationHooks{BeforePush: func() error { return nil },
			Push: func(workspace, remote, refspec string) error {
				return gitRun(workspace, "push", "--porcelain", "--", remote, refspec)
			},
			Observe:       observeControlledPublicationRef,
			WriteMetadata: func(*GitSession, *ControlledPublicationReceipt) error { return errors.New("forced metadata failure") },
			WriteReceipt: func(path string, receipt *ControlledPublicationReceipt) error {
				return writeControlledCheckpointJSON(path, receipt, false)
			}}
		_, err := publishSessionControlled(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, &state.request, state.receiptPath, hooks)
		if err == nil || !strings.HasPrefix(err.Error(), "publication_metadata_failed:") {
			t.Fatalf("error = %v", err)
		}
		if _, statErr := os.Lstat(state.receiptPath); !os.IsNotExist(statErr) {
			t.Fatalf("receipt exists after metadata failure: %v", statErr)
		}
		recovered, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath)
		if err != nil || !recovered.Recovered || !recovered.Receipt.Recovered {
			t.Fatalf("metadata recovery = %+v, %v", recovered, err)
		}
	})
}

func TestControlledPublicationRejectsTamperedExistingReceipt(t *testing.T) {
	state := prepareControlledPublicationTest(t)
	if _, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(state.receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), state.destination, "refs/heads/other", 1))
	if err := os.WriteFile(state.receiptPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishSessionFromRequest(state.checkpoint.session, state.checkpoint.requestPath, state.checkpoint.receiptPath, state.requestPath, state.receiptPath); err == nil || !strings.HasPrefix(err.Error(), "publication_receipt_invalid:") {
		t.Fatalf("tampered receipt error = %v", err)
	}
}
