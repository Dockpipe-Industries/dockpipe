package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const (
	ControlledPublicationRequestContract = "dockpipe.session-publication-request/v1"
	ControlledPublicationReceiptContract = "dockpipe.session-publication-receipt/v1"
	ControlledPublicationScope           = "exact_checkpoint_commit_to_exact_branch_ref_once"
)

var controlledPublicationRemoteName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type ControlledPublicationRequest struct {
	ContractVersion              string `json:"contract_version"`
	RequestID                    string `json:"request_id"`
	RequestFingerprint           string `json:"request_fingerprint"`
	AuthorizationFingerprint     string `json:"authorization_fingerprint"`
	CheckpointRequestFingerprint string `json:"checkpoint_request_fingerprint"`
	CheckpointReceiptFingerprint string `json:"checkpoint_receipt_fingerprint"`
	SessionID                    string `json:"session_id"`
	WorkspaceID                  string `json:"workspace_id"`
	ExpectedBranch               string `json:"expected_branch"`
	SourceCommit                 string `json:"source_commit"`
	SourceParent                 string `json:"source_parent"`
	RemoteName                   string `json:"remote_name"`
	RemoteIdentity               string `json:"remote_identity"`
	DestinationRef               string `json:"destination_ref"`
	PublicationScope             string `json:"publication_scope"`
	Reason                       string `json:"reason"`
}

type ControlledPublicationRuntime struct {
	Owner           string `json:"owner"`
	Operation       string `json:"operation"`
	ExactCommit     bool   `json:"exact_commit"`
	ExactRef        bool   `json:"exact_ref"`
	RawGitDelegated bool   `json:"raw_git_delegated"`
}

type ControlledPublicationPush struct {
	RefspecKind                 string `json:"refspec_kind"`
	Force                       bool   `json:"force"`
	UpstreamConfigured          bool   `json:"upstream_configured"`
	CredentialMaterialPersisted bool   `json:"credential_material_persisted"`
}

type ControlledPublicationActions struct {
	Publication bool `json:"publication"`
	Checkpoint  bool `json:"checkpoint"`
	Sync        bool `json:"sync"`
	Fetch       bool `json:"fetch"`
	Merge       bool `json:"merge"`
	Force       bool `json:"force"`
}

type ControlledPublicationReceipt struct {
	ContractVersion              string                       `json:"contract_version"`
	RequestID                    string                       `json:"request_id"`
	RequestFingerprint           string                       `json:"request_fingerprint"`
	AuthorizationFingerprint     string                       `json:"authorization_fingerprint"`
	CheckpointRequestFingerprint string                       `json:"checkpoint_request_fingerprint"`
	CheckpointReceiptFingerprint string                       `json:"checkpoint_receipt_fingerprint"`
	CheckpointID                 string                       `json:"checkpoint_id"`
	SessionID                    string                       `json:"session_id"`
	WorkspaceID                  string                       `json:"workspace_id"`
	Branch                       string                       `json:"branch"`
	SourceCommit                 string                       `json:"source_commit"`
	SourceParent                 string                       `json:"source_parent"`
	RemoteName                   string                       `json:"remote_name"`
	RemoteIdentity               string                       `json:"remote_identity"`
	DestinationRef               string                       `json:"destination_ref"`
	PublicationScope             string                       `json:"publication_scope"`
	Status                       string                       `json:"status"`
	CreatedAt                    string                       `json:"created_at"`
	Recovered                    bool                         `json:"recovered"`
	Runtime                      ControlledPublicationRuntime `json:"runtime"`
	Push                         ControlledPublicationPush    `json:"push"`
	Actions                      ControlledPublicationActions `json:"actions"`
}

type ControlledPublicationResult struct {
	Receipt    *ControlledPublicationReceipt
	Idempotent bool
	Recovered  bool
}

type controlledPublicationHooks struct {
	BeforePush    func() error
	Push          func(string, string, string) error
	Observe       func(string, string, string) (string, error)
	WriteMetadata func(*GitSession, *ControlledPublicationReceipt) error
	WriteReceipt  func(string, *ControlledPublicationReceipt) error
}

func FinalizeControlledPublicationRequest(request ControlledPublicationRequest) (ControlledPublicationRequest, error) {
	request.RequestFingerprint = ""
	fingerprint, err := controlledPublicationRequestFingerprint(request)
	if err != nil {
		return ControlledPublicationRequest{}, err
	}
	request.RequestFingerprint = fingerprint
	if err := ValidateControlledPublicationRequest(&request); err != nil {
		return ControlledPublicationRequest{}, err
	}
	return request, nil
}

