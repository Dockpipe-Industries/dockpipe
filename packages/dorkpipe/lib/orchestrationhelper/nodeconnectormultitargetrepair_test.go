package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNodeConnectorMultiTargetRepairApprovedOneMultipleAndAllFailedTargets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failed []int
	}{
		{name: "one", failed: []int{1}},
		{name: "multiple", failed: []int{0, 2}},
		{name: "all", failed: []int{0, 1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, expected, fixture, aggregate := nodeConnectorMultiTargetRepairFixture(t, tc.failed, "approved")
			decision, request := mustDecideNodeConnectorMultiTargetRepair(t, mustOpenNodeConnectorMultiTargetRepair(t, root, expected), fixture)
			if decision.Decision != "approved" || request == nil || request.RequestID != fixture.RepairRequestID || request.RepairDispatched {
				t.Fatal("approved decision did not emit its one exact undispatched repair request")
			}
			if decision.Authority != (NodeConnectorMultiTargetValidationAuthority{}) || request.Authority != (NodeConnectorMultiTargetValidationAuthority{}) {
				t.Fatal("repair decision or request gained execution or lifecycle authority")
			}
			if len(request.Targets) != len(tc.failed) || request.AggregateFingerprint != aggregate.AggregateFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint {
				t.Fatal("repair request omitted an exact aggregate, decision, or failed-target binding")
			}
			for index, binding := range request.Targets {
				if binding.TargetID != aggregate.FailedTargetIDs[index] || binding.Outcome != "failed" || !reflect.DeepEqual(binding, aggregate.ReceiptBindings[nodeConnectorMultiTargetRepairBindingIndex(aggregate, binding.TargetID)]) {
					t.Fatal("repair request did not preserve the exact immutable failed-target binding")
				}
			}
			decisionRaw := mustReadNodeConnectorMultiTargetRepairFile(t, root, nodeConnectorMultiTargetRepairDecisionName)
			requestRaw := mustReadNodeConnectorMultiTargetRepairFile(t, root, nodeConnectorMultiTargetRepairRequestName)
			if len(decisionRaw) > nodeConnectorMultiTargetRepairMaxArtifactBytes || len(requestRaw) > nodeConnectorMultiTargetRepairMaxArtifactBytes ||
				decisionRaw[len(decisionRaw)-1] != '\n' || requestRaw[len(requestRaw)-1] != '\n' {
				t.Fatal("repair artifacts are not bounded canonical newline-terminated JSON")
			}
		})
	}
}

func TestNodeConnectorMultiTargetRepairRejectsPassedAggregateAndRejectedDecisionEmitsNoRequest(t *testing.T) {
	expectedValidation, input := nodeConnectorMultiTargetValidationFixture(t)
	passedRoot := t.TempDir()
	passedAggregate := mustAcceptNodeConnectorMultiTargetValidation(t, mustOpenNodeConnectorMultiTargetValidation(t, passedRoot, expectedValidation), input)
	passedExpected := NodeConnectorMultiTargetRepairExpected{
		Aggregate: expectedValidation, AggregateFingerprint: passedAggregate.AggregateFingerprint,
		FailedTargetIDs: []string{passedAggregate.Targets[0].TargetID},
	}
	if _, err := OpenNodeConnectorMultiTargetRepair(passedRoot, passedExpected); err == nil {
		t.Fatal("passed aggregate was accepted as repair input")
	}

	root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{1}, "rejected")
	repair := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	decision, request := mustDecideNodeConnectorMultiTargetRepair(t, repair, fixture)
	if decision.Decision != "rejected" || decision.RepairRequestID != "" || request != nil {
		t.Fatal("rejected decision emitted or bound a repair request")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorMultiTargetRepairRequestName)); !os.IsNotExist(err) {
		t.Fatal("rejected decision published a repair request artifact")
	}
	restarted := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	replayedDecision, replayedRequest := mustDecideNodeConnectorMultiTargetRepair(t, restarted, fixture)
	if !reflect.DeepEqual(decision, replayedDecision) || replayedRequest != nil {
		t.Fatal("rejected decision restart changed state or created a request")
	}
}

