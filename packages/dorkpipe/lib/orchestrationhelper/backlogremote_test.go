package orchestrationhelper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const backlogTestBaseline = "0123456789abcdef0123456789abcdef01234567"

const backlogTestPatch = "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\nindex 1111111..2222222 100644\n--- a/packages/dorkpipe/README.md\n+++ b/packages/dorkpipe/README.md\n@@ -1 +1,2 @@\n # Fixture package\n+Untrusted remote fixture change.\n"

func TestBacklogRemoteArtifactsAreDeterministicAndRestartSafe(t *testing.T) {
	repo := writeBacklogTestRepo(t)
	compatibilityFixture := writeBacklogCompatibilityFixture(t)
	fixture := filepath.Join(t.TempDir(), "dispatch.json")
	writeBacklogTestFile(t, fixture, `{
  "contract_version": "dorkpipe.remote-dispatch-fixture/v1",
  "adapter_identity": "codex-cloud-fixture-v1",
  "remote_task_id": "remote_fixture_task_015",
  "submitted_at": "2026-07-19T00:00:00Z"
}`)

	compile := func(root string) {
		t.Helper()
		if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded offline fixture dispatch slice.", backlogTestBaseline, root); err != nil {
			t.Fatal(err)
		}
		if err := compileBacklogRemoteRequest(
			repo, root, "fixture-environment", "js/dev",
			`["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]`,
			`["No live provider invocation","No apply, commit, push, or publication"]`,
			`["go test ./packages/dorkpipe/lib/orchestrationhelper"]`,
			`["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]`,
		); err != nil {
			t.Fatal(err)
		}
		if err := preflightBacklogRemoteCompatibility(root, compatibilityFixture); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, "remote-task.json")); !os.IsNotExist(err) {
			t.Fatalf("compatibility preflight created remote-task.json: %v", err)
		}
		if err := dispatchBacklogFixture(root, fixture); err != nil {
			t.Fatal(err)
		}
		if err := dispatchBacklogFixture(root, fixture); err != nil {
			t.Fatalf("idempotent fixture dispatch failed: %v", err)
		}
	}

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	compile(first)
	compile(second)
	firstCandidate := writeBacklogCompletionFixture(t, first, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	secondCandidate := writeBacklogCompletionFixture(t, second, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	if err := ingestBacklogCompletionCandidate(first, firstCandidate); err != nil {
		t.Fatalf("artifact-only completion candidate ingestion failed: %v", err)
	}
	if err := ingestBacklogCompletionCandidate(second, secondCandidate); err != nil {
		t.Fatalf("second clean completion candidate ingestion failed: %v", err)
	}
	firstStatus := writeBacklogStatusFixture(t, first, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	secondStatus := writeBacklogStatusFixture(t, second, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(first, firstStatus); err != nil {
		t.Fatalf("artifact-only remote status retrieval failed: %v", err)
	}
	if err := retrieveBacklogRemoteStatusFixture(second, secondStatus); err != nil {
		t.Fatalf("second clean remote status retrieval failed: %v", err)
	}
	immutableBeforeDiff := map[string][]byte{}
	for _, name := range []string{"remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json"} {
		immutableBeforeDiff[name] = mustReadFile(t, filepath.Join(first, name))
	}
	firstDiff := writeBacklogDiffFixture(t, first, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	secondDiff := writeBacklogDiffFixture(t, second, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(first, firstDiff); err != nil {
		t.Fatalf("artifact-only remote diff retrieval failed: %v", err)
	}
	if err := retrieveBacklogRemoteDiffFixture(second, secondDiff); err != nil {
		t.Fatalf("second clean remote diff retrieval failed: %v", err)
	}
	for name, before := range immutableBeforeDiff {
		if string(mustReadFile(t, filepath.Join(first, name))) != string(before) {
			t.Fatalf("remote diff retrieval changed immutable artifact %s", name)
		}
	}
	immutableBeforeResult := map[string][]byte{}
	for _, name := range []string{"remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json", "remote-diff.json", "remote-diff.patch"} {
		immutableBeforeResult[name] = mustReadFile(t, filepath.Join(first, name))
	}
	firstResult := writeBacklogResultFixture(t, first, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")
	secondResult := writeBacklogResultFixture(t, second, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")
	if err := retrieveBacklogRemoteResultFixture(first, firstResult); err != nil {
		t.Fatalf("artifact-only remote result retrieval failed: %v", err)
	}
	if err := retrieveBacklogRemoteResultFixture(second, secondResult); err != nil {
		t.Fatalf("second clean remote result retrieval failed: %v", err)
	}
	for name, before := range immutableBeforeResult {
		if string(mustReadFile(t, filepath.Join(first, name))) != string(before) {
			t.Fatalf("remote result retrieval changed immutable artifact %s", name)
		}
	}
	immutableBeforeReceipt := map[string][]byte{}
	for _, name := range []string{"remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json", "remote-diff.json", "remote-diff.patch", "remote-result.json"} {
		immutableBeforeReceipt[name] = mustReadFile(t, filepath.Join(first, name))
	}
	firstReceipt := writeBacklogValidationReceiptFixture(t, first, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")
	secondReceipt := writeBacklogValidationReceiptFixture(t, second, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")
	if err := retrieveBacklogValidationReceiptFixture(first, firstReceipt); err != nil {
		t.Fatalf("artifact-only validation receipt retrieval failed: %v", err)
	}
	if err := retrieveBacklogValidationReceiptFixture(second, secondReceipt); err != nil {
		t.Fatalf("second clean validation receipt retrieval failed: %v", err)
	}
	for name, before := range immutableBeforeReceipt {
		if string(mustReadFile(t, filepath.Join(first, name))) != string(before) {
			t.Fatalf("validation receipt retrieval changed immutable artifact %s", name)
		}
	}
	immutableBeforeBoundary := map[string][]byte{}
	for _, name := range []string{"remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json", "remote-diff.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json"} {
		immutableBeforeBoundary[name] = mustReadFile(t, filepath.Join(first, name))
	}
	if err := verifyBacklogPatchBoundary(first); err != nil {
		t.Fatalf("artifact-only patch-boundary verification failed: %v", err)
	}
	firstBoundary := mustReadFile(t, filepath.Join(first, "patch-boundary.json"))
	if err := verifyBacklogPatchBoundary(first); err != nil {
		t.Fatalf("idempotent patch-boundary verification failed: %v", err)
	}
	if string(mustReadFile(t, filepath.Join(first, "patch-boundary.json"))) != string(firstBoundary) {
		t.Fatal("idempotent patch-boundary verification changed its artifact")
	}
	if err := verifyBacklogPatchBoundary(second); err != nil {
		t.Fatalf("second clean patch-boundary verification failed: %v", err)
	}
	for name, before := range immutableBeforeBoundary {
		if string(mustReadFile(t, filepath.Join(first, name))) != string(before) {
			t.Fatalf("patch-boundary verification changed immutable artifact %s", name)
		}
	}
	for _, name := range []string{"backlog-selection.json", "remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json", "remote-diff.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json", "patch-boundary.json"} {
		firstRaw := mustReadFile(t, filepath.Join(first, name))
		secondRaw := mustReadFile(t, filepath.Join(second, name))
		if string(firstRaw) != string(secondRaw) {
			t.Fatalf("%s is not deterministic", name)
		}
	}
	compatibility := readJSONMap(filepath.Join(first, "remote-adapter-compatibility.json"))
	if stringValue(mapValue(compatibility["compatibility"])["status"]) != "unsupported" || backlogTestBool(compatibility["live_submission_enabled"]) {
		t.Fatalf("unexpected compatibility artifact: %#v", compatibility)
	}
	binding := mapValue(compatibility["request_binding"])
	request := readJSONMap(filepath.Join(first, "remote-request.json"))
	if stringValue(binding["request_fingerprint"]) != stringValue(request["request_fingerprint"]) || !jsonMapsEqual(map[string]any{"environment_ref": binding["environment_ref"], "branch_ref": binding["branch_ref"]}, mapValue(request["target"])) {
		t.Fatalf("compatibility artifact is not bound to the immutable request: %#v", binding)
	}
	task := readJSONMap(filepath.Join(first, "remote-task.json"))
	if backlogTestBool(mapValue(task["adapter"])["provider_invoked"]) {
		t.Fatal("fixture dispatch claims a live provider invocation")
	}
	capabilities := mapValue(task["capabilities"])
	for _, name := range []string{"status", "diff", "result", "apply", "commit", "push", "publication"} {
		if backlogTestBool(capabilities[name]) {
			t.Fatalf("fixture unexpectedly enables %s", name)
		}
	}
	candidate := readJSONMap(filepath.Join(first, "completion-candidate.json"))
	if stringValue(candidate["state"]) != "completion_candidate" || backlogTestBool(mapValue(candidate["source"])["terminal_claim_trusted"]) {
		t.Fatalf("unexpected completion candidate state or trust: %#v", candidate)
	}
	for name, value := range mapValue(candidate["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("completion candidate unexpectedly enables %s", name)
		}
	}
	status := readJSONMap(filepath.Join(first, "remote-status.json"))
	statusEvidence := mapValue(status["evidence"])
	if stringValue(status["state"]) != "completion_candidate" || backlogTestBool(statusEvidence["trusted"]) || backlogTestBool(statusEvidence["authoritative"]) || backlogTestBool(statusEvidence["provider_invoked"]) {
		t.Fatalf("unexpected remote status state, trust, authority, or provider claim: %#v", status)
	}
	if stringValue(statusEvidence["claimed_remote_status"]) != "completed" {
		t.Fatalf("unexpected fixture status evidence: %#v", statusEvidence)
	}
	for name, value := range mapValue(status["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("remote status unexpectedly enables %s", name)
		}
	}
	diffMetadata := readJSONMap(filepath.Join(first, "remote-diff.json"))
	patch := mapValue(diffMetadata["patch"])
	if stringValue(diffMetadata["state"]) != "completion_candidate" || !backlogTestBool(patch["opaque"]) || backlogTestBool(patch["trusted"]) || string(mustReadFile(t, filepath.Join(first, "remote-diff.patch"))) != backlogTestPatch {
		t.Fatalf("unexpected remote diff metadata or patch: %#v", diffMetadata)
	}
	for name, value := range mapValue(diffMetadata["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("remote diff unexpectedly enables %s", name)
		}
	}
	result := readJSONMap(filepath.Join(first, "remote-result.json"))
	resultEvidence := mapValue(result["evidence"])
	resultSource := mapValue(result["source"])
	resultDiff := mapValue(result["remote_diff"])
	if stringValue(result["state"]) != "completion_candidate" || !backlogTestBool(resultEvidence["opaque"]) || backlogTestBool(resultEvidence["trusted"]) || backlogTestBool(resultEvidence["authoritative"]) || backlogTestBool(resultEvidence["interpreted"]) {
		t.Fatalf("unexpected remote result state or evidence trust: %#v", result)
	}
	if stringValue(resultEvidence["opaque_result"]) != "fixture-owned opaque result evidence" || stringValue(resultSource["fixture_contract"]) != backlogResultFixtureContract || !backlogTestBool(resultSource["package_owned_metadata"]) || backlogTestBool(resultSource["provider_invoked"]) || backlogTestBool(resultSource["provider_response"]) || backlogTestBool(resultSource["callback"]) || backlogTestBool(resultSource["signed_receipt"]) {
		t.Fatalf("unexpected remote result evidence or fixture provenance: %#v", result)
	}
	if stringValue(resultDiff["patch_sha256"]) != stringValue(patch["sha256"]) || int(resultDiff["patch_bytes"].(float64)) != len(backlogTestPatch) {
		t.Fatalf("remote result does not bind the accepted patch bytes: %#v", resultDiff)
	}
	for name, value := range mapValue(result["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("remote result unexpectedly enables %s", name)
		}
	}
	receipt := readJSONMap(filepath.Join(first, "validation-receipt.json"))
	receiptEvidence := mapValue(receipt["evidence"])
	receiptSource := mapValue(receipt["source"])
	receiptValidation := mapValue(receipt["request_validation"])
	receiptBinding := mapValue(receipt["binding"])
	if stringValue(receipt["state"]) != "completion_candidate" || !backlogTestBool(receiptEvidence["opaque"]) || backlogTestBool(receiptEvidence["trusted"]) || backlogTestBool(receiptEvidence["authoritative"]) || backlogTestBool(receiptEvidence["interpreted"]) || backlogTestBool(receiptEvidence["validation_success_interpreted"]) {
		t.Fatalf("unexpected validation receipt state or evidence trust: %#v", receipt)
	}
	if stringValue(receiptEvidence["opaque_receipt"]) != "fixture-owned opaque validation receipt evidence" || stringValue(receiptSource["fixture_contract"]) != backlogValidationReceiptFixtureContract || !backlogTestBool(receiptSource["package_owned_metadata"]) || backlogTestBool(receiptSource["provider_invoked"]) || backlogTestBool(receiptSource["provider_response"]) || backlogTestBool(receiptSource["callback"]) || backlogTestBool(receiptSource["signed_receipt"]) || backlogTestBool(receiptSource["hidden_transcript"]) {
		t.Fatalf("unexpected validation receipt evidence or fixture provenance: %#v", receipt)
	}
	if backlogTestBool(receiptValidation["executed"]) || backlogTestBool(receiptValidation["interpreted"]) || !jsonMapsEqual(map[string]any{"required_validation": receiptValidation["required_validation"]}, map[string]any{"required_validation": request["required_validation"]}) {
		t.Fatalf("validation receipt changed or interpreted required validation: %#v", receiptValidation)
	}
	requiredValidationFingerprint, err := backlogRequiredValidationFingerprint(request)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(receiptValidation["fingerprint"]) != requiredValidationFingerprint || stringValue(receiptBinding["compatibility_fingerprint"]) == "" {
		t.Fatalf("validation receipt lacks immutable validation or compatibility binding: %#v", receipt)
	}
	for name, value := range mapValue(receipt["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("validation receipt unexpectedly enables %s", name)
		}
	}
	boundary := readJSONMap(filepath.Join(first, "patch-boundary.json"))
	boundaryScope := mapValue(boundary["scope"])
	boundaryVerification := mapValue(boundary["verification"])
	boundaryActions := mapValue(boundary["actions"])
	if stringValue(boundary["contract_version"]) != backlogPatchBoundaryContract || stringValue(boundary["state"]) != "completion_candidate" {
		t.Fatalf("unexpected patch-boundary contract or state: %#v", boundary)
	}
	if stringValue(boundaryVerification["patch_structure"]) != "verified_ordinary_unified_text_modifications" || stringValue(boundaryVerification["allowed_path_boundary"]) != "verified_segment_aware_lexical_containment" || !backlogTestBool(boundaryVerification["mechanical_only"]) {
		t.Fatalf("unexpected patch-boundary verification scope: %#v", boundaryVerification)
	}
	if stringValue(boundaryScope["matching_rule"]) != "exact_or_true_descendant" || !backlogTestBool(boundaryScope["lexical_only"]) || stringValue(boundaryScope["allowed_paths_fingerprint"]) == "" {
		t.Fatalf("unexpected patch-boundary allowed-path contract: %#v", boundaryScope)
	}
	if !jsonMapsEqual(map[string]any{"allowed_paths": boundaryScope["allowed_paths"]}, map[string]any{"allowed_paths": mapValue(request["scope"])["allowed_paths"]}) || !jsonMapsEqual(map[string]any{"changed_paths": boundary["changed_paths"]}, map[string]any{"changed_paths": []any{"packages/dorkpipe/README.md"}}) {
		t.Fatalf("patch-boundary paths do not match immutable scope and accepted patch: %#v", boundary)
	}
	for name, value := range boundaryActions {
		if backlogTestBool(value) {
			t.Fatalf("patch-boundary unexpectedly performed %s", name)
		}
	}
	for name, value := range mapValue(boundary["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("patch-boundary unexpectedly enables %s", name)
		}
	}
	followup, err := loadBacklogFollowup(first)
	if err != nil {
		t.Fatalf("artifact-only follow-up failed: %v", err)
	}
	if stringValue(followup["remote_task_id"]) != "remote_fixture_task_015" {
		t.Fatalf("unexpected follow-up identity: %#v", followup)
	}
	tamperedTaskPath := filepath.Join(second, "remote-task.json")
	tamperedTask := readJSONMap(tamperedTaskPath)
	tamperedTask["remote_task_id"] = "remote_fixture_task_tampered"
	tamperedRaw, err := json.MarshalIndent(tamperedTask, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeBacklogTestFile(t, tamperedTaskPath, string(tamperedRaw)+"\n")
	if _, err := loadBacklogFollowup(second); err == nil {
		t.Fatal("tampered remote task unexpectedly recovered")
	}
}

func TestBacklogInspectRejectsSelectionFailuresWithoutDispatchArtifact(t *testing.T) {
	validIndex := `schema: 1
description: Fixture open-only backlog.
tasks:
  - id: TASK-015
    topic: Backlog remote fixture
    path: docs/agents/tasks/backlog-driven-remote-tasks.md
maintenance:
  - Keep open-only.
`
	tests := []struct {
		name     string
		index    string
		taskID   string
		slice    string
		baseline string
		taskDoc  string
		wantCode string
	}{
		{name: "absent id", index: validIndex, taskID: "", slice: "Implement the bounded fixture slice.", wantCode: "task_id_required"},
		{name: "malformed id", index: validIndex, taskID: "TASK-15", slice: "Implement the bounded fixture slice.", wantCode: "malformed_task_id"},
		{name: "unknown id", index: validIndex, taskID: "TASK-014", slice: "Implement the bounded fixture slice.", wantCode: "unknown_task_id"},
		{name: "duplicate", index: strings.Replace(validIndex, "maintenance:", "  - id: TASK-015\n    topic: Duplicate\n    path: docs/agents/tasks/backlog-driven-remote-tasks.md\nmaintenance:", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "ambiguous_task_id"},
		{name: "ambiguous linked path", index: strings.Replace(validIndex, "maintenance:", "  - id: TASK-014\n    topic: Same linked task\n    path: docs/agents/tasks/backlog-driven-remote-tasks.md\nmaintenance:", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "ambiguous_linked_task"},
		{name: "malformed entry", index: strings.Replace(validIndex, "    topic: Backlog remote fixture\n", "", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "malformed_index_entry"},
		{name: "missing link", index: strings.Replace(validIndex, "backlog-driven-remote-tasks.md", "missing.md", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "invalid_linked_task"},
		{name: "escaping link", index: strings.Replace(validIndex, "docs/agents/tasks/backlog-driven-remote-tasks.md", "docs/agents/tasks/../../../outside.md", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "malformed_index_entry"},
		{name: "mismatched link", index: validIndex, taskID: "TASK-015", slice: "Implement the bounded fixture slice.", taskDoc: "# TASK-014 Wrong task\n", wantCode: "mismatched_linked_task"},
		{name: "closed path", index: strings.Replace(validIndex, "docs/agents/tasks/backlog-driven-remote-tasks.md", "docs/agents/tasks/closed/backlog-driven-remote-tasks.md", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "task_closed"},
		{name: "blocked fixture", index: strings.Replace(validIndex, "    path: docs/agents/tasks/backlog-driven-remote-tasks.md", "    path: docs/agents/tasks/backlog-driven-remote-tasks.md\n    dispatch_state: blocked", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "task_blocked"},
		{name: "externally active fixture", index: strings.Replace(validIndex, "    path: docs/agents/tasks/backlog-driven-remote-tasks.md", "    path: docs/agents/tasks/backlog-driven-remote-tasks.md\n    dispatch_state: external_active", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "task_externally_active"},
		{name: "empty slice", index: validIndex, taskID: "TASK-015", slice: "", wantCode: "invalid_bounded_slice"},
		{name: "padded slice", index: validIndex, taskID: "TASK-015", slice: " Implement the bounded fixture slice. ", wantCode: "invalid_bounded_slice"},
		{name: "multiline slice", index: validIndex, taskID: "TASK-015", slice: "Implement this slice.\nThen widen it.", wantCode: "invalid_bounded_slice"},
		{name: "invalid baseline", index: validIndex, taskID: "TASK-015", slice: "Implement the bounded fixture slice.", baseline: "not-a-commit", wantCode: "invalid_baseline"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := writeBacklogTestRepo(t)
			writeBacklogTestFile(t, filepath.Join(repo, filepath.FromSlash(backlogIndexPath)), test.index)
			if test.taskDoc != "" {
				writeBacklogTestFile(t, filepath.Join(repo, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md"), test.taskDoc)
			}
			root := filepath.Join(t.TempDir(), "artifacts")
			baseline := test.baseline
			if baseline == "" {
				baseline = backlogTestBaseline
			}
			err := inspectBacklogSelection(repo, backlogIndexPath, test.taskID, test.slice, baseline, root)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			selection := readJSONMap(filepath.Join(root, "backlog-selection.json"))
			if stringValue(mapValue(selection["rejection"])["code"]) != test.wantCode {
				t.Fatalf("rejection artifact = %#v", selection)
			}
			for _, name := range []string{"remote-request.json", "remote-request.md", "remote-task.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected selection left %s", name)
				}
			}
		})
	}
}

func TestBacklogCompatibilityRejectsMalformedContractWithoutDispatchArtifact(t *testing.T) {
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded compatibility preflight slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	fixtureRoot := t.TempDir()
	writeBacklogTestFile(t, filepath.Join(fixtureRoot, "contract.json"), "{}\n")
	if err := preflightBacklogRemoteCompatibility(root, fixtureRoot); err == nil {
		t.Fatal("malformed compatibility contract unexpectedly passed")
	}
	compatibility := readJSONMap(filepath.Join(root, "remote-adapter-compatibility.json"))
	status := mapValue(compatibility["compatibility"])
	if stringValue(status["status"]) != "error" || stringValue(status["reason_code"]) != "invalid_compatibility_fixture" {
		t.Fatalf("unexpected compatibility failure artifact: %#v", compatibility)
	}
	if _, err := os.Stat(filepath.Join(root, "remote-task.json")); !os.IsNotExist(err) {
		t.Fatalf("malformed compatibility contract left remote-task.json: %v", err)
	}
}

func TestBacklogFollowupRejectsTamperedImmutableRequest(t *testing.T) {
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded offline fixture dispatch slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, "remote-request.json")
	request := readJSONMap(requestPath)
	request["target"] = map[string]any{"environment_ref": "tampered", "branch_ref": "js/dev"}
	raw, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeBacklogTestFile(t, requestPath, string(raw)+"\n")
	if _, _, err := loadAndVerifyBacklogRequest(root); err == nil {
		t.Fatal("tampered immutable request unexpectedly validated")
	}
}

func TestBacklogCompletionCandidateRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogCompletionTest(t)
	acceptedFixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := ingestBacklogCompletionCandidate(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	candidatePath := filepath.Join(root, "completion-candidate.json")
	acceptedRaw := mustReadFile(t, candidatePath)
	dispatchRaw := mustReadFile(t, filepath.Join(root, "remote-task.json"))

	if err := ingestBacklogCompletionCandidate(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "completion_candidate_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_016", "completion_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := ingestBacklogCompletionCandidate(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "completion_candidate_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	if string(mustReadFile(t, candidatePath)) != string(acceptedRaw) {
		t.Fatal("duplicate or replay rejection changed the accepted completion candidate")
	}
	if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(dispatchRaw) {
		t.Fatal("duplicate or replay rejection changed the accepted dispatch identity")
	}
}

func TestBacklogCompletionCandidateRejectsStaleMismatchedMalformedAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale", wantCode: "completion_candidate_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:00:00Z" })
		}},
		{name: "wrong remote task", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "wrong dispatch", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("1", 64) })
		}},
		{name: "wrong adapter", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "malformed fixture", wantCode: "completion_candidate_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "tampered request", wantCode: "completion_candidate_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "tampered compatibility", wantCode: "completion_candidate_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "completion_candidate_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogCompletionTest(t)
			fixturePath := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
			test.mutate(t, root, fixturePath)
			dispatchBefore := mustReadFile(t, filepath.Join(root, "remote-task.json"))
			err := ingestBacklogCompletionCandidate(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			if _, statErr := os.Stat(filepath.Join(root, "completion-candidate.json")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected candidate left completion-candidate.json: %v", statErr)
			}
			if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(dispatchBefore) {
				t.Fatal("rejected candidate changed the dispatch artifact")
			}
			for _, name := range []string{"ready-for-review.json", "remote-status.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected candidate left forbidden artifact %s", name)
				}
			}
		})
	}
}

func TestBacklogRemoteStatusRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogStatusTest(t)
	acceptedFixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(root, "remote-status.json")
	acceptedStatus := mustReadFile(t, statusPath)
	acceptedCandidate := mustReadFile(t, filepath.Join(root, "completion-candidate.json"))
	acceptedDispatch := mustReadFile(t, filepath.Join(root, "remote-task.json"))

	if err := retrieveBacklogRemoteStatusFixture(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_status_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_016", "status_fixture_replay_015", "2026-07-19T00:03:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_status_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	if string(mustReadFile(t, statusPath)) != string(acceptedStatus) {
		t.Fatal("duplicate or replay rejection changed the accepted remote status")
	}
	if string(mustReadFile(t, filepath.Join(root, "completion-candidate.json"))) != string(acceptedCandidate) {
		t.Fatal("duplicate or replay rejection changed the accepted completion candidate")
	}
	if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(acceptedDispatch) {
		t.Fatal("duplicate or replay rejection changed the accepted dispatch identity")
	}
}

func TestBacklogRemoteStatusRejectsStaleMismatchedMalformedAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale at candidate time", wantCode: "remote_status_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:01:00Z" })
		}},
		{name: "wrong candidate", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_id"] = "completion_fixture_candidate_wrong"
			})
		}},
		{name: "wrong candidate fingerprint", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
			})
		}},
		{name: "wrong remote task", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("1", 64) })
		}},
		{name: "wrong dispatch", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "wrong adapter", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "tampered status claim", wantCode: "remote_status_claim_invalid", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["claimed_remote_status"] = "ready_for_review" })
		}},
		{name: "malformed fixture", wantCode: "remote_status_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "tampered request", wantCode: "remote_status_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("3", 64) })
		}},
		{name: "tampered compatibility", wantCode: "remote_status_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "remote_status_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
		{name: "tampered candidate state", wantCode: "remote_status_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		}},
		{name: "tampered candidate lifecycle", wantCode: "remote_status_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogStatusTest(t)
			fixturePath := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
			test.mutate(t, root, fixturePath)
			candidateBefore := mustReadFile(t, filepath.Join(root, "completion-candidate.json"))
			dispatchBefore := mustReadFile(t, filepath.Join(root, "remote-task.json"))
			err := retrieveBacklogRemoteStatusFixture(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			if _, statErr := os.Stat(filepath.Join(root, "remote-status.json")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected status observation left remote-status.json: %v", statErr)
			}
			if string(mustReadFile(t, filepath.Join(root, "completion-candidate.json"))) != string(candidateBefore) {
				t.Fatal("rejected status observation changed the accepted completion candidate")
			}
			if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(dispatchBefore) {
				t.Fatal("rejected status observation changed the dispatch artifact")
			}
			for _, name := range []string{"ready-for-review.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected status observation left forbidden artifact %s", name)
				}
			}
		})
	}
}

func TestBacklogRemoteDiffRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogDiffTest(t)
	acceptedFixture := writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	accepted := map[string][]byte{}
	for _, name := range []string{"remote-diff.json", "remote-diff.patch", "remote-status.json", "completion-candidate.json", "remote-task.json"} {
		accepted[name] = mustReadFile(t, filepath.Join(root, name))
	}
	if err := retrieveBacklogRemoteDiffFixture(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_diff_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogDiffFixture(t, root, "diff_fixture_observation_016", "diff_fixture_replay_015", "2026-07-19T00:04:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_diff_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	for name, before := range accepted {
		if string(mustReadFile(t, filepath.Join(root, name))) != string(before) {
			t.Fatalf("duplicate or replay rejection changed %s", name)
		}
	}
}

func TestBacklogRemoteDiffRejectsStaleMismatchedMalformedMissingAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale at status time", wantCode: "remote_diff_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:02:00Z" })
		}},
		{name: "wrong status observation", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_observation_id"] = "status_fixture_observation_wrong"
			})
		}},
		{name: "wrong status fingerprint", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
			})
		}},
		{name: "wrong candidate", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_id"] = "completion_fixture_candidate_wrong"
			})
		}},
		{name: "wrong candidate fingerprint", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("1", 64)
			})
		}},
		{name: "wrong remote task", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "wrong dispatch", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("3", 64) })
		}},
		{name: "wrong adapter", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "malformed fixture", wantCode: "remote_diff_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "missing fixture", wantCode: "remote_diff_fixture_missing", mutate: func(t *testing.T, _, fixturePath string) {
			if err := os.Remove(fixturePath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing patch", wantCode: "remote_diff_patch_missing", mutate: func(t *testing.T, _, fixturePath string) {
			if err := os.Remove(filepath.Join(filepath.Dir(fixturePath), "remote-diff.patch")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "tampered patch bytes", wantCode: "remote_diff_patch_tampered", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, filepath.Join(filepath.Dir(fixturePath), "remote-diff.patch"), backlogTestPatch+"tampered\n")
		}},
		{name: "tampered request", wantCode: "remote_diff_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("4", 64) })
		}},
		{name: "tampered compatibility", wantCode: "remote_diff_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "remote_diff_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
		{name: "tampered candidate", wantCode: "remote_diff_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		}},
		{name: "tampered status", wantCode: "remote_diff_status_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-status.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogDiffTest(t)
			fixturePath := writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
			test.mutate(t, root, fixturePath)
			before := map[string][]byte{}
			for _, name := range []string{"remote-status.json", "completion-candidate.json", "remote-task.json"} {
				before[name] = mustReadFile(t, filepath.Join(root, name))
			}
			err := retrieveBacklogRemoteDiffFixture(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			for _, name := range []string{"remote-diff.json", "remote-diff.patch", "ready-for-review.json", "remote-result.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected diff observation left forbidden artifact %s: %v", name, statErr)
				}
			}
			for name, content := range before {
				if string(mustReadFile(t, filepath.Join(root, name))) != string(content) {
					t.Fatalf("rejected diff observation changed %s", name)
				}
			}
		})
	}
}

func TestBacklogRemoteResultRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogResultTest(t)
	acceptedFixture := writeBacklogResultFixture(t, root, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")
	if err := retrieveBacklogRemoteResultFixture(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	accepted := map[string][]byte{}
	for _, name := range []string{"remote-result.json", "remote-diff.json", "remote-diff.patch", "remote-status.json", "completion-candidate.json", "remote-task.json"} {
		accepted[name] = mustReadFile(t, filepath.Join(root, name))
	}
	if err := retrieveBacklogRemoteResultFixture(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_result_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogResultFixture(t, root, "result_fixture_observation_016", "result_fixture_replay_015", "2026-07-19T00:05:00Z", "second opaque fixture claim")
	if err := retrieveBacklogRemoteResultFixture(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_result_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	for name, before := range accepted {
		if string(mustReadFile(t, filepath.Join(root, name))) != string(before) {
			t.Fatalf("duplicate or replay rejection changed %s", name)
		}
	}
}

func TestBacklogRemoteResultRejectsStaleMismatchedMalformedMissingAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale at diff time", wantCode: "remote_result_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:03:00Z" })
		}},
		{name: "wrong diff observation", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_diff_observation_id"] = "diff_fixture_observation_wrong" })
		}},
		{name: "wrong diff fingerprint", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_diff_fingerprint"] = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "wrong patch checksum", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["patch_sha256"] = "sha256:" + strings.Repeat("1", 64) })
		}},
		{name: "wrong patch byte count", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["patch_bytes"] = float64(len(backlogTestPatch) + 1) })
		}},
		{name: "wrong status observation", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_observation_id"] = "status_fixture_observation_wrong"
			})
		}},
		{name: "wrong status fingerprint", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_fingerprint"] = "sha256:" + strings.Repeat("5", 64)
			})
		}},
		{name: "wrong candidate", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_id"] = "completion_fixture_candidate_wrong"
			})
		}},
		{name: "wrong candidate fingerprint", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("6", 64)
			})
		}},
		{name: "wrong remote task", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "wrong dispatch", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("3", 64) })
		}},
		{name: "wrong adapter", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "remote_result_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "malformed fixture", wantCode: "remote_result_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "missing fixture", wantCode: "remote_result_fixture_missing", mutate: func(t *testing.T, _, fixturePath string) {
			if err := os.Remove(fixturePath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "tampered request", wantCode: "remote_result_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("4", 64) })
		}},
		{name: "tampered compatibility", wantCode: "remote_result_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "remote_result_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
		{name: "tampered candidate", wantCode: "remote_result_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		}},
		{name: "tampered status", wantCode: "remote_result_status_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-status.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
		{name: "tampered diff metadata", wantCode: "remote_result_diff_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-diff.json"), func(payload map[string]any) { mapValue(payload["patch"])["bytes"] = float64(1) })
		}},
		{name: "tampered accepted patch bytes", wantCode: "remote_result_diff_invalid", mutate: func(t *testing.T, root, _ string) {
			writeBacklogTestFile(t, filepath.Join(root, "remote-diff.patch"), backlogTestPatch+"tampered\n")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogResultTest(t)
			fixturePath := writeBacklogResultFixture(t, root, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")
			test.mutate(t, root, fixturePath)
			before := map[string][]byte{}
			for _, name := range []string{"remote-diff.json", "remote-diff.patch", "remote-status.json", "completion-candidate.json", "remote-task.json"} {
				before[name] = mustReadFile(t, filepath.Join(root, name))
			}
			err := retrieveBacklogRemoteResultFixture(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			for _, name := range []string{"remote-result.json", "ready-for-review.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected result observation left forbidden artifact %s: %v", name, statErr)
				}
			}
			for name, content := range before {
				if string(mustReadFile(t, filepath.Join(root, name))) != string(content) {
					t.Fatalf("rejected result observation changed %s", name)
				}
			}
		})
	}
}

func TestBacklogValidationReceiptRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogValidationReceiptTest(t)
	acceptedFixture := writeBacklogValidationReceiptFixture(t, root, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")
	if err := retrieveBacklogValidationReceiptFixture(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	accepted := map[string][]byte{}
	for _, name := range []string{"validation-receipt.json", "remote-result.json", "remote-diff.json", "remote-diff.patch", "remote-status.json", "completion-candidate.json", "remote-task.json", "remote-request.json", "remote-adapter-compatibility.json"} {
		accepted[name] = mustReadFile(t, filepath.Join(root, name))
	}
	if err := retrieveBacklogValidationReceiptFixture(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "validation_receipt_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogValidationReceiptFixture(t, root, "receipt_fixture_observation_016", "receipt_fixture_replay_015", "2026-07-19T00:06:00Z", "second opaque fixture receipt")
	if err := retrieveBacklogValidationReceiptFixture(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "validation_receipt_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	for name, before := range accepted {
		if string(mustReadFile(t, filepath.Join(root, name))) != string(before) {
			t.Fatalf("duplicate or replay rejection changed %s", name)
		}
	}
}

func TestBacklogValidationReceiptRejectsStaleMismatchedMalformedMissingAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale at result time", wantCode: "validation_receipt_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:04:00Z" })
		}},
		{name: "wrong result observation", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_result_observation_id"] = "result_fixture_observation_wrong"
			})
		}},
		{name: "wrong result fingerprint", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_result_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
			})
		}},
		{name: "wrong diff observation", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_diff_observation_id"] = "diff_fixture_observation_wrong" })
		}},
		{name: "wrong diff fingerprint", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_diff_fingerprint"] = "sha256:" + strings.Repeat("1", 64) })
		}},
		{name: "wrong patch checksum", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["patch_sha256"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "wrong patch byte count", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["patch_bytes"] = float64(len(backlogTestPatch) + 1) })
		}},
		{name: "wrong status", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_observation_id"] = "status_fixture_observation_wrong"
			})
		}},
		{name: "wrong status fingerprint", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_fingerprint"] = "sha256:" + strings.Repeat("7", 64)
			})
		}},
		{name: "wrong candidate", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_id"] = "completion_fixture_candidate_wrong"
			})
		}},
		{name: "wrong candidate fingerprint", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("8", 64)
			})
		}},
		{name: "wrong task", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("3", 64) })
		}},
		{name: "wrong required validation", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["required_validation_fingerprint"] = "sha256:" + strings.Repeat("4", 64)
			})
		}},
		{name: "wrong compatibility", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["compatibility_fingerprint"] = "sha256:" + strings.Repeat("5", 64)
			})
		}},
		{name: "wrong dispatch", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("6", 64) })
		}},
		{name: "wrong adapter", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "validation_receipt_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "malformed fixture", wantCode: "validation_receipt_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "missing fixture", wantCode: "validation_receipt_fixture_missing", mutate: func(t *testing.T, _, fixturePath string) {
			if err := os.Remove(fixturePath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "tampered request", wantCode: "validation_receipt_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["required_validation"] = []any{"tampered validation"} })
		}},
		{name: "tampered compatibility", wantCode: "validation_receipt_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "validation_receipt_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
		{name: "tampered candidate", wantCode: "validation_receipt_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		}},
		{name: "tampered status", wantCode: "validation_receipt_status_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-status.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
		{name: "tampered diff", wantCode: "validation_receipt_diff_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-diff.json"), func(payload map[string]any) { mapValue(payload["patch"])["bytes"] = float64(1) })
		}},
		{name: "tampered patch bytes", wantCode: "validation_receipt_diff_invalid", mutate: func(t *testing.T, root, _ string) {
			writeBacklogTestFile(t, filepath.Join(root, "remote-diff.patch"), backlogTestPatch+"tampered\n")
		}},
		{name: "tampered result", wantCode: "validation_receipt_result_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-result.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogValidationReceiptTest(t)
			fixturePath := writeBacklogValidationReceiptFixture(t, root, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")
			test.mutate(t, root, fixturePath)
			before := map[string][]byte{}
			for _, name := range []string{"remote-result.json", "remote-diff.json", "remote-diff.patch", "remote-status.json", "completion-candidate.json", "remote-task.json"} {
				before[name] = mustReadFile(t, filepath.Join(root, name))
			}
			err := retrieveBacklogValidationReceiptFixture(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			for _, name := range []string{"validation-receipt.json", "ready-for-review.json", "validation-execution.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected validation receipt left forbidden artifact %s: %v", name, statErr)
				}
			}
			for name, content := range before {
				if string(mustReadFile(t, filepath.Join(root, name))) != string(content) {
					t.Fatalf("rejected validation receipt changed %s", name)
				}
			}
		})
	}
}

