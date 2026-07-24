package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	ControlledCheckpointRequestContract       = "dockpipe.session-checkpoint-request/v1"
	ControlledCheckpointReceiptContract       = "dockpipe.session-checkpoint-receipt/v1"
	ControlledCheckpointScope                 = "exact_paths_one_checkpoint"
	controlledCheckpointMaxPaths              = 256
	controlledCheckpointMaxBytes        int64 = 8 << 20
)

var (
	controlledCheckpointIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	controlledCheckpointSHA256     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	controlledCheckpointCommitHash = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type ControlledCheckpointPostimage struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ControlledCheckpointRequest struct {
	ContractVersion          string                          `json:"contract_version"`
	RequestID                string                          `json:"request_id"`
	RequestFingerprint       string                          `json:"request_fingerprint"`
	AuthorizationFingerprint string                          `json:"authorization_fingerprint"`
	SessionID                string                          `json:"session_id"`
	WorkspaceID              string                          `json:"workspace_id"`
	ExpectedBranch           string                          `json:"expected_branch"`
	ExpectedParent           string                          `json:"expected_parent"`
	CheckpointScope          string                          `json:"checkpoint_scope"`
	Message                  string                          `json:"message"`
	Paths                    []string                        `json:"paths"`
	Postimages               []ControlledCheckpointPostimage `json:"postimages"`
}

type ControlledCheckpointRuntime struct {
	Owner           string `json:"owner"`
	Operation       string `json:"operation"`
	ExactPaths      bool   `json:"exact_paths"`
	RawGitDelegated bool   `json:"raw_git_delegated"`
}

type ControlledCheckpointActions struct {
	Checkpoint  bool `json:"checkpoint"`
	Push        bool `json:"push"`
	Publication bool `json:"publication"`
	Sync        bool `json:"sync"`
	Merge       bool `json:"merge"`
}

type ControlledCheckpointReceipt struct {
	ContractVersion          string                          `json:"contract_version"`
	RequestID                string                          `json:"request_id"`
	RequestFingerprint       string                          `json:"request_fingerprint"`
	AuthorizationFingerprint string                          `json:"authorization_fingerprint"`
	CheckpointID             string                          `json:"checkpoint_id"`
	SessionID                string                          `json:"session_id"`
	WorkspaceID              string                          `json:"workspace_id"`
	Branch                   string                          `json:"branch"`
	Parent                   string                          `json:"parent"`
	Commit                   string                          `json:"commit"`
	Paths                    []string                        `json:"paths"`
	Postimages               []ControlledCheckpointPostimage `json:"postimages"`
	Status                   string                          `json:"status"`
	CreatedAt                string                          `json:"created_at"`
	Runtime                  ControlledCheckpointRuntime     `json:"runtime"`
	Actions                  ControlledCheckpointActions     `json:"actions"`
}

type ControlledCheckpointResult struct {
	Receipt    *ControlledCheckpointReceipt
	Idempotent bool
	Recovered  bool
}

type controlledCheckpointHooks struct {
	BeforeCommit  func() error
	Commit        func(string, string) error
	WriteMetadata func(*GitSession, *GitCheckpoint) error
	WriteReceipt  func(string, *ControlledCheckpointReceipt) error
}

func FinalizeControlledCheckpointRequest(request ControlledCheckpointRequest) (ControlledCheckpointRequest, error) {
	request.RequestFingerprint = ""
	fingerprint, err := controlledCheckpointRequestFingerprint(request)
	if err != nil {
		return ControlledCheckpointRequest{}, err
	}
	request.RequestFingerprint = fingerprint
	if err := ValidateControlledCheckpointRequest(&request); err != nil {
		return ControlledCheckpointRequest{}, err
	}
	return request, nil
}

func ValidateControlledCheckpointRequest(request *ControlledCheckpointRequest) error {
	if request == nil {
		return errors.New("checkpoint_request_invalid: request is nil")
	}
	if request.ContractVersion != ControlledCheckpointRequestContract ||
		!controlledCheckpointIDPattern.MatchString(request.RequestID) ||
		!controlledCheckpointIDPattern.MatchString(request.SessionID) ||
		!controlledCheckpointIDPattern.MatchString(request.WorkspaceID) {
		return errors.New("checkpoint_request_invalid: contract or bounded identity is invalid")
	}
	if !controlledCheckpointSHA256.MatchString(request.AuthorizationFingerprint) ||
		!controlledCheckpointSHA256.MatchString(request.RequestFingerprint) {
		return errors.New("checkpoint_request_invalid: request or authorization fingerprint is invalid")
	}
	if request.CheckpointScope != ControlledCheckpointScope {
		return fmt.Errorf("checkpoint_request_invalid: checkpoint scope must be %q", ControlledCheckpointScope)
	}
	if !controlledCheckpointCommitHash.MatchString(request.ExpectedParent) {
		return errors.New("checkpoint_request_invalid: expected parent must be a lowercase 40-character commit")
	}
	if err := validateControlledCheckpointBranch(request.ExpectedBranch); err != nil {
		return fmt.Errorf("checkpoint_request_invalid: %w", err)
	}
	if err := validateControlledCheckpointMessage(request.Message); err != nil {
		return fmt.Errorf("checkpoint_request_invalid: %w", err)
	}
	if len(request.Paths) == 0 || len(request.Paths) > controlledCheckpointMaxPaths || len(request.Paths) != len(request.Postimages) || !sort.StringsAreSorted(request.Paths) {
		return errors.New("checkpoint_request_invalid: paths and postimages must be non-empty, aligned, and sorted")
	}
	var aggregateBytes int64
	for index, path := range request.Paths {
		if err := validateControlledCheckpointPath(path); err != nil {
			return fmt.Errorf("checkpoint_request_invalid: %w", err)
		}
		if index > 0 && path == request.Paths[index-1] {
			return errors.New("checkpoint_request_invalid: duplicate path")
		}
		postimage := request.Postimages[index]
		if postimage.Path != path || postimage.Bytes < 0 || !controlledCheckpointSHA256.MatchString(postimage.SHA256) {
			return fmt.Errorf("checkpoint_request_invalid: postimage for %q is malformed or out of order", path)
		}
		if postimage.Bytes > controlledCheckpointMaxBytes-aggregateBytes {
			return fmt.Errorf("checkpoint_request_invalid: postimages exceed the %d-byte aggregate limit", controlledCheckpointMaxBytes)
		}
		aggregateBytes += postimage.Bytes
	}
	expected, err := controlledCheckpointRequestFingerprint(*request)
	if err != nil {
		return fmt.Errorf("checkpoint_request_invalid: %w", err)
	}
	if expected != request.RequestFingerprint {
		return errors.New("checkpoint_request_invalid: canonical request fingerprint does not match")
	}
	return nil
}

func WriteControlledCheckpointRequest(path string, request ControlledCheckpointRequest) error {
	if err := ValidateControlledCheckpointRequest(&request); err != nil {
		return err
	}
	return writeControlledCheckpointJSON(path, request, false)
}

func LoadControlledCheckpointRequest(path string) (*ControlledCheckpointRequest, error) {
	var request ControlledCheckpointRequest
	if err := readControlledCheckpointJSON(path, &request); err != nil {
		return nil, fmt.Errorf("checkpoint_request_invalid: %w", err)
	}
	if err := ValidateControlledCheckpointRequest(&request); err != nil {
		return nil, err
	}
	return &request, nil
}

// ValidateControlledCheckpointReceiptForSession revalidates the immutable request, runtime
// session, checkpoint metadata, commit, paths, postimages, and trailers without creating a commit.
func ValidateControlledCheckpointReceiptForSession(session *GitSession, requestPath, receiptPath string) (*ControlledCheckpointReceipt, string, error) {
	request, err := LoadControlledCheckpointRequest(requestPath)
	if err != nil {
		return nil, "", err
	}
	if session == nil || session.SessionID != request.SessionID || session.WorkspaceID != request.WorkspaceID || session.Repo.SessionRef != request.ExpectedBranch {
		return nil, "", errors.New("checkpoint_session_mismatch: request session, workspace, or branch does not match runtime metadata")
	}
	workspace, err := sessionGitTop(session)
	if err != nil || !sameControlledCheckpointPath(workspace, session.Storage.Workspace) {
		return nil, "", errors.New("checkpoint_session_mismatch: session workspace is not the Git work tree root")
	}
	receipt, exists, err := loadControlledCheckpointReceipt(receiptPath)
	if err != nil {
		return nil, "", err
	}
	if !exists {
		return nil, "", errors.New("checkpoint_receipt_invalid: receipt does not exist")
	}
	if err := validateControlledCheckpointReceipt(receipt, request, session, workspace); err != nil {
		return nil, "", err
	}
	metadataPath := filepath.Join(session.Storage.Metadata, "checkpoints", receipt.CheckpointID+".json")
	metadata, exists, err := loadControlledGitCheckpoint(metadataPath)
	if err != nil || !exists || !reflect.DeepEqual(metadata, controlledCheckpointMetadata(receipt, request.Message)) {
		return nil, "", errors.New("checkpoint_receipt_invalid: runtime checkpoint metadata is missing, tampered, or conflicting")
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return nil, "", fmt.Errorf("checkpoint_receipt_invalid: %w", err)
	}
	return receipt, controlledCheckpointBytesSHA256(raw), nil
}

func CheckpointSessionFromRequest(session *GitSession, requestPath, receiptPath string) (*ControlledCheckpointResult, error) {
	request, err := LoadControlledCheckpointRequest(requestPath)
	if err != nil {
		return nil, err
	}
	hooks := controlledCheckpointHooks{
		BeforeCommit:  func() error { return nil },
		Commit:        func(workspace, message string) error { return gitRun(workspace, "commit", "-m", message) },
		WriteMetadata: writeControlledCheckpointMetadata,
		WriteReceipt: func(path string, receipt *ControlledCheckpointReceipt) error {
			return writeControlledCheckpointJSON(path, receipt, false)
		},
	}
	return checkpointSessionControlled(session, request, receiptPath, hooks)
}

func checkpointSessionControlled(session *GitSession, request *ControlledCheckpointRequest, receiptPath string, hooks controlledCheckpointHooks) (*ControlledCheckpointResult, error) {
	if session == nil {
		return nil, errors.New("checkpoint_session_mismatch: session is nil")
	}
	if err := ValidateControlledCheckpointRequest(request); err != nil {
		return nil, err
	}
	if strings.TrimSpace(receiptPath) == "" {
		return nil, errors.New("checkpoint_receipt_invalid: receipt path is empty")
	}
	if session.SessionID != request.SessionID || session.WorkspaceID != request.WorkspaceID || session.Repo.SessionRef != request.ExpectedBranch {
		return nil, errors.New("checkpoint_session_mismatch: request session, workspace, or branch does not match runtime metadata")
	}
	workspace, err := sessionGitTop(session)
	if err != nil {
		return nil, fmt.Errorf("checkpoint_session_mismatch: %w", err)
	}
	if !sameControlledCheckpointPath(workspace, session.Storage.Workspace) {
		return nil, errors.New("checkpoint_session_mismatch: session workspace is not the Git work tree root")
	}
	branch, err := gitOutputTrimmed(workspace, "branch", "--show-current")
	if err != nil || branch != request.ExpectedBranch {
		return nil, fmt.Errorf("checkpoint_branch_mismatch: current branch %q does not match %q", branch, request.ExpectedBranch)
	}

	if existing, exists, loadErr := loadControlledCheckpointReceipt(receiptPath); loadErr != nil {
		return nil, loadErr
	} else if exists {
		if err := validateControlledCheckpointReceipt(existing, request, session, workspace); err != nil {
			return nil, err
		}
		return &ControlledCheckpointResult{Receipt: existing, Idempotent: true}, nil
	}

	head, err := GitRevParse(workspace, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("checkpoint_parent_mismatch: %w", err)
	}
	head = strings.TrimSpace(head)
	if head != request.ExpectedParent {
		receipt, recoveryErr := controlledCheckpointReceiptForCommit(session, request, workspace, head)
		if recoveryErr != nil {
			return nil, fmt.Errorf("checkpoint_recovery_ambiguous: HEAD %s is not the expected parent and is not the exact requested checkpoint: %w", head, recoveryErr)
		}
		if err := hooks.WriteMetadata(session, controlledCheckpointMetadata(receipt, request.Message)); err != nil {
			return nil, fmt.Errorf("checkpoint_metadata_failed: exact checkpoint was recovered but metadata could not be written: %w", err)
		}
		if err := hooks.WriteReceipt(receiptPath, receipt); err != nil {
			return nil, fmt.Errorf("checkpoint_receipt_failed: exact checkpoint was recovered but receipt could not be written: %w", err)
		}
		return &ControlledCheckpointResult{Receipt: receipt, Recovered: true}, nil
	}

	if err := preflightControlledCheckpointWorkspace(workspace, request); err != nil {
		return nil, err
	}
	args := append([]string{"add", "--"}, request.Paths...)
	if err := gitRun(workspace, args...); err != nil {
		_ = unstageControlledCheckpointPaths(workspace, request.Paths)
		return nil, fmt.Errorf("checkpoint_stage_failed: %w", err)
	}
	rollbackStage := true
	defer func() {
		if rollbackStage {
			_ = unstageControlledCheckpointPaths(workspace, request.Paths)
		}
	}()
	if err := verifyControlledCheckpointStagedState(workspace, request); err != nil {
		return nil, err
	}
	if hooks.BeforeCommit != nil {
		if err := hooks.BeforeCommit(); err != nil {
			return nil, fmt.Errorf("checkpoint_precommit_failed: %w", err)
		}
	}
	checkpointID := controlledCheckpointID(request.RequestFingerprint)
	message := controlledCheckpointCommitMessage(request, checkpointID)
	if err := hooks.Commit(workspace, message); err != nil {
		return nil, fmt.Errorf("checkpoint_commit_failed: %w", err)
	}
	rollbackStage = false
	commit, err := GitRevParse(workspace, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("checkpoint_commit_verification_failed: %w", err)
	}
	receipt, err := controlledCheckpointReceiptForCommit(session, request, workspace, strings.TrimSpace(commit))
	if err != nil {
		return nil, fmt.Errorf("checkpoint_commit_verification_failed: %w", err)
	}
	if err := hooks.WriteMetadata(session, controlledCheckpointMetadata(receipt, request.Message)); err != nil {
		return nil, fmt.Errorf("checkpoint_metadata_failed: commit %s was created but metadata could not be written: %w", receipt.Commit, err)
	}
	if err := hooks.WriteReceipt(receiptPath, receipt); err != nil {
		return nil, fmt.Errorf("checkpoint_receipt_failed: commit %s was created but receipt could not be written: %w", receipt.Commit, err)
	}
	return &ControlledCheckpointResult{Receipt: receipt}, nil
}

func preflightControlledCheckpointWorkspace(workspace string, request *ControlledCheckpointRequest) error {
	if out, err := gitCombined(workspace, "diff", "--cached", "--quiet", "--"); err != nil {
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("checkpoint_staged_changes_present: %s", strings.TrimSpace(out))
		}
		return errors.New("checkpoint_staged_changes_present: the index already contains changes")
	}
	status, err := controlledCheckpointStatus(workspace)
	if err != nil {
		return err
	}
	if len(status) != len(request.Paths) {
		return errors.New("checkpoint_change_set_mismatch: workspace changes are not exactly the requested paths")
	}
	for _, path := range request.Paths {
		if status[path] != " M" {
			return fmt.Errorf("checkpoint_change_set_mismatch: %q must be one unstaged tracked modification and no other change kind", path)
		}
	}
	return verifyControlledCheckpointPostimages(workspace, request, false)
}