func TestNodeConnectorMultiTargetRepairRequiresExactFailedTargetSetAndAggregate(t *testing.T) {
	root, expected, fixture, aggregate := nodeConnectorMultiTargetRepairFixture(t, []int{0, 2}, "approved")
	cases := []struct {
		name   string
		mutate func(*NodeConnectorMultiTargetRepairDecisionFixture)
	}{
		{name: "missing", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.FailedTargetIDs = value.FailedTargetIDs[:1]
		}},
		{name: "duplicate", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.FailedTargetIDs[1] = value.FailedTargetIDs[0]
		}},
		{name: "extra", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.FailedTargetIDs = append(value.FailedTargetIDs, aggregate.Targets[1].TargetID)
		}},
		{name: "reordered", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.FailedTargetIDs[0], value.FailedTargetIDs[1] = value.FailedTargetIDs[1], value.FailedTargetIDs[0]
		}},
		{name: "substituted", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.FailedTargetIDs[0] = aggregate.Targets[1].TargetID
		}},
		{name: "wrong aggregate id", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.AggregateID = "aggregate-multi-target-validation-wrong"
		}},
		{name: "wrong aggregate fingerprint", mutate: func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.AggregateFingerprint = "sha256:" + strings.Repeat("f", 64)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneNodeConnectorMultiTargetRepairDecisionFixture(fixture)
			tc.mutate(&changed)
			if _, _, err := mustOpenNodeConnectorMultiTargetRepair(t, root, expected).Decide(mustMarshalNodeConnectorMultiTargetRepair(t, changed)); err == nil {
				t.Fatal("inexact failed-target or aggregate binding was accepted")
			}
			if _, err := os.Lstat(filepath.Join(root, nodeConnectorMultiTargetRepairDecisionName)); !os.IsNotExist(err) {
				t.Fatal("rejected decision published durable state")
			}
		})
	}
}

func TestNodeConnectorMultiTargetRepairRejectsCrossTargetRequestBindingSubstitution(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*NodeConnectorMultiTargetRepairRequest)
	}{
		{name: "machine", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].MachineID = value.Targets[1].MachineID
		}},
		{name: "capability", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].CapabilitySnapshotID = value.Targets[1].CapabilitySnapshotID
		}},
		{name: "operation", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].OperationID = value.Targets[1].OperationID
		}},
		{name: "request", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].RequestFingerprint = value.Targets[1].RequestFingerprint
		}},
		{name: "lease", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].LeaseID = value.Targets[1].LeaseID
		}},
		{name: "receipt", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].ReceiptID = value.Targets[1].ReceiptID
		}},
		{name: "profile", mutate: func(value *NodeConnectorMultiTargetRepairRequest) {
			value.Targets[0].Profile = value.Targets[1].Profile
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{0, 2}, "approved")
			_, request := mustDecideNodeConnectorMultiTargetRepair(t, mustOpenNodeConnectorMultiTargetRepair(t, root, expected), fixture)
			changed := cloneNodeConnectorMultiTargetRepairRequest(*request)
			tc.mutate(&changed)
			changed.RequestFingerprint = ""
			fingerprint, err := nodeConnectorMultiTargetRepairRequestFingerprint(changed)
			if err != nil {
				t.Fatal(err)
			}
			changed.RequestFingerprint = fingerprint
			mustWriteCanonicalNodeConnectorMultiTargetRepair(t, filepath.Join(root, nodeConnectorMultiTargetRepairRequestName), changed)
			if _, err := OpenNodeConnectorMultiTargetRepair(root, expected); err == nil {
				t.Fatal("cross-target immutable request binding substitution was accepted")
			}
		})
	}
}

func TestNodeConnectorMultiTargetRepairRejectsMalformedNoncanonicalOversizedAndContextClaims(t *testing.T) {
	root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{1}, "approved")
	valid := mustMarshalNodeConnectorMultiTargetRepair(t, fixture)
	repair := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	inputs := [][]byte{[]byte("{not-json"), make([]byte, nodeConnectorMultiTargetRepairMaxDecisionBytes+1)}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs = append(inputs, pretty.Bytes())
	for _, field := range []string{"unknown", "provider", "connection", "availability", "ingress", "quota"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":{"authoritative":true}}`)...))
	}
	for _, raw := range inputs {
		if _, _, err := repair.Decide(raw); err == nil {
			t.Fatal("malformed, noncanonical, oversized, unknown, or contextual decision input was accepted")
		}
	}
	if decision, request := repair.Artifacts(); decision != nil || request != nil {
		t.Fatal("rejected decision input published durable state")
	}
}

