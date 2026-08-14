package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const (
	backlogCheckoutApplicationFixtureContract  = "dorkpipe.checkout-application-approval-fixture/v1"
	backlogCheckoutApplicationApprovalContract = "dorkpipe.checkout-application-approval/v1"
	backlogCheckoutApplicationContract         = "dorkpipe.checkout-application/v1"
	backlogCheckoutApplicationScope            = "accepted_changed_paths_exact_patch_once"
)

var (
	backlogCheckoutCreateTemp    = os.CreateTemp
	backlogCheckoutRename        = os.Rename
	backlogCheckoutRemove        = os.Remove
	backlogCheckoutAfterMutation = func(int) error { return nil }
)

type backlogCheckoutApplicationFixture struct {
	ContractVersion                string   `json:"contract_version"`
	ApprovalID                     string   `json:"approval_id"`
	ReplayIdentity                 string   `json:"replay_identity"`
	Decision                       string   `json:"decision"`
	ApplicationScope               string   `json:"application_scope"`
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
	SemanticReviewDecisionID       string   `json:"semantic_review_decision_id"`
	SemanticReviewFingerprint      string   `json:"semantic_review_fingerprint"`
	ReadinessFingerprint           string   `json:"readiness_fingerprint"`
	PatchSHA256                    string   `json:"patch_sha256"`
	PatchBytes                     *int     `json:"patch_bytes"`
	ChangedPaths                   []string `json:"changed_paths"`
	ConsumerPreimageFingerprint    string   `json:"consumer_preimage_fingerprint"`
}

type backlogCheckoutApplicationChain struct {
	Semantic             *backlogSemanticReviewChain
	Decision             map[string]any
	Ready                map[string]any
	DecisionFingerprint  string
	ReadyFingerprint     string
	PreimageManifest     map[string]any
	PostimageManifest    map[string]any
	PreimageFingerprint  string
	PostimageFingerprint string
}

type backlogCheckoutFile struct {
	Path       string
	Target     string
	Preimage   []byte
	Postimage  []byte
	Mode       os.FileMode
	StagedPath string
	BackupPath string
	Mutated    bool
}

type backlogCheckoutManifestEntry struct {
	SHA256 string
	Bytes  int
}

func applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixturePath string) error {
	chain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
	if err != nil {
		return rejectBacklog("checkout_application_chain_invalid", "%v", err)
	}
	fixture, err := loadBacklogCheckoutApplicationFixture(fixturePath)
	if err != nil {
		return err
	}
	if err := validateBacklogCheckoutApplicationFixture(fixture); err != nil {
		return err
	}
	expected := backlogCheckoutApplicationFixtureForChain(chain, fixture.ApprovalID, fixture.ReplayIdentity, fixture.Decision)
	if !reflect.DeepEqual(*fixture, expected) {
		return rejectBacklog("checkout_application_binding_mismatch", "checkout application decision does not match the exact readiness, semantic decision, validation execution, application, boundary, receipt, result, diff, status, candidate, task, request, compatibility, dispatch, adapter, target, baseline, patch, changed paths, consumer preimages, and application scope")
	}

	approvalPath, err := backlogArtifactPath(artifactRoot, "checkout-application-approval.json")
	if err != nil {
		return err
	}
	receiptPath, err := backlogArtifactPath(artifactRoot, "checkout-application.json")
	if err != nil {
		return err
	}
	approval, err := backlogCheckoutApplicationApprovalPayload(chain, fixture.ApprovalID, fixture.ReplayIdentity, fixture.Decision)
	if err != nil {
		return err
	}

	if existing, exists, loadErr := loadExistingBacklogCheckoutArtifact(approvalPath, "checkout application approval"); loadErr != nil {
		return loadErr
	} else if exists {
		if err := validateBacklogCheckoutApplicationApproval(existing, chain); err != nil {
			return rejectBacklog("checkout_application_approval_artifact_invalid", "%v", err)
		}
		existingIdentity := mapValue(existing["identity"])
		existingDecision := stringValue(mapValue(existing["decision"])["value"])
		switch {
		case jsonMapsEqual(existing, approval):
			approval = existing
		case stringValue(existingIdentity["approval_id"]) == fixture.ApprovalID && stringValue(existingIdentity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("checkout_application_decision_conflict", "accepted checkout application identity cannot change from %q to %q", existingDecision, fixture.Decision)
		case stringValue(existingIdentity["approval_id"]) == fixture.ApprovalID:
			return rejectBacklog("checkout_application_duplicate", "checkout application approval identity %q was already recorded", fixture.ApprovalID)
		case stringValue(existingIdentity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("checkout_application_replay", "checkout application replay identity %q was already recorded", fixture.ReplayIdentity)
		default:
			return rejectBacklog("checkout_application_already_recorded", "one checkout application decision is already recorded for the accepted readiness")
		}
	} else {
		if _, exists, loadErr := loadExistingBacklogCheckoutArtifact(receiptPath, "checkout application receipt"); loadErr != nil {
			return loadErr
		} else if exists {
			return rejectBacklog("checkout_application_artifact_invalid", "checkout-application.json exists without an accepted checkout application approval")
		}
		if err := writeJSONFileAtomic(approvalPath, approval); err != nil {
			return err
		}
	}

	if fixture.Decision == backlogSemanticReviewRejected {
		if _, exists, loadErr := loadExistingBacklogCheckoutArtifact(receiptPath, "checkout application receipt"); loadErr != nil {
			return loadErr
		} else if exists {
			return rejectBacklog("checkout_application_artifact_invalid", "rejected checkout application approval must not have a successful receipt")
		}
		return nil
	}

	root, err := validateBacklogConsumerRoot(consumerRoot)
	if err != nil {
		return rejectBacklog("checkout_application_source_invalid", "%v", err)
	}
	files, state, hunkCount, err := prepareBacklogCheckoutFiles(root, artifactRoot, chain)
	if err != nil {
		return err
	}
	receipt, err := backlogCheckoutApplicationPayload(chain, approval, len(files), hunkCount)
	if err != nil {
		return err
	}
	if existing, exists, loadErr := loadExistingBacklogCheckoutArtifact(receiptPath, "checkout application receipt"); loadErr != nil {
		return loadErr
	} else if exists {
		if !jsonMapsEqual(existing, receipt) {
			return rejectBacklog("checkout_application_artifact_invalid", "existing checkout-application.json is malformed, tampered, or does not match the accepted approval")
		}
		if state != "postimage" {
			return rejectBacklog("checkout_application_postimage_mismatch", "existing receipt requires every accepted consumer file to match the exact postimage manifest")
		}
		return nil
	}

	switch state {
	case "postimage":
		return writeJSONFileAtomic(receiptPath, receipt)
	case "preimage":
		if err := stageBacklogCheckoutFiles(files); err != nil {
			return err
		}
		if err := mutateBacklogCheckoutFiles(files); err != nil {
			return err
		}
		if err := verifyBacklogCheckoutFiles(root, files, true); err != nil {
			return failAndRollbackBacklogCheckout(files, "checkout_application_postimage_mismatch", err)
		}
		if err := cleanupBacklogCheckoutFiles(files); err != nil {
			return failAndRollbackBacklogCheckout(files, "checkout_application_cleanup_failed", err)
		}
		if err := verifyBacklogCheckoutFiles(root, files, true); err != nil {
			return failAndRollbackBacklogCheckout(files, "checkout_application_postimage_mismatch", err)
		}
		if err := writeJSONFileAtomic(receiptPath, receipt); err != nil {
			return failAndRollbackBacklogCheckout(files, "checkout_application_receipt_failed", err)
		}
		return nil
	default:
		return rejectBacklog("checkout_application_mixed_state", "consumer changed paths are mixed, stale, unexpected, or match neither the complete preimage nor complete postimage manifest")
	}
}

func loadBacklogCheckoutApplicationChain(artifactRoot string) (*backlogCheckoutApplicationChain, error) {
	semantic, err := loadBacklogSemanticReviewChain(artifactRoot)
	if err != nil {
		return nil, err
	}
	decisionPath, err := requireBacklogRegularArtifact(artifactRoot, "semantic-review-decision.json")
	if err != nil {
		return nil, err
	}
	readyPath, err := requireBacklogRegularArtifact(artifactRoot, "ready-for-review.json")
	if err != nil {
		return nil, err
	}
	decision, err := readStrictJSONMap(decisionPath)
	if err != nil {
		return nil, err
	}
	if err := validateBacklogSemanticReviewDecision(decision, semantic); err != nil {
		return nil, err
	}
	if stringValue(mapValue(decision["decision"])["value"]) != backlogSemanticReviewApproved {
		return nil, errors.New("checkout application requires an approved semantic review decision")
	}
	if err := validateBacklogReadyForReviewState(readyPath, decision, semantic); err != nil {
		return nil, err
	}
	ready, err := readStrictJSONMap(readyPath)
	if err != nil {
		return nil, err
	}
	decisionFingerprint := stringValue(decision["artifact_fingerprint"])
	readyFingerprint := stringValue(ready["artifact_fingerprint"])
	preimage := mapValue(semantic.Application["preimage_manifest"])
	postimage := mapValue(semantic.Application["postimage_manifest"])
	preimageFingerprint := stringValue(preimage["fingerprint"])
	postimageFingerprint := stringValue(postimage["fingerprint"])
	for _, value := range []string{decisionFingerprint, readyFingerprint, preimageFingerprint, postimageFingerprint} {
		if !backlogFingerprint.MatchString(value) {
			return nil, errors.New("checkout application chain has a missing or malformed canonical fingerprint")
		}
	}
	return &backlogCheckoutApplicationChain{
		Semantic: semantic, Decision: decision, Ready: ready, DecisionFingerprint: decisionFingerprint,
		ReadyFingerprint: readyFingerprint, PreimageManifest: preimage, PostimageManifest: postimage,
		PreimageFingerprint: preimageFingerprint, PostimageFingerprint: postimageFingerprint,
	}, nil
}

func loadBacklogCheckoutApplicationFixture(path string) (*backlogCheckoutApplicationFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, rejectBacklog("checkout_application_fixture_missing", "checkout application approval fixture cannot be read: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	fixture := backlogCheckoutApplicationFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		return nil, rejectBacklog("checkout_application_fixture_malformed", "checkout application approval fixture is malformed: %v", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, rejectBacklog("checkout_application_fixture_malformed", "checkout application approval fixture is malformed: %v", err)
	}
	return &fixture, nil
}

func validateBacklogCheckoutApplicationFixture(fixture *backlogCheckoutApplicationFixture) error {
	if fixture.ContractVersion != backlogCheckoutApplicationFixtureContract ||
		!backlogOpaqueID.MatchString(fixture.ApprovalID) || !backlogOpaqueID.MatchString(fixture.ReplayIdentity) ||
		fixture.ApprovalID == fixture.ReplayIdentity || !backlogTaskIDPattern.MatchString(fixture.TaskID) ||
		!backlogOpaqueID.MatchString(fixture.RemoteTaskID) || !backlogOpaqueID.MatchString(fixture.AdapterIdentity) ||
		!backlogOpaqueID.MatchString(fixture.SemanticReviewDecisionID) || !backlogBaseline.MatchString(fixture.BaselineCommit) {
		return rejectBacklog("checkout_application_identity_invalid", "checkout application contract or bounded identity fields are invalid")
	}
	if fixture.Decision != backlogSemanticReviewApproved && fixture.Decision != backlogSemanticReviewRejected {
		return rejectBacklog("checkout_application_decision_invalid", "checkout application decision must be exactly approved or rejected")
	}
	if fixture.ApplicationScope != backlogCheckoutApplicationScope {
		return rejectBacklog("checkout_application_scope_invalid", "checkout application scope must be exactly %q", backlogCheckoutApplicationScope)
	}
	if err := validateBacklogReference("environment", fixture.EnvironmentRef); err != nil {
		return rejectBacklog("checkout_application_identity_invalid", "%v", err)
	}
	if err := validateBacklogReference("branch", fixture.BranchRef); err != nil {
		return rejectBacklog("checkout_application_identity_invalid", "%v", err)
	}
	for _, value := range []string{
		fixture.RequestFingerprint, fixture.CompatibilityFingerprint, fixture.DispatchFingerprint,
		fixture.CompletionCandidateFingerprint, fixture.RemoteStatusFingerprint, fixture.RemoteDiffFingerprint,
		fixture.RemoteResultFingerprint, fixture.ValidationReceiptFingerprint, fixture.PatchBoundaryFingerprint,
		fixture.PatchApplicationFingerprint, fixture.ValidationExecutionFingerprint, fixture.SemanticReviewFingerprint,
		fixture.ReadinessFingerprint, fixture.PatchSHA256, fixture.ConsumerPreimageFingerprint,
	} {
		if !backlogFingerprint.MatchString(value) {
			return rejectBacklog("checkout_application_identity_invalid", "checkout application fingerprint binding is invalid")
		}
	}
	for _, value := range []string{
		fixture.CompletionCandidateID, fixture.RemoteStatusObservationID, fixture.RemoteDiffObservationID,
		fixture.RemoteResultObservationID, fixture.ValidationReceiptObservationID,
	} {
		if !backlogOpaqueID.MatchString(value) {
			return rejectBacklog("checkout_application_identity_invalid", "checkout application upstream identity binding is invalid")
		}
	}
	if fixture.PatchBytes == nil || *fixture.PatchBytes < 0 || len(fixture.ChangedPaths) == 0 || !sort.StringsAreSorted(fixture.ChangedPaths) {
		return rejectBacklog("checkout_application_identity_invalid", "checkout application patch byte count or changed paths are invalid")
	}
	for index, path := range fixture.ChangedPaths {
		if err := validateBacklogPatchPath(path); err != nil || (index > 0 && path == fixture.ChangedPaths[index-1]) {
			return rejectBacklog("checkout_application_identity_invalid", "checkout application changed paths are malformed, duplicated, or unsorted")
		}
	}
	return nil
}

func backlogCheckoutApplicationFixtureForChain(chain *backlogCheckoutApplicationChain, approvalID, replayIdentity, decision string) backlogCheckoutApplicationFixture {
	execution := chain.Semantic.Execution
	binding := mapValue(execution["binding"])
	selection := mapValue(chain.Semantic.Request["selection"])
	candidate := mapValue(execution["completion_candidate"])
	status := mapValue(execution["remote_status"])
	diff := mapValue(execution["remote_diff"])
	result := mapValue(execution["remote_result"])
	receipt := mapValue(execution["validation_receipt"])
	patch := mapValue(execution["accepted_patch"])
	patchBytes, _ := backlogJSONInt(patch["bytes"])
	decisionIdentity := mapValue(chain.Decision["identity"])
	return backlogCheckoutApplicationFixture{
		ContractVersion: backlogCheckoutApplicationFixtureContract, ApprovalID: approvalID, ReplayIdentity: replayIdentity,
		Decision: decision, ApplicationScope: backlogCheckoutApplicationScope, TaskID: stringValue(binding["task_id"]),
		RemoteTaskID: stringValue(binding["remote_task_id"]), RequestFingerprint: stringValue(binding["request_fingerprint"]),
		CompatibilityFingerprint: stringValue(binding["compatibility_fingerprint"]), DispatchFingerprint: stringValue(binding["dispatch_fingerprint"]),
		AdapterIdentity: stringValue(binding["adapter_identity"]), EnvironmentRef: stringValue(binding["environment_ref"]),
		BranchRef: stringValue(binding["branch_ref"]), BaselineCommit: stringValue(selection["baseline_commit"]),
		CompletionCandidateID: stringValue(candidate["candidate_id"]), CompletionCandidateFingerprint: stringValue(candidate["fingerprint"]),
		RemoteStatusObservationID: stringValue(status["observation_id"]), RemoteStatusFingerprint: stringValue(status["fingerprint"]),
		RemoteDiffObservationID: stringValue(diff["observation_id"]), RemoteDiffFingerprint: stringValue(diff["fingerprint"]),
		RemoteResultObservationID: stringValue(result["observation_id"]), RemoteResultFingerprint: stringValue(result["fingerprint"]),
		ValidationReceiptObservationID: stringValue(receipt["observation_id"]), ValidationReceiptFingerprint: stringValue(receipt["fingerprint"]),
		PatchBoundaryFingerprint: chain.Semantic.BoundaryFingerprint, PatchApplicationFingerprint: chain.Semantic.ApplicationFingerprint,
		ValidationExecutionFingerprint: chain.Semantic.ExecutionFingerprint,
		SemanticReviewDecisionID:       stringValue(decisionIdentity["decision_id"]), SemanticReviewFingerprint: chain.DecisionFingerprint,
		ReadinessFingerprint: chain.ReadyFingerprint, PatchSHA256: stringValue(patch["sha256"]), PatchBytes: &patchBytes,
		ChangedPaths: append([]string{}, chain.Semantic.ChangedPaths...), ConsumerPreimageFingerprint: chain.PreimageFingerprint,
	}
}

func backlogCheckoutApplicationApprovalPayload(chain *backlogCheckoutApplicationChain, approvalID, replayIdentity, decision string) (map[string]any, error) {
	affirmative := decision == backlogSemanticReviewApproved
	ready := chain.Ready
	payload := map[string]any{
		"contract_version":         backlogCheckoutApplicationApprovalContract,
		"state":                    "ready_for_review",
		"identity":                 map[string]any{"approval_id": approvalID, "replay_identity": replayIdentity},
		"decision":                 map[string]any{"value": decision, "affirmative": affirmative, "explicit_local_decision": true, "application_scope": backlogCheckoutApplicationScope},
		"readiness":                map[string]any{"fingerprint": chain.ReadyFingerprint},
		"semantic_review_decision": mapValue(ready["semantic_review_decision"]),
		"validation_execution":     mapValue(ready["validation_execution"]), "patch_application": mapValue(ready["patch_application"]),
		"patch_boundary": mapValue(ready["patch_boundary"]), "validation_receipt": mapValue(ready["validation_receipt"]),
		"remote_result": mapValue(ready["remote_result"]), "remote_diff": mapValue(ready["remote_diff"]),
		"remote_status": mapValue(ready["remote_status"]), "completion_candidate": mapValue(ready["completion_candidate"]),
		"binding": mapValue(ready["binding"]), "accepted_patch": mapValue(ready["accepted_patch"]),
		"changed_paths": anyStrings(chain.Semantic.ChangedPaths), "consumer_preimage_manifest": chain.PreimageManifest,
		"source": map[string]any{"mode": "fixture", "package_owned_metadata": true, "fixture_contract": backlogCheckoutApplicationFixtureContract, "unbounded_approval_prose": false},
		"capabilities": map[string]any{
			"apply_exact_patch_once": affirmative, "commit": false, "push": false, "publication": false,
			"checkpoint": false, "sync": false, "start_another_backlog_item": false,
		},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}

func validateBacklogCheckoutApplicationApproval(payload map[string]any, chain *backlogCheckoutApplicationChain) error {
	if stringValue(payload["contract_version"]) != backlogCheckoutApplicationApprovalContract || stringValue(payload["state"]) != "ready_for_review" {
		return errors.New("checkout application approval has an unsupported contract or state")
	}
	identity := mapValue(payload["identity"])
	decision := stringValue(mapValue(payload["decision"])["value"])
	approvalID := stringValue(identity["approval_id"])
	replayIdentity := stringValue(identity["replay_identity"])
	if !backlogOpaqueID.MatchString(approvalID) || !backlogOpaqueID.MatchString(replayIdentity) || approvalID == replayIdentity ||
		(decision != backlogSemanticReviewApproved && decision != backlogSemanticReviewRejected) {
		return errors.New("checkout application approval identity or decision is malformed")
	}
	expected, err := backlogCheckoutApplicationApprovalPayload(chain, approvalID, replayIdentity, decision)
	if err != nil || !jsonMapsEqual(payload, expected) {
		return errors.New("checkout application approval is malformed, tampered, or does not match the immutable readiness")
	}
	return nil
}

func loadExistingBacklogCheckoutArtifact(path, label string) (map[string]any, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, rejectBacklog("checkout_application_artifact_invalid", "%s cannot be inspected: %v", label, err)
	}
	if backlogFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
		return nil, false, rejectBacklog("checkout_application_artifact_invalid", "%s is not a regular non-link file", label)
	}
	payload, err := readStrictJSONMap(path)
	if err != nil {
		return nil, false, rejectBacklog("checkout_application_artifact_invalid", "%s is malformed: %v", label, err)
	}
	return payload, true, nil
}

func prepareBacklogCheckoutFiles(root, artifactRoot string, chain *backlogCheckoutApplicationChain) ([]*backlogCheckoutFile, string, int, error) {
	preimages, err := parseBacklogCheckoutManifest(chain.PreimageManifest, chain.Semantic.ChangedPaths)
	if err != nil {
		return nil, "", 0, rejectBacklog("checkout_application_chain_invalid", "preimage manifest is invalid: %v", err)
	}
	postimages, err := parseBacklogCheckoutManifest(chain.PostimageManifest, chain.Semantic.ChangedPaths)
	if err != nil {
		return nil, "", 0, rejectBacklog("checkout_application_chain_invalid", "postimage manifest is invalid: %v", err)
	}
	patchPath, err := requireBacklogRegularArtifact(artifactRoot, "remote-diff.patch")
	if err != nil {
		return nil, "", 0, rejectBacklog("checkout_application_chain_invalid", "%v", err)
	}
	patchRaw, err := os.ReadFile(patchPath)
	if err != nil {
		return nil, "", 0, rejectBacklog("checkout_application_chain_invalid", "%v", err)
	}
	patch, err := parseBacklogApplicationPatch(patchRaw)
	if err != nil {
		return nil, "", 0, rejectBacklog("checkout_application_chain_invalid", "%v", err)
	}
	patchByPath := map[string]backlogApplicationFile{}
	hunkCount := 0
	for _, file := range patch.Files {
		patchByPath[file.Path] = file
		hunkCount += len(file.Hunks)
	}
	files := make([]*backlogCheckoutFile, 0, len(chain.Semantic.ChangedPaths))
	preCount, postCount := 0, 0
	for _, path := range chain.Semantic.ChangedPaths {
		target := filepath.Join(root, filepath.FromSlash(path))
		raw, readErr := readBacklogConsumerSource(root, path)
		if readErr != nil {
			return nil, "", 0, rejectBacklog("checkout_application_source_invalid", "%v", readErr)
		}
		info, statErr := os.Lstat(target)
		if statErr != nil {
			return nil, "", 0, rejectBacklog("checkout_application_source_invalid", "%v", statErr)
		}
		file := &backlogCheckoutFile{Path: path, Target: target, Mode: info.Mode().Perm()}
		if backlogCheckoutBytesMatch(raw, preimages[path]) {
			file.Preimage = append([]byte{}, raw...)
			postimage, applyErr := applyBacklogApplicationFile(raw, patchByPath[path])
			if applyErr != nil || !backlogCheckoutBytesMatch(postimage, postimages[path]) {
				return nil, "", 0, rejectBacklog("checkout_application_chain_invalid", "patch-derived postimage for %q does not match the accepted postimage manifest", path)
			}
			file.Postimage = postimage
			preCount++
		} else if backlogCheckoutBytesMatch(raw, postimages[path]) {
			file.Postimage = append([]byte{}, raw...)
			postCount++
		} else {
			files = append(files, file)
			continue
		}
		files = append(files, file)
	}
	state := "mixed"
	if preCount == len(files) {
		state = "preimage"
	} else if postCount == len(files) {
		state = "postimage"
	}
	return files, state, hunkCount, nil
}

func parseBacklogCheckoutManifest(manifest map[string]any, changedPaths []string) (map[string]backlogCheckoutManifestEntry, error) {
	files := listValue(manifest["files"])
	if len(files) != len(changedPaths) {
		return nil, errors.New("manifest file count differs from changed paths")
	}
	entries := make(map[string]backlogCheckoutManifestEntry, len(files))
	canonicalFiles := make([]any, 0, len(files))
	for index, value := range files {
		entry := mapValue(value)
		path := stringValue(entry["path"])
		sha := stringValue(entry["sha256"])
		byteCount, ok := backlogJSONInt(entry["bytes"])
		if !ok || index >= len(changedPaths) || path != changedPaths[index] || !backlogFingerprint.MatchString(sha) || byteCount < 0 {
			return nil, errors.New("manifest entry is malformed, duplicated, or not ordered by changed paths")
		}
		entries[path] = backlogCheckoutManifestEntry{SHA256: sha, Bytes: byteCount}
		canonicalFiles = append(canonicalFiles, map[string]any{"path": path, "sha256": sha, "bytes": byteCount})
	}
	expectedFingerprint, err := backlogJSONFingerprint(map[string]any{"files": canonicalFiles})
	if err != nil || stringValue(manifest["fingerprint"]) != expectedFingerprint {
		return nil, errors.New("manifest fingerprint is invalid")
	}
	return entries, nil
}

func backlogCheckoutBytesMatch(raw []byte, expected backlogCheckoutManifestEntry) bool {
	return len(raw) == expected.Bytes && sha256String(raw) == expected.SHA256
}

func stageBacklogCheckoutFiles(files []*backlogCheckoutFile) error {
	for _, file := range files {
		staged, err := writeBacklogCheckoutTemporary(file.Target, ".dorkpipe-apply-*", file.Postimage, file.Mode)
		if err != nil {
			if cleanupErr := cleanupBacklogCheckoutFiles(files); cleanupErr != nil {
				return rejectBacklog("checkout_application_cleanup_failed", "%v; preparation failure: %v", cleanupErr, err)
			}
			return rejectBacklog("checkout_application_prepare_failed", "postimage for %q cannot be staged: %v", file.Path, err)
		}
		file.StagedPath = staged
		backup, err := writeBacklogCheckoutTemporary(file.Target, ".dorkpipe-rollback-*", file.Preimage, file.Mode)
		if err != nil {
			if cleanupErr := cleanupBacklogCheckoutFiles(files); cleanupErr != nil {
				return rejectBacklog("checkout_application_cleanup_failed", "%v; preparation failure: %v", cleanupErr, err)
			}
			return rejectBacklog("checkout_application_prepare_failed", "rollback image for %q cannot be staged: %v", file.Path, err)
		}
		file.BackupPath = backup
	}
	return nil
}

func writeBacklogCheckoutTemporary(target, pattern string, raw []byte, mode os.FileMode) (string, error) {
	temporary, err := backlogCheckoutCreateTemp(filepath.Dir(target), pattern)
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	ok := false
	defer func() {
		_ = temporary.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := temporary.Write(raw); err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Chmod(mode); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func mutateBacklogCheckoutFiles(files []*backlogCheckoutFile) error {
	for index, file := range files {
		if err := backlogCheckoutRename(file.StagedPath, file.Target); err != nil {
			return failAndRollbackBacklogCheckout(files, "checkout_application_mutation_failed", fmt.Errorf("postimage replacement for %q failed: %w", file.Path, err))
		}
		file.StagedPath = ""
		file.Mutated = true
		if err := backlogCheckoutAfterMutation(index + 1); err != nil {
			return failAndRollbackBacklogCheckout(files, "checkout_application_mutation_failed", err)
		}
	}
	return nil
}

func failAndRollbackBacklogCheckout(files []*backlogCheckoutFile, code string, cause error) error {
	rollbackErr := rollbackBacklogCheckoutFiles(files)
	cleanupErr := cleanupBacklogCheckoutFiles(files)
	if rollbackErr != nil {
		return rejectBacklog("checkout_application_rollback_failed", "%v; original failure: %v", rollbackErr, cause)
	}
	if cleanupErr != nil {
		return rejectBacklog("checkout_application_cleanup_failed", "%v; original failure: %v", cleanupErr, cause)
	}
	return rejectBacklog(code, "%v; every changed consumer file was restored to its exact preimage", cause)
}

func rollbackBacklogCheckoutFiles(files []*backlogCheckoutFile) error {
	var failures []error
	for index := len(files) - 1; index >= 0; index-- {
		file := files[index]
		if !file.Mutated {
			continue
		}
		restore, err := writeBacklogCheckoutTemporary(file.Target, ".dorkpipe-restore-*", file.Preimage, file.Mode)
		if err == nil {
			err = backlogCheckoutRename(restore, file.Target)
		}
		if err != nil {
			if restore != "" {
				_ = os.Remove(restore)
			}
			failures = append(failures, fmt.Errorf("restore %q: %w", file.Path, err))
			continue
		}
		file.Mutated = false
		raw, verifyErr := os.ReadFile(file.Target)
		if verifyErr != nil || !bytes.Equal(raw, file.Preimage) {
			failures = append(failures, fmt.Errorf("restored preimage verification failed for %q", file.Path))
		}
	}
	return errors.Join(failures...)
}

func cleanupBacklogCheckoutFiles(files []*backlogCheckoutFile) error {
	var failures []error
	for _, file := range files {
		paths := []*string{&file.StagedPath, &file.BackupPath}
		for _, pathPointer := range paths {
			path := *pathPointer
			if path == "" {
				continue
			}
			if err := backlogCheckoutRemove(path); err != nil && !os.IsNotExist(err) {
				failures = append(failures, err)
				continue
			}
			*pathPointer = ""
		}
	}
	return errors.Join(failures...)
}

func verifyBacklogCheckoutFiles(root string, files []*backlogCheckoutFile, postimage bool) error {
	for _, file := range files {
		raw, err := readBacklogConsumerSource(root, file.Path)
		if err != nil {
			return err
		}
		expected := file.Preimage
		if postimage {
			expected = file.Postimage
		}
		if !bytes.Equal(raw, expected) {
			return fmt.Errorf("consumer file %q does not match the exact expected image", file.Path)
		}
	}
	return nil
}

func backlogCheckoutApplicationPayload(chain *backlogCheckoutApplicationChain, approval map[string]any, fileCount, hunkCount int) (map[string]any, error) {
	ready := chain.Ready
	payload := map[string]any{
		"contract_version":              backlogCheckoutApplicationContract,
		"state":                         "applied_for_review",
		"checkout_application_approval": map[string]any{"fingerprint": stringValue(approval["artifact_fingerprint"]), "decision": backlogSemanticReviewApproved, "application_scope": backlogCheckoutApplicationScope},
		"readiness":                     map[string]any{"fingerprint": chain.ReadyFingerprint},
		"semantic_review_decision":      mapValue(ready["semantic_review_decision"]),
		"validation_execution":          mapValue(ready["validation_execution"]), "patch_application": mapValue(ready["patch_application"]),
		"patch_boundary": mapValue(ready["patch_boundary"]), "validation_receipt": mapValue(ready["validation_receipt"]),
		"remote_result": mapValue(ready["remote_result"]), "remote_diff": mapValue(ready["remote_diff"]),
		"remote_status": mapValue(ready["remote_status"]), "completion_candidate": mapValue(ready["completion_candidate"]),
		"binding": mapValue(ready["binding"]), "accepted_patch": mapValue(ready["accepted_patch"]),
		"changed_paths":     anyStrings(chain.Semantic.ChangedPaths),
		"preimage_manifest": chain.PreimageManifest, "postimage_manifest": chain.PostimageManifest,
		"application": map[string]any{
			"application_scope": backlogCheckoutApplicationScope, "consumer_checkout_applied": true,
			"consumer_postimages_verified": true, "files_applied": fileCount, "hunks_applied": hunkCount,
			"same_directory_replacements_prepared": true, "rollback_plan_prepared": true,
			"rollback_required": false, "rollback_succeeded": false, "cleanup_succeeded": true, "file_modes_preserved": true,
		},
		"actions": map[string]any{
			"apply_to_checkout_performed": true, "commit_performed": false, "push_performed": false,
			"publication_performed": false, "checkpoint_performed": false, "sync_performed": false, "next_task_selected": false,
		},
		"capabilities": map[string]any{
			"apply_to_checkout": false, "commit": false, "push": false, "publication": false,
			"checkpoint": false, "sync": false, "start_another_backlog_item": false,
		},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}
