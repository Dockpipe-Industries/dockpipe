package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
)

const (
	backlogSemanticReviewFixtureContract = "dorkpipe.semantic-review-decision-fixture/v1"
	backlogSemanticReviewContract        = "dorkpipe.semantic-review-decision/v1"
	backlogReadyForReviewContract        = "dorkpipe.ready-for-review/v1"
	backlogSemanticReviewScope           = "semantic_correctness_of_bound_candidate"
	backlogSemanticReviewApproved        = "approved"
	backlogSemanticReviewRejected        = "rejected"
)

type backlogSemanticReviewFixture struct {
	ContractVersion                string   `json:"contract_version"`
	DecisionID                     string   `json:"decision_id"`
	ReplayIdentity                 string   `json:"replay_identity"`
	Decision                       string   `json:"decision"`
	ReviewScope                    string   `json:"review_scope"`
	TaskID                         string   `json:"task_id"`
	RemoteTaskID                   string   `json:"remote_task_id"`
	RequestFingerprint             string   `json:"request_fingerprint"`
	CompatibilityFingerprint       string   `json:"compatibility_fingerprint"`
	DispatchFingerprint            string   `json:"dispatch_fingerprint"`
	AdapterIdentity                string   `json:"adapter_identity"`
	EnvironmentRef                 string   `json:"environment_ref"`
	BranchRef                      string   `json:"branch_ref"`
	BaselineCommit                 string   `json:"baseline_commit"`
	CompletionCandidateID          string   `json:"completion_candidate_id"`
	CompletionCandidateFingerprint string   `json:"completion_candidate_fingerprint"`
	RemoteStatusObservationID      string   `json:"remote_status_observation_id"`
	RemoteStatusFingerprint        string   `json:"remote_status_fingerprint"`
	RemoteDiffObservationID        string   `json:"remote_diff_observation_id"`
	RemoteDiffFingerprint          string   `json:"remote_diff_fingerprint"`
	RemoteResultObservationID      string   `json:"remote_result_observation_id"`
	RemoteResultFingerprint        string   `json:"remote_result_fingerprint"`
	ValidationReceiptObservationID string   `json:"validation_receipt_observation_id"`
	ValidationReceiptFingerprint   string   `json:"validation_receipt_fingerprint"`
	PatchBoundaryFingerprint       string   `json:"patch_boundary_fingerprint"`
	PatchApplicationFingerprint    string   `json:"patch_application_fingerprint"`
	ValidationExecutionFingerprint string   `json:"validation_execution_fingerprint"`
	PatchSHA256                    string   `json:"patch_sha256"`
	PatchBytes                     *int     `json:"patch_bytes"`
	ChangedPaths                   []string `json:"changed_paths"`
	ValidationStatus               string   `json:"validation_status"`
}

type backlogSemanticReviewChain struct {
	Request                map[string]any
	Boundary               map[string]any
	Application            map[string]any
	Execution              map[string]any
	BoundaryFingerprint    string
	ApplicationFingerprint string
	ExecutionFingerprint   string
	ValidationStatus       string
	ChangedPaths           []string
}

