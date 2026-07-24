package application

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
		return cmdSession([]string{"list", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listOut, "cmd-session") || !strings.Contains(listOut, session.Repo.SessionRef) {
		t.Fatalf("list output missing session: %q", listOut)
	}

	inspectOut, err := captureStdout(t, func() error {
		return cmdSession([]string{"inspect", "cmd", "--workdir", repo})
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
		return cmdSession([]string{"inspect", "cmd-session", "--workdir", repo, "--json"})
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
		return cmdSession([]string{"switch", "cmd-session", "--workdir", repo})
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
		return cmdSession([]string{"checkpoint", session.SessionID, "--workdir", repo, "--request", requestPath, "--receipt", receiptPath, "--json"})
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