func TestNodeConnectorMultiTargetRepairReplayRestartConflictChangedExpectationsAndTamperFailClosed(t *testing.T) {
	root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{0, 2}, "approved")
	repair := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	acceptedDecision, acceptedRequest := mustDecideNodeConnectorMultiTargetRepair(t, repair, fixture)

	originalDecisionWriter := nodeConnectorMultiTargetRepairWriteDecisionAtomic
	originalRequestWriter := nodeConnectorMultiTargetRepairWriteRequestAtomic
	writes := 0
	nodeConnectorMultiTargetRepairWriteDecisionAtomic = func(path string, value any) error { writes++; return originalDecisionWriter(path, value) }
	nodeConnectorMultiTargetRepairWriteRequestAtomic = func(path string, value any) error { writes++; return originalRequestWriter(path, value) }
	t.Cleanup(func() {
		nodeConnectorMultiTargetRepairWriteDecisionAtomic = originalDecisionWriter
		nodeConnectorMultiTargetRepairWriteRequestAtomic = originalRequestWriter
	})
	if decision, request := mustDecideNodeConnectorMultiTargetRepair(t, repair, fixture); !reflect.DeepEqual(decision, acceptedDecision) || !reflect.DeepEqual(request, acceptedRequest) || writes != 0 {
		t.Fatal("exact decision replay changed or rewrote durable artifacts")
	}
	restarted := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	if decision, request := mustDecideNodeConnectorMultiTargetRepair(t, restarted, fixture); !reflect.DeepEqual(decision, acceptedDecision) || !reflect.DeepEqual(request, acceptedRequest) || writes != 0 {
		t.Fatal("restart replay changed or rewrote durable artifacts")
	}
	for _, mutate := range []func(*NodeConnectorMultiTargetRepairDecisionFixture){
		func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.DecisionID = "decision-repair-conflict-001"
		},
		func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.ReplayID = "replay-repair-conflict-001"
		},
		func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.RepairRequestID = "repair-request-conflict-001"
		},
		func(value *NodeConnectorMultiTargetRepairDecisionFixture) {
			value.Decision = "rejected"
			value.RepairRequestID = ""
		},
	} {
		changed := cloneNodeConnectorMultiTargetRepairDecisionFixture(fixture)
		mutate(&changed)
		if _, _, err := restarted.Decide(mustMarshalNodeConnectorMultiTargetRepair(t, changed)); err == nil || writes != 0 {
			t.Fatal("conflicting decision or request replay was accepted or rewrote state")
		}
	}

	changedExpected := expected
	changedExpected.FailedTargetIDs = append([]string{}, expected.FailedTargetIDs[:1]...)
	if _, err := OpenNodeConnectorMultiTargetRepair(root, changedExpected); err == nil {
		t.Fatal("changed expected failed-target bindings were accepted")
	}
	changedExpected = expected
	changedExpected.AggregateFingerprint = "sha256:" + strings.Repeat("f", 64)
	if _, err := OpenNodeConnectorMultiTargetRepair(root, changedExpected); err == nil {
		t.Fatal("changed expected aggregate fingerprint was accepted")
	}

	decisionPath := filepath.Join(root, nodeConnectorMultiTargetRepairDecisionName)
	decisionRaw := mustReadNodeConnectorMultiTargetRepairFile(t, root, nodeConnectorMultiTargetRepairDecisionName)
	tamperedDecision := bytes.Replace(decisionRaw, []byte(`"decision": "approved"`), []byte(`"decision": "rejected"`), 1)
	if err := os.WriteFile(decisionPath, tamperedDecision, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorMultiTargetRepair(root, expected); err == nil {
		t.Fatal("tampered durable decision was accepted")
	}
	if err := os.WriteFile(decisionPath, decisionRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, nodeConnectorMultiTargetRepairRequestName)
	requestRaw := mustReadNodeConnectorMultiTargetRepairFile(t, root, nodeConnectorMultiTargetRepairRequestName)
	tamperedRequest := bytes.Replace(requestRaw, []byte(`"repair_dispatched": false`), []byte(`"repair_dispatched": true`), 1)
	if err := os.WriteFile(requestPath, tamperedRequest, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorMultiTargetRepair(root, expected); err == nil {
		t.Fatal("tampered durable request was accepted")
	}
}

func TestNodeConnectorMultiTargetRepairRevalidatesAggregateOnEveryDecision(t *testing.T) {
	root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{1}, "approved")
	repair := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	aggregatePath := filepath.Join(root, nodeConnectorMultiTargetValidationArtifactName)
	raw := mustReadNodeConnectorMultiTargetRepairFile(t, root, nodeConnectorMultiTargetValidationArtifactName)
	tampered := bytes.Replace(raw, []byte(`"status": "failed"`), []byte(`"status": "passed"`), 1)
	if err := os.WriteFile(aggregatePath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repair.Decide(mustMarshalNodeConnectorMultiTargetRepair(t, fixture)); err == nil {
		t.Fatal("decision accepted after direct aggregate tampering")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorMultiTargetRepairDecisionName)); !os.IsNotExist(err) {
		t.Fatal("aggregate tampering published a decision")
	}
}