func TestBacklogPatchBoundaryAcceptsExactAndDescendantPaths(t *testing.T) {
	for _, test := range []struct {
		name         string
		changedPath  string
		allowedPaths []string
	}{
		{name: "exact", changedPath: "packages/dorkpipe", allowedPaths: []string{"packages/dorkpipe"}},
		{name: "descendant", changedPath: "packages/dorkpipe/README.md", allowedPaths: []string{"packages/dorkpipe"}},
		{name: "second declaration", changedPath: "docs/agents/tasks/backlog-driven-remote-tasks.md", allowedPaths: []string{"packages/dorkpipe", "docs/agents/tasks/backlog-driven-remote-tasks.md"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !backlogPatchPathAllowed(test.changedPath, test.allowedPaths) {
				t.Fatalf("expected %q to match %#v", test.changedPath, test.allowedPaths)
			}
		})
	}
	for _, changedPath := range []string{"packages/dorkpipe-evil", "packages/dorkpipeline/file.go", "packages/dork"} {
		if backlogPatchPathAllowed(changedPath, []string{"packages/dorkpipe"}) {
			t.Fatalf("prefix collision %q unexpectedly matched", changedPath)
		}
	}
	exactPatch := strings.ReplaceAll(backlogTestPatch, "packages/dorkpipe/README.md", "docs/agents/tasks/backlog-driven-remote-tasks.md")
	exactRoot := prepareBacklogPatchBoundaryTest(t, exactPatch, `["docs/agents/tasks/backlog-driven-remote-tasks.md"]`)
	if err := verifyBacklogPatchBoundary(exactRoot); err != nil {
		t.Fatalf("exact allowed-path verification failed: %v", err)
	}
	multiPatch := backlogTestPatch + exactPatch
	paths, err := parseBacklogPatchChangedPaths([]byte(multiPatch))
	if err != nil {
		t.Fatal(err)
	}
	if !stringSlicesEqual(paths, []string{"docs/agents/tasks/backlog-driven-remote-tasks.md", "packages/dorkpipe/README.md"}) {
		t.Fatalf("changed paths are not deterministically sorted: %#v", paths)
	}
}