func recordBacklogSemanticReviewDecision(artifactRoot, fixturePath string) error {
	chain, err := loadBacklogSemanticReviewChain(artifactRoot)
	if err != nil {
		return rejectBacklog("semantic_review_chain_invalid", "%v", err)
	}
	fixture, err := loadBacklogSemanticReviewFixture(fixturePath)
	if err != nil {
		return err
	}
	if err := validateBacklogSemanticReviewFixture(fixture); err != nil {
		return err
	}
	expected := backlogSemanticReviewFixtureForChain(chain, fixture.DecisionID, fixture.ReplayIdentity, fixture.Decision)
	if !reflect.DeepEqual(*fixture, expected) {
		return rejectBacklog("semantic_review_binding_mismatch", "semantic review decision does not match the exact accepted task, request, compatibility, dispatch, adapter, target, baseline, candidate, status, diff, result, receipt, boundary, application, validation execution, patch, changed paths, validation status, and review scope")
	}
	if fixture.Decision == backlogSemanticReviewApproved && chain.ValidationStatus != "passed" {
		return rejectBacklog("semantic_review_validation_not_passed", "an approved semantic review requires passed validation execution evidence")
	}

	decisionPath, err := backlogArtifactPath(artifactRoot, "semantic-review-decision.json")
	if err != nil {
		return err
	}
	readyPath, err := backlogArtifactPath(artifactRoot, "ready-for-review.json")
	if err != nil {
		return err
	}
	decisionPayload, err := backlogSemanticReviewDecisionPayload(chain, fixture.DecisionID, fixture.ReplayIdentity, fixture.Decision)
	if err != nil {
		return err
	}

	if info, statErr := os.Lstat(decisionPath); statErr == nil {
		if backlogFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
			return rejectBacklog("semantic_review_artifact_invalid", "existing semantic-review-decision.json is not a regular non-link file")
		}
		existing, readErr := readStrictJSONMap(decisionPath)
		if readErr != nil {
			return rejectBacklog("semantic_review_artifact_invalid", "existing semantic-review-decision.json is malformed: %v", readErr)
		}
		if err := validateBacklogSemanticReviewDecision(existing, chain); err != nil {
			return rejectBacklog("semantic_review_artifact_invalid", "%v", err)
		}
		existingIdentity := mapValue(existing["identity"])
		existingDecision := stringValue(mapValue(existing["decision"])["value"])
		switch {
		case jsonMapsEqual(existing, decisionPayload):
			return validateBacklogReadyForReviewState(readyPath, existing, chain)
		case stringValue(existingIdentity["decision_id"]) == fixture.DecisionID && stringValue(existingIdentity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("semantic_review_decision_conflict", "accepted semantic review identity cannot change from %q to %q", existingDecision, fixture.Decision)
		case stringValue(existingIdentity["decision_id"]) == fixture.DecisionID:
			return rejectBacklog("semantic_review_duplicate", "semantic review decision identity %q was already recorded", fixture.DecisionID)
		case stringValue(existingIdentity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("semantic_review_replay", "semantic review replay identity %q was already recorded", fixture.ReplayIdentity)
		default:
			return rejectBacklog("semantic_review_already_recorded", "one semantic review decision is already recorded for the accepted validation execution")
		}
	} else if !os.IsNotExist(statErr) {
		return rejectBacklog("semantic_review_artifact_invalid", "%v", statErr)
	}
	if _, statErr := os.Lstat(readyPath); statErr == nil {
		return rejectBacklog("semantic_review_readiness_invalid", "ready-for-review.json exists without an accepted semantic review decision")
	} else if !os.IsNotExist(statErr) {
		return rejectBacklog("semantic_review_readiness_invalid", "%v", statErr)
	}

	if fixture.Decision == backlogSemanticReviewRejected {
		return writeJSONFileAtomic(decisionPath, decisionPayload)
	}
	readyPayload, err := backlogReadyForReviewPayload(chain, decisionPayload)
	if err != nil {
		return err
	}
	if err := writeJSONFileAtomic(decisionPath, decisionPayload); err != nil {
		return err
	}
	if err := writeJSONFileAtomic(readyPath, readyPayload); err != nil {
		if removeErr := os.Remove(decisionPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("ready-for-review publication failed and decision rollback failed: %v; %w", removeErr, err)
		}
		return err
	}
	return nil
}

func loadBacklogSemanticReviewChain(artifactRoot string) (*backlogSemanticReviewChain, error) {
	boundaryPath, err := requireBacklogRegularArtifact(artifactRoot, "patch-boundary.json")
	if err != nil {
		return nil, err
	}
	applicationPath, err := requireBacklogRegularArtifact(artifactRoot, "patch-application.json")
	if err != nil {
		return nil, err
	}
	executionPath, err := requireBacklogRegularArtifact(artifactRoot, "validation-execution.json")
	if err != nil {
		return nil, err
	}
	if err := verifyBacklogPatchBoundary(artifactRoot); err != nil {
		return nil, err
	}
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return nil, err
	}
	boundary, err := readStrictJSONMap(boundaryPath)
	if err != nil {
		return nil, err
	}
	application, applicationFingerprint, err := loadAndVerifyBacklogPatchApplication(artifactRoot, applicationPath, request, boundary)
	if err != nil {
		return nil, err
	}
	requiredValidation, requiredValidationFingerprint, commands, err := backlogValidationCommands(request)
	if err != nil {
		return nil, err
	}
	execution, err := readStrictJSONMap(executionPath)
	if err != nil {
		return nil, err
	}
	if err := validateBacklogValidationExecution(execution, request, application, applicationFingerprint, requiredValidation, requiredValidationFingerprint, commands); err != nil {
		return nil, err
	}
	boundaryFingerprint, err := backlogJSONFingerprint(boundary)
	if err != nil {
		return nil, err
	}
	executionFingerprint := stringValue(execution["artifact_fingerprint"])
	if !backlogFingerprint.MatchString(executionFingerprint) {
		return nil, errors.New("validation execution canonical fingerprint is missing or malformed")
	}
	changedPaths, err := strictBacklogStringArray(execution["changed_paths"])
	if err != nil || len(changedPaths) == 0 || !sort.StringsAreSorted(changedPaths) {
		return nil, errors.New("validation execution changed paths are missing, malformed, or unsorted")
	}
	validationStatus := stringValue(mapValue(execution["aggregate"])["status"])
	if validationStatus != "passed" && validationStatus != "failed" {
		return nil, errors.New("validation execution aggregate status is invalid")
	}
	return &backlogSemanticReviewChain{
		Request: request, Boundary: boundary, Application: application, Execution: execution,
		BoundaryFingerprint: boundaryFingerprint, ApplicationFingerprint: applicationFingerprint,
		ExecutionFingerprint: executionFingerprint, ValidationStatus: validationStatus, ChangedPaths: changedPaths,
	}, nil
}

