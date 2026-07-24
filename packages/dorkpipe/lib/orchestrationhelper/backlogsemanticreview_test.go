package orchestrationhelper

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBacklogSemanticReviewApprovedDecisionEmitsDeterministicReadinessAndRestartsArtifactOnly(t *testing.T) {
	artifactRoot, consumerRoot := prepareBacklogSemanticReviewArtifacts(t, 0)
	upstream := snapshotBacklogSemanticReviewUpstream(t, artifactRoot)
	chain, err := loadBacklogSemanticReviewChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := writeBacklogSemanticReviewTestFixture(t, chain, "review_decision_fixture_015", "review_replay_fixture_015", backlogSemanticReviewApproved)

	if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err != nil {
		t.Fatal(err)
	}
	assertBacklogSemanticReviewUpstreamUnchanged(t, artifactRoot, upstream)
	decisionPath := filepath.Join(artifactRoot, "semantic-review-decision.json")
	readyPath := filepath.Join(artifactRoot, "ready-for-review.json")
	decisionRaw := mustReadFile(t, decisionPath)
	readyRaw := mustReadFile(t, readyPath)
	decision := readJSONMap(decisionPath)
	ready := readJSONMap(readyPath)
	if stringValue(decision["contract_version"]) != backlogSemanticReviewContract || stringValue(decision["state"]) != "completion_candidate" || stringValue(mapValue(decision["decision"])["value"]) != backlogSemanticReviewApproved {
		t.Fatalf("unexpected semantic review decision: %#v", decision)
	}
	if stringValue(ready["contract_version"]) != backlogReadyForReviewContract || stringValue(ready["state"]) != "ready_for_review" {
		t.Fatalf("unexpected readiness artifact: %#v", ready)
	}
	for label, payload := range map[string]map[string]any{"decision": decision, "ready": ready} {
		capabilities := mapValue(payload["capabilities"])
		for _, capability := range []string{"apply_to_checkout", "commit", "push", "publication", "start_another_backlog_item"} {
			if backlogTestBool(capabilities[capability]) {
				t.Fatalf("%s artifact enabled forbidden capability %s", label, capability)
			}
		}
	}
	if strings.Contains(string(decisionRaw), consumerRoot) || strings.Contains(string(readyRaw), consumerRoot) {
		t.Fatal("semantic review artifacts leaked an absolute consumer path")
	}

	if err := os.RemoveAll(consumerRoot); err != nil {
		t.Fatal(err)
	}
	if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err != nil {
		t.Fatalf("artifact-only restart failed: %v", err)
	}
	if got := mustReadFile(t, decisionPath); !bytesEqual(got, decisionRaw) {
		t.Fatal("artifact-only restart changed semantic-review-decision.json")
	}
	if got := mustReadFile(t, readyPath); !bytesEqual(got, readyRaw) {
		t.Fatal("artifact-only restart changed ready-for-review.json")
	}
}

func TestBacklogSemanticReviewRejectedDecisionNeverEmitsReadiness(t *testing.T) {
	for _, exitCode := range []int{0, 7} {
		t.Run(map[bool]string{true: "passed_validation", false: "failed_validation"}[exitCode == 0], func(t *testing.T) {
			artifactRoot, consumerRoot := prepareBacklogSemanticReviewArtifacts(t, exitCode)
			chain, err := loadBacklogSemanticReviewChain(artifactRoot)
			if err != nil {
				t.Fatal(err)
			}
			fixturePath := writeBacklogSemanticReviewTestFixture(t, chain, "review_rejected_fixture_015", "review_rejected_replay_015", backlogSemanticReviewRejected)
			if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err != nil {
				t.Fatal(err)
			}
			decisionRaw := mustReadFile(t, filepath.Join(artifactRoot, "semantic-review-decision.json"))
			decision := readJSONMap(filepath.Join(artifactRoot, "semantic-review-decision.json"))
			if stringValue(mapValue(decision["decision"])["value"]) != backlogSemanticReviewRejected || stringValue(mapValue(decision["decision"])["validation_status"]) != chain.ValidationStatus {
				t.Fatalf("unexpected rejected decision: %#v", decision)
			}
			assertBacklogReadyForReviewAbsent(t, artifactRoot)
			if err := os.RemoveAll(consumerRoot); err != nil {
				t.Fatal(err)
			}
			if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err != nil {
				t.Fatalf("rejected artifact-only restart failed: %v", err)
			}
			if got := mustReadFile(t, filepath.Join(artifactRoot, "semantic-review-decision.json")); !bytesEqual(got, decisionRaw) {
				t.Fatal("rejected artifact-only restart changed the decision receipt")
			}
			assertBacklogReadyForReviewAbsent(t, artifactRoot)
		})
	}
}