func TestBacklogPatchBoundaryRejectsMalformedUnsupportedAndInvalidPaths(t *testing.T) {
	validHeader := func(path string) string {
		return "diff --git a/" + path + " b/" + path + "\nindex 1111111..2222222 100644\n--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1 @@\n-old\n+new\n"
	}
	tests := map[string]string{
		"absolute":           validHeader("/etc/passwd"),
		"windows absolute":   validHeader("C:/secret.txt"),
		"backslash":          validHeader(`packages\dorkpipe\README.md`),
		"traversal":          validHeader("packages/dorkpipe/../secret.txt"),
		"empty component":    validHeader("packages//dorkpipe/file.go"),
		"quoted":             "diff --git \"a/packages/dorkpipe/README.md\" \"b/packages/dorkpipe/README.md\"\nindex 1111111..2222222 100644\n--- \"a/packages/dorkpipe/README.md\"\n+++ \"b/packages/dorkpipe/README.md\"\n@@ -1 +1 @@\n-old\n+new\n",
		"control":            validHeader("packages/dorkpipe/bad\tpath.go"),
		"git internal":       validHeader(".git/config"),
		"generated":          validHeader("bin/.dockpipe/internal/state.json"),
		"secret like":        validHeader("config/secrets/token.txt"),
		"combined":           "diff --cc packages/dorkpipe/README.md\nindex 1111111,2222222..3333333\n--- a/packages/dorkpipe/README.md\n+++ b/packages/dorkpipe/README.md\n@@@ -1,1 -1,1 +1,1 @@@\n-old\n+new\n",
		"binary":             "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\nindex 1111111..2222222 100644\nBinary files a/packages/dorkpipe/README.md and b/packages/dorkpipe/README.md differ\n",
		"submodule":          "diff --git a/packages/dorkpipe/submodule b/packages/dorkpipe/submodule\nindex 1111111..2222222 160000\n--- a/packages/dorkpipe/submodule\n+++ b/packages/dorkpipe/submodule\n@@ -1 +1 @@\n-Subproject commit 1111111\n+Subproject commit 2222222\n",
		"rename":             "diff --git a/packages/dorkpipe/old.go b/packages/dorkpipe/new.go\nsimilarity index 100%\nrename from packages/dorkpipe/old.go\nrename to packages/dorkpipe/new.go\n",
		"copy":               "diff --git a/packages/dorkpipe/old.go b/packages/dorkpipe/new.go\nsimilarity index 100%\ncopy from packages/dorkpipe/old.go\ncopy to packages/dorkpipe/new.go\n",
		"mismatched headers": "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\nindex 1111111..2222222 100644\n--- a/packages/dorkpipe/OTHER.md\n+++ b/packages/dorkpipe/README.md\n@@ -1 +1 @@\n-old\n+new\n",
		"malformed hunk":     "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\nindex 1111111..2222222 100644\n--- a/packages/dorkpipe/README.md\n+++ b/packages/dorkpipe/README.md\n@@ malformed @@\n-old\n+new\n",
		"unsupported mode":   "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\nold mode 100644\nnew mode 100755\n",
		"not terminated":     strings.TrimSuffix(validHeader("packages/dorkpipe/README.md"), "\n"),
	}
	for name, patch := range tests {
		t.Run(name, func(t *testing.T) {
			if paths, err := parseBacklogPatchChangedPaths([]byte(patch)); err == nil {
				t.Fatalf("unsupported patch unexpectedly accepted paths %#v", paths)
			}
		})
	}
}

