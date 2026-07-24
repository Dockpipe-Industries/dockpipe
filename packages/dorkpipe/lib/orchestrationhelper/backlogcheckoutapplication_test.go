package orchestrationhelper

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBacklogCheckoutApplicationApprovedAppliesExactlyAndRestartsDeterministically(t *testing.T) {
	artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
	chain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_approval_fixture_015", "checkout_approval_replay_015", backlogSemanticReviewApproved)
	upstream := snapshotBacklogCheckoutApplicationUpstream(t, artifactRoot)
	consumerPath := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
	unrelatedPath := filepath.Join(consumerRoot, "AGENTS.md")
	unrelatedBefore := mustReadFile(t, unrelatedPath)

	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
		t.Fatal(err)
	}
	want := "# Fixture package\nUntrusted remote fixture change.\n"
	if got := string(mustReadFile(t, consumerPath)); got != want {
		t.Fatalf("consumer postimage = %q, want %q", got, want)
	}
	if got := mustReadFile(t, unrelatedPath); !bytesEqual(got, unrelatedBefore) {
		t.Fatal("checkout application changed an unrelated consumer file")
	}
	assertBacklogCheckoutApplicationUpstreamUnchanged(t, artifactRoot, upstream)
	assertNoBacklogCheckoutTemporaryFiles(t, consumerRoot)

	approvalPath := filepath.Join(artifactRoot, "checkout-application-approval.json")
	receiptPath := filepath.Join(artifactRoot, "checkout-application.json")
	approvalRaw := mustReadFile(t, approvalPath)
	receiptRaw := mustReadFile(t, receiptPath)
	approval := readJSONMap(approvalPath)
	receipt := readJSONMap(receiptPath)
	if stringValue(approval["contract_version"]) != backlogCheckoutApplicationApprovalContract || stringValue(mapValue(approval["decision"])["value"]) != backlogSemanticReviewApproved {
		t.Fatalf("unexpected checkout approval: %#v", approval)
	}
	if stringValue(receipt["contract_version"]) != backlogCheckoutApplicationContract || stringValue(receipt["state"]) != "applied_for_review" {
		t.Fatalf("unexpected checkout receipt: %#v", receipt)
	}
	application := mapValue(receipt["application"])
	if !backlogTestBool(application["consumer_checkout_applied"]) || !backlogTestBool(application["consumer_postimages_verified"]) ||
		!backlogTestBool(application["cleanup_succeeded"]) || int(application["files_applied"].(float64)) != 1 || int(application["hunks_applied"].(float64)) != 1 {
		t.Fatalf("unexpected checkout application evidence: %#v", application)
	}
	for _, capability := range []string{"apply_to_checkout", "commit", "push", "publication", "checkpoint", "sync", "start_another_backlog_item"} {
		if backlogTestBool(mapValue(receipt["capabilities"])[capability]) {
			t.Fatalf("checkout receipt enabled forbidden capability %s", capability)
		}
	}
	for _, action := range []string{"commit_performed", "push_performed", "publication_performed", "checkpoint_performed", "sync_performed", "next_task_selected"} {
		if backlogTestBool(mapValue(receipt["actions"])[action]) {
			t.Fatalf("checkout receipt performed forbidden action %s", action)
		}
	}
	if strings.Contains(string(approvalRaw), consumerRoot) || strings.Contains(string(receiptRaw), consumerRoot) {
		t.Fatal("checkout application artifacts leaked an absolute consumer path")
	}

	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
		t.Fatalf("idempotent rerun failed: %v", err)
	}
	if !bytesEqual(mustReadFile(t, approvalPath), approvalRaw) || !bytesEqual(mustReadFile(t, receiptPath), receiptRaw) {
		t.Fatal("idempotent rerun changed checkout application artifacts")
	}
	if err := os.Remove(receiptPath); err != nil {
		t.Fatal(err)
	}
	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
		t.Fatalf("approved postimage-only recovery failed: %v", err)
	}
	if !bytesEqual(mustReadFile(t, receiptPath), receiptRaw) {
		t.Fatal("postimage-only recovery produced a non-identical receipt")
	}
}