func loadBacklogSemanticReviewFixture(path string) (*backlogSemanticReviewFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, rejectBacklog("semantic_review_fixture_missing", "semantic review decision fixture cannot be read: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	fixture := backlogSemanticReviewFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		return nil, rejectBacklog("semantic_review_fixture_malformed", "semantic review decision fixture is malformed: %v", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, rejectBacklog("semantic_review_fixture_malformed", "semantic review decision fixture is malformed: %v", err)
	}
	return &fixture, nil
}

func validateBacklogSemanticReviewFixture(fixture *backlogSemanticReviewFixture) error {
	if fixture.ContractVersion != backlogSemanticReviewFixtureContract ||
		!backlogOpaqueID.MatchString(fixture.DecisionID) || !backlogOpaqueID.MatchString(fixture.ReplayIdentity) ||
		fixture.DecisionID == fixture.ReplayIdentity || !backlogTaskIDPattern.MatchString(fixture.TaskID) ||
		!backlogOpaqueID.MatchString(fixture.RemoteTaskID) || !backlogOpaqueID.MatchString(fixture.AdapterIdentity) ||
		!backlogBaseline.MatchString(fixture.BaselineCommit) {
		return rejectBacklog("semantic_review_identity_invalid", "semantic review contract or bounded identity fields are invalid")
	}
	if fixture.Decision != backlogSemanticReviewApproved && fixture.Decision != backlogSemanticReviewRejected {
		return rejectBacklog("semantic_review_decision_invalid", "semantic review decision must be exactly approved or rejected")
	}
	if fixture.ReviewScope != backlogSemanticReviewScope {
		return rejectBacklog("semantic_review_scope_invalid", "semantic review scope must be exactly %q", backlogSemanticReviewScope)
	}
	if fixture.ValidationStatus != "passed" && fixture.ValidationStatus != "failed" {
		return rejectBacklog("semantic_review_identity_invalid", "semantic review validation status is invalid")
	}
	if err := validateBacklogReference("environment", fixture.EnvironmentRef); err != nil {
		return rejectBacklog("semantic_review_identity_invalid", "%v", err)
	}
	if err := validateBacklogReference("branch", fixture.BranchRef); err != nil {
		return rejectBacklog("semantic_review_identity_invalid", "%v", err)
	}
	for _, value := range []string{
		fixture.RequestFingerprint, fixture.CompatibilityFingerprint, fixture.DispatchFingerprint,
		fixture.CompletionCandidateFingerprint, fixture.RemoteStatusFingerprint, fixture.RemoteDiffFingerprint,
		fixture.RemoteResultFingerprint, fixture.ValidationReceiptFingerprint, fixture.PatchBoundaryFingerprint,
		fixture.PatchApplicationFingerprint, fixture.ValidationExecutionFingerprint, fixture.PatchSHA256,
	} {
		if !backlogFingerprint.MatchString(value) {
			return rejectBacklog("semantic_review_identity_invalid", "semantic review fingerprint binding is invalid")
		}
	}
	for _, value := range []string{
		fixture.CompletionCandidateID, fixture.RemoteStatusObservationID, fixture.RemoteDiffObservationID,
		fixture.RemoteResultObservationID, fixture.ValidationReceiptObservationID,
	} {
		if !backlogOpaqueID.MatchString(value) {
			return rejectBacklog("semantic_review_identity_invalid", "semantic review upstream identity binding is invalid")
		}
	}
	if fixture.PatchBytes == nil || *fixture.PatchBytes < 0 || len(fixture.ChangedPaths) == 0 || !sort.StringsAreSorted(fixture.ChangedPaths) {
		return rejectBacklog("semantic_review_identity_invalid", "semantic review patch byte count or changed paths are invalid")
	}
	for index, path := range fixture.ChangedPaths {
		if err := validateBacklogPatchPath(path); err != nil || (index > 0 && path == fixture.ChangedPaths[index-1]) {
			return rejectBacklog("semantic_review_identity_invalid", "semantic review changed paths are malformed, duplicated, or unsorted")
		}
	}
	return nil
}