func TestBacklogPatchBoundaryRejectsOutOfScopeAndTamperedChainWithoutArtifact(t *testing.T) {
	outOfScopePatch := strings.ReplaceAll(backlogTestPatch, "packages/dorkpipe/README.md", "packages/dorkpipe-evil/README.md")
	root := prepareBacklogPatchBoundaryTest(t, outOfScopePatch, `["packages/dorkpipe"]`)
	if err := verifyBacklogPatchBoundary(root); err == nil || !strings.HasPrefix(err.Error(), "patch_boundary_path_out_of_scope:") {
		t.Fatalf("out-of-scope prefix collision returned %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "patch-boundary.json")); !os.IsNotExist(err) {
		t.Fatalf("out-of-scope patch created patch-boundary.json: %v", err)
	}

	mutations := map[string]func(string){
		"remote-request.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["scope"])["allowed_paths"] = []any{"packages/dorkpipe-evil"}
			})
		},
		"remote-request.md": func(path string) { writeBacklogTestFile(t, path, "tampered request markdown\n") },
		"remote-adapter-compatibility.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["adapter_identity"] = "tampered-adapter" })
		},
		"remote-task.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["remote_task_id"] = "remote_fixture_task_tampered" })
		},
		"completion-candidate.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["identity"])["candidate_id"] = "completion_fixture_candidate_tampered"
			})
		},
		"remote-status.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["identity"])["observation_id"] = "status_fixture_observation_tampered"
			})
		},
		"remote-diff.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["observed_at"] = "2026-07-19T00:03:01Z" })
		},
		"remote-diff.patch": func(path string) { writeBacklogTestFile(t, path, backlogTestPatch+"tampered\n") },
		"remote-result.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { mapValue(value["evidence"])["opaque_result"] = "tampered result evidence" })
		},
		"validation-receipt.json": func(path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["remote_result"])["fingerprint"] = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			})
		},
	}
	for artifact, mutate := range mutations {
		t.Run(artifact, func(t *testing.T) {
			tamperedRoot := cloneBacklogTestArtifacts(t, prepareBacklogPatchBoundaryTest(t, backlogTestPatch, `["packages/dorkpipe"]`))
			mutate(filepath.Join(tamperedRoot, artifact))
			if err := verifyBacklogPatchBoundary(tamperedRoot); err == nil {
				t.Fatalf("tampered %s unexpectedly verified", artifact)
			}
			if _, err := os.Stat(filepath.Join(tamperedRoot, "patch-boundary.json")); !os.IsNotExist(err) {
				t.Fatalf("tampered %s created patch-boundary.json: %v", artifact, err)
			}
		})
	}
}

func TestBacklogPatchBoundaryRejectsTamperedExistingArtifact(t *testing.T) {
	root := prepareBacklogPatchBoundaryTest(t, backlogTestPatch, `["packages/dorkpipe"]`)
	if err := verifyBacklogPatchBoundary(root); err != nil {
		t.Fatal(err)
	}
	mutateBacklogJSONFile(t, filepath.Join(root, "patch-boundary.json"), func(value map[string]any) {
		value["state"] = "ready_for_review"
	})
	if err := verifyBacklogPatchBoundary(root); err == nil || !strings.HasPrefix(err.Error(), "patch_boundary_artifact_invalid:") {
		t.Fatalf("tampered existing patch-boundary artifact returned %v", err)
	}
}

