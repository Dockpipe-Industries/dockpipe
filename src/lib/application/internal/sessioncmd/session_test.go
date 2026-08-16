package sessioncmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestSessionCommandsListInspectSwitch(t *testing.T) {
	repo := initSessionCommandRepo(t)
	session, err := infrastructure.CreateSessionBranch(infrastructure.GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BranchPrefix: "ai",
		SessionID:    "cmd-session",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	defer removeSessionCommandWorktree(t, repo, session.Storage.Workspace)

	listOut, err := captureStdout(t, func() error {
		return Run([]string{"list", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listOut, "cmd-session") || !strings.Contains(listOut, session.Repo.SessionRef) {
		t.Fatalf("list output missing session: %q", listOut)
	}

	inspectOut, err := captureStdout(t, func() error {
		return Run([]string{"inspect", "cmd", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspectOut, "Session:") || !strings.Contains(inspectOut, session.Storage.Workspace) {
		t.Fatalf("inspect output missing details: %q", inspectOut)
	}
	if !strings.Contains(inspectOut, filepath.Join(session.Storage.Metadata, "events.jsonl")) {
		t.Fatalf("inspect output missing event log path: %q", inspectOut)
	}

	inspectJSON, err := captureStdout(t, func() error {
		return Run([]string{"inspect", "cmd-session", "--workdir", repo, "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var inspected infrastructure.GitSession
	if err := json.Unmarshal([]byte(inspectJSON), &inspected); err != nil {
		t.Fatalf("inspect json should decode: %v\n%s", err, inspectJSON)
	}
	if want := filepath.Join(session.Storage.Metadata, "events.jsonl"); inspected.Storage.EventLog != want {
		t.Fatalf("inspect json event_log = %q want %q", inspected.Storage.EventLog, want)
	}

	switchOut, err := captureStdout(t, func() error {
		return Run([]string{"switch", "cmd-session", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(switchOut, session.Storage.Workspace) || !strings.Contains(switchOut, "Branch:") {
		t.Fatalf("switch output missing handoff: %q", switchOut)
	}
}

func TestSessionCheckpointCommandUsesControlledRuntimeRequest(t *testing.T) {
	repo := initSessionCommandRepo(t)
	session, err := infrastructure.CreateSessionBranch(infrastructure.GitSessionRequest{
		WorkspaceID: "checkpoint-cli", SourceDir: repo, Mode: "managed", BranchPrefix: "ai",
		SessionID: "checkpoint-cli-session", Checkpoint: "manual", Publish: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeSessionCommandWorktree(t, repo, session.Storage.Workspace)
	parent, err := infrastructure.GitRevParse(session.Storage.Workspace, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	postimage := []byte("checkpoint from cli\n")
	if err := os.WriteFile(filepath.Join(session.Storage.Workspace, "README.md"), postimage, 0o644); err != nil {
		t.Fatal(err)
	}
	request, err := infrastructure.FinalizeControlledCheckpointRequest(infrastructure.ControlledCheckpointRequest{
		ContractVersion: infrastructure.ControlledCheckpointRequestContract, RequestID: "cli-checkpoint-request",
		AuthorizationFingerprint: sessionCommandSHA256([]byte("approved")), SessionID: session.SessionID,
		WorkspaceID: session.WorkspaceID, ExpectedBranch: session.Repo.SessionRef, ExpectedParent: strings.TrimSpace(parent),
		CheckpointScope: infrastructure.ControlledCheckpointScope, Message: "checkpoint(runtime): CLI exact request",
		Paths: []string{"README.md"}, Postimages: []infrastructure.ControlledCheckpointPostimage{{Path: "README.md", SHA256: sessionCommandSHA256(postimage), Bytes: int64(len(postimage))}},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := t.TempDir()
	requestPath := filepath.Join(artifactRoot, "checkpoint-request.json")
	receiptPath := filepath.Join(artifactRoot, "checkpoint-receipt.json")
	if err := infrastructure.WriteControlledCheckpointRequest(requestPath, request); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return Run([]string{"checkpoint", session.SessionID, "--workdir", repo, "--request", requestPath, "--receipt", receiptPath, "--json"})
	})
	if err != nil {
		t.Fatalf("session checkpoint command: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("checkpoint JSON: %v\n%s", err, out)
	}
	receipt := payload["receipt"].(map[string]any)
	if receipt["parent"] != strings.TrimSpace(parent) || receipt["request_fingerprint"] != request.RequestFingerprint {
		t.Fatalf("checkpoint receipt = %+v", receipt)
	}
}

func TestSessionPublishCommandStrictRequestPublishesWithoutCheckpointOrUpstream(t *testing.T) {
	repo := initSessionCommandRepo(t)
	session, err := infrastructure.CreateSessionBranch(infrastructure.GitSessionRequest{
		WorkspaceID: "publication-cli", SourceDir: repo, Mode: "managed", BranchPrefix: "ai",
		SessionID: "publication-cli-session", Checkpoint: "manual", Publish: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeSessionCommandWorktree(t, repo, session.Storage.Workspace)
	parent, _ := infrastructure.GitRevParse(session.Storage.Workspace, "HEAD")
	postimage := []byte("published from cli\n")
	if err := os.WriteFile(filepath.Join(session.Storage.Workspace, "README.md"), postimage, 0o644); err != nil {
		t.Fatal(err)
	}
	checkpointRequest, err := infrastructure.FinalizeControlledCheckpointRequest(infrastructure.ControlledCheckpointRequest{
		ContractVersion: infrastructure.ControlledCheckpointRequestContract, RequestID: "cli-publication-checkpoint",
		AuthorizationFingerprint: sessionCommandSHA256([]byte("checkpoint approval")), SessionID: session.SessionID,
		WorkspaceID: session.WorkspaceID, ExpectedBranch: session.Repo.SessionRef, ExpectedParent: strings.TrimSpace(parent),
		CheckpointScope: infrastructure.ControlledCheckpointScope, Message: "checkpoint(runtime): CLI publication request",
		Paths: []string{"README.md"}, Postimages: []infrastructure.ControlledCheckpointPostimage{{Path: "README.md", SHA256: sessionCommandSHA256(postimage), Bytes: int64(len(postimage))}},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := t.TempDir()
	checkpointRequestPath := filepath.Join(artifactRoot, "checkpoint-request.json")
	checkpointReceiptPath := filepath.Join(artifactRoot, "checkpoint-receipt.json")
	if err := infrastructure.WriteControlledCheckpointRequest(checkpointRequestPath, checkpointRequest); err != nil {
		t.Fatal(err)
	}
	checkpointResult, err := infrastructure.CheckpointSessionFromRequest(session, checkpointRequestPath, checkpointReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(t.TempDir(), "reviewed.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", session.Storage.Workspace, "remote", "add", "reviewed", remote).CombinedOutput(); err != nil {
		t.Fatalf("add remote: %v\n%s", err, out)
	}
	remoteIdentity, err := infrastructure.ControlledPublicationRemoteIdentity(session.Storage.Workspace, "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	_, checkpointReceiptFP, err := infrastructure.ValidateControlledCheckpointReceiptForSession(session, checkpointRequestPath, checkpointReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	publicationRequest, err := infrastructure.FinalizeControlledPublicationRequest(infrastructure.ControlledPublicationRequest{
		ContractVersion: infrastructure.ControlledPublicationRequestContract, RequestID: "cli-publication-request",
		AuthorizationFingerprint:     sessionCommandSHA256([]byte("publication approval")),
		CheckpointRequestFingerprint: checkpointRequest.RequestFingerprint, CheckpointReceiptFingerprint: checkpointReceiptFP,
		SessionID: session.SessionID, WorkspaceID: session.WorkspaceID, ExpectedBranch: session.Repo.SessionRef,
		SourceCommit: checkpointResult.Receipt.Commit, SourceParent: checkpointResult.Receipt.Parent,
		RemoteName: "reviewed", RemoteIdentity: remoteIdentity, DestinationRef: "refs/heads/review/cli",
		PublicationScope: infrastructure.ControlledPublicationScope, Reason: "publish exact CLI checkpoint",
	})
	if err != nil {
		t.Fatal(err)
	}
	publicationRequestPath := filepath.Join(artifactRoot, "publication-request.json")
	publicationReceiptPath := filepath.Join(artifactRoot, "publication-receipt.json")
	if err := infrastructure.WriteControlledPublicationRequest(publicationRequestPath, publicationRequest); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return Run([]string{"publish", session.SessionID, "--workdir", repo,
			"--checkpoint-request", checkpointRequestPath, "--checkpoint-receipt", checkpointReceiptPath,
			"--request", publicationRequestPath, "--receipt", publicationReceiptPath, "--json"})
	})
	if err != nil {
		t.Fatalf("strict publish: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("publish JSON: %v\n%s", err, out)
	}
	receipt := payload["receipt"].(map[string]any)
	if receipt["source_commit"] != checkpointResult.Receipt.Commit || receipt["destination_ref"] != "refs/heads/review/cli" {
		t.Fatalf("publication receipt = %+v", receipt)
	}
	remoteCommit, err := exec.Command("git", "--git-dir", remote, "rev-parse", "refs/heads/review/cli").Output()
	if err != nil || strings.TrimSpace(string(remoteCommit)) != checkpointResult.Receipt.Commit {
		t.Fatalf("remote commit = %q, %v", remoteCommit, err)
	}
	if count, _ := exec.Command("git", "-C", session.Storage.Workspace, "rev-list", "--count", strings.TrimSpace(parent)+"..HEAD").Output(); strings.TrimSpace(string(count)) != "1" {
		t.Fatalf("strict publication created another checkpoint: %q", count)
	}
	if upstream, _ := exec.Command("git", "-C", session.Storage.Workspace, "config", "--get", "branch."+session.Repo.SessionRef+".remote").Output(); strings.TrimSpace(string(upstream)) != "" {
		t.Fatalf("strict publication configured upstream: %q", upstream)
	}
}

func TestRunUsageAndWorkerValidation(t *testing.T) {
	helpOut, err := captureStdout(t, func() error { return Run(nil) })
	if err != nil {
		t.Fatal(err)
	}
	if helpOut != strings.TrimSpace(sessionUsageText) {
		t.Fatalf("help output changed:\n%s", helpOut)
	}
	if err := Run([]string{"worker-acquire", "demo"}); err == nil || err.Error() != "--worker is required" {
		t.Fatalf("worker-acquire validation = %v", err)
	}
	if err := Run([]string{"worker-release", "demo"}); err == nil || err.Error() != "--worker is required" {
		t.Fatalf("worker-release validation = %v", err)
	}
	if err := Run([]string{"unknown"}); err == nil || err.Error() != "unknown session subcommand \"unknown\" (try: list, inspect, switch, checkpoint, publish, worker-acquire, or worker-release)" {
		t.Fatalf("unknown validation = %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	runErr := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String()), runErr
}

func sessionCommandSHA256(raw []byte) string {
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", digest)
}

func initSessionCommandRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "DockPipe Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-m", "init")
	return repo
}

func removeSessionCommandWorktree(t *testing.T, repo, workspace string) {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "worktree", "remove", "--force", workspace).CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree remove: %v\n%s", err, out)
	}
}
