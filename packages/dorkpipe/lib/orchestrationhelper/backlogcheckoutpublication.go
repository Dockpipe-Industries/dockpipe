package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"

	"dockpipe/src/lib/infrastructure"
)

const (
	backlogCheckoutPublicationFixtureContract  = "dorkpipe.checkout-publication-approval-fixture/v1"
	backlogCheckoutPublicationApprovalContract = "dorkpipe.checkout-publication-approval/v1"
	backlogCheckoutPublicationScope            = "approved_checkpoint_exact_commit_exact_branch_ref_once"
)

type backlogCheckoutPublicationFixture struct {
	ContractVersion               string `json:"contract_version"`
	ApprovalID                    string `json:"approval_id"`
	ReplayIdentity                string `json:"replay_identity"`
	Decision                      string `json:"decision"`
	PublicationScope              string `json:"publication_scope"`
	TaskID                        string `json:"task_id"`
	ImmutableChainFingerprint     string `json:"immutable_chain_fingerprint"`
	CheckpointApprovalFingerprint string `json:"checkpoint_approval_fingerprint"`
	CheckpointRequestFingerprint  string `json:"checkpoint_request_fingerprint"`
	CheckpointReceiptFingerprint  string `json:"checkpoint_receipt_fingerprint"`
	RuntimeSessionID              string `json:"runtime_session_id"`
	RuntimeWorkspaceID            string `json:"runtime_workspace_id"`
	RuntimeSessionBranch          string `json:"runtime_session_branch"`
	SourceCommit                  string `json:"source_commit"`
	SourceParent                  string `json:"source_parent"`
	RemoteName                    string `json:"remote_name"`
	RemoteIdentity                string `json:"remote_identity"`
	DestinationRef                string `json:"destination_ref"`
	PublicationReason             string `json:"publication_reason"`
}

type backlogCheckoutPublicationChain struct {
	CheckpointChain      *backlogCheckoutCheckpointChain
	CheckpointApproval   map[string]any
	CheckpointApprovalFP string
	CheckpointRequest    *infrastructure.ControlledCheckpointRequest
	CheckpointReceipt    *infrastructure.ControlledCheckpointReceipt
	CheckpointReceiptFP  string
	ImmutableChainFP     string
	Session              *infrastructure.GitSession
}