func prepareBacklogPatchBoundaryTest(t *testing.T, patch, allowedPathsJSON string) string {
	t.Helper()
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded patch-boundary slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", allowedPathsJSON, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	if err := preflightBacklogRemoteCompatibility(root, writeBacklogCompatibilityFixture(t)); err != nil {
		t.Fatal(err)
	}
	dispatchFixture := filepath.Join(t.TempDir(), "dispatch.json")
	writeBacklogTestFile(t, dispatchFixture, `{
  "contract_version": "dorkpipe.remote-dispatch-fixture/v1",
  "adapter_identity": "codex-cloud-fixture-v1",
  "remote_task_id": "remote_fixture_task_015",
  "submitted_at": "2026-07-19T00:00:00Z"
}`)
	if err := dispatchBacklogFixture(root, dispatchFixture); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	candidateFixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := ingestBacklogCompletionCandidate(root, candidateFixture); err != nil {
		t.Fatal(err)
	}
	statusFixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, statusFixture); err != nil {
		t.Fatal(err)
	}
	diffFixture := writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", patch)
	if err := retrieveBacklogRemoteDiffFixture(root, diffFixture); err != nil {
		t.Fatal(err)
	}
	resultFixture := writeBacklogResultFixture(t, root, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")
	if err := retrieveBacklogRemoteResultFixture(root, resultFixture); err != nil {
		t.Fatal(err)
	}
	receiptFixture := writeBacklogValidationReceiptFixture(t, root, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")
	if err := retrieveBacklogValidationReceiptFixture(root, receiptFixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func cloneBacklogTestArtifacts(t *testing.T, source string) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "artifacts")
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	return destination
}

func prepareBacklogCompletionTest(t *testing.T) string {
	t.Helper()
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded completion candidate slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	if err := preflightBacklogRemoteCompatibility(root, writeBacklogCompatibilityFixture(t)); err != nil {
		t.Fatal(err)
	}
	dispatchFixture := filepath.Join(t.TempDir(), "dispatch.json")
	writeBacklogTestFile(t, dispatchFixture, `{
  "contract_version": "dorkpipe.remote-dispatch-fixture/v1",
  "adapter_identity": "codex-cloud-fixture-v1",
  "remote_task_id": "remote_fixture_task_015",
  "submitted_at": "2026-07-19T00:00:00Z"
}`)
	if err := dispatchBacklogFixture(root, dispatchFixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareBacklogStatusTest(t *testing.T) string {
	t.Helper()
	root := prepareBacklogCompletionTest(t)
	fixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := ingestBacklogCompletionCandidate(root, fixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareBacklogDiffTest(t *testing.T) string {
	t.Helper()
	root := prepareBacklogStatusTest(t)
	fixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, fixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareBacklogResultTest(t *testing.T) string {
	t.Helper()
	root := prepareBacklogDiffTest(t)
	fixture := writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(root, fixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareBacklogValidationReceiptTest(t *testing.T) string {
	t.Helper()
	root := prepareBacklogResultTest(t)
	fixture := writeBacklogResultFixture(t, root, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")
	if err := retrieveBacklogRemoteResultFixture(root, fixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeBacklogCompletionFixture(t *testing.T, root, candidateID, replayIdentity, observedAt string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	payload := backlogCompletionFixture{
		ContractVersion: backlogCompletionFixtureContract, CandidateID: candidateID, ReplayIdentity: replayIdentity,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, ClaimedTerminalSignal: "completed",
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "completion-candidate.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	return path
}

func writeBacklogStatusFixture(t *testing.T, root, observationID, replayIdentity, observedAt string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	candidate := readJSONMap(filepath.Join(root, "completion-candidate.json"))
	candidateFingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		t.Fatal(err)
	}
	payload := backlogStatusFixture{
		ContractVersion: backlogStatusFixtureContract, ObservationID: observationID, ReplayIdentity: replayIdentity,
		CompletionCandidateID: stringValue(mapValue(candidate["identity"])["candidate_id"]), CompletionCandidateFingerprint: candidateFingerprint,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, ClaimedRemoteStatus: "completed",
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "remote-status.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	return path
}

func writeBacklogDiffFixture(t *testing.T, root, observationID, replayIdentity, observedAt, patch string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	candidate := readJSONMap(filepath.Join(root, "completion-candidate.json"))
	candidateFingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		t.Fatal(err)
	}
	status := readJSONMap(filepath.Join(root, "remote-status.json"))
	statusFingerprint, err := backlogJSONFingerprint(status)
	if err != nil {
		t.Fatal(err)
	}
	payload := backlogDiffFixture{
		ContractVersion: backlogDiffFixtureContract, ObservationID: observationID, ReplayIdentity: replayIdentity,
		RemoteStatusObservationID: stringValue(mapValue(status["identity"])["observation_id"]), RemoteStatusFingerprint: statusFingerprint,
		CompletionCandidateID: stringValue(mapValue(candidate["identity"])["candidate_id"]), CompletionCandidateFingerprint: candidateFingerprint,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, PatchFile: "remote-diff.patch", PatchSHA256: sha256String([]byte(patch)),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	rootPath := t.TempDir()
	path := filepath.Join(rootPath, "remote-diff.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	writeBacklogTestFile(t, filepath.Join(rootPath, "remote-diff.patch"), patch)
	return path
}

func writeBacklogResultFixture(t *testing.T, root, observationID, replayIdentity, observedAt, opaqueResult string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	candidate := readJSONMap(filepath.Join(root, "completion-candidate.json"))
	candidateFingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		t.Fatal(err)
	}
	status := readJSONMap(filepath.Join(root, "remote-status.json"))
	statusFingerprint, err := backlogJSONFingerprint(status)
	if err != nil {
		t.Fatal(err)
	}
	diff := readJSONMap(filepath.Join(root, "remote-diff.json"))
	diffFingerprint, err := backlogJSONFingerprint(diff)
	if err != nil {
		t.Fatal(err)
	}
	diffPatch := mapValue(diff["patch"])
	patchBytes := int(diffPatch["bytes"].(float64))
	payload := backlogResultFixture{
		ContractVersion: backlogResultFixtureContract, ObservationID: observationID, ReplayIdentity: replayIdentity,
		RemoteDiffObservationID: stringValue(mapValue(diff["identity"])["observation_id"]), RemoteDiffFingerprint: diffFingerprint,
		PatchSHA256: stringValue(diffPatch["sha256"]), PatchBytes: &patchBytes,
		RemoteStatusObservationID: stringValue(mapValue(status["identity"])["observation_id"]), RemoteStatusFingerprint: statusFingerprint,
		CompletionCandidateID: stringValue(mapValue(candidate["identity"])["candidate_id"]), CompletionCandidateFingerprint: candidateFingerprint,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, OpaqueResult: opaqueResult,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "remote-result.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	return path
}

func writeBacklogValidationReceiptFixture(t *testing.T, root, observationID, replayIdentity, observedAt, opaqueReceipt string) string {
	t.Helper()
	request := readJSONMap(filepath.Join(root, "remote-request.json"))
	requiredValidationFingerprint, err := backlogRequiredValidationFingerprint(request)
	if err != nil {
		t.Fatal(err)
	}
	compatibility := readJSONMap(filepath.Join(root, "remote-adapter-compatibility.json"))
	compatibilityFingerprint, err := backlogJSONFingerprint(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	candidate := readJSONMap(filepath.Join(root, "completion-candidate.json"))
	candidateFingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		t.Fatal(err)
	}
	status := readJSONMap(filepath.Join(root, "remote-status.json"))
	statusFingerprint, err := backlogJSONFingerprint(status)
	if err != nil {
		t.Fatal(err)
	}
	diff := readJSONMap(filepath.Join(root, "remote-diff.json"))
	diffFingerprint, err := backlogJSONFingerprint(diff)
	if err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(filepath.Join(root, "remote-result.json"))
	resultFingerprint, err := backlogJSONFingerprint(result)
	if err != nil {
		t.Fatal(err)
	}
	diffPatch := mapValue(diff["patch"])
	patchBytes := int(diffPatch["bytes"].(float64))
	payload := backlogValidationReceiptFixture{
		ContractVersion: backlogValidationReceiptFixtureContract, ObservationID: observationID, ReplayIdentity: replayIdentity,
		RemoteResultObservationID: stringValue(mapValue(result["identity"])["observation_id"]), RemoteResultFingerprint: resultFingerprint,
		RemoteDiffObservationID: stringValue(mapValue(diff["identity"])["observation_id"]), RemoteDiffFingerprint: diffFingerprint,
		PatchSHA256: stringValue(diffPatch["sha256"]), PatchBytes: &patchBytes,
		RemoteStatusObservationID: stringValue(mapValue(status["identity"])["observation_id"]), RemoteStatusFingerprint: statusFingerprint,
		CompletionCandidateID: stringValue(mapValue(candidate["identity"])["candidate_id"]), CompletionCandidateFingerprint: candidateFingerprint,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), RequiredValidationFingerprint: requiredValidationFingerprint,
		CompatibilityFingerprint: compatibilityFingerprint, DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, OpaqueReceipt: opaqueReceipt,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "validation-receipt.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	return path
}

func mutateBacklogJSONFile(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	payload := readJSONMap(path)
	mutate(payload)
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeBacklogTestFile(t, path, string(raw)+"\n")
}

func writeBacklogTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md":      "# Fixture agent guidance\n",
		backlogIndexPath: "schema: 1\ndescription: Fixture open-only backlog.\ntasks:\n  - id: TASK-015\n    topic: Backlog remote fixture\n    path: docs/agents/tasks/backlog-driven-remote-tasks.md\nmaintenance:\n  - Keep open-only.\n",
		"docs/agents/tasks/backlog-driven-remote-tasks.md": "# TASK-015 Backlog remote fixture\n\nFixture task body.\n",
		"docs/agents/packages/package-authoring.md":        "# Package authoring fixture\n",
		"docs/agents/workflows/yaml-workflows.md":          "# YAML workflow fixture\n",
		"packages/dorkpipe/README.md":                      "# Fixture package\n",
	}
	for rel, content := range files {
		writeBacklogTestFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	if err := os.MkdirAll(filepath.Join(root, "packages", "dorkpipe", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeBacklogCompatibilityFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeBacklogTestFile(t, filepath.Join(root, "contract.json"), `{
  "contract_version": "dorkpipe.codex-cloud-cli-compatibility-fixture/v1",
  "adapter_identity": "codex-cloud-cli",
  "cli": {"reference": "codex", "version": "codex-cli 0.144.1"},
  "inspected_commands": [
    {"argv": ["codex", "--version"], "fixture": "codex-version.txt"},
    {"argv": ["codex", "cloud", "--help"], "fixture": "codex-cloud-help.txt"},
    {"argv": ["codex", "cloud", "exec", "--help"], "fixture": "codex-cloud-exec-help.txt"}
  ],
  "recognized_inputs": [
    {"name": "environment", "flag": "--env", "value": "ENV_ID", "required": true},
    {"name": "branch", "flag": "--branch", "value": "BRANCH", "required": false}
  ],
  "submission_receipt": {"machine_readable_documented": false, "stable_opaque_task_id_recoverable": false},
  "exact_gap": "codex cloud exec --help for codex-cli 0.144.1 documents no machine-readable submission receipt and no stable opaque task-ID response contract."
}
`)
	writeBacklogTestFile(t, filepath.Join(root, "codex-version.txt"), "codex-cli 0.144.1\n")
	writeBacklogTestFile(t, filepath.Join(root, "codex-cloud-help.txt"), "Usage: codex cloud [OPTIONS] [COMMAND]\nexec    Submit a new Codex Cloud task without launching the TUI\n")
	writeBacklogTestFile(t, filepath.Join(root, "codex-cloud-exec-help.txt"), "Usage: codex cloud exec [OPTIONS] --env <ENV_ID> [QUERY]\n--env <ENV_ID>\n--branch <BRANCH>\n")
	return root
}

func writeBacklogTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func backlogTestBool(value any) bool {
	parsed, _ := value.(bool)
	return parsed
}
