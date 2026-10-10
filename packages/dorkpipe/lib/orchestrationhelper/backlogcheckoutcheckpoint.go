package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const (
	backlogCheckoutCheckpointFixtureContract  = "dorkpipe.checkout-checkpoint-approval-fixture/v1"
	backlogCheckoutCheckpointApprovalContract = "dorkpipe.checkout-checkpoint-approval/v1"
	backlogCheckoutCheckpointScope            = "accepted_postimages_exact_runtime_checkpoint_once"
)

type backlogCheckoutCheckpointFixture struct {
	ContractVersion                        string   `json:"contract_version"`
	ApprovalID                             string   `json:"approval_id"`
	ReplayIdentity                         string   `json:"replay_identity"`
	Decision                               string   `json:"decision"`
	CheckpointScope                        string   `json:"checkpoint_scope"`
	TaskID                                 string   `json:"task_id"`
	RemoteTaskID                           string   `json:"remote_task_id"`
	RequestFingerprint                     string   `json:"request_fingerprint"`
	CompatibilityFingerprint               string   `json:"compatibility_fingerprint"`
	DispatchFingerprint                    string   `json:"dispatch_fingerprint"`
	AdapterIdentity                        string   `json:"adapter_identity"`
	EnvironmentRef                         string   `json:"environment_ref"`
	BranchRef                              string   `json:"branch_ref"`
	BaselineCommit                         string   `json:"baseline_commit"`
	CompletionCandidateFingerprint         string   `json:"completion_candidate_fingerprint"`
	RemoteStatusFingerprint                string   `json:"remote_status_fingerprint"`
	RemoteDiffFingerprint                  string   `json:"remote_diff_fingerprint"`
	RemoteResultFingerprint                string   `json:"remote_result_fingerprint"`
	ValidationReceiptFingerprint           string   `json:"validation_receipt_fingerprint"`
	PatchBoundaryFingerprint               string   `json:"patch_boundary_fingerprint"`
	PatchApplicationFingerprint            string   `json:"patch_application_fingerprint"`
	ValidationExecutionFingerprint         string   `json:"validation_execution_fingerprint"`
	SemanticReviewFingerprint              string   `json:"semantic_review_fingerprint"`
	ReadinessFingerprint                   string   `json:"readiness_fingerprint"`
	CheckoutApplicationApprovalFingerprint string   `json:"checkout_application_approval_fingerprint"`
	CheckoutApplicationFingerprint         string   `json:"checkout_application_fingerprint"`
	PatchSHA256                            string   `json:"patch_sha256"`
	PatchBytes                             *int     `json:"patch_bytes"`
	ChangedPaths                           []string `json:"changed_paths"`
	ConsumerPostimageFingerprint           string   `json:"consumer_postimage_fingerprint"`
	RuntimeSessionID                       string   `json:"runtime_session_id"`
	RuntimeWorkspaceID                     string   `json:"runtime_workspace_id"`
	RuntimeSessionBranch                   string   `json:"runtime_session_branch"`
	ExpectedParentCommit                   string   `json:"expected_parent_commit"`
	CheckpointMessage                      string   `json:"checkpoint_message"`
}

type backlogRuntimeCheckpointBinding struct {
	SessionID   string
	WorkspaceID string
	Branch      string
	Workspace   string
}

type backlogCheckoutCheckpointChain struct {
	ApplicationChain      *backlogCheckoutApplicationChain
	ApplicationApproval   map[string]any
	Application           map[string]any
	ApplicationApprovalFP string
	ApplicationFP         string
	Postimages            map[string]backlogCheckoutManifestEntry
}