func requestBacklogCheckoutPublication(consumerRoot, artifactRoot, fixturePath string, runtimeBinding backlogRuntimeCheckpointBinding) error {
	chain, err := loadBacklogCheckoutPublicationChain(consumerRoot, artifactRoot, runtimeBinding)
	if err != nil {
		return rejectBacklog("checkout_publication_chain_invalid", "%v", err)
	}
	fixture, err := loadBacklogCheckoutPublicationFixture(fixturePath)
	if err != nil {
		return err
	}
	if err := validateBacklogCheckoutPublicationFixture(fixture); err != nil {
		return err
	}
	expected := backlogCheckoutPublicationFixtureForChain(chain, fixture.ApprovalID, fixture.ReplayIdentity, fixture.Decision, fixture.RemoteName, fixture.RemoteIdentity, fixture.DestinationRef, fixture.PublicationReason)
	if !reflect.DeepEqual(*fixture, expected) {
		return rejectBacklog("checkout_publication_binding_mismatch", "publication decision does not match the exact immutable checkpoint chain, runtime session, commit, remote identity, destination ref, and fixed scope")
	}

	approvalPath, err := backlogArtifactPath(artifactRoot, "checkout-publication-approval.json")
	if err != nil {
		return err
	}
	requestPath, err := backlogArtifactPath(artifactRoot, "publication-request.json")
	if err != nil {
		return err
	}
	receiptPath, err := backlogArtifactPath(artifactRoot, "publication-receipt.json")
	if err != nil {
		return err
	}
	approval, err := backlogCheckoutPublicationApprovalPayload(chain, fixture)
	if err != nil {
		return err
	}
	if existing, exists, loadErr := loadExistingBacklogCheckoutArtifact(approvalPath, "checkout publication approval"); loadErr != nil {
		return loadErr
	} else if exists {
		if err := validateBacklogCheckoutPublicationApproval(existing, chain); err != nil {
			return rejectBacklog("checkout_publication_approval_artifact_invalid", "%v", err)
		}
		identity := mapValue(existing["identity"])
		switch {
		case jsonMapsEqual(existing, approval):
			approval = existing
		case stringValue(identity["approval_id"]) == fixture.ApprovalID && stringValue(identity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("checkout_publication_decision_conflict", "accepted publication identity cannot change")
		case stringValue(identity["approval_id"]) == fixture.ApprovalID:
			return rejectBacklog("checkout_publication_duplicate", "publication approval identity %q was already recorded", fixture.ApprovalID)
		case stringValue(identity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("checkout_publication_replay", "publication replay identity %q was already recorded", fixture.ReplayIdentity)
		default:
			return rejectBacklog("checkout_publication_already_recorded", "one publication decision is already recorded for the checkpoint")
		}
	} else {
		for _, path := range []string{requestPath, receiptPath} {
			if _, statErr := os.Lstat(path); statErr == nil {
				return rejectBacklog("checkout_publication_artifact_invalid", "publication request or receipt exists without its accepted approval")
			} else if !os.IsNotExist(statErr) {
				return rejectBacklog("checkout_publication_artifact_invalid", "%v", statErr)
			}
		}
		if err := writeJSONFileAtomic(approvalPath, approval); err != nil {
			return err
		}
	}

	if fixture.Decision == backlogSemanticReviewRejected {
		for _, path := range []string{requestPath, receiptPath} {
			if _, statErr := os.Lstat(path); statErr == nil {
				return rejectBacklog("checkout_publication_artifact_invalid", "rejected publication approval must not have a request or receipt")
			} else if !os.IsNotExist(statErr) {
				return rejectBacklog("checkout_publication_artifact_invalid", "%v", statErr)
			}
		}
		return nil
	}

	request, err := infrastructure.FinalizeControlledPublicationRequest(infrastructure.ControlledPublicationRequest{
		ContractVersion: infrastructure.ControlledPublicationRequestContract, RequestID: fixture.ApprovalID,
		AuthorizationFingerprint:     stringValue(approval["artifact_fingerprint"]),
		CheckpointRequestFingerprint: chain.CheckpointRequest.RequestFingerprint, CheckpointReceiptFingerprint: chain.CheckpointReceiptFP,
		SessionID: fixture.RuntimeSessionID, WorkspaceID: fixture.RuntimeWorkspaceID, ExpectedBranch: fixture.RuntimeSessionBranch,
		SourceCommit: fixture.SourceCommit, SourceParent: fixture.SourceParent, RemoteName: fixture.RemoteName,
		RemoteIdentity: fixture.RemoteIdentity, DestinationRef: fixture.DestinationRef,
		PublicationScope: infrastructure.ControlledPublicationScope, Reason: fixture.PublicationReason,
	})
	if err != nil {
		return rejectBacklog("checkout_publication_request_invalid", "%v", err)
	}
	if existing, loadErr := infrastructure.LoadControlledPublicationRequest(requestPath); loadErr == nil {
		if !reflect.DeepEqual(*existing, request) {
			return rejectBacklog("checkout_publication_request_artifact_invalid", "existing publication-request.json is tampered or does not match the approval")
		}
		return nil
	} else if _, statErr := os.Lstat(requestPath); statErr == nil {
		return rejectBacklog("checkout_publication_request_artifact_invalid", "%v", loadErr)
	} else if !os.IsNotExist(statErr) {
		return rejectBacklog("checkout_publication_request_artifact_invalid", "%v", statErr)
	}
	if err := infrastructure.WriteControlledPublicationRequest(requestPath, request); err != nil {
		return rejectBacklog("checkout_publication_request_failed", "%v", err)
	}
	return nil
}

func loadBacklogCheckoutPublicationChain(consumerRoot, artifactRoot string, runtimeBinding backlogRuntimeCheckpointBinding) (*backlogCheckoutPublicationChain, error) {
	checkpointChain, err := loadBacklogCheckoutCheckpointChain(consumerRoot, artifactRoot)
	if err != nil {
		return nil, err
	}
	if !sameBacklogPath(consumerRoot, runtimeBinding.Workspace) {
		return nil, errors.New("consumer root must be the exact runtime-owned session workspace")
	}
	session, err := infrastructure.LoadGitSession(consumerRoot, runtimeBinding.SessionID)
	if err != nil || session.SessionID != runtimeBinding.SessionID || session.WorkspaceID != runtimeBinding.WorkspaceID ||
		session.Repo.SessionRef != runtimeBinding.Branch || !sameBacklogPath(session.Storage.Workspace, runtimeBinding.Workspace) {
		return nil, errors.New("runtime session metadata does not match the supplied workspace binding")
	}
	approvalPath, err := requireBacklogRegularArtifact(artifactRoot, "checkout-checkpoint-approval.json")
	if err != nil {
		return nil, err
	}
	approval, err := readStrictJSONMap(approvalPath)
	if err != nil || validateBacklogCheckoutCheckpointApproval(approval, checkpointChain, runtimeBinding) != nil ||
		stringValue(mapValue(approval["decision"])["value"]) != backlogSemanticReviewApproved {
		return nil, errors.New("checkpoint approval is missing, rejected, malformed, or tampered")
	}
	requestPath, err := requireBacklogRegularArtifact(artifactRoot, "checkpoint-request.json")
	if err != nil {
		return nil, err
	}
	receiptPath, err := requireBacklogRegularArtifact(artifactRoot, "checkpoint-receipt.json")
	if err != nil {
		return nil, err
	}
	request, err := infrastructure.LoadControlledCheckpointRequest(requestPath)
	if err != nil || request.AuthorizationFingerprint != stringValue(approval["artifact_fingerprint"]) {
		return nil, errors.New("checkpoint request is malformed or does not match its package approval")
	}
	receipt, receiptFP, err := infrastructure.ValidateControlledCheckpointReceiptForSession(session, requestPath, receiptPath)
	if err != nil {
		return nil, err
	}
	chainValues := map[string]any{
		"task_id":                       stringValue(mapValue(checkpointChain.Application["binding"])["task_id"]),
		"request":                       stringValue(mapValue(checkpointChain.Application["binding"])["request_fingerprint"]),
		"compatibility":                 stringValue(mapValue(checkpointChain.Application["binding"])["compatibility_fingerprint"]),
		"dispatch":                      stringValue(mapValue(checkpointChain.Application["binding"])["dispatch_fingerprint"]),
		"completion_candidate":          stringValue(mapValue(checkpointChain.Application["completion_candidate"])["fingerprint"]),
		"remote_status":                 stringValue(mapValue(checkpointChain.Application["remote_status"])["fingerprint"]),
		"remote_diff":                   stringValue(mapValue(checkpointChain.Application["remote_diff"])["fingerprint"]),
		"remote_result":                 stringValue(mapValue(checkpointChain.Application["remote_result"])["fingerprint"]),
		"validation_receipt":            stringValue(mapValue(checkpointChain.Application["validation_receipt"])["fingerprint"]),
		"patch_boundary":                stringValue(mapValue(checkpointChain.Application["patch_boundary"])["fingerprint"]),
		"patch_application":             stringValue(mapValue(checkpointChain.Application["patch_application"])["fingerprint"]),
		"validation_execution":          stringValue(mapValue(checkpointChain.Application["validation_execution"])["fingerprint"]),
		"semantic_review":               stringValue(mapValue(checkpointChain.Application["semantic_review_decision"])["fingerprint"]),
		"readiness":                     checkpointChain.ApplicationChain.ReadyFingerprint,
		"checkout_application_approval": checkpointChain.ApplicationApprovalFP,
		"checkout_application":          checkpointChain.ApplicationFP,
		"checkpoint_approval":           stringValue(approval["artifact_fingerprint"]),
		"checkpoint_request":            request.RequestFingerprint,
		"checkpoint_receipt":            receiptFP,
	}
	chainFP, err := backlogJSONFingerprint(chainValues)
	if err != nil {
		return nil, err
	}
	return &backlogCheckoutPublicationChain{CheckpointChain: checkpointChain, CheckpointApproval: approval,
		CheckpointApprovalFP: stringValue(approval["artifact_fingerprint"]), CheckpointRequest: request,
		CheckpointReceipt: receipt, CheckpointReceiptFP: receiptFP, ImmutableChainFP: chainFP, Session: session}, nil
}

func loadBacklogCheckoutPublicationFixture(path string) (*backlogCheckoutPublicationFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, rejectBacklog("checkout_publication_fixture_missing", "publication approval fixture cannot be read: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	fixture := backlogCheckoutPublicationFixture{}
	if err := decoder.Decode(&fixture); err != nil || ensureJSONEOF(decoder) != nil {
		return nil, rejectBacklog("checkout_publication_fixture_malformed", "publication approval fixture is malformed")
	}
	return &fixture, nil
}

func validateBacklogCheckoutPublicationFixture(fixture *backlogCheckoutPublicationFixture) error {
	if fixture.ContractVersion != backlogCheckoutPublicationFixtureContract || !backlogOpaqueID.MatchString(fixture.ApprovalID) ||
		!backlogOpaqueID.MatchString(fixture.ReplayIdentity) || fixture.ApprovalID == fixture.ReplayIdentity || !backlogTaskIDPattern.MatchString(fixture.TaskID) ||
		!backlogOpaqueID.MatchString(fixture.RuntimeSessionID) || !backlogOpaqueID.MatchString(fixture.RuntimeWorkspaceID) ||
		!backlogBaseline.MatchString(fixture.SourceCommit) || !backlogBaseline.MatchString(fixture.SourceParent) {
		return rejectBacklog("checkout_publication_identity_invalid", "publication contract or bounded identity fields are invalid")
	}
	if fixture.Decision != backlogSemanticReviewApproved && fixture.Decision != backlogSemanticReviewRejected {
		return rejectBacklog("checkout_publication_decision_invalid", "publication decision must be exactly approved or rejected")
	}
	if fixture.PublicationScope != backlogCheckoutPublicationScope {
		return rejectBacklog("checkout_publication_scope_invalid", "publication scope must be exactly %q", backlogCheckoutPublicationScope)
	}
	for _, value := range []string{fixture.ImmutableChainFingerprint, fixture.CheckpointApprovalFingerprint, fixture.CheckpointRequestFingerprint, fixture.CheckpointReceiptFingerprint, fixture.RemoteIdentity} {
		if !backlogFingerprint.MatchString(value) {
			return rejectBacklog("checkout_publication_identity_invalid", "publication fingerprint binding is invalid")
		}
	}
	request, err := infrastructure.FinalizeControlledPublicationRequest(infrastructure.ControlledPublicationRequest{
		ContractVersion: infrastructure.ControlledPublicationRequestContract, RequestID: fixture.ApprovalID,
		AuthorizationFingerprint: fixture.CheckpointApprovalFingerprint, CheckpointRequestFingerprint: fixture.CheckpointRequestFingerprint,
		CheckpointReceiptFingerprint: fixture.CheckpointReceiptFingerprint, SessionID: fixture.RuntimeSessionID,
		WorkspaceID: fixture.RuntimeWorkspaceID, ExpectedBranch: fixture.RuntimeSessionBranch, SourceCommit: fixture.SourceCommit,
		SourceParent: fixture.SourceParent, RemoteName: fixture.RemoteName, RemoteIdentity: fixture.RemoteIdentity,
		DestinationRef: fixture.DestinationRef, PublicationScope: infrastructure.ControlledPublicationScope, Reason: fixture.PublicationReason,
	})
	if err != nil || request.RequestFingerprint == "" {
		return rejectBacklog("checkout_publication_destination_invalid", "publication remote, destination ref, reason, or runtime binding is invalid: %v", err)
	}
	return nil
}

func backlogCheckoutPublicationFixtureForChain(chain *backlogCheckoutPublicationChain, approvalID, replayIdentity, decision, remoteName, remoteIdentity, destinationRef, reason string) backlogCheckoutPublicationFixture {
	return backlogCheckoutPublicationFixture{
		ContractVersion: backlogCheckoutPublicationFixtureContract, ApprovalID: approvalID, ReplayIdentity: replayIdentity,
		Decision: decision, PublicationScope: backlogCheckoutPublicationScope,
		TaskID: stringValue(mapValue(chain.CheckpointChain.Application["binding"])["task_id"]), ImmutableChainFingerprint: chain.ImmutableChainFP,
		CheckpointApprovalFingerprint: chain.CheckpointApprovalFP, CheckpointRequestFingerprint: chain.CheckpointRequest.RequestFingerprint,
		CheckpointReceiptFingerprint: chain.CheckpointReceiptFP, RuntimeSessionID: chain.CheckpointReceipt.SessionID,
		RuntimeWorkspaceID: chain.CheckpointReceipt.WorkspaceID, RuntimeSessionBranch: chain.CheckpointReceipt.Branch,
		SourceCommit: chain.CheckpointReceipt.Commit, SourceParent: chain.CheckpointReceipt.Parent,
		RemoteName: remoteName, RemoteIdentity: remoteIdentity, DestinationRef: destinationRef, PublicationReason: reason,
	}
}

func backlogCheckoutPublicationApprovalPayload(chain *backlogCheckoutPublicationChain, fixture *backlogCheckoutPublicationFixture) (map[string]any, error) {
	affirmative := fixture.Decision == backlogSemanticReviewApproved
	payload := map[string]any{
		"contract_version": backlogCheckoutPublicationApprovalContract, "state": "checkpoint_created",
		"identity":        map[string]any{"approval_id": fixture.ApprovalID, "replay_identity": fixture.ReplayIdentity},
		"decision":        map[string]any{"value": fixture.Decision, "affirmative": affirmative, "explicit_local_decision": true, "publication_scope": backlogCheckoutPublicationScope},
		"immutable_chain": map[string]any{"fingerprint": chain.ImmutableChainFP, "task_id": fixture.TaskID},
		"checkpoint":      map[string]any{"approval_fingerprint": chain.CheckpointApprovalFP, "request_fingerprint": chain.CheckpointRequest.RequestFingerprint, "receipt_fingerprint": chain.CheckpointReceiptFP, "checkpoint_id": chain.CheckpointReceipt.CheckpointID, "commit": chain.CheckpointReceipt.Commit, "parent": chain.CheckpointReceipt.Parent},
		"runtime":         map[string]any{"session_id": fixture.RuntimeSessionID, "workspace_id": fixture.RuntimeWorkspaceID, "session_branch": fixture.RuntimeSessionBranch},
		"destination":     map[string]any{"remote_name": fixture.RemoteName, "remote_identity": fixture.RemoteIdentity, "fully_qualified_ref": fixture.DestinationRef},
		"publication":     map[string]any{"reason": fixture.PublicationReason, "scope": infrastructure.ControlledPublicationScope, "exact_commit_source": true, "one_refspec": true, "force": false, "upstream_configuration": false},
		"source":          map[string]any{"mode": "fixture", "package_owned_metadata": true, "fixture_contract": backlogCheckoutPublicationFixtureContract, "credential_material": false},
		"actions":         map[string]any{"publication_request_emitted": false, "publication_performed": false, "checkpoint_performed": false, "sync_performed": false, "fetch_performed": false, "merge_performed": false, "force_performed": false, "next_task_selected": false},
		"capabilities":    map[string]any{"submit_exact_runtime_publication_request": affirmative, "push": false, "checkpoint": false, "sync": false, "merge": false, "start_another_backlog_item": false},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}

func validateBacklogCheckoutPublicationApproval(payload map[string]any, chain *backlogCheckoutPublicationChain) error {
	identity := mapValue(payload["identity"])
	decision := stringValue(mapValue(payload["decision"])["value"])
	destination := mapValue(payload["destination"])
	publication := mapValue(payload["publication"])
	fixture := backlogCheckoutPublicationFixtureForChain(chain, stringValue(identity["approval_id"]), stringValue(identity["replay_identity"]), decision,
		stringValue(destination["remote_name"]), stringValue(destination["remote_identity"]), stringValue(destination["fully_qualified_ref"]), stringValue(publication["reason"]))
	expected, err := backlogCheckoutPublicationApprovalPayload(chain, &fixture)
	if err != nil || !jsonMapsEqual(payload, expected) {
		return errors.New("publication approval is malformed, tampered, or does not match the checkpoint chain and destination")
	}
	return nil
}