func verifyControlledCheckpointStagedState(workspace string, request *ControlledCheckpointRequest) error {
	status, err := controlledCheckpointStatus(workspace)
	if err != nil {
		return err
	}
	if len(status) != len(request.Paths) {
		return errors.New("checkpoint_stage_verification_failed: staged change set is not exactly the request")
	}
	for _, path := range request.Paths {
		if status[path] != "M " {
			return fmt.Errorf("checkpoint_stage_verification_failed: %q is not exactly staged with a clean worktree", path)
		}
	}
	return verifyControlledCheckpointPostimages(workspace, request, true)
}

func verifyControlledCheckpointPostimages(workspace string, request *ControlledCheckpointRequest, staged bool) error {
	for _, expected := range request.Postimages {
		var raw []byte
		var err error
		if staged {
			raw, err = gitOutputBytes(workspace, "show", ":"+expected.Path)
		} else {
			raw, err = readControlledCheckpointWorkspaceFile(workspace, expected.Path)
		}
		if err != nil {
			return fmt.Errorf("checkpoint_postimage_mismatch: %q cannot be read: %w", expected.Path, err)
		}
		if int64(len(raw)) != expected.Bytes || controlledCheckpointBytesSHA256(raw) != expected.SHA256 {
			return fmt.Errorf("checkpoint_postimage_mismatch: %q does not match the approved postimage", expected.Path)
		}
	}
	return nil
}