func requestBacklogCheckoutCheckpoint(consumerRoot, artifactRoot, fixturePath string, runtimeBinding backlogRuntimeCheckpointBinding) error {
	chain, err := loadBacklogCheckoutCheckpointChain(consumerRoot, artifactRoot)
	if err != nil {
		return rejectBacklog("checkout_checkpoint_chain_invalid", "%v", err)
	}
	fixture, err := loadBacklogCheckoutCheckpointFixture(fixturePath)
	if err != nil {
		return err
	}
	if err := validateBacklogCheckoutCheckpointFixture(fixture); err != nil {
		return err
	}
	expected := backlogCheckoutCheckpointFixtureForChain(chain, fixture.ApprovalID, fixture.ReplayIdentity, fixture.Decision, runtimeBinding, fixture.ExpectedParentCommit, fixture.CheckpointMessage)
	if !reflect.DeepEqual(*fixture, expected) {
		return rejectBacklog("checkout_checkpoint_binding_mismatch", "checkpoint decision does not match the exact checkout application, immutable chain, consumer postimages, runtime session/workspace/branch, parent, paths, and fixed scope")
	}
	if !sameBacklogPath(consumerRoot, runtimeBinding.Workspace) {
		return rejectBacklog("checkout_checkpoint_session_mismatch", "consumer root must be the exact runtime-owned session workspace")
	}

	approvalPath, err := backlogArtifactPath(artifactRoot, "checkout-checkpoint-approval.json")
	if err != nil {
		return err
	}
	requestPath, err := backlogArtifactPath(artifactRoot, "checkpoint-request.json")
	if err != nil {
		return err
	}
	receiptPath, err := backlogArtifactPath(artifactRoot, "checkpoint-receipt.json")
	if err != nil {
		return err
	}
	approval, err := backlogCheckoutCheckpointApprovalPayload(chain, fixture, runtimeBinding)
	if err != nil {
		return err
	}
	if existing, exists, loadErr := loadExistingBacklogCheckoutArtifact(approvalPath, "checkout checkpoint approval"); loadErr != nil {
		return loadErr
	} else if exists {
		if err := validateBacklogCheckoutCheckpointApproval(existing, chain, runtimeBinding); err != nil {
			return rejectBacklog("checkout_checkpoint_approval_artifact_invalid", "%v", err)
		}
		existingIdentity := mapValue(existing["identity"])
		existingDecision := stringValue(mapValue(existing["decision"])["value"])
		switch {
		case jsonMapsEqual(existing, approval):
			approval = existing
		case stringValue(existingIdentity["approval_id"]) == fixture.ApprovalID && stringValue(existingIdentity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("checkout_checkpoint_decision_conflict", "accepted checkpoint identity cannot change from %q to %q", existingDecision, fixture.Decision)
		case stringValue(existingIdentity["approval_id"]) == fixture.ApprovalID:
			return rejectBacklog("checkout_checkpoint_duplicate", "checkpoint approval identity %q was already recorded", fixture.ApprovalID)
		case stringValue(existingIdentity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("checkout_checkpoint_replay", "checkpoint replay identity %q was already recorded", fixture.ReplayIdentity)
		default:
			return rejectBacklog("checkout_checkpoint_already_recorded", "one checkpoint decision is already recorded for the accepted checkout application")
		}
	} else {
		if _, exists, loadErr := loadExistingBacklogCheckoutArtifact(requestPath, "checkpoint request"); loadErr != nil {
			return loadErr
		} else if exists {
			return rejectBacklog("checkout_checkpoint_request_artifact_invalid", "checkpoint-request.json exists without its accepted approval")
		}
		if _, err := os.Lstat(receiptPath); err == nil {
			return rejectBacklog("checkout_checkpoint_receipt_artifact_invalid", "checkpoint-receipt.json exists without its accepted approval and request")
		} else if !os.IsNotExist(err) {
			return rejectBacklog("checkout_checkpoint_receipt_artifact_invalid", "%v", err)
		}
		if err := writeJSONFileAtomic(approvalPath, approval); err != nil {
			return err
		}
	}

	if fixture.Decision == backlogSemanticReviewRejected {
		if _, err := os.Lstat(requestPath); err == nil {
			return rejectBacklog("checkout_checkpoint_request_artifact_invalid", "rejected checkpoint approval must not have a request")
		} else if !os.IsNotExist(err) {
			return rejectBacklog("checkout_checkpoint_request_artifact_invalid", "%v", err)
		}
		if _, err := os.Lstat(receiptPath); err == nil {
			return rejectBacklog("checkout_checkpoint_receipt_artifact_invalid", "rejected checkpoint approval must not have a receipt")
		} else if !os.IsNotExist(err) {
			return rejectBacklog("checkout_checkpoint_receipt_artifact_invalid", "%v", err)
		}
		return nil
	}

	request, err := backlogControlledCheckpointRequest(chain, approval, fixture, runtimeBinding)
	if err != nil {
		return err
	}
	if existing, loadErr := infrastructure.LoadControlledCheckpointRequest(requestPath); loadErr == nil {
		if !reflect.DeepEqual(*existing, request) {
			return rejectBacklog("checkout_checkpoint_request_artifact_invalid", "existing checkpoint-request.json is tampered or does not match the approval")
		}
		return nil
	} else if _, statErr := os.Lstat(requestPath); statErr == nil {
		return rejectBacklog("checkout_checkpoint_request_artifact_invalid", "%v", loadErr)
	} else if !os.IsNotExist(statErr) {
		return rejectBacklog("checkout_checkpoint_request_artifact_invalid", "%v", statErr)
	}
	if err := infrastructure.WriteControlledCheckpointRequest(requestPath, request); err != nil {
		return rejectBacklog("checkout_checkpoint_request_failed", "%v", err)
	}
	return nil
}

func loadBacklogCheckoutCheckpointChain(consumerRoot, artifactRoot string) (*backlogCheckoutCheckpointChain, error) {
	applicationChain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
	if err != nil {
		return nil, err
	}
	approvalPath, err := requireBacklogRegularArtifact(artifactRoot, "checkout-application-approval.json")
	if err != nil {
		return nil, err
	}
	applicationPath, err := requireBacklogRegularArtifact(artifactRoot, "checkout-application.json")
	if err != nil {
		return nil, err
	}
	approval, err := readStrictJSONMap(approvalPath)
	if err != nil {
		return nil, err
	}
	if err := validateBacklogCheckoutApplicationApproval(approval, applicationChain); err != nil {
		return nil, err
	}
	if stringValue(mapValue(approval["decision"])["value"]) != backlogSemanticReviewApproved {
		return nil, errors.New("checkpoint requires an approved checkout application")
	}
	application, err := readStrictJSONMap(applicationPath)
	if err != nil {
		return nil, err
	}
	applicationDetails := mapValue(application["application"])
	fileCount, fileOK := backlogJSONInt(applicationDetails["files_applied"])
	hunkCount, hunkOK := backlogJSONInt(applicationDetails["hunks_applied"])
	if !fileOK || !hunkOK || fileCount != len(applicationChain.Semantic.ChangedPaths) || hunkCount < 1 {
		return nil, errors.New("checkout application file or hunk count is invalid")
	}
	expectedApplication, err := backlogCheckoutApplicationPayload(applicationChain, approval, fileCount, hunkCount)
	if err != nil || !jsonMapsEqual(application, expectedApplication) {
		return nil, errors.New("checkout application receipt is malformed, tampered, or does not match its approval")
	}
	root, err := validateBacklogConsumerRoot(consumerRoot)
	if err != nil {
		return nil, err
	}
	postimages, err := parseBacklogCheckoutManifest(applicationChain.PostimageManifest, applicationChain.Semantic.ChangedPaths)
	if err != nil {
		return nil, err
	}
	for _, path := range applicationChain.Semantic.ChangedPaths {
		raw, readErr := readBacklogConsumerSource(root, path)
		expected := postimages[path]
		if readErr != nil || len(raw) != expected.Bytes || sha256String(raw) != expected.SHA256 {
			return nil, fmt.Errorf("consumer postimage for %q does not match the accepted checkout application", path)
		}
	}
	approvalFingerprint := stringValue(approval["artifact_fingerprint"])
	applicationFingerprint := stringValue(application["artifact_fingerprint"])
	if !backlogFingerprint.MatchString(approvalFingerprint) || !backlogFingerprint.MatchString(applicationFingerprint) {
		return nil, errors.New("checkout application fingerprints are malformed")
	}
	return &backlogCheckoutCheckpointChain{
		ApplicationChain: applicationChain, ApplicationApproval: approval, Application: application,
		ApplicationApprovalFP: approvalFingerprint, ApplicationFP: applicationFingerprint, Postimages: postimages,
	}, nil
}

func loadBacklogCheckoutCheckpointFixture(path string) (*backlogCheckoutCheckpointFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, rejectBacklog("checkout_checkpoint_fixture_missing", "checkpoint approval fixture cannot be read: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	fixture := backlogCheckoutCheckpointFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		return nil, rejectBacklog("checkout_checkpoint_fixture_malformed", "checkpoint approval fixture is malformed: %v", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, rejectBacklog("checkout_checkpoint_fixture_malformed", "checkpoint approval fixture is malformed: %v", err)
	}
	return &fixture, nil
}

func validateBacklogCheckoutCheckpointFixture(fixture *backlogCheckoutCheckpointFixture) error {
	if fixture.ContractVersion != backlogCheckoutCheckpointFixtureContract ||
		!backlogOpaqueID.MatchString(fixture.ApprovalID) || !backlogOpaqueID.MatchString(fixture.ReplayIdentity) || fixture.ApprovalID == fixture.ReplayIdentity ||
		!backlogTaskIDPattern.MatchString(fixture.TaskID) || !backlogOpaqueID.MatchString(fixture.RemoteTaskID) ||
		!backlogOpaqueID.MatchString(fixture.AdapterIdentity) || !backlogOpaqueID.MatchString(fixture.RuntimeSessionID) ||
		!backlogOpaqueID.MatchString(fixture.RuntimeWorkspaceID) || !backlogBaseline.MatchString(fixture.BaselineCommit) ||
		!backlogBaseline.MatchString(fixture.ExpectedParentCommit) {
		return rejectBacklog("checkout_checkpoint_identity_invalid", "checkpoint contract or bounded identity fields are invalid")
	}
	if fixture.Decision != backlogSemanticReviewApproved && fixture.Decision != backlogSemanticReviewRejected {
		return rejectBacklog("checkout_checkpoint_decision_invalid", "checkpoint decision must be exactly approved or rejected")
	}
	if fixture.CheckpointScope != backlogCheckoutCheckpointScope {
		return rejectBacklog("checkout_checkpoint_scope_invalid", "checkpoint scope must be exactly %q", backlogCheckoutCheckpointScope)
	}
	for _, value := range []string{fixture.EnvironmentRef, fixture.BranchRef, fixture.RuntimeSessionBranch} {
		if err := validateBacklogReference("checkpoint reference", value); err != nil {
			return rejectBacklog("checkout_checkpoint_identity_invalid", "%v", err)
		}
	}
	for _, value := range []string{
		fixture.RequestFingerprint, fixture.CompatibilityFingerprint, fixture.DispatchFingerprint,
		fixture.CompletionCandidateFingerprint, fixture.RemoteStatusFingerprint, fixture.RemoteDiffFingerprint,
		fixture.RemoteResultFingerprint, fixture.ValidationReceiptFingerprint, fixture.PatchBoundaryFingerprint,
		fixture.PatchApplicationFingerprint, fixture.ValidationExecutionFingerprint, fixture.SemanticReviewFingerprint,
		fixture.ReadinessFingerprint, fixture.CheckoutApplicationApprovalFingerprint, fixture.CheckoutApplicationFingerprint,
		fixture.PatchSHA256, fixture.ConsumerPostimageFingerprint,
	} {
		if !backlogFingerprint.MatchString(value) {
			return rejectBacklog("checkout_checkpoint_identity_invalid", "checkpoint fingerprint binding is invalid")
		}
	}
	if fixture.PatchBytes == nil || *fixture.PatchBytes < 0 || len(fixture.ChangedPaths) == 0 || !sort.StringsAreSorted(fixture.ChangedPaths) {
		return rejectBacklog("checkout_checkpoint_identity_invalid", "checkpoint patch byte count or changed paths are invalid")
	}
	for index, path := range fixture.ChangedPaths {
		if err := validateBacklogPatchPath(path); err != nil || (index > 0 && path == fixture.ChangedPaths[index-1]) {
			return rejectBacklog("checkout_checkpoint_identity_invalid", "checkpoint changed paths are malformed, duplicated, or unsorted")
		}
	}
	if fixture.CheckpointMessage == "" || fixture.CheckpointMessage != strings.TrimSpace(fixture.CheckpointMessage) || len(fixture.CheckpointMessage) > 200 || strings.ContainsAny(fixture.CheckpointMessage, "\r\n") {
		return rejectBacklog("checkout_checkpoint_identity_invalid", "checkpoint message must be one trimmed line of at most 200 bytes")
	}
	return nil
}

func backlogCheckoutCheckpointFixtureForChain(chain *backlogCheckoutCheckpointChain, approvalID, replayIdentity, decision string, runtimeBinding backlogRuntimeCheckpointBinding, expectedParent, message string) backlogCheckoutCheckpointFixture {
	application := chain.Application
	binding := mapValue(application["binding"])
	patch := mapValue(application["accepted_patch"])
	patchBytes, _ := backlogJSONInt(patch["bytes"])
	return backlogCheckoutCheckpointFixture{
		ContractVersion: backlogCheckoutCheckpointFixtureContract, ApprovalID: approvalID, ReplayIdentity: replayIdentity,
		Decision: decision, CheckpointScope: backlogCheckoutCheckpointScope,
		TaskID: stringValue(binding["task_id"]), RemoteTaskID: stringValue(binding["remote_task_id"]),
		RequestFingerprint: stringValue(binding["request_fingerprint"]), CompatibilityFingerprint: stringValue(binding["compatibility_fingerprint"]),
		DispatchFingerprint: stringValue(binding["dispatch_fingerprint"]), AdapterIdentity: stringValue(binding["adapter_identity"]),
		EnvironmentRef: stringValue(binding["environment_ref"]), BranchRef: stringValue(binding["branch_ref"]), BaselineCommit: stringValue(binding["baseline_commit"]),
		CompletionCandidateFingerprint: stringValue(mapValue(application["completion_candidate"])["fingerprint"]),
		RemoteStatusFingerprint:        stringValue(mapValue(application["remote_status"])["fingerprint"]),
		RemoteDiffFingerprint:          stringValue(mapValue(application["remote_diff"])["fingerprint"]),
		RemoteResultFingerprint:        stringValue(mapValue(application["remote_result"])["fingerprint"]),
		ValidationReceiptFingerprint:   stringValue(mapValue(application["validation_receipt"])["fingerprint"]),
		PatchBoundaryFingerprint:       stringValue(mapValue(application["patch_boundary"])["fingerprint"]),
		PatchApplicationFingerprint:    stringValue(mapValue(application["patch_application"])["fingerprint"]),
		ValidationExecutionFingerprint: stringValue(mapValue(application["validation_execution"])["fingerprint"]),
		SemanticReviewFingerprint:      stringValue(mapValue(application["semantic_review_decision"])["fingerprint"]),
		ReadinessFingerprint:           chain.ApplicationChain.ReadyFingerprint, CheckoutApplicationApprovalFingerprint: chain.ApplicationApprovalFP,
		CheckoutApplicationFingerprint: chain.ApplicationFP, PatchSHA256: stringValue(patch["sha256"]), PatchBytes: &patchBytes,
		ChangedPaths: append([]string{}, chain.ApplicationChain.Semantic.ChangedPaths...), ConsumerPostimageFingerprint: chain.ApplicationChain.PostimageFingerprint,
		RuntimeSessionID: runtimeBinding.SessionID, RuntimeWorkspaceID: runtimeBinding.WorkspaceID, RuntimeSessionBranch: runtimeBinding.Branch,
		ExpectedParentCommit: expectedParent, CheckpointMessage: message,
	}
}

func backlogCheckoutCheckpointApprovalPayload(chain *backlogCheckoutCheckpointChain, fixture *backlogCheckoutCheckpointFixture, runtimeBinding backlogRuntimeCheckpointBinding) (map[string]any, error) {
	affirmative := fixture.Decision == backlogSemanticReviewApproved
	payload := map[string]any{
		"contract_version": backlogCheckoutCheckpointApprovalContract, "state": "applied_for_review",
		"identity":                      map[string]any{"approval_id": fixture.ApprovalID, "replay_identity": fixture.ReplayIdentity},
		"decision":                      map[string]any{"value": fixture.Decision, "affirmative": affirmative, "explicit_local_decision": true, "checkpoint_scope": backlogCheckoutCheckpointScope},
		"checkout_application":          map[string]any{"fingerprint": chain.ApplicationFP},
		"checkout_application_approval": map[string]any{"fingerprint": chain.ApplicationApprovalFP},
		"readiness":                     map[string]any{"fingerprint": chain.ApplicationChain.ReadyFingerprint},
		"semantic_review_decision":      mapValue(chain.Application["semantic_review_decision"]),
		"validation_execution":          mapValue(chain.Application["validation_execution"]), "patch_application": mapValue(chain.Application["patch_application"]),
		"patch_boundary": mapValue(chain.Application["patch_boundary"]), "validation_receipt": mapValue(chain.Application["validation_receipt"]),
		"remote_result": mapValue(chain.Application["remote_result"]), "remote_diff": mapValue(chain.Application["remote_diff"]),
		"remote_status": mapValue(chain.Application["remote_status"]), "completion_candidate": mapValue(chain.Application["completion_candidate"]),
		"binding": mapValue(chain.Application["binding"]), "accepted_patch": mapValue(chain.Application["accepted_patch"]),
		"changed_paths": anyStrings(chain.ApplicationChain.Semantic.ChangedPaths), "consumer_postimage_manifest": chain.ApplicationChain.PostimageManifest,
		"runtime":      map[string]any{"session_id": runtimeBinding.SessionID, "workspace_id": runtimeBinding.WorkspaceID, "session_branch": runtimeBinding.Branch, "expected_parent_commit": fixture.ExpectedParentCommit, "checkpoint_message": fixture.CheckpointMessage, "checkpoint_scope": infrastructure.ControlledCheckpointScope},
		"source":       map[string]any{"mode": "fixture", "package_owned_metadata": true, "fixture_contract": backlogCheckoutCheckpointFixtureContract, "unbounded_approval_prose": false},
		"actions":      map[string]any{"checkpoint_request_emitted": false, "checkpoint_performed": false, "push_performed": false, "publication_performed": false, "sync_performed": false, "merge_performed": false, "next_task_selected": false},
		"capabilities": map[string]any{"submit_exact_runtime_checkpoint_request": affirmative, "checkpoint": false, "push": false, "publication": false, "sync": false, "merge": false, "start_another_backlog_item": false},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}

func validateBacklogCheckoutCheckpointApproval(payload map[string]any, chain *backlogCheckoutCheckpointChain, runtimeBinding backlogRuntimeCheckpointBinding) error {
	if stringValue(payload["contract_version"]) != backlogCheckoutCheckpointApprovalContract || stringValue(payload["state"]) != "applied_for_review" {
		return errors.New("checkpoint approval has an unsupported contract or state")
	}
	identity := mapValue(payload["identity"])
	decision := stringValue(mapValue(payload["decision"])["value"])
	approvalID := stringValue(identity["approval_id"])
	replayIdentity := stringValue(identity["replay_identity"])
	runtimePayload := mapValue(payload["runtime"])
	expectedParent := stringValue(runtimePayload["expected_parent_commit"])
	message := stringValue(runtimePayload["checkpoint_message"])
	if !backlogOpaqueID.MatchString(approvalID) || !backlogOpaqueID.MatchString(replayIdentity) || approvalID == replayIdentity ||
		(decision != backlogSemanticReviewApproved && decision != backlogSemanticReviewRejected) {
		return errors.New("checkpoint approval identity or decision is malformed")
	}
	fixture := backlogCheckoutCheckpointFixtureForChain(chain, approvalID, replayIdentity, decision, runtimeBinding, expectedParent, message)
	expected, err := backlogCheckoutCheckpointApprovalPayload(chain, &fixture, runtimeBinding)
	if err != nil || !jsonMapsEqual(payload, expected) {
		return errors.New("checkpoint approval is malformed, tampered, or does not match the checkout application and runtime binding")
	}
	return nil
}

func backlogControlledCheckpointRequest(chain *backlogCheckoutCheckpointChain, approval map[string]any, fixture *backlogCheckoutCheckpointFixture, runtimeBinding backlogRuntimeCheckpointBinding) (infrastructure.ControlledCheckpointRequest, error) {
	postimages := make([]infrastructure.ControlledCheckpointPostimage, 0, len(fixture.ChangedPaths))
	for _, path := range fixture.ChangedPaths {
		entry, ok := chain.Postimages[path]
		if !ok {
			return infrastructure.ControlledCheckpointRequest{}, fmt.Errorf("checkout checkpoint postimage for %q is missing", path)
		}
		postimages = append(postimages, infrastructure.ControlledCheckpointPostimage{Path: path, SHA256: entry.SHA256, Bytes: int64(entry.Bytes)})
	}
	request, err := infrastructure.FinalizeControlledCheckpointRequest(infrastructure.ControlledCheckpointRequest{
		ContractVersion: infrastructure.ControlledCheckpointRequestContract, RequestID: fixture.ApprovalID,
		AuthorizationFingerprint: stringValue(approval["artifact_fingerprint"]), SessionID: runtimeBinding.SessionID,
		WorkspaceID: runtimeBinding.WorkspaceID, ExpectedBranch: runtimeBinding.Branch, ExpectedParent: fixture.ExpectedParentCommit,
		CheckpointScope: infrastructure.ControlledCheckpointScope, Message: fixture.CheckpointMessage,
		Paths: append([]string{}, fixture.ChangedPaths...), Postimages: postimages,
	})
	if err != nil {
		return infrastructure.ControlledCheckpointRequest{}, rejectBacklog("checkout_checkpoint_request_invalid", "%v", err)
	}
	return request, nil
}

func sameBacklogPath(left, right string) bool {
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