func TestBacklogSemanticReviewFailedValidationCannotBecomeReady(t *testing.T) {
	artifactRoot, _ := prepareBacklogSemanticReviewArtifacts(t, 9)
	chain, err := loadBacklogSemanticReviewChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := writeBacklogSemanticReviewTestFixture(t, chain, "review_failed_validation_015", "review_failed_replay_015", backlogSemanticReviewApproved)
	err = recordBacklogSemanticReviewDecision(artifactRoot, fixturePath)
	if err == nil || !strings.HasPrefix(err.Error(), "semantic_review_validation_not_passed:") {
		t.Fatalf("approved failed validation returned %v", err)
	}
	assertBacklogSemanticReviewArtifactsAbsent(t, artifactRoot)
}

func TestBacklogSemanticReviewRejectsMissingAmbiguousMalformedAndUnknownDecision(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any) []byte
	}{
		{name: "missing", mutate: func(payload map[string]any) []byte {
			delete(payload, "decision")
			return marshalBacklogTestJSON(t, payload)
		}},
		{name: "ambiguous", mutate: func(payload map[string]any) []byte {
			payload["decision"] = "approved rejected"
			return marshalBacklogTestJSON(t, payload)
		}},
		{name: "unknown", mutate: func(payload map[string]any) []byte {
			payload["decision"] = "accept"
			return marshalBacklogTestJSON(t, payload)
		}},
		{name: "wrong_scope", mutate: func(payload map[string]any) []byte {
			payload["review_scope"] = "all_repository_changes"
			return marshalBacklogTestJSON(t, payload)
		}},
		{name: "malformed", mutate: func(map[string]any) []byte { return []byte(`{"decision":`) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot, _ := prepareBacklogSemanticReviewArtifacts(t, 0)
			chain, err := loadBacklogSemanticReviewChain(artifactRoot)
			if err != nil {
				t.Fatal(err)
			}
			fixture := backlogSemanticReviewFixtureForChain(chain, "review_invalid_fixture_015", "review_invalid_replay_015", backlogSemanticReviewApproved)
			payload := map[string]any{}
			raw := marshalBacklogTestJSON(t, fixture)
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			fixturePath := filepath.Join(t.TempDir(), "semantic-review.json")
			if err := os.WriteFile(fixturePath, test.mutate(payload), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err == nil {
				t.Fatal("invalid semantic review decision was accepted")
			}
			assertBacklogSemanticReviewArtifactsAbsent(t, artifactRoot)
		})
	}
}