func controlledCheckpointReceiptForCommit(session *GitSession, request *ControlledCheckpointRequest, workspace, commit string) (*ControlledCheckpointReceipt, error) {
	if !controlledCheckpointCommitHash.MatchString(commit) || commit == request.ExpectedParent {
		return nil, errors.New("resulting commit is missing or still equals the expected parent")
	}
	parent, err := gitOutputTrimmed(workspace, "rev-parse", commit+"^")
	if err != nil || parent != request.ExpectedParent {
		return nil, fmt.Errorf("commit parent %q does not match expected parent %q", parent, request.ExpectedParent)
	}
	branch, err := gitOutputTrimmed(workspace, "branch", "--show-current")
	if err != nil || branch != request.ExpectedBranch {
		return nil, fmt.Errorf("commit branch %q does not match expected branch %q", branch, request.ExpectedBranch)
	}
	changed, err := controlledCheckpointCommitPaths(workspace, commit)
	if err != nil || !equalControlledCheckpointStrings(changed, request.Paths) {
		return nil, errors.New("commit changed paths do not exactly match the request")
	}
	for _, expected := range request.Postimages {
		raw, err := gitOutputBytes(workspace, "show", commit+":"+expected.Path)
		if err != nil || int64(len(raw)) != expected.Bytes || controlledCheckpointBytesSHA256(raw) != expected.SHA256 {
			return nil, fmt.Errorf("commit postimage for %q does not match the request", expected.Path)
		}
	}
	message, err := gitOutputTrimmed(workspace, "log", "-1", "--format=%B", commit)
	if err != nil || !strings.Contains(message, "DockPipe-Request: "+request.RequestFingerprint) ||
		!strings.Contains(message, "DockPipe-Authorization: "+request.AuthorizationFingerprint) ||
		!strings.Contains(message, "DockPipe-Session: "+request.SessionID) {
		return nil, errors.New("commit trailers do not bind the exact request, authorization, and session")
	}
	status, err := controlledCheckpointStatus(workspace)
	if err != nil || len(status) != 0 {
		return nil, errors.New("workspace is not clean after the requested checkpoint")
	}
	createdAt, err := gitOutputTrimmed(workspace, "show", "-s", "--format=%cI", commit)
	if err != nil || createdAt == "" {
		return nil, errors.New("commit timestamp cannot be read")
	}
	return &ControlledCheckpointReceipt{
		ContractVersion: ControlledCheckpointReceiptContract,
		RequestID:       request.RequestID, RequestFingerprint: request.RequestFingerprint,
		AuthorizationFingerprint: request.AuthorizationFingerprint,
		CheckpointID:             controlledCheckpointID(request.RequestFingerprint), SessionID: request.SessionID,
		WorkspaceID: request.WorkspaceID, Branch: request.ExpectedBranch, Parent: request.ExpectedParent,
		Commit: commit, Paths: append([]string{}, request.Paths...), Postimages: append([]ControlledCheckpointPostimage{}, request.Postimages...),
		Status: "created", CreatedAt: createdAt,
		Runtime: ControlledCheckpointRuntime{Owner: "dockpipe", Operation: "session.checkpoint", ExactPaths: true, RawGitDelegated: false},
		Actions: ControlledCheckpointActions{Checkpoint: true, Push: false, Publication: false, Sync: false, Merge: false},
	}, nil
}