func TestBacklogCheckoutApplicationAppliesMultipleFilesAndHunksExactly(t *testing.T) {
	patch := backlogCheckoutMultiPatch()
	artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifactsForPatch(t, patch, `["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]`)
	chain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_multi_fixture_015", "checkout_multi_replay_015", backlogSemanticReviewApproved)
	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"))); got != "# Fixture package\nFirst addition.\nSecond addition.\n" {
		t.Fatalf("unexpected first multi-file postimage: %q", got)
	}
	if got := string(mustReadFile(t, filepath.Join(consumerRoot, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md"))); got != "# TASK-015 Backlog remote fixture\n\nReviewed task body.\n" {
		t.Fatalf("unexpected second multi-file postimage: %q", got)
	}
	receipt := readJSONMap(filepath.Join(artifactRoot, "checkout-application.json"))
	application := mapValue(receipt["application"])
	if int(application["files_applied"].(float64)) != 2 || int(application["hunks_applied"].(float64)) != 2 {
		t.Fatalf("unexpected multi-file receipt counts: %#v", application)
	}
}

func TestBacklogCheckoutApplicationRejectedDecisionNeverMutatesOrEmitsSuccess(t *testing.T) {
	artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
	chain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_rejected_fixture_015", "checkout_rejected_replay_015", backlogSemanticReviewRejected)
	consumerPath := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
	before := mustReadFile(t, consumerPath)
	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
		t.Fatal(err)
	}
	if got := mustReadFile(t, consumerPath); !bytesEqual(got, before) {
		t.Fatal("rejected checkout application mutated the consumer")
	}
	approval := readJSONMap(filepath.Join(artifactRoot, "checkout-application-approval.json"))
	if stringValue(mapValue(approval["decision"])["value"]) != backlogSemanticReviewRejected || backlogTestBool(mapValue(approval["capabilities"])["apply_exact_patch_once"]) {
		t.Fatalf("unexpected rejected approval: %#v", approval)
	}
	assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
	if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
		t.Fatalf("rejected restart failed: %v", err)
	}
	assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
}

func TestBacklogCheckoutApplicationRequiresSeparateValidApprovalAndReadiness(t *testing.T) {
	t.Run("readiness alone", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		before := mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"))
		err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, filepath.Join(t.TempDir(), "missing.json"))
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_fixture_missing:") {
			t.Fatalf("missing approval returned %v", err)
		}
		assertBacklogCheckoutApplicationAbsent(t, artifactRoot)
		if !bytesEqual(before, mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"))) {
			t.Fatal("readiness alone mutated the consumer")
		}
	})

	t.Run("semantic approval alone", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
		if err != nil {
			t.Fatal(err)
		}
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_missing_ready_015", "checkout_missing_ready_replay_015", backlogSemanticReviewApproved)
		if err := os.Remove(filepath.Join(artifactRoot, "ready-for-review.json")); err != nil {
			t.Fatal(err)
		}
		err = applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_chain_invalid:") {
			t.Fatalf("semantic approval without readiness returned %v", err)
		}
		assertBacklogCheckoutApplicationAbsent(t, artifactRoot)
	})
}

func TestBacklogCheckoutApplicationRejectsMalformedUnknownAndWrongBindings(t *testing.T) {
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
			payload["decision"] = "apply"
			return marshalBacklogTestJSON(t, payload)
		}},
		{name: "unbounded_scope", mutate: func(payload map[string]any) []byte {
			payload["application_scope"] = "entire_checkout"
			return marshalBacklogTestJSON(t, payload)
		}},
		{name: "malformed", mutate: func(map[string]any) []byte { return []byte(`{"decision":`) }},
		{name: "wrong_task", mutate: mutateBacklogCheckoutFixtureField(t, "task_id", "TASK-014")},
		{name: "wrong_readiness", mutate: mutateBacklogCheckoutFixtureField(t, "readiness_fingerprint", "sha256:"+strings.Repeat("1", 64))},
		{name: "wrong_semantic", mutate: mutateBacklogCheckoutFixtureField(t, "semantic_review_fingerprint", "sha256:"+strings.Repeat("2", 64))},
		{name: "wrong_patch", mutate: mutateBacklogCheckoutFixtureField(t, "patch_sha256", "sha256:"+strings.Repeat("3", 64))},
		{name: "wrong_paths", mutate: mutateBacklogCheckoutFixtureField(t, "changed_paths", []any{"packages/dorkpipe/OTHER.md"})},
		{name: "wrong_execution", mutate: mutateBacklogCheckoutFixtureField(t, "validation_execution_fingerprint", "sha256:"+strings.Repeat("4", 64))},
		{name: "wrong_upstream", mutate: mutateBacklogCheckoutFixtureField(t, "remote_result_fingerprint", "sha256:"+strings.Repeat("5", 64))},
		{name: "wrong_preimages", mutate: mutateBacklogCheckoutFixtureField(t, "consumer_preimage_fingerprint", "sha256:"+strings.Repeat("6", 64))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
			chain, err := loadBacklogCheckoutApplicationChain(artifactRoot)
			if err != nil {
				t.Fatal(err)
			}
			fixtureValue := backlogCheckoutApplicationFixtureForChain(chain, "checkout_invalid_fixture_015", "checkout_invalid_replay_015", backlogSemanticReviewApproved)
			payload := map[string]any{}
			if err := json.Unmarshal(marshalBacklogTestJSON(t, fixtureValue), &payload); err != nil {
				t.Fatal(err)
			}
			fixturePath := filepath.Join(t.TempDir(), "checkout-approval.json")
			if err := os.WriteFile(fixturePath, test.mutate(payload), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixturePath); err == nil {
				t.Fatal("invalid checkout application approval was accepted")
			}
			assertBacklogCheckoutApplicationAbsent(t, artifactRoot)
		})
	}
}