func backlogSemanticReviewFixtureForChain(chain *backlogSemanticReviewChain, decisionID, replayIdentity, decision string) backlogSemanticReviewFixture {
	execution := chain.Execution
	binding := mapValue(execution["binding"])
	selection := mapValue(chain.Request["selection"])
	candidate := mapValue(execution["completion_candidate"])
	status := mapValue(execution["remote_status"])
	diff := mapValue(execution["remote_diff"])
	result := mapValue(execution["remote_result"])
	receipt := mapValue(execution["validation_receipt"])
	patch := mapValue(execution["accepted_patch"])
	patchBytes, _ := backlogJSONInt(patch["bytes"])
	return backlogSemanticReviewFixture{
		ContractVersion: backlogSemanticReviewFixtureContract, DecisionID: decisionID, ReplayIdentity: replayIdentity,
		Decision: decision, ReviewScope: backlogSemanticReviewScope, TaskID: stringValue(binding["task_id"]),
		RemoteTaskID: stringValue(binding["remote_task_id"]), RequestFingerprint: stringValue(binding["request_fingerprint"]),
		CompatibilityFingerprint: stringValue(binding["compatibility_fingerprint"]), DispatchFingerprint: stringValue(binding["dispatch_fingerprint"]),
		AdapterIdentity: stringValue(binding["adapter_identity"]), EnvironmentRef: stringValue(binding["environment_ref"]),
		BranchRef: stringValue(binding["branch_ref"]), BaselineCommit: stringValue(selection["baseline_commit"]),
		CompletionCandidateID: stringValue(candidate["candidate_id"]), CompletionCandidateFingerprint: stringValue(candidate["fingerprint"]),
		RemoteStatusObservationID: stringValue(status["observation_id"]), RemoteStatusFingerprint: stringValue(status["fingerprint"]),
		RemoteDiffObservationID: stringValue(diff["observation_id"]), RemoteDiffFingerprint: stringValue(diff["fingerprint"]),
		RemoteResultObservationID: stringValue(result["observation_id"]), RemoteResultFingerprint: stringValue(result["fingerprint"]),
		ValidationReceiptObservationID: stringValue(receipt["observation_id"]), ValidationReceiptFingerprint: stringValue(receipt["fingerprint"]),
		PatchBoundaryFingerprint: chain.BoundaryFingerprint, PatchApplicationFingerprint: chain.ApplicationFingerprint,
		ValidationExecutionFingerprint: chain.ExecutionFingerprint, PatchSHA256: stringValue(patch["sha256"]),
		PatchBytes: &patchBytes, ChangedPaths: append([]string{}, chain.ChangedPaths...), ValidationStatus: chain.ValidationStatus,
	}
}