func validateControlledCheckpointReceipt(receipt *ControlledCheckpointReceipt, request *ControlledCheckpointRequest, session *GitSession, workspace string) error {
	if receipt == nil || receipt.ContractVersion != ControlledCheckpointReceiptContract || receipt.RequestID != request.RequestID ||
		receipt.RequestFingerprint != request.RequestFingerprint || receipt.AuthorizationFingerprint != request.AuthorizationFingerprint ||
		receipt.CheckpointID != controlledCheckpointID(request.RequestFingerprint) || receipt.SessionID != session.SessionID ||
		receipt.WorkspaceID != session.WorkspaceID || receipt.Branch != request.ExpectedBranch || receipt.Parent != request.ExpectedParent ||
		receipt.Status != "created" || !equalControlledCheckpointStrings(receipt.Paths, request.Paths) ||
		!reflect.DeepEqual(receipt.Postimages, request.Postimages) || receipt.Runtime.Owner != "dockpipe" ||
		receipt.Runtime.Operation != "session.checkpoint" || !receipt.Runtime.ExactPaths || receipt.Runtime.RawGitDelegated ||
		!receipt.Actions.Checkpoint || receipt.Actions.Push || receipt.Actions.Publication || receipt.Actions.Sync || receipt.Actions.Merge {
		return errors.New("checkpoint_receipt_invalid: existing receipt is malformed, tampered, or does not match the request")
	}
	expected, err := controlledCheckpointReceiptForCommit(session, request, workspace, receipt.Commit)
	if err != nil || !reflect.DeepEqual(receipt, expected) {
		return errors.New("checkpoint_receipt_invalid: existing receipt does not match the exact runtime commit")
	}
	return nil
}