func TestBacklogSemanticReviewRejectsWrongImmutableBindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "task", mutate: func(payload map[string]any) { payload["task_id"] = "TASK-014" }},
		{name: "patch", mutate: func(payload map[string]any) { payload["patch_sha256"] = "sha256:" + strings.Repeat("0", 64) }},
		{name: "changed_paths", mutate: func(payload map[string]any) { payload["changed_paths"] = []any{"packages/dorkpipe/OTHER.md"} }},
		{name: "validation_execution", mutate: func(payload map[string]any) {
			payload["validation_execution_fingerprint"] = "sha256:" + strings.Repeat("1", 64)
		}},
		{name: "candidate", mutate: func(payload map[string]any) {
			payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("2", 64)
		}},
		{name: "status", mutate: func(payload map[string]any) {
			payload["remote_status_fingerprint"] = "sha256:" + strings.Repeat("3", 64)
		}},
		{name: "diff", mutate: func(payload map[string]any) { payload["remote_diff_fingerprint"] = "sha256:" + strings.Repeat("4", 64) }},
		{name: "result", mutate: func(payload map[string]any) {
			payload["remote_result_fingerprint"] = "sha256:" + strings.Repeat("5", 64)
		}},
		{name: "receipt", mutate: func(payload map[string]any) {
			payload["validation_receipt_fingerprint"] = "sha256:" + strings.Repeat("6", 64)
		}},
		{name: "boundary", mutate: func(payload map[string]any) {
			payload["patch_boundary_fingerprint"] = "sha256:" + strings.Repeat("7", 64)
		}},
		{name: "application", mutate: func(payload map[string]any) {
			payload["patch_application_fingerprint"] = "sha256:" + strings.Repeat("8", 64)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot, _ := prepareBacklogSemanticReviewArtifacts(t, 0)
			chain, err := loadBacklogSemanticReviewChain(artifactRoot)
			if err != nil {
				t.Fatal(err)
			}
			fixture := backlogSemanticReviewFixtureForChain(chain, "review_binding_fixture_015", "review_binding_replay_015", backlogSemanticReviewApproved)
			payload := map[string]any{}
			if err := json.Unmarshal(marshalBacklogTestJSON(t, fixture), &payload); err != nil {
				t.Fatal(err)
			}
			test.mutate(payload)
			fixturePath := filepath.Join(t.TempDir(), "semantic-review.json")
			if err := os.WriteFile(fixturePath, marshalBacklogTestJSON(t, payload), 0o644); err != nil {
				t.Fatal(err)
			}
			err = recordBacklogSemanticReviewDecision(artifactRoot, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), "semantic_review_binding_mismatch:") {
				t.Fatalf("wrong %s binding returned %v", test.name, err)
			}
			assertBacklogSemanticReviewArtifactsAbsent(t, artifactRoot)
		})
	}
}

func TestBacklogSemanticReviewRejectsChainAndExistingArtifactTamperingWithoutOverwrite(t *testing.T) {
	t.Run("upstream", func(t *testing.T) {
		artifactRoot, _ := prepareBacklogSemanticReviewArtifacts(t, 0)
		chain, err := loadBacklogSemanticReviewChain(artifactRoot)
		if err != nil {
			t.Fatal(err)
		}
		fixturePath := writeBacklogSemanticReviewTestFixture(t, chain, "review_tamper_fixture_015", "review_tamper_replay_015", backlogSemanticReviewApproved)
		mutateBacklogJSONFile(t, filepath.Join(artifactRoot, "validation-execution.json"), func(payload map[string]any) {
			payload["state"] = "ready_for_review"
		})
		if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err == nil || !strings.HasPrefix(err.Error(), "semantic_review_chain_invalid:") {
			t.Fatalf("tampered chain returned %v", err)
		}
		assertBacklogSemanticReviewArtifactsAbsent(t, artifactRoot)
	})

	for _, name := range []string{"semantic-review-decision.json", "ready-for-review.json"} {
		t.Run(name, func(t *testing.T) {
			artifactRoot, _ := prepareBacklogSemanticReviewArtifacts(t, 0)
			chain, err := loadBacklogSemanticReviewChain(artifactRoot)
			if err != nil {
				t.Fatal(err)
			}
			fixturePath := writeBacklogSemanticReviewTestFixture(t, chain, "review_artifact_fixture_015", "review_artifact_replay_015", backlogSemanticReviewApproved)
			if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(artifactRoot, name)
			mutateBacklogJSONFile(t, path, func(payload map[string]any) { payload["state"] = "tampered" })
			tampered := mustReadFile(t, path)
			if err := recordBacklogSemanticReviewDecision(artifactRoot, fixturePath); err == nil {
				t.Fatalf("tampered %s was accepted", name)
			}
			if got := mustReadFile(t, path); !bytesEqual(got, tampered) {
				t.Fatalf("tampered %s was overwritten", name)
			}
		})
	}
}