func TestBacklogCheckoutApplicationRejectsStaleMixedMissingLinkedAndTamperedState(t *testing.T) {
	t.Run("stale preimage", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_stale_fixture_015", "checkout_stale_replay_015", backlogSemanticReviewApproved)
		writeBacklogTestFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"), "# Stale consumer\n")
		err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_mixed_state:") {
			t.Fatalf("stale preimage returned %v", err)
		}
		assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
	})

	for _, kind := range []string{"missing", "non_regular", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
			chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
			fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_source_fixture_015", "checkout_source_replay_015", backlogSemanticReviewApproved)
			path := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "non_regular":
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(consumerRoot, "target.md")
				writeBacklogTestFile(t, target, "# Fixture package\n")
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
			if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_source_invalid:") {
				t.Fatalf("%s source returned %v", kind, err)
			}
			assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
		})
	}

	t.Run("tampered readiness", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_tamper_fixture_015", "checkout_tamper_replay_015", backlogSemanticReviewApproved)
		mutateBacklogJSONFile(t, filepath.Join(artifactRoot, "ready-for-review.json"), func(payload map[string]any) { payload["state"] = "tampered" })
		err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_chain_invalid:") {
			t.Fatalf("tampered readiness returned %v", err)
		}
		assertBacklogCheckoutApplicationAbsent(t, artifactRoot)
	})
}

func TestBacklogCheckoutApplicationRollsBackMutationAndReportsRollbackAndCleanupFailures(t *testing.T) {
	t.Run("forced mutation failure restores exact preimage", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifactsForPatch(t, backlogCheckoutMultiPatch(), `["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]`)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_rollback_fixture_015", "checkout_rollback_replay_015", backlogSemanticReviewApproved)
		path := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
		secondPath := filepath.Join(consumerRoot, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md")
		before := mustReadFile(t, path)
		secondBefore := mustReadFile(t, secondPath)
		original := backlogCheckoutAfterMutation
		backlogCheckoutAfterMutation = func(int) error { return errors.New("injected after-mutation failure") }
		t.Cleanup(func() { backlogCheckoutAfterMutation = original })
		err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_mutation_failed:") {
			t.Fatalf("forced mutation failure returned %v", err)
		}
		if got := mustReadFile(t, path); !bytesEqual(got, before) {
			t.Fatal("forced mutation failure did not restore the exact preimage")
		}
		if got := mustReadFile(t, secondPath); !bytesEqual(got, secondBefore) {
			t.Fatal("forced mutation failure did not preserve every changed file at the exact preimage")
		}
		assertNoBacklogCheckoutTemporaryFiles(t, consumerRoot)
		assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
	})

	t.Run("rollback failure is distinct", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_rollback_fail_fixture_015", "checkout_rollback_fail_replay_015", backlogSemanticReviewApproved)
		originalAfter := backlogCheckoutAfterMutation
		originalRename := backlogCheckoutRename
		backlogCheckoutAfterMutation = func(int) error { return errors.New("injected after-mutation failure") }
		renames := 0
		backlogCheckoutRename = func(oldPath, newPath string) error {
			renames++
			if renames == 2 {
				return errors.New("injected rollback failure")
			}
			return originalRename(oldPath, newPath)
		}
		t.Cleanup(func() { backlogCheckoutAfterMutation = originalAfter; backlogCheckoutRename = originalRename })
		err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_rollback_failed:") {
			t.Fatalf("rollback failure returned %v", err)
		}
		assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
	})

	t.Run("cleanup failure rolls back and is distinct", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_cleanup_fail_fixture_015", "checkout_cleanup_fail_replay_015", backlogSemanticReviewApproved)
		path := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
		before := mustReadFile(t, path)
		originalRemove := backlogCheckoutRemove
		calls := 0
		backlogCheckoutRemove = func(path string) error {
			calls++
			if calls == 1 {
				return errors.New("injected cleanup failure")
			}
			return originalRemove(path)
		}
		t.Cleanup(func() { backlogCheckoutRemove = originalRemove })
		err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture)
		if err == nil || !strings.HasPrefix(err.Error(), "checkout_application_cleanup_failed:") {
			t.Fatalf("cleanup failure returned %v", err)
		}
		if got := mustReadFile(t, path); !bytesEqual(got, before) {
			t.Fatal("cleanup failure did not roll back the consumer")
		}
		assertNoBacklogCheckoutTemporaryFiles(t, consumerRoot)
		assertBacklogCheckoutApplicationReceiptAbsent(t, artifactRoot)
	})
}