func ValidateControlledPublicationRequest(request *ControlledPublicationRequest) error {
	if request == nil || request.ContractVersion != ControlledPublicationRequestContract ||
		!controlledCheckpointIDPattern.MatchString(request.RequestID) || !controlledCheckpointIDPattern.MatchString(request.SessionID) ||
		!controlledCheckpointIDPattern.MatchString(request.WorkspaceID) {
		return errors.New("publication_request_invalid: contract or bounded identity is invalid")
	}
	for _, value := range []string{request.RequestFingerprint, request.AuthorizationFingerprint, request.CheckpointRequestFingerprint, request.CheckpointReceiptFingerprint, request.RemoteIdentity} {
		if !controlledCheckpointSHA256.MatchString(value) {
			return errors.New("publication_request_invalid: one or more fingerprints are invalid")
		}
	}
	if request.PublicationScope != ControlledPublicationScope || !controlledCheckpointCommitHash.MatchString(request.SourceCommit) ||
		!controlledCheckpointCommitHash.MatchString(request.SourceParent) || request.SourceCommit == request.SourceParent {
		return errors.New("publication_request_invalid: publication scope or source commit binding is invalid")
	}
	if err := validateControlledCheckpointBranch(request.ExpectedBranch); err != nil {
		return fmt.Errorf("publication_request_invalid: %w", err)
	}
	if !controlledPublicationRemoteName.MatchString(request.RemoteName) || strings.HasPrefix(request.RemoteName, "-") {
		return errors.New("publication_request_invalid: remote name is malformed or option-like")
	}
	if err := validateControlledPublicationDestination(request.DestinationRef); err != nil {
		return fmt.Errorf("publication_request_invalid: %w", err)
	}
	if request.Reason == "" || request.Reason != strings.TrimSpace(request.Reason) || len(request.Reason) > 200 || strings.ContainsAny(request.Reason, "\r\n") {
		return errors.New("publication_request_invalid: reason must be one trimmed line of at most 200 bytes")
	}
	expected, err := controlledPublicationRequestFingerprint(*request)
	if err != nil || expected != request.RequestFingerprint {
		return errors.New("publication_request_invalid: canonical request fingerprint does not match")
	}
	return nil
}

func WriteControlledPublicationRequest(path string, request ControlledPublicationRequest) error {
	if err := ValidateControlledPublicationRequest(&request); err != nil {
		return err
	}
	return writeControlledCheckpointJSON(path, request, false)
}

func LoadControlledPublicationRequest(path string) (*ControlledPublicationRequest, error) {
	var request ControlledPublicationRequest
	if err := readControlledCheckpointJSON(path, &request); err != nil {
		return nil, fmt.Errorf("publication_request_invalid: %w", err)
	}
	if err := ValidateControlledPublicationRequest(&request); err != nil {
		return nil, err
	}
	return &request, nil
}

// ControlledPublicationRemoteIdentity hashes the effective push destination. The URL itself,
// including any credential material, is never returned or persisted in an artifact.
func ControlledPublicationRemoteIdentity(workspace, remote string) (string, error) {
	if !controlledPublicationRemoteName.MatchString(remote) || strings.HasPrefix(remote, "-") {
		return "", errors.New("publication_remote_invalid: remote name is malformed or option-like")
	}
	url, err := gitOutputTrimmed(workspace, "remote", "get-url", "--push", remote)
	if err != nil || url == "" || strings.ContainsAny(url, "\r\n") {
		return "", errors.New("publication_remote_invalid: configured remote push destination cannot be resolved unambiguously")
	}
	return controlledCheckpointBytesSHA256([]byte(url)), nil
}

func PublishSessionFromRequest(session *GitSession, checkpointRequestPath, checkpointReceiptPath, requestPath, receiptPath string) (*ControlledPublicationResult, error) {
	request, err := LoadControlledPublicationRequest(requestPath)
	if err != nil {
		return nil, err
	}
	hooks := controlledPublicationHooks{
		BeforePush: func() error { return nil },
		Push: func(workspace, remote, refspec string) error {
			_, err := gitCombined(workspace, "push", "--porcelain", "--", remote, refspec)
			return err
		},
		Observe:       observeControlledPublicationRef,
		WriteMetadata: writeControlledPublicationMetadata,
		WriteReceipt: func(path string, receipt *ControlledPublicationReceipt) error {
			return writeControlledCheckpointJSON(path, receipt, false)
		},
	}
	return publishSessionControlled(session, checkpointRequestPath, checkpointReceiptPath, request, receiptPath, hooks)
}

