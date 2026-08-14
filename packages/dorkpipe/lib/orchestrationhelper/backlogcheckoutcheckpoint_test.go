package orchestrationhelper

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

type backlogCheckpointTestState struct {
	artifactRoot string
	consumerRoot string
	chain        *backlogCheckoutCheckpointChain
	binding      backlogRuntimeCheckpointBinding
	parent       string
	session      *infrastructure.GitSession
}

func prepareBacklogCheckpointTest(t *testing.T) *backlogCheckpointTestState {
	t.Helper()
	artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", consumerRoot}, args...)...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "DorkPipe Checkpoint Test")
	git("add", "-A")
	git("commit", "-m", "fixture baseline")
	branch := git("branch", "--show-current")
	parent := git("rev-parse", "HEAD")

	applicationChain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	applicationFixture := writeBacklogCheckoutApplicationTestFixture(t, applicationChain, "checkout_checkpoint_application_015", "checkout_checkpoint_application_replay_015", backlogSemanticReviewApproved)
	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, applicationFixture); err != nil {
		t.Fatal(err)
	}
	chain, err := loadBacklogCheckoutCheckpointChain(consumerRoot, artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	metadata := t.TempDir()
	binding := backlogRuntimeCheckpointBinding{
		SessionID: "runtime_checkpoint_session_015", WorkspaceID: "runtime_checkpoint_workspace_015",
		Branch: branch, Workspace: consumerRoot,
	}
	session := &infrastructure.GitSession{
		Schema: 1, SessionID: binding.SessionID, WorkspaceID: binding.WorkspaceID,
		Repo:    infrastructure.GitSessionRepo{SessionRef: binding.Branch},
		Storage: infrastructure.GitSessionStorage{Mode: "managed", Backend: "worktree", Workspace: consumerRoot, Metadata: metadata},
		Policy:  infrastructure.GitSessionPolicy{Checkpoint: "manual", Publish: "none", AllowAgentGit: false},
	}
	return &backlogCheckpointTestState{
		artifactRoot: artifactRoot, consumerRoot: consumerRoot, chain: chain, binding: binding,
		parent: parent, session: session,
	}
}