func backlogCheckoutMultiPatch() string {
	return "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\n" +
		"index 1111111..2222222 100644\n--- a/packages/dorkpipe/README.md\n+++ b/packages/dorkpipe/README.md\n" +
		"@@ -1 +1,3 @@\n # Fixture package\n+First addition.\n+Second addition.\n" +
		"diff --git a/docs/agents/tasks/backlog-driven-remote-tasks.md b/docs/agents/tasks/backlog-driven-remote-tasks.md\n" +
		"index 3333333..4444444 100644\n--- a/docs/agents/tasks/backlog-driven-remote-tasks.md\n+++ b/docs/agents/tasks/backlog-driven-remote-tasks.md\n" +
		"@@ -1,3 +1,3 @@\n # TASK-015 Backlog remote fixture\n \n-Fixture task body.\n+Reviewed task body.\n"
}

func TestBacklogCheckoutApplicationRejectsDuplicateReplayDecisionChangeAndArtifactTampering(t *testing.T) {
	tests := []struct {
		name, approvalID, replayID, decision, code string
	}{
		{name: "duplicate", approvalID: "checkout_original_fixture_015", replayID: "checkout_second_replay_015", decision: backlogSemanticReviewApproved, code: "checkout_application_duplicate:"},
		{name: "replay", approvalID: "checkout_second_fixture_015", replayID: "checkout_original_replay_015", decision: backlogSemanticReviewApproved, code: "checkout_application_replay:"},
		{name: "decision_change", approvalID: "checkout_original_fixture_015", replayID: "checkout_original_replay_015", decision: backlogSemanticReviewApproved, code: "checkout_application_decision_conflict:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
			chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
			first := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_original_fixture_015", "checkout_original_replay_015", backlogSemanticReviewRejected)
			if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, first); err != nil {
				t.Fatal(err)
			}
			accepted := mustReadFile(t, filepath.Join(artifactRoot, "checkout-application-approval.json"))
			second := writeBacklogCheckoutApplicationTestFixture(t, chain, test.approvalID, test.replayID, test.decision)
			err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, second)
			if err == nil || !strings.HasPrefix(err.Error(), test.code) {
				t.Fatalf("%s returned %v", test.name, err)
			}
			if !bytesEqual(accepted, mustReadFile(t, filepath.Join(artifactRoot, "checkout-application-approval.json"))) {
				t.Fatal("accepted approval was overwritten")
			}
		})
	}

	t.Run("tampered existing approval", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_artifact_fixture_015", "checkout_artifact_replay_015", backlogSemanticReviewRejected)
		if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(artifactRoot, "checkout-application-approval.json")
		mutateBacklogJSONFile(t, path, func(payload map[string]any) { payload["state"] = "tampered" })
		tampered := mustReadFile(t, path)
		if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err == nil {
			t.Fatal("tampered approval was accepted")
		}
		if !bytesEqual(tampered, mustReadFile(t, path)) {
			t.Fatal("tampered approval was overwritten")
		}
	})

	t.Run("tampered existing receipt", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogCheckoutApplicationArtifacts(t)
		chain, _ := loadBacklogCheckoutApplicationChain(artifactRoot)
		fixture := writeBacklogCheckoutApplicationTestFixture(t, chain, "checkout_receipt_fixture_015", "checkout_receipt_replay_015", backlogSemanticReviewApproved)
		if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(artifactRoot, "checkout-application.json")
		mutateBacklogJSONFile(t, path, func(payload map[string]any) { payload["state"] = "tampered" })
		tampered := mustReadFile(t, path)
		if err := applyBacklogPatchToCheckout(consumerRoot, artifactRoot, fixture); err == nil {
			t.Fatal("tampered receipt was accepted")
		}
		if !bytesEqual(tampered, mustReadFile(t, path)) {
			t.Fatal("tampered receipt was overwritten")
		}
	})
}