func backlogSemanticReviewDecisionPayload(chain *backlogSemanticReviewChain, decisionID, replayIdentity, decision string) (map[string]any, error) {
	execution := chain.Execution
	affirmative := decision == backlogSemanticReviewApproved
	payload := map[string]any{
		"contract_version": backlogSemanticReviewContract,
		"state":            "completion_candidate",
		"identity":         map[string]any{"decision_id": decisionID, "replay_identity": replayIdentity},
		"decision": map[string]any{
			"value": decision, "review_scope": backlogSemanticReviewScope, "affirmative": affirmative,
			"explicit_local_decision": true, "validation_status": chain.ValidationStatus,
		},
		"validation_execution": map[string]any{"fingerprint": chain.ExecutionFingerprint, "status": chain.ValidationStatus},
		"patch_application":    map[string]any{"fingerprint": chain.ApplicationFingerprint},
		"patch_boundary":       map[string]any{"fingerprint": chain.BoundaryFingerprint},
		"validation_receipt":   mapValue(execution["validation_receipt"]),
		"remote_result":        mapValue(execution["remote_result"]), "remote_diff": mapValue(execution["remote_diff"]),
		"remote_status": mapValue(execution["remote_status"]), "completion_candidate": mapValue(execution["completion_candidate"]),
		"binding": mapValue(execution["binding"]), "accepted_patch": mapValue(execution["accepted_patch"]),
		"changed_paths": anyStrings(chain.ChangedPaths),
		"source": map[string]any{
			"mode": "fixture", "local_review_evidence": true, "package_owned_metadata": true,
			"fixture_contract": backlogSemanticReviewFixtureContract, "provider_response": false,
			"callback": false, "signed_receipt": false, "hidden_transcript": false, "unbounded_review_prose": false,
		},
		"actions": map[string]any{
			"semantic_correctness_reviewed": true, "validation_reexecuted": false,
			"ready_for_review_emitted":    affirmative && chain.ValidationStatus == "passed",
			"apply_to_checkout_performed": false, "commit_performed": false, "push_performed": false, "publication_performed": false,
		},
		"capabilities": map[string]any{
			"apply_to_checkout": false, "commit": false, "push": false, "publication": false, "start_another_backlog_item": false,
		},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}

func validateBacklogSemanticReviewDecision(payload map[string]any, chain *backlogSemanticReviewChain) error {
	if stringValue(payload["contract_version"]) != backlogSemanticReviewContract || stringValue(payload["state"]) != "completion_candidate" {
		return errors.New("semantic review decision has an unsupported contract or lifecycle state")
	}
	identity := mapValue(payload["identity"])
	decision := mapValue(payload["decision"])
	decisionID := stringValue(identity["decision_id"])
	replayIdentity := stringValue(identity["replay_identity"])
	decisionValue := stringValue(decision["value"])
	if !backlogOpaqueID.MatchString(decisionID) || !backlogOpaqueID.MatchString(replayIdentity) || decisionID == replayIdentity ||
		(decisionValue != backlogSemanticReviewApproved && decisionValue != backlogSemanticReviewRejected) {
		return errors.New("semantic review decision identity or value is malformed")
	}
	if decisionValue == backlogSemanticReviewApproved && chain.ValidationStatus != "passed" {
		return errors.New("approved semantic review is bound to failed validation")
	}
	expected, err := backlogSemanticReviewDecisionPayload(chain, decisionID, replayIdentity, decisionValue)
	if err != nil || !jsonMapsEqual(payload, expected) {
		return errors.New("semantic review decision is malformed, tampered, or does not match the immutable chain")
	}
	return nil
}

func backlogReadyForReviewPayload(chain *backlogSemanticReviewChain, decision map[string]any) (map[string]any, error) {
	decisionIdentity := mapValue(decision["identity"])
	decisionFingerprint := stringValue(decision["artifact_fingerprint"])
	execution := chain.Execution
	payload := map[string]any{
		"contract_version": backlogReadyForReviewContract,
		"state":            "ready_for_review",
		"semantic_review_decision": map[string]any{
			"decision_id": stringValue(decisionIdentity["decision_id"]), "fingerprint": decisionFingerprint,
			"decision": backlogSemanticReviewApproved, "review_scope": backlogSemanticReviewScope,
		},
		"validation_execution": map[string]any{"fingerprint": chain.ExecutionFingerprint, "status": "passed"},
		"patch_application":    map[string]any{"fingerprint": chain.ApplicationFingerprint},
		"patch_boundary":       map[string]any{"fingerprint": chain.BoundaryFingerprint},
		"validation_receipt":   mapValue(execution["validation_receipt"]),
		"remote_result":        mapValue(execution["remote_result"]), "remote_diff": mapValue(execution["remote_diff"]),
		"remote_status": mapValue(execution["remote_status"]), "completion_candidate": mapValue(execution["completion_candidate"]),
		"binding": mapValue(execution["binding"]), "accepted_patch": mapValue(execution["accepted_patch"]),
		"changed_paths": anyStrings(chain.ChangedPaths),
		"actions": map[string]any{
			"ready_for_review_recorded": true, "apply_to_checkout_performed": false, "commit_performed": false,
			"push_performed": false, "publication_performed": false,
		},
		"capabilities": map[string]any{
			"apply_to_checkout": false, "commit": false, "push": false, "publication": false, "start_another_backlog_item": false,
		},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}

func validateBacklogReadyForReviewState(path string, decision map[string]any, chain *backlogSemanticReviewChain) error {
	decisionValue := stringValue(mapValue(decision["decision"])["value"])
	info, statErr := os.Lstat(path)
	if decisionValue == backlogSemanticReviewRejected {
		if statErr == nil {
			return rejectBacklog("semantic_review_readiness_invalid", "rejected semantic review must not have ready-for-review.json")
		}
		if !os.IsNotExist(statErr) {
			return rejectBacklog("semantic_review_readiness_invalid", "%v", statErr)
		}
		return nil
	}
	if statErr != nil {
		return rejectBacklog("semantic_review_readiness_invalid", "approved semantic review is missing ready-for-review.json: %v", statErr)
	}
	if backlogFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
		return rejectBacklog("semantic_review_readiness_invalid", "existing ready-for-review.json is not a regular non-link file")
	}
	ready, err := readStrictJSONMap(path)
	if err != nil {
		return rejectBacklog("semantic_review_readiness_invalid", "existing ready-for-review.json is malformed: %v", err)
	}
	expected, err := backlogReadyForReviewPayload(chain, decision)
	if err != nil || !jsonMapsEqual(ready, expected) {
		return rejectBacklog("semantic_review_readiness_invalid", "ready-for-review.json is malformed, tampered, or does not match the accepted semantic review decision")
	}
	return nil
}