func controlledCheckpointMetadata(receipt *ControlledCheckpointReceipt, reason string) *GitCheckpoint {
	return &GitCheckpoint{
		Schema: 1, CheckpointID: receipt.CheckpointID, SessionID: receipt.SessionID,
		WorkspaceID: receipt.WorkspaceID, Branch: receipt.Branch, Parent: receipt.Parent,
		Commit: receipt.Commit, Reason: reason, DirtyBefore: true, Status: receipt.Status,
		CreatedAt: receipt.CreatedAt, Paths: append([]string{}, receipt.Paths...),
		RequestFingerprint: receipt.RequestFingerprint,
	}
}

func writeControlledCheckpointMetadata(session *GitSession, checkpoint *GitCheckpoint) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "checkpoints", checkpoint.CheckpointID+".json")
	if existing, exists, loadErr := loadControlledGitCheckpoint(path); loadErr != nil {
		return loadErr
	} else if exists {
		if !reflect.DeepEqual(existing, checkpoint) {
			return errors.New("existing checkpoint metadata is tampered or conflicts with the request")
		}
		return nil
	}
	return writeControlledCheckpointJSON(path, checkpoint, false)
}

func loadControlledGitCheckpoint(path string) (*GitCheckpoint, bool, error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	var checkpoint GitCheckpoint
	if err := readControlledCheckpointJSON(path, &checkpoint); err != nil {
		return nil, false, err
	}
	return &checkpoint, true, nil
}