func prepareBacklogCheckoutApplicationArtifacts(t *testing.T) (string, string) {
	t.Helper()
	artifactRoot, consumerRoot := prepareBacklogSemanticReviewArtifacts(t, 0)
	chain, err := loadBacklogSemanticReviewChain(artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixture := writeBacklogSemanticReviewTestFixture(t, chain, "review_checkout_fixture_015", "review_checkout_replay_015", backlogSemanticReviewApproved)
	if err := recordBacklogSemanticReviewDecision(artifactRoot, fixture); err != nil {
		t.Fatal(err)
	}
	return artifactRoot, consumerRoot
}

func prepareBacklogCheckoutApplicationArtifactsForPatch(t *testing.T, patch, allowedPathsJSON string) (string, string) {
	t.Helper()
	consumer := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(consumer, backlogIndexPath, "TASK-015", "Implement only bounded checkout application.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(consumer, root, "fixture-environment", "js/dev", allowedPathsJSON, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`, `[]`); err != nil {
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
	if err := ingestBacklogCompletionCandidate(root, writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogRemoteStatusFixture(root, writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogRemoteDiffFixture(root, writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", patch)); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogRemoteResultFixture(root, writeBacklogResultFixture(t, root, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogValidationReceiptFixture(root, writeBacklogValidationReceiptFixture(t, root, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")); err != nil {
		t.Fatal(err)
	}
	if err := verifyBacklogPatchBoundary(root); err != nil {
		t.Fatal(err)
	}
	if err := applyBacklogPatchTemporaryCopy(consumer, root); err != nil {
		t.Fatal(err)
	}
	originalRunner := backlogValidationRunCommand
	backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) { return 0, true, nil }
	t.Cleanup(func() { backlogValidationRunCommand = originalRunner })
	if err := executeBacklogValidation(consumer, root); err != nil {
		t.Fatal(err)
	}
	semantic, err := loadBacklogSemanticReviewChain(root)
	if err != nil {
		t.Fatal(err)
	}
	decision := writeBacklogSemanticReviewTestFixture(t, semantic, "review_checkout_multi_015", "review_checkout_multi_replay_015", backlogSemanticReviewApproved)
	if err := recordBacklogSemanticReviewDecision(root, decision); err != nil {
		t.Fatal(err)
	}
	return root, consumer
}

func writeBacklogCheckoutApplicationTestFixture(t *testing.T, chain *backlogCheckoutApplicationChain, approvalID, replayIdentity, decision string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "checkout-application-approval.json")
	raw := marshalBacklogTestJSON(t, backlogCheckoutApplicationFixtureForChain(chain, approvalID, replayIdentity, decision))
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mutateBacklogCheckoutFixtureField(t *testing.T, field string, value any) func(map[string]any) []byte {
	t.Helper()
	return func(payload map[string]any) []byte { payload[field] = value; return marshalBacklogTestJSON(t, payload) }
}

func snapshotBacklogCheckoutApplicationUpstream(t *testing.T, root string) map[string][]byte {
	t.Helper()
	upstream := snapshotBacklogSemanticReviewUpstream(t, root)
	upstream["semantic-review-decision.json"] = mustReadFile(t, filepath.Join(root, "semantic-review-decision.json"))
	upstream["ready-for-review.json"] = mustReadFile(t, filepath.Join(root, "ready-for-review.json"))
	return upstream
}

func assertBacklogCheckoutApplicationUpstreamUnchanged(t *testing.T, root string, upstream map[string][]byte) {
	t.Helper()
	for name, expected := range upstream {
		if got := mustReadFile(t, filepath.Join(root, name)); !bytesEqual(got, expected) {
			t.Fatalf("checkout application changed upstream artifact %s", name)
		}
	}
}

func assertBacklogCheckoutApplicationReceiptAbsent(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, "checkout-application.json")); !os.IsNotExist(err) {
		t.Fatalf("checkout application receipt exists unexpectedly: %v", err)
	}
}

func assertBacklogCheckoutApplicationAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"checkout-application-approval.json", "checkout-application.json"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("checkout application rejection left %s: %v", name, err)
		}
	}
}

func assertNoBacklogCheckoutTemporaryFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".dorkpipe-apply-") || strings.HasPrefix(entry.Name(), ".dorkpipe-rollback-") || strings.HasPrefix(entry.Name(), ".dorkpipe-restore-") {
			t.Fatalf("checkout application temporary file survived: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