func TestNodeConnectorMultiTargetRepairAtomicFailuresAndDurableDecisionRecovery(t *testing.T) {
	root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{1}, "approved")
	repair := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	originalDecisionWriter := nodeConnectorMultiTargetRepairWriteDecisionAtomic
	originalRequestWriter := nodeConnectorMultiTargetRepairWriteRequestAtomic
	t.Cleanup(func() {
		nodeConnectorMultiTargetRepairWriteDecisionAtomic = originalDecisionWriter
		nodeConnectorMultiTargetRepairWriteRequestAtomic = originalRequestWriter
	})
	nodeConnectorMultiTargetRepairWriteDecisionAtomic = func(string, any) error { return errors.New("injected decision write failure") }
	if _, _, err := repair.Decide(mustMarshalNodeConnectorMultiTargetRepair(t, fixture)); err == nil {
		t.Fatal("decision atomic-write failure was accepted")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 {
		t.Fatalf("decision write failure published a repair artifact: %#v, %v", entries, err)
	}

	nodeConnectorMultiTargetRepairWriteDecisionAtomic = originalDecisionWriter
	nodeConnectorMultiTargetRepairWriteRequestAtomic = func(string, any) error { return errors.New("injected request write failure") }
	if _, _, err := repair.Decide(mustMarshalNodeConnectorMultiTargetRepair(t, fixture)); err == nil {
		t.Fatal("request atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorMultiTargetRepairDecisionName)); err != nil {
		t.Fatal("approved decision was not durable before request failure")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorMultiTargetRepairRequestName)); !os.IsNotExist(err) {
		t.Fatal("request atomic-write failure published a partial request")
	}

	nodeConnectorMultiTargetRepairWriteRequestAtomic = originalRequestWriter
	restarted := mustOpenNodeConnectorMultiTargetRepair(t, root, expected)
	decision, request := mustDecideNodeConnectorMultiTargetRepair(t, restarted, fixture)
	if decision.Decision != "approved" || request == nil || request.RequestID != fixture.RepairRequestID {
		t.Fatal("restart did not safely publish the one request bound by the durable decision")
	}
}

func TestNodeConnectorMultiTargetRepairJSONShapeLeaksNoAuthorityOrRawEvidence(t *testing.T) {
	root, expected, fixture, _ := nodeConnectorMultiTargetRepairFixture(t, []int{0, 1, 2}, "approved")
	decision, request := mustDecideNodeConnectorMultiTargetRepair(t, mustOpenNodeConnectorMultiTargetRepair(t, root, expected), fixture)
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeConnectorMultiTargetRepairDecisionFixture{}, []string{
		"schema", "decision_id", "replay_id", "decision", "aggregate_id", "aggregate_fingerprint", "failed_target_ids", "repair_request_id", "provenance",
	})
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeConnectorMultiTargetRepairDecision{}, []string{
		"schema", "decision_id", "replay_id", "decision", "aggregate_id", "aggregate_fingerprint", "failed_target_ids", "repair_request_id", "provenance", "authority", "decision_fingerprint",
	})
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeConnectorMultiTargetRepairRequest{}, []string{
		"schema", "request_id", "decision_id", "decision_fingerprint", "aggregate_id", "aggregate_fingerprint", "targets", "provenance", "repair_dispatched", "authority", "request_fingerprint",
	})
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeConnectorMultiTargetValidationReceiptBinding{}, []string{
		"target_id", "profile", "machine_id", "machine_fingerprint", "capability_snapshot_id", "operation_id", "request_fingerprint", "lease_id", "lease_fingerprint", "attempt", "events_fingerprint", "receipt_id", "receipt_fingerprint", "local_run_id", "final_cursor", "artifact_manifest_fingerprint", "terminal_result", "cleanup_status", "cleanup_evidence_digest", "outcome",
	})
	raw, err := json.Marshal(struct {
		Decision NodeConnectorMultiTargetRepairDecision `json:"decision"`
		Request  NodeConnectorMultiTargetRepairRequest  `json:"request"`
	}{Decision: decision, Request: *request})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"command", "instruction", "replacement", "retry_count", "schedule_at", "provider", "connection", "availability", "ingress", "quota", "credential", "raw_event", "raw_receipt", "workspace_path", "filesystem_path", "hostname", "endpoint"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("repair contract leaked forbidden field or evidence %q", forbidden)
		}
	}
	validatorCalls, executorCalls, schedulerCalls, transportCalls, providerCalls, gitCalls, repairCalls := 0, 0, 0, 0, 0, 0, 0
	_ = []func(){func() { validatorCalls++ }, func() { executorCalls++ }, func() { schedulerCalls++ }, func() { transportCalls++ }, func() { providerCalls++ }, func() { gitCalls++ }, func() { repairCalls++ }}
	if validatorCalls+executorCalls+schedulerCalls+transportCalls+providerCalls+gitCalls+repairCalls != 0 {
		t.Fatal("repair decision/request invoked a forbidden callback")
	}
}