func loadControlledCheckpointReceipt(path string) (*ControlledCheckpointReceipt, bool, error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("checkpoint_receipt_invalid: %w", err)
	}
	var receipt ControlledCheckpointReceipt
	if err := readControlledCheckpointJSON(path, &receipt); err != nil {
		return nil, false, fmt.Errorf("checkpoint_receipt_invalid: %w", err)
	}
	return &receipt, true, nil
}

func controlledCheckpointRequestFingerprint(request ControlledCheckpointRequest) (string, error) {
	request.RequestFingerprint = ""
	raw, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	return controlledCheckpointBytesSHA256(raw), nil
}

func controlledCheckpointBytesSHA256(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func controlledCheckpointID(fingerprint string) string {
	value := strings.TrimPrefix(fingerprint, "sha256:")
	if len(value) > 16 {
		value = value[:16]
	}
	return "cp-request-" + value
}

func controlledCheckpointCommitMessage(request *ControlledCheckpointRequest, checkpointID string) string {
	return fmt.Sprintf("%s\n\nDockPipe-Session: %s\nDockPipe-Checkpoint: %s\nDockPipe-Request: %s\nDockPipe-Authorization: %s\nDockPipe-Reason: approved-exact-request\n",
		request.Message, request.SessionID, checkpointID, request.RequestFingerprint, request.AuthorizationFingerprint)
}

func controlledCheckpointStatus(workspace string) (map[string]string, error) {
	raw, err := gitOutputBytes(workspace, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, fmt.Errorf("checkpoint_status_failed: %w", err)
	}
	result := map[string]string{}
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' || entry[0] == 'R' || entry[1] == 'R' || entry[0] == 'C' || entry[1] == 'C' {
			return nil, errors.New("checkpoint_change_set_mismatch: unsupported or ambiguous Git status entry")
		}
		path := string(entry[3:])
		if err := validateControlledCheckpointPath(path); err != nil {
			return nil, fmt.Errorf("checkpoint_change_set_mismatch: %w", err)
		}
		if _, exists := result[path]; exists {
			return nil, errors.New("checkpoint_change_set_mismatch: duplicate Git status path")
		}
		result[path] = string(entry[:2])
	}
	return result, nil
}