func publishSessionControlled(session *GitSession, checkpointRequestPath, checkpointReceiptPath string, request *ControlledPublicationRequest, receiptPath string, hooks controlledPublicationHooks) (*ControlledPublicationResult, error) {
	if err := ValidateControlledPublicationRequest(request); err != nil {
		return nil, err
	}
	if session == nil || session.SessionID != request.SessionID || session.WorkspaceID != request.WorkspaceID || session.Repo.SessionRef != request.ExpectedBranch {
		return nil, errors.New("publication_session_mismatch: request session, workspace, or branch does not match runtime metadata")
	}
	workspace, err := sessionGitTop(session)
	if err != nil || !sameControlledCheckpointPath(workspace, session.Storage.Workspace) {
		return nil, errors.New("publication_session_mismatch: session workspace is not the Git work tree root")
	}
	checkpointRequest, err := LoadControlledCheckpointRequest(checkpointRequestPath)
	if err != nil {
		return nil, err
	}
	checkpointReceipt, checkpointReceiptFP, err := ValidateControlledCheckpointReceiptForSession(session, checkpointRequestPath, checkpointReceiptPath)
	if err != nil {
		return nil, err
	}
	if request.CheckpointRequestFingerprint != checkpointRequest.RequestFingerprint || request.CheckpointReceiptFingerprint != checkpointReceiptFP ||
		request.SourceCommit != checkpointReceipt.Commit || request.SourceParent != checkpointReceipt.Parent {
		return nil, errors.New("publication_checkpoint_mismatch: request does not bind the exact validated checkpoint")
	}
	if strings.TrimSpace(receiptPath) == "" {
		return nil, errors.New("publication_receipt_invalid: receipt path is empty")
	}
	if err := preflightControlledPublicationWorkspace(workspace, request); err != nil {
		return nil, err
	}
	remoteIdentity, err := ControlledPublicationRemoteIdentity(workspace, request.RemoteName)
	if err != nil || remoteIdentity != request.RemoteIdentity {
		return nil, errors.New("publication_remote_mismatch: configured effective push destination does not match the approved identity")
	}

	metadata, metadataExists, err := loadControlledPublicationMetadata(session, request)
	if err != nil {
		return nil, err
	}
	existing, receiptExists, err := loadControlledPublicationReceipt(receiptPath)
	if err != nil {
		return nil, err
	}
	if receiptExists {
		if err := validateControlledPublicationReceipt(existing, request, checkpointReceipt); err != nil {
			return nil, err
		}
		if metadataExists && !reflect.DeepEqual(metadata, existing) {
			return nil, errors.New("publication_metadata_invalid: runtime metadata conflicts with the receipt")
		}
		if !metadataExists {
			if err := hooks.WriteMetadata(session, existing); err != nil {
				return nil, fmt.Errorf("publication_metadata_failed: valid receipt exists but runtime metadata could not be restored: %w", err)
			}
		}
		return &ControlledPublicationResult{Receipt: existing, Idempotent: true, Recovered: existing.Recovered}, nil
	}
	if metadataExists {
		if err := validateControlledPublicationReceipt(metadata, request, checkpointReceipt); err != nil {
			return nil, fmt.Errorf("publication_metadata_invalid: %w", err)
		}
	}

	observed, observeErr := hooks.Observe(workspace, request.RemoteName, request.DestinationRef)
	recovered := observeErr == nil && observed == request.SourceCommit
	if !recovered {
		if hooks.BeforePush != nil {
			if err := hooks.BeforePush(); err != nil {
				return nil, fmt.Errorf("publication_prepush_failed: %w", err)
			}
		}
		refspec := request.SourceCommit + ":" + request.DestinationRef
		if err := hooks.Push(workspace, request.RemoteName, refspec); err != nil {
			_ = appendGitSessionEvent(session, map[string]string{"type": "session.publication.failed", "actor": "runtime", "session_id": session.SessionID, "remote": request.RemoteName, "destination_ref": request.DestinationRef}, "")
			return nil, fmt.Errorf("publication_push_failed: exact non-force push failed")
		}
		observed, observeErr = hooks.Observe(workspace, request.RemoteName, request.DestinationRef)
		if observeErr != nil || observed != request.SourceCommit {
			return nil, errors.New("publication_postpush_unverified: push returned but the exact destination ref could not be proven")
		}
	}

	receipt := metadata
	if receipt == nil {
		receipt = controlledPublicationReceipt(request, checkpointReceipt, recovered, time.Now().UTC().Format(time.RFC3339))
	}
	if !metadataExists {
		if err := hooks.WriteMetadata(session, receipt); err != nil {
			return nil, fmt.Errorf("publication_metadata_failed: remote may already equal the exact approved commit: %w", err)
		}
	}
	if err := hooks.WriteReceipt(receiptPath, receipt); err != nil {
		return nil, fmt.Errorf("publication_receipt_failed: remote equals the exact approved commit but the receipt could not be written: %w", err)
	}
	if err := appendGitSessionEvent(session, map[string]string{"type": "session.publication.completed", "actor": "runtime", "session_id": session.SessionID, "commit": request.SourceCommit, "remote": request.RemoteName, "destination_ref": request.DestinationRef, "recovered": fmt.Sprintf("%t", recovered)}, ""); err != nil {
		return nil, fmt.Errorf("publication_metadata_failed: publication receipt exists but the runtime event could not be recorded: %w", err)
	}
	return &ControlledPublicationResult{Receipt: receipt, Recovered: recovered}, nil
}