func nodeConnectorMultiTargetRepairFixture(t *testing.T, failed []int, decision string) (string, NodeConnectorMultiTargetRepairExpected, NodeConnectorMultiTargetRepairDecisionFixture, NodeConnectorMultiTargetValidationAggregate) {
	t.Helper()
	expectedValidation, input := nodeConnectorMultiTargetValidationFixture(t)
	for _, index := range failed {
		input.Targets[index].Receipt.Result = "failed"
		input.Targets[index].Receipt = mustFinalizeNodeConnectorMultiTargetValidationReceipt(t, input.Targets[index].Receipt)
		expectedValidation.Targets[index].ReceiptFingerprint = input.Targets[index].Receipt.ReceiptFingerprint
	}
	root := t.TempDir()
	aggregate := mustAcceptNodeConnectorMultiTargetValidation(t, mustOpenNodeConnectorMultiTargetValidation(t, root, expectedValidation), input)
	expected := NodeConnectorMultiTargetRepairExpected{
		Aggregate: expectedValidation, AggregateFingerprint: aggregate.AggregateFingerprint,
		FailedTargetIDs: append([]string{}, aggregate.FailedTargetIDs...),
	}
	fixture := NodeConnectorMultiTargetRepairDecisionFixture{
		Schema: NodeConnectorMultiTargetRepairDecisionFixtureSchema, DecisionID: "decision-multi-target-repair-001", ReplayID: "replay-multi-target-repair-001",
		Decision: decision, AggregateID: aggregate.AggregateID, AggregateFingerprint: aggregate.AggregateFingerprint,
		FailedTargetIDs: append([]string{}, aggregate.FailedTargetIDs...), Provenance: nodeConnectorMultiTargetRepairDecisionProvenance,
	}
	if decision == "approved" {
		fixture.RepairRequestID = "repair-request-multi-target-001"
	}
	return root, expected, fixture, aggregate
}

func mustOpenNodeConnectorMultiTargetRepair(t *testing.T, root string, expected NodeConnectorMultiTargetRepairExpected) *NodeConnectorMultiTargetRepair {
	t.Helper()
	repair, err := OpenNodeConnectorMultiTargetRepair(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	return repair
}

func mustDecideNodeConnectorMultiTargetRepair(t *testing.T, repair *NodeConnectorMultiTargetRepair, fixture NodeConnectorMultiTargetRepairDecisionFixture) (NodeConnectorMultiTargetRepairDecision, *NodeConnectorMultiTargetRepairRequest) {
	t.Helper()
	decision, request, err := repair.Decide(mustMarshalNodeConnectorMultiTargetRepair(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorMultiTargetRepair(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorMultiTargetRepairDecisionFixture(value NodeConnectorMultiTargetRepairDecisionFixture) NodeConnectorMultiTargetRepairDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorMultiTargetRepairDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func mustReadNodeConnectorMultiTargetRepairFile(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustWriteCanonicalNodeConnectorMultiTargetRepair(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func nodeConnectorMultiTargetRepairBindingIndex(aggregate NodeConnectorMultiTargetValidationAggregate, targetID string) int {
	for index, binding := range aggregate.ReceiptBindings {
		if binding.TargetID == targetID {
			return index
		}
	}
	return -1
}