func controlledCheckpointCommitPaths(workspace, commit string) ([]string, error) {
	raw, err := gitOutputBytes(workspace, "diff-tree", "--no-commit-id", "--name-status", "-r", "-z", commit)
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(raw, []byte{0})
	paths := []string{}
	for index := 0; index < len(parts); {
		if len(parts[index]) == 0 {
			index++
			continue
		}
		status := string(parts[index])
		index++
		if status != "M" || index >= len(parts) || len(parts[index]) == 0 {
			return nil, errors.New("commit contains an unsupported change kind")
		}
		path := string(parts[index])
		index++
		if err := validateControlledCheckpointPath(path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func readControlledCheckpointWorkspaceFile(workspace, rel string) ([]byte, error) {
	if err := validateControlledCheckpointPath(rel); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || controlledCheckpointFileInfoIsLinkOrReparse(rootInfo) || !rootInfo.IsDir() {
		return nil, errors.New("workspace root is missing, linked, reparsed, or not a directory")
	}
	candidate := filepath.Join(root, filepath.FromSlash(rel))
	current := root
	parts := strings.Split(rel, "/")
	var inspected os.FileInfo
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, inspectErr := os.Lstat(current)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if controlledCheckpointFileInfoIsLinkOrReparse(info) {
			return nil, errors.New("path contains a filesystem link or reparse point")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, errors.New("path has a non-directory ancestor")
		}
		if index == len(parts)-1 {
			inspected = info
		}
	}
	if inspected == nil || !inspected.Mode().IsRegular() {
		return nil, errors.New("path is not a regular file")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil || !sameOrWithinControlledCheckpointPath(resolvedRoot, resolvedCandidate) {
		return nil, errors.New("path escapes the workspace")
	}
	file, err := os.Open(candidate)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || controlledCheckpointFileInfoIsLinkOrReparse(opened) || !opened.Mode().IsRegular() || !os.SameFile(inspected, opened) {
		return nil, errors.New("path changed while being opened")
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	final, err := file.Stat()
	if err != nil || !os.SameFile(opened, final) || int64(len(raw)) != final.Size() {
		return nil, errors.New("path changed while being read")
	}
	return raw, nil
}

func validateControlledCheckpointPath(value string) error {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, "\\") || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return fmt.Errorf("path %q must be a canonical repository-relative path", value)
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.EqualFold(part, ".git") {
			return fmt.Errorf("path %q is empty, escaping, ambiguous, or Git-internal", value)
		}
		for _, char := range part {
			if char < 0x20 || char == 0x7f {
				return fmt.Errorf("path %q contains a control character", value)
			}
		}
	}
	if filepath.ToSlash(filepath.Clean(filepath.FromSlash(value))) != value {
		return fmt.Errorf("path %q is not canonical", value)
	}
	return nil
}

func validateControlledCheckpointBranch(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 255 || strings.ContainsAny(value, " ~^:?*[\\") || strings.Contains(value, "..") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return errors.New("expected branch is empty or malformed")
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return errors.New("expected branch contains a control character")
		}
	}
	return nil
}