func preflightControlledPublicationWorkspace(workspace string, request *ControlledPublicationRequest) error {
	branch, err := gitOutputTrimmed(workspace, "branch", "--show-current")
	if err != nil || branch != request.ExpectedBranch {
		return errors.New("publication_branch_mismatch: current branch is detached or does not match the approved session branch")
	}
	head, err := GitRevParse(workspace, "HEAD")
	if err != nil || strings.TrimSpace(head) != request.SourceCommit {
		return errors.New("publication_commit_mismatch: HEAD is not the exact approved checkpoint commit")
	}
	parent, err := gitOutputTrimmed(workspace, "rev-parse", request.SourceCommit+"^")
	if err != nil || parent != request.SourceParent {
		return errors.New("publication_commit_mismatch: source parent is not the approved checkpoint parent")
	}
	status, err := controlledCheckpointStatus(workspace)
	if err != nil || len(status) != 0 {
		return errors.New("publication_workspace_dirty: worktree and index must be empty")
	}
	return nil
}

func controlledPublicationReceipt(request *ControlledPublicationRequest, checkpoint *ControlledCheckpointReceipt, recovered bool, createdAt string) *ControlledPublicationReceipt {
	return &ControlledPublicationReceipt{
		ContractVersion: ControlledPublicationReceiptContract, RequestID: request.RequestID, RequestFingerprint: request.RequestFingerprint,
		AuthorizationFingerprint: request.AuthorizationFingerprint, CheckpointRequestFingerprint: request.CheckpointRequestFingerprint,
		CheckpointReceiptFingerprint: request.CheckpointReceiptFingerprint, CheckpointID: checkpoint.CheckpointID,
		SessionID: request.SessionID, WorkspaceID: request.WorkspaceID, Branch: request.ExpectedBranch,
		SourceCommit: request.SourceCommit, SourceParent: request.SourceParent, RemoteName: request.RemoteName,
		RemoteIdentity: request.RemoteIdentity, DestinationRef: request.DestinationRef, PublicationScope: request.PublicationScope,
		Status: "published", CreatedAt: createdAt, Recovered: recovered,
		Runtime: ControlledPublicationRuntime{Owner: "dockpipe", Operation: "session.publication", ExactCommit: true, ExactRef: true, RawGitDelegated: false},
		Push:    ControlledPublicationPush{RefspecKind: "exact_commit_to_fully_qualified_ref", Force: false, UpstreamConfigured: false, CredentialMaterialPersisted: false},
		Actions: ControlledPublicationActions{Publication: true},
	}
}

func validateControlledPublicationReceipt(receipt *ControlledPublicationReceipt, request *ControlledPublicationRequest, checkpoint *ControlledCheckpointReceipt) error {
	if receipt == nil || receipt.ContractVersion != ControlledPublicationReceiptContract || receipt.RequestID != request.RequestID ||
		receipt.RequestFingerprint != request.RequestFingerprint || receipt.AuthorizationFingerprint != request.AuthorizationFingerprint ||
		receipt.CheckpointRequestFingerprint != request.CheckpointRequestFingerprint || receipt.CheckpointReceiptFingerprint != request.CheckpointReceiptFingerprint ||
		receipt.CheckpointID != checkpoint.CheckpointID || receipt.SessionID != request.SessionID || receipt.WorkspaceID != request.WorkspaceID ||
		receipt.Branch != request.ExpectedBranch || receipt.SourceCommit != request.SourceCommit || receipt.SourceParent != request.SourceParent ||
		receipt.RemoteName != request.RemoteName || receipt.RemoteIdentity != request.RemoteIdentity || receipt.DestinationRef != request.DestinationRef ||
		receipt.PublicationScope != request.PublicationScope || receipt.Status != "published" || receipt.Runtime.Owner != "dockpipe" ||
		receipt.Runtime.Operation != "session.publication" || !receipt.Runtime.ExactCommit || !receipt.Runtime.ExactRef || receipt.Runtime.RawGitDelegated ||
		receipt.Push.RefspecKind != "exact_commit_to_fully_qualified_ref" || receipt.Push.Force || receipt.Push.UpstreamConfigured || receipt.Push.CredentialMaterialPersisted ||
		!receipt.Actions.Publication || receipt.Actions.Checkpoint || receipt.Actions.Sync || receipt.Actions.Fetch || receipt.Actions.Merge || receipt.Actions.Force {
		return errors.New("publication_receipt_invalid: receipt is malformed, tampered, or does not match the exact request")
	}
	if _, err := time.Parse(time.RFC3339, receipt.CreatedAt); err != nil {
		return errors.New("publication_receipt_invalid: created_at is not canonical RFC3339")
	}
	return nil
}