func TestBacklogSemanticReviewRejectsDuplicateReplayAndDecisionChange(t *testing.T) {
	tests := []struct {
		name       string
		decisionID string
		replayID   string
		decision   string
		code       string
	}{
		{name: "duplicate", decisionID: "review_original_fixture_015", replayID: "review_second_replay_015", decision: backlogSemanticReviewRejected, code: "semantic_review_duplicate:"},
		{name: "replay", decisionID: "review_second_fixture_015", replayID: "review_original_replay_015", decision: backlogSemanticReviewRejected, code: "semantic_review_replay:"},
		{name: "decision_change", decisionID: "review_original_fixture_015", replayID: "review_original_replay_015", decision: backlogSemanticReviewApproved, code: "semantic_review_decision_conflict:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot, _ := prepareBacklogSemanticReviewArtifacts(t, 0)
			chain, err := loadBacklogSemanticReviewChain(artifactRoot)
			if err != nil {
				t.Fatal(err)
			}
			first := writeBacklogSemanticReviewTestFixture(t, chain, "review_original_fixture_015", "review_original_replay_015", backlogSemanticReviewRejected)
			if err := recordBacklogSemanticReviewDecision(artifactRoot, first); err != nil {
				t.Fatal(err)
			}
			accepted := mustReadFile(t, filepath.Join(artifactRoot, "semantic-review-decision.json"))
			second := writeBacklogSemanticReviewTestFixture(t, chain, test.decisionID, test.replayID, test.decision)
			err = recordBacklogSemanticReviewDecision(artifactRoot, second)
			if err == nil || !strings.HasPrefix(err.Error(), test.code) {
				t.Fatalf("%s returned %v", test.name, err)
			}
			if got := mustReadFile(t, filepath.Join(artifactRoot, "semantic-review-decision.json")); !bytesEqual(got, accepted) {
				t.Fatal("accepted decision changed after duplicate or replay rejection")
			}
			assertBacklogReadyForReviewAbsent(t, artifactRoot)
		})
	}
}

func prepareBacklogSemanticReviewArtifacts(t *testing.T, exitCode int) (string, string) {
	t.Helper()
	artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
	originalRunner := backlogValidationRunCommand
	backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) { return exitCode, true, nil }
	t.Cleanup(func() { backlogValidationRunCommand = originalRunner })
	if err := executeBacklogValidation(consumerRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	assertBacklogSemanticReviewArtifactsAbsent(t, artifactRoot)
	return artifactRoot, consumerRoot
}

func writeBacklogSemanticReviewTestFixture(t *testing.T, chain *backlogSemanticReviewChain, decisionID, replayIdentity, decision string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "semantic-review.json")
	raw := marshalBacklogTestJSON(t, backlogSemanticReviewFixtureForChain(chain, decisionID, replayIdentity, decision))
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func marshalBacklogTestJSON(t *testing.T, payload any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func snapshotBacklogSemanticReviewUpstream(t *testing.T, root string) map[string][]byte {
	t.Helper()
	upstream := snapshotBacklogValidationArtifacts(t, root)
	upstream["validation-execution.json"] = mustReadFile(t, filepath.Join(root, "validation-execution.json"))
	return upstream
}

func assertBacklogSemanticReviewUpstreamUnchanged(t *testing.T, root string, upstream map[string][]byte) {
	t.Helper()
	for name, expected := range upstream {
		if got := mustReadFile(t, filepath.Join(root, name)); !bytesEqual(got, expected) {
			t.Fatalf("semantic review changed upstream artifact %s", name)
		}
	}
}

func assertBacklogSemanticReviewArtifactsAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"semantic-review-decision.json", "ready-for-review.json"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("semantic review rejection left %s: %v", name, err)
		}
	}
}

func assertBacklogReadyForReviewAbsent(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, "ready-for-review.json")); !os.IsNotExist(err) {
		t.Fatalf("ready-for-review.json exists unexpectedly: %v", err)
	}
}