func validateControlledCheckpointMessage(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 200 || strings.ContainsAny(value, "\r\n") {
		return errors.New("checkpoint message must be one trimmed line of at most 200 bytes")
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return errors.New("checkpoint message contains a control character")
		}
	}
	return nil
}

func controlledCheckpointFileInfoIsLinkOrReparse(info os.FileInfo) bool {
	if info == nil || info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	attributes := value.FieldByName("FileAttributes")
	return attributes.IsValid() && attributes.CanUint() && attributes.Uint()&0x400 != 0
}

func readControlledCheckpointJSON(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if controlledCheckpointFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
		return errors.New("file must be a regular non-link path")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("file contains multiple JSON values")
		}
		return err
	}
	return nil
}

func writeControlledCheckpointJSON(path string, value any, replace bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !replace {
		if _, err := os.Lstat(path); err == nil {
			return errors.New("target already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".dockpipe-checkpoint-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		_ = temporary.Close()
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(append(raw, '\n')); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if !replace {
		if _, err := os.Lstat(path); err == nil {
			return errors.New("target already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func gitOutputBytes(workdir string, args ...string) ([]byte, error) {
	dir, err := gitDir(workdir)
	if err != nil {
		return nil, err
	}
	command := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", command...).Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func gitOutputTrimmed(workdir string, args ...string) (string, error) {
	raw, err := gitOutputBytes(workdir, args...)
	return strings.TrimSpace(string(raw)), err
}

func unstageControlledCheckpointPaths(workspace string, paths []string) error {
	args := append([]string{"reset", "--mixed", "HEAD", "--"}, paths...)
	return gitRun(workspace, args...)
}

func sameControlledCheckpointPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func sameOrWithinControlledCheckpointPath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func equalControlledCheckpointStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