func controlledPublicationRequestFingerprint(request ControlledPublicationRequest) (string, error) {
	request.RequestFingerprint = ""
	raw, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	return controlledCheckpointBytesSHA256(raw), nil
}

func validateControlledPublicationDestination(value string) error {
	if !strings.HasPrefix(value, "refs/heads/") || len(value) > 255 || value != strings.TrimSpace(value) || strings.ContainsAny(value, " ~^:?*[\\") ||
		strings.Contains(value, "..") || strings.Contains(value, "@{") || strings.Contains(value, "//") || strings.Contains(value, "*") || strings.HasSuffix(value, "/") ||
		strings.HasSuffix(value, ".") || strings.HasSuffix(value, ".lock") || strings.HasPrefix(value, "-") {
		return errors.New("destination ref must be one canonical fully qualified refs/heads ref")
	}
	tail := strings.TrimPrefix(value, "refs/heads/")
	for _, part := range strings.Split(tail, "/") {
		if part == "" || part == "@" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, ".lock") {
			return errors.New("destination ref must be one canonical fully qualified refs/heads ref")
		}
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return errors.New("destination ref contains a control character")
		}
	}
	return nil
}

func observeControlledPublicationRef(workspace, remote, destination string) (string, error) {
	raw, err := gitOutputBytes(workspace, "ls-remote", "--refs", "--", remote, destination)
	if err != nil {
		return "", err
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return "", nil
	}
	if len(lines) != 1 {
		return "", errors.New("publication remote observation returned an ambiguous result")
	}
	fields := strings.Fields(string(lines[0]))
	if len(fields) != 2 || fields[1] != destination || !controlledCheckpointCommitHash.MatchString(fields[0]) {
		return "", errors.New("publication remote observation is malformed or does not match the exact ref")
	}
	return fields[0], nil
}

func controlledPublicationMetadataPath(session *GitSession, request *ControlledPublicationRequest) (string, error) {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return "", err
	}
	value := strings.TrimPrefix(request.RequestFingerprint, "sha256:")
	if len(value) > 16 {
		value = value[:16]
	}
	return filepath.Join(dir, "publications", "pub-request-"+value+".json"), nil
}

func writeControlledPublicationMetadata(session *GitSession, receipt *ControlledPublicationReceipt) error {
	request := &ControlledPublicationRequest{RequestFingerprint: receipt.RequestFingerprint}
	path, err := controlledPublicationMetadataPath(session, request)
	if err != nil {
		return err
	}
	if existing, exists, err := loadControlledPublicationReceipt(path); err != nil {
		return err
	} else if exists {
		if !reflect.DeepEqual(existing, receipt) {
			return errors.New("existing publication metadata is tampered or conflicts with the request")
		}
		return nil
	}
	return writeControlledCheckpointJSON(path, receipt, false)
}

func loadControlledPublicationMetadata(session *GitSession, request *ControlledPublicationRequest) (*ControlledPublicationReceipt, bool, error) {
	path, err := controlledPublicationMetadataPath(session, request)
	if err != nil {
		return nil, false, err
	}
	return loadControlledPublicationReceipt(path)
}

func loadControlledPublicationReceipt(path string) (*ControlledPublicationReceipt, bool, error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("publication_receipt_invalid: %w", err)
	}
	var receipt ControlledPublicationReceipt
	if err := readControlledCheckpointJSON(path, &receipt); err != nil {
		return nil, false, fmt.Errorf("publication_receipt_invalid: %w", err)
	}
	return &receipt, true, nil
}