func writeBacklogCheckpointTestFixture(t *testing.T, state *backlogCheckpointTestState, approvalID, replayIdentity, decision string) string {
	t.Helper()
	fixture := backlogCheckoutCheckpointFixtureForChain(state.chain, approvalID, replayIdentity, decision, state.binding, state.parent, "checkpoint(runtime): reviewed remote patch")
	path := filepath.Join(t.TempDir(), "checkout-checkpoint-approval.json")
	if err := os.WriteFile(path, append(marshalBacklogTestJSON(t, fixture), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBacklogCheckoutCheckpointApprovedRequestCreatesOneExactRuntimeCheckpoint(t *testing.T) {
	state := prepareBacklogCheckpointTest(t)
	upstream := snapshotBacklogCheckoutApplicationUpstream(t, state.artifactRoot)
	upstream["checkout-application-approval.json"] = mustReadFile(t, filepath.Join(state.artifactRoot, "checkout-application-approval.json"))
	upstream["checkout-application.json"] = mustReadFile(t, filepath.Join(state.artifactRoot, "checkout-application.json"))
	fixture := writeBacklogCheckpointTestFixture(t, state, "checkout_checkpoint_approval_015", "checkout_checkpoint_replay_015", backlogSemanticReviewApproved)
	if err := requestBacklogCheckoutCheckpoint(state.consumerRoot, state.artifactRoot, fixture, state.binding); err != nil {
		t.Fatalf("request checkpoint: %v", err)
	}
	requestPath := filepath.Join(state.artifactRoot, "checkpoint-request.json")
	request, err := infrastructure.LoadControlledCheckpointRequest(requestPath)
	if err != nil {
		t.Fatalf("load checkpoint request: %v", err)
	}
	if !reflect.DeepEqual(request.Paths, state.chain.ApplicationChain.Semantic.ChangedPaths) || request.ExpectedParent != state.parent || request.SessionID != state.binding.SessionID {
		t.Fatalf("checkpoint request binding = %+v", request)
	}
	receiptPath := filepath.Join(state.artifactRoot, "checkpoint-receipt.json")
	result, err := infrastructure.CheckpointSessionFromRequest(state.session, requestPath, receiptPath)
	if err != nil {
		t.Fatalf("runtime checkpoint: %v", err)
	}
	if result.Receipt.Parent != state.parent || !reflect.DeepEqual(result.Receipt.Paths, state.chain.ApplicationChain.Semantic.ChangedPaths) {
		t.Fatalf("runtime receipt = %+v", result.Receipt)
	}
	if got := backlogCheckpointGit(t, state.consumerRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"); got != strings.Join(state.chain.ApplicationChain.Semantic.ChangedPaths, "\n") {
		t.Fatalf("committed paths = %q", got)
	}
	if got := backlogCheckpointGit(t, state.consumerRoot, "rev-list", "--count", state.parent+"..HEAD"); got != "1" {
		t.Fatalf("checkpoint count = %q", got)
	}
	if second, err := infrastructure.CheckpointSessionFromRequest(state.session, requestPath, receiptPath); err != nil || !second.Idempotent || second.Receipt.Commit != result.Receipt.Commit {
		t.Fatalf("idempotent runtime checkpoint = %+v, %v", second, err)
	}
	for name, raw := range upstream {
		if got := mustReadFile(t, filepath.Join(state.artifactRoot, name)); !bytesEqual(raw, got) {
			t.Fatalf("checkpoint flow changed upstream evidence %s", name)
		}
	}
}

func TestBacklogCheckoutCheckpointRejectedDecisionCreatesNoRequestOrCommit(t *testing.T) {
	state := prepareBacklogCheckpointTest(t)
	fixture := writeBacklogCheckpointTestFixture(t, state, "checkout_checkpoint_rejected_015", "checkout_checkpoint_rejected_replay_015", backlogSemanticReviewRejected)
	if err := requestBacklogCheckoutCheckpoint(state.consumerRoot, state.artifactRoot, fixture, state.binding); err != nil {
		t.Fatalf("rejected checkpoint decision: %v", err)
	}
	for _, name := range []string{"checkpoint-request.json", "checkpoint-receipt.json"} {
		if _, err := os.Lstat(filepath.Join(state.artifactRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("rejected decision left %s: %v", name, err)
		}
	}
	if got := backlogCheckpointGit(t, state.consumerRoot, "rev-parse", "HEAD"); got != state.parent {
		t.Fatalf("rejected decision changed HEAD to %s", got)
	}
}

func TestBacklogCheckoutCheckpointRejectsMissingMalformedWrongAndStaleBindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *backlogCheckpointTestState, map[string]any)
		code   string
	}{
		{name: "unknown decision", mutate: func(t *testing.T, state *backlogCheckpointTestState, payload map[string]any) {
			payload["decision"] = "maybe"
		}, code: "checkout_checkpoint_decision_invalid:"},
		{name: "wrong application", mutate: func(t *testing.T, state *backlogCheckpointTestState, payload map[string]any) {
			payload["checkout_application_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
		}, code: "checkout_checkpoint_binding_mismatch:"},
		{name: "wrong session", mutate: func(t *testing.T, state *backlogCheckpointTestState, payload map[string]any) {
			payload["runtime_session_id"] = "wrong_session_015"
		}, code: "checkout_checkpoint_binding_mismatch:"},
		{name: "wrong branch", mutate: func(t *testing.T, state *backlogCheckpointTestState, payload map[string]any) {
			payload["runtime_session_branch"] = "wrong/branch"
		}, code: "checkout_checkpoint_binding_mismatch:"},
		{name: "stale postimage", mutate: func(t *testing.T, state *backlogCheckpointTestState, payload map[string]any) {
			if err := os.WriteFile(filepath.Join(state.consumerRoot, state.chain.ApplicationChain.Semantic.ChangedPaths[0]), []byte("stale\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, code: "checkout_checkpoint_chain_invalid:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := prepareBacklogCheckpointTest(t)
			fixture := backlogCheckoutCheckpointFixtureForChain(state.chain, "checkpoint_invalid_015", "checkpoint_invalid_replay_015", backlogSemanticReviewApproved, state.binding, state.parent, "checkpoint(runtime): reviewed remote patch")
			payload := readJSONMap(writeBacklogCheckpointFixtureMap(t, fixture))
			test.mutate(t, state, payload)
			path := filepath.Join(t.TempDir(), "fixture.json")
			if err := os.WriteFile(path, append(marshalBacklogTestJSON(t, payload), '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			err := requestBacklogCheckoutCheckpoint(state.consumerRoot, state.artifactRoot, path, state.binding)
			if err == nil || !strings.HasPrefix(err.Error(), test.code) {
				t.Fatalf("error = %v, want %s", err, test.code)
			}
			for _, name := range []string{"checkout-checkpoint-approval.json", "checkpoint-request.json", "checkpoint-receipt.json"} {
				if _, statErr := os.Lstat(filepath.Join(state.artifactRoot, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejection left %s: %v", name, statErr)
				}
			}
		})
	}
}

func TestBacklogCheckoutCheckpointRejectsReplayAndArtifactTampering(t *testing.T) {
	state := prepareBacklogCheckpointTest(t)
	first := writeBacklogCheckpointTestFixture(t, state, "checkpoint_original_015", "checkpoint_original_replay_015", backlogSemanticReviewRejected)
	if err := requestBacklogCheckoutCheckpoint(state.consumerRoot, state.artifactRoot, first, state.binding); err != nil {
		t.Fatal(err)
	}
	accepted := mustReadFile(t, filepath.Join(state.artifactRoot, "checkout-checkpoint-approval.json"))
	second := writeBacklogCheckpointTestFixture(t, state, "checkpoint_second_015", "checkpoint_original_replay_015", backlogSemanticReviewRejected)
	if err := requestBacklogCheckoutCheckpoint(state.consumerRoot, state.artifactRoot, second, state.binding); err == nil || !strings.HasPrefix(err.Error(), "checkout_checkpoint_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	if !bytesEqual(accepted, mustReadFile(t, filepath.Join(state.artifactRoot, "checkout-checkpoint-approval.json"))) {
		t.Fatal("replay overwrote accepted approval")
	}

	tamperedState := prepareBacklogCheckpointTest(t)
	fixture := writeBacklogCheckpointTestFixture(t, tamperedState, "checkpoint_tamper_015", "checkpoint_tamper_replay_015", backlogSemanticReviewApproved)
	if err := requestBacklogCheckoutCheckpoint(tamperedState.consumerRoot, tamperedState.artifactRoot, fixture, tamperedState.binding); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(tamperedState.artifactRoot, "checkpoint-request.json")
	tampered := strings.Replace(string(mustReadFile(t, requestPath)), tamperedState.parent, strings.Repeat("f", 40), 1)
	if err := os.WriteFile(requestPath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requestBacklogCheckoutCheckpoint(tamperedState.consumerRoot, tamperedState.artifactRoot, fixture, tamperedState.binding); err == nil || !strings.HasPrefix(err.Error(), "checkout_checkpoint_request_artifact_invalid:") {
		t.Fatalf("tampered request error = %v", err)
	}
}

func TestCheckoutApplicationReceiptCannotActAsRuntimeCheckpointRequest(t *testing.T) {
	state := prepareBacklogCheckpointTest(t)
	if _, err := infrastructure.LoadControlledCheckpointRequest(filepath.Join(state.artifactRoot, "checkout-application.json")); err == nil || !strings.HasPrefix(err.Error(), "checkpoint_request_invalid:") {
		t.Fatalf("checkout application receipt was accepted as checkpoint request: %v", err)
	}
}

func TestBacklogCheckoutCheckpointCheckedFixtureMatchesContract(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "fixtures", "backlog.remote", "checkout-checkpoint-approval.json")
	fixture, err := loadBacklogCheckoutCheckpointFixture(path)
	if err != nil {
		t.Fatalf("load checked checkpoint fixture: %v", err)
	}
	if err := validateBacklogCheckoutCheckpointFixture(fixture); err != nil {
		t.Fatalf("validate checked checkpoint fixture: %v", err)
	}
	if fixture.Decision != backlogSemanticReviewApproved || fixture.CheckpointScope != backlogCheckoutCheckpointScope {
		t.Fatalf("checked fixture has unexpected authority: %+v", fixture)
	}
}

func backlogCheckpointGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeBacklogCheckpointFixtureMap(t *testing.T, fixture backlogCheckoutCheckpointFixture) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture-map.json")
	if err := os.WriteFile(path, append(marshalBacklogTestJSON(t, fixture), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
