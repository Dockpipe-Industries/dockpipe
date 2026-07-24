package orchestrationhelper

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

type backlogPublicationTestState struct {
	checkpoint  *backlogCheckpointTestState
	chain       *backlogCheckoutPublicationChain
	remote      string
	remoteID    string
	destination string
}

func prepareBacklogPublicationTest(t *testing.T) *backlogPublicationTestState {
	t.Helper()
	checkpoint := prepareBacklogCheckpointTest(t)
	metadata, err := infrastructure.GitSessionRoot(checkpoint.consumerRoot, checkpoint.session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.session.Storage.Metadata = metadata
	checkpoint.session.Storage.EventLog = filepath.Join(metadata, "events.jsonl")
	checkpoint.session.CreatedAt = "2026-07-24T00:00:00Z"
	checkpoint.session.UpdatedAt = checkpoint.session.CreatedAt
	if err := os.MkdirAll(metadata, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(checkpoint.session, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata, "session.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	exclude, err := os.OpenFile(filepath.Join(checkpoint.consumerRoot, ".git", "info", "exclude"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exclude.WriteString("\n/bin/\n"); err != nil {
		_ = exclude.Close()
		t.Fatal(err)
	}
	if err := exclude.Close(); err != nil {
		t.Fatal(err)
	}
	checkpointFixture := writeBacklogCheckpointTestFixture(t, checkpoint, "publication_checkpoint_approval_015", "publication_checkpoint_replay_015", backlogSemanticReviewApproved)
	if err := requestBacklogCheckoutCheckpoint(checkpoint.consumerRoot, checkpoint.artifactRoot, checkpointFixture, checkpoint.binding); err != nil {
		t.Fatalf("request checkpoint: %v", err)
	}
	if _, err := infrastructure.CheckpointSessionFromRequest(checkpoint.session, filepath.Join(checkpoint.artifactRoot, "checkpoint-request.json"), filepath.Join(checkpoint.artifactRoot, "checkpoint-receipt.json")); err != nil {
		t.Fatalf("runtime checkpoint: %v", err)
	}
	remote := filepath.Join(t.TempDir(), "reviewed.git")
	command := exec.Command("git", "init", "--bare", remote)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("init bare remote: %v\n%s", err, out)
	}
	backlogCheckpointGit(t, checkpoint.consumerRoot, "remote", "add", "reviewed", remote)
	remoteID, err := infrastructure.ControlledPublicationRemoteIdentity(checkpoint.consumerRoot, "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	chain, err := loadBacklogCheckoutPublicationChain(checkpoint.consumerRoot, checkpoint.artifactRoot, checkpoint.binding)
	if err != nil {
		t.Fatalf("load publication chain: %v", err)
	}
	return &backlogPublicationTestState{checkpoint: checkpoint, chain: chain, remote: remote, remoteID: remoteID, destination: "refs/heads/review/task-015"}
}

func writeBacklogPublicationTestFixture(t *testing.T, state *backlogPublicationTestState, approvalID, replayIdentity, decision string) string {
	t.Helper()
	fixture := backlogCheckoutPublicationFixtureForChain(state.chain, approvalID, replayIdentity, decision, "reviewed", state.remoteID, state.destination, "publish the separately approved reviewed checkpoint")
	path := filepath.Join(t.TempDir(), "checkout-publication-approval.json")
	if err := os.WriteFile(path, append(marshalBacklogTestJSON(t, fixture), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBacklogCheckoutPublicationApprovedRequestPublishesExactCheckpoint(t *testing.T) {
	state := prepareBacklogPublicationTest(t)
	upstream := snapshotBacklogCheckoutApplicationUpstream(t, state.checkpoint.artifactRoot)
	for _, name := range []string{"checkout-application-approval.json", "checkout-application.json", "checkout-checkpoint-approval.json", "checkpoint-request.json", "checkpoint-receipt.json"} {
		upstream[name] = mustReadFile(t, filepath.Join(state.checkpoint.artifactRoot, name))
	}
	fixture := writeBacklogPublicationTestFixture(t, state, "checkout_publication_approval_015", "checkout_publication_replay_015", backlogSemanticReviewApproved)
	if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, fixture, state.checkpoint.binding); err != nil {
		t.Fatalf("request publication: %v", err)
	}
	approval, err := readStrictJSONMap(filepath.Join(state.checkpoint.artifactRoot, "checkout-publication-approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	if backlogTestBool(mapValue(approval["capabilities"])["push"]) || !backlogTestBool(mapValue(approval["capabilities"])["submit_exact_runtime_publication_request"]) {
		t.Fatalf("package approval widened authority: %+v", approval["capabilities"])
	}
	requestPath := filepath.Join(state.checkpoint.artifactRoot, "publication-request.json")
	request, err := infrastructure.LoadControlledPublicationRequest(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	if request.SourceCommit != state.chain.CheckpointReceipt.Commit || request.DestinationRef != state.destination || request.RemoteIdentity != state.remoteID {
		t.Fatalf("request binding = %+v", request)
	}
	result, err := infrastructure.PublishSessionFromRequest(state.chain.Session,
		filepath.Join(state.checkpoint.artifactRoot, "checkpoint-request.json"), filepath.Join(state.checkpoint.artifactRoot, "checkpoint-receipt.json"),
		requestPath, filepath.Join(state.checkpoint.artifactRoot, "publication-receipt.json"))
	if err != nil {
		t.Fatalf("runtime publication: %v", err)
	}
	if result.Receipt.SourceCommit != state.chain.CheckpointReceipt.Commit || result.Receipt.DestinationRef != state.destination || result.Receipt.Push.Force || result.Receipt.Push.UpstreamConfigured {
		t.Fatalf("runtime receipt = %+v", result.Receipt)
	}
	if got := backlogCheckpointGit(t, state.remote, "rev-parse", state.destination); got != state.chain.CheckpointReceipt.Commit {
		t.Fatalf("remote ref = %s", got)
	}
	if refs := backlogCheckpointGit(t, state.remote, "for-each-ref", "--format=%(refname)", "refs/heads"); refs != state.destination {
		t.Fatalf("remote refs = %q", refs)
	}
	for name, original := range upstream {
		if got := mustReadFile(t, filepath.Join(state.checkpoint.artifactRoot, name)); !bytesEqual(original, got) {
			t.Fatalf("publication changed immutable evidence %s", name)
		}
	}
}

func TestBacklogCheckoutPublicationRejectedDecisionCreatesNoRequestOrPush(t *testing.T) {
	state := prepareBacklogPublicationTest(t)
	fixture := writeBacklogPublicationTestFixture(t, state, "checkout_publication_rejected_015", "checkout_publication_rejected_replay_015", backlogSemanticReviewRejected)
	if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, fixture, state.checkpoint.binding); err != nil {
		t.Fatalf("rejected publication: %v", err)
	}
	for _, name := range []string{"publication-request.json", "publication-receipt.json"} {
		if _, err := os.Lstat(filepath.Join(state.checkpoint.artifactRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("rejected decision left %s: %v", name, err)
		}
	}
	if got, _ := exec.Command("git", "--git-dir", state.remote, "for-each-ref", "--format=%(refname)").Output(); strings.TrimSpace(string(got)) != "" {
		t.Fatalf("rejected decision changed remote refs: %q", got)
	}
}

func TestBacklogCheckoutPublicationRejectsMalformedWrongReplayAndTampering(t *testing.T) {
	t.Run("checkpoint receipt alone cannot publish", func(t *testing.T) {
		state := prepareBacklogPublicationTest(t)
		if _, err := os.Lstat(filepath.Join(state.checkpoint.artifactRoot, "publication-request.json")); !os.IsNotExist(err) {
			t.Fatalf("checkpoint receipt implied a publication request: %v", err)
		}
	})

	t.Run("wrong chain binding", func(t *testing.T) {
		state := prepareBacklogPublicationTest(t)
		fixture := backlogCheckoutPublicationFixtureForChain(state.chain, "publication_wrong_015", "publication_wrong_replay_015", backlogSemanticReviewApproved, "reviewed", state.remoteID, state.destination, "publish the separately approved reviewed checkpoint")
		fixture.ImmutableChainFingerprint = sha256String([]byte("wrong"))
		path := filepath.Join(t.TempDir(), "wrong.json")
		_ = os.WriteFile(path, append(marshalBacklogTestJSON(t, fixture), '\n'), 0o644)
		if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, path, state.checkpoint.binding); err == nil || !strings.HasPrefix(err.Error(), "checkout_publication_binding_mismatch:") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid destination", func(t *testing.T) {
		state := prepareBacklogPublicationTest(t)
		fixture := backlogCheckoutPublicationFixtureForChain(state.chain, "publication_tag_015", "publication_tag_replay_015", backlogSemanticReviewApproved, "reviewed", state.remoteID, "refs/tags/v1", "publish the separately approved reviewed checkpoint")
		path := filepath.Join(t.TempDir(), "tag.json")
		_ = os.WriteFile(path, append(marshalBacklogTestJSON(t, fixture), '\n'), 0o644)
		if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, path, state.checkpoint.binding); err == nil || !strings.HasPrefix(err.Error(), "checkout_publication_destination_invalid:") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("replay and artifact tampering", func(t *testing.T) {
		state := prepareBacklogPublicationTest(t)
		first := writeBacklogPublicationTestFixture(t, state, "publication_original_015", "publication_original_replay_015", backlogSemanticReviewRejected)
		if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, first, state.checkpoint.binding); err != nil {
			t.Fatal(err)
		}
		accepted := mustReadFile(t, filepath.Join(state.checkpoint.artifactRoot, "checkout-publication-approval.json"))
		second := writeBacklogPublicationTestFixture(t, state, "publication_other_015", "publication_original_replay_015", backlogSemanticReviewRejected)
		if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, second, state.checkpoint.binding); err == nil || !strings.HasPrefix(err.Error(), "checkout_publication_replay:") {
			t.Fatalf("replay error = %v", err)
		}
		if !reflect.DeepEqual(accepted, mustReadFile(t, filepath.Join(state.checkpoint.artifactRoot, "checkout-publication-approval.json"))) {
			t.Fatal("replay overwrote accepted approval")
		}
		tampered := []byte(strings.Replace(string(accepted), state.destination, "refs/heads/tampered", 1))
		if err := os.WriteFile(filepath.Join(state.checkpoint.artifactRoot, "checkout-publication-approval.json"), tampered, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := requestBacklogCheckoutPublication(state.checkpoint.consumerRoot, state.checkpoint.artifactRoot, first, state.checkpoint.binding); err == nil || !strings.HasPrefix(err.Error(), "checkout_publication_approval_artifact_invalid:") {
			t.Fatalf("tamper error = %v", err)
		}
	})
}

func TestBacklogCheckoutPublicationCheckedFixtureMatchesContract(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "fixtures", "backlog.remote", "checkout-publication-approval.json")
	fixture, err := loadBacklogCheckoutPublicationFixture(path)
	if err != nil {
		t.Fatalf("load checked publication fixture: %v", err)
	}
	if err := validateBacklogCheckoutPublicationFixture(fixture); err != nil {
		t.Fatalf("validate checked publication fixture: %v", err)
	}
	if fixture.Decision != backlogSemanticReviewApproved || fixture.PublicationScope != backlogCheckoutPublicationScope {
		t.Fatalf("checked fixture has unexpected authority: %+v", fixture)
	}
}
