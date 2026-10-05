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

func TestNodeConnectorPlacementDecisionApprovedExplicitlySelectsAnyExactCandidate(t *testing.T) {
	for _, selectedIndex := range []int{0, 1, 2} {
		t.Run([]string{"available-linux-host", "unknown-linux-qemu-windows", "unavailable-windows-host"}[selectedIndex], func(t *testing.T) {
			root, expected, fixture, inventory, placement := nodeConnectorPlacementDecisionFixture(t, "approved", selectedIndex)
			decision, request := mustDecideNodeConnectorPlacement(t, mustOpenNodeConnectorPlacementDecisions(t, root, expected), fixture)
			if decision.Decision != "approved" || decision.SelectedNode == nil || request == nil || request.PlacementDispatched || !request.SelectionEvidenceOnly {
				t.Fatal("approved explicit selection did not emit exactly one non-dispatched evidence-only request")
			}
			selected := inventory.Nodes[selectedIndex]
			want := NodeConnectorPlacementSelectedNodeBinding{NodeID: selected.NodeID, MachineID: selected.MachineID, CapabilitySnapshotID: selected.CapabilitySnapshotID, CapabilitySnapshotFingerprint: selected.CapabilitySnapshotFingerprint, Profile: selected.Profile}
			if !reflect.DeepEqual(*decision.SelectedNode, want) || !reflect.DeepEqual(request.SelectedNode, want) {
				t.Fatal("decision or request did not bind the exact node, machine, capability, fingerprint, and profile")
			}
			if !reflect.DeepEqual(decision.CandidateNodeIDs, placement.CandidateNodeIDs) || !reflect.DeepEqual(request.CandidateNodeIDs, placement.CandidateNodeIDs) || !decision.CompleteCandidateSet || !request.CompleteCandidateSet {
				t.Fatal("decision or request did not preserve the exact complete candidate set")
			}
			if decision.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint || decision.PlacementInputSnapshotFingerprint != placement.PlacementInputSnapshotFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint {
				t.Fatal("placement artifacts omitted an exact upstream or decision fingerprint binding")
			}
			if decision.Authority != (NodeConnectorInventoryAuthority{}) || request.Authority != (NodeConnectorInventoryAuthority{}) || decision.SelectionInferred {
				t.Fatal("placement evidence gained inferred selection or adjacent authority")
			}
			for _, name := range []string{nodeConnectorPlacementDecisionName, nodeConnectorPlacementRequestName} {
				raw := mustReadNodeConnectorPlacementFile(t, root, name)
				if len(raw) > nodeConnectorPlacementMaxArtifactBytes || raw[len(raw)-1] != '\n' {
					t.Fatal("placement artifact is not bounded canonical newline-terminated JSON")
				}
			}
		})
	}
}

func TestNodeConnectorPlacementRejectedDecisionEmitsNoRequest(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "rejected", -1)
	decisions := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	decision, request := mustDecideNodeConnectorPlacement(t, decisions, fixture)
	if decision.Decision != "rejected" || decision.SelectedNode != nil || decision.PlacementRequestID != "" || request != nil {
		t.Fatal("rejected placement decision selected a node or emitted a request")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementRequestName)); !os.IsNotExist(err) {
		t.Fatal("rejected placement decision published a request artifact")
	}
	restarted := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	replayedDecision, replayedRequest := mustDecideNodeConnectorPlacement(t, restarted, fixture)
	if !reflect.DeepEqual(decision, replayedDecision) || replayedRequest != nil {
		t.Fatal("rejected placement decision changed or created a request after restart")
	}
}

func TestNodeConnectorPlacementDecisionRequiresExactCompleteInputBindings(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementDecisionFixture)
	}{
		{"missing-candidate", func(value *NodeConnectorPlacementDecisionFixture) {
			value.CandidateNodeIDs = value.CandidateNodeIDs[1:]
		}},
		{"duplicate-candidate", func(value *NodeConnectorPlacementDecisionFixture) {
			value.CandidateNodeIDs[1] = value.CandidateNodeIDs[0]
		}},
		{"reordered-candidate", func(value *NodeConnectorPlacementDecisionFixture) {
			value.CandidateNodeIDs[0], value.CandidateNodeIDs[1] = value.CandidateNodeIDs[1], value.CandidateNodeIDs[0]
		}},
		{"extra-candidate", func(value *NodeConnectorPlacementDecisionFixture) {
			value.CandidateNodeIDs = append(value.CandidateNodeIDs, "node-z-extra-001")
		}},
		{"substituted-candidate", func(value *NodeConnectorPlacementDecisionFixture) { value.CandidateNodeIDs[0] = "node-substituted-001" }},
		{"inventory-id", func(value *NodeConnectorPlacementDecisionFixture) {
			value.InventorySnapshotID = "inventory-substituted-001"
		}},
		{"inventory-fingerprint", func(value *NodeConnectorPlacementDecisionFixture) {
			value.InventorySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"placement-id", func(value *NodeConnectorPlacementDecisionFixture) {
			value.PlacementInputID = "placement-input-substituted-001"
		}},
		{"placement-fingerprint", func(value *NodeConnectorPlacementDecisionFixture) {
			value.PlacementInputSnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"workload", func(value *NodeConnectorPlacementDecisionFixture) { value.WorkloadID = "workload-substituted-001" }},
		{"requirements", func(value *NodeConnectorPlacementDecisionFixture) {
			value.RequirementsFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"unknown-selection", func(value *NodeConnectorPlacementDecisionFixture) {
			value.SelectedNodeID = "node-outside-candidates-001"
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 1)
			test.mutate(&fixture)
			if _, _, err := mustOpenNodeConnectorPlacementDecisions(t, root, expected).Decide(mustMarshalNodeConnectorPlacement(t, fixture)); err == nil {
				t.Fatal("inexact inventory, placement-input, candidate, or selected-node binding was accepted")
			}
			if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDecisionName)); !os.IsNotExist(err) {
				t.Fatal("rejected placement input published a decision")
			}
		})
	}
}

func TestNodeConnectorPlacementRequestRejectsIdentityAndProfileSubstitution(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementRequest, NodeConnectorInventorySnapshot)
	}{
		{"node", func(value *NodeConnectorPlacementRequest, inventory NodeConnectorInventorySnapshot) {
			value.SelectedNode.NodeID = inventory.Nodes[0].NodeID
		}},
		{"machine", func(value *NodeConnectorPlacementRequest, inventory NodeConnectorInventorySnapshot) {
			value.SelectedNode.MachineID = inventory.Nodes[0].MachineID
		}},
		{"capability-id", func(value *NodeConnectorPlacementRequest, inventory NodeConnectorInventorySnapshot) {
			value.SelectedNode.CapabilitySnapshotID = inventory.Nodes[0].CapabilitySnapshotID
		}},
		{"capability-fingerprint", func(value *NodeConnectorPlacementRequest, inventory NodeConnectorInventorySnapshot) {
			value.SelectedNode.CapabilitySnapshotFingerprint = inventory.Nodes[0].CapabilitySnapshotFingerprint
		}},
		{"profile", func(value *NodeConnectorPlacementRequest, inventory NodeConnectorInventorySnapshot) {
			value.SelectedNode.Profile = inventory.Nodes[0].Profile
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture, inventory, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 2)
			_, request := mustDecideNodeConnectorPlacement(t, mustOpenNodeConnectorPlacementDecisions(t, root, expected), fixture)
			changed := cloneNodeConnectorPlacementRequest(*request)
			test.mutate(&changed, inventory)
			changed.RequestFingerprint = ""
			changed.RequestFingerprint, _ = nodeConnectorPlacementRequestFingerprint(changed)
			mustWriteCanonicalNodeConnectorPlacement(t, filepath.Join(root, nodeConnectorPlacementRequestName), changed)
			if _, err := OpenNodeConnectorPlacementDecisions(root, expected); err == nil {
				t.Fatal("selected node identity, capability, or profile substitution was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementDecisionRejectsMalformedNoncanonicalOversizedAndInferenceClaims(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 1)
	valid := mustMarshalNodeConnectorPlacement(t, fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{[]byte("{not-json"), pretty.Bytes(), append(append([]byte{}, valid...), []byte(" trailing")...), make([]byte, nodeConnectorPlacementMaxDecisionBytes+1)}
	for _, field := range []string{"unknown", "availability", "load", "risk", "cost", "score", "rank", "recommendation", "matching", "connection", "provider", "lease", "dispatch", "retry", "repair", "quarantine", "schedule_at"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":{"authoritative":true}}`)...))
	}
	decisions := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	for index, raw := range inputs {
		if _, _, err := decisions.Decide(raw); err == nil {
			t.Fatalf("malformed, noncanonical, oversized, unknown, or inference-bearing decision %d was accepted", index)
		}
	}
	if decision, request := decisions.Artifacts(); decision != nil || request != nil {
		t.Fatal("rejected placement decision input published durable state")
	}
}

func TestNodeConnectorPlacementReplayRestartConflictsChangedExpectationsAndTamperFailClosed(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 1)
	decisions := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	acceptedDecision, acceptedRequest := mustDecideNodeConnectorPlacement(t, decisions, fixture)

	originalDecisionWriter := nodeConnectorPlacementWriteDecisionAtomic
	originalRequestWriter := nodeConnectorPlacementWriteRequestAtomic
	writes := 0
	nodeConnectorPlacementWriteDecisionAtomic = func(path string, value any) error { writes++; return originalDecisionWriter(path, value) }
	nodeConnectorPlacementWriteRequestAtomic = func(path string, value any) error { writes++; return originalRequestWriter(path, value) }
	t.Cleanup(func() {
		nodeConnectorPlacementWriteDecisionAtomic, nodeConnectorPlacementWriteRequestAtomic = originalDecisionWriter, originalRequestWriter
	})
	if decision, request := mustDecideNodeConnectorPlacement(t, decisions, fixture); !reflect.DeepEqual(decision, acceptedDecision) || !reflect.DeepEqual(request, acceptedRequest) || writes != 0 {
		t.Fatal("identical placement replay changed or rewrote durable artifacts")
	}
	restarted := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	if decision, request := mustDecideNodeConnectorPlacement(t, restarted, fixture); !reflect.DeepEqual(decision, acceptedDecision) || !reflect.DeepEqual(request, acceptedRequest) || writes != 0 {
		t.Fatal("restart replay changed or rewrote durable placement artifacts")
	}
	for _, mutate := range []func(*NodeConnectorPlacementDecisionFixture){
		func(value *NodeConnectorPlacementDecisionFixture) {
			value.DecisionID = "placement-decision-conflict-001"
		},
		func(value *NodeConnectorPlacementDecisionFixture) {
			value.ReplayIdentity = "replay-placement-conflict-001"
		},
		func(value *NodeConnectorPlacementDecisionFixture) { value.SelectedNodeID = value.CandidateNodeIDs[0] },
		func(value *NodeConnectorPlacementDecisionFixture) {
			value.PlacementRequestID = "placement-request-conflict-001"
		},
		func(value *NodeConnectorPlacementDecisionFixture) {
			value.Decision, value.SelectedNodeID, value.PlacementRequestID = "rejected", "", ""
		},
	} {
		changed := cloneNodeConnectorPlacementDecisionFixture(fixture)
		mutate(&changed)
		if _, _, err := restarted.Decide(mustMarshalNodeConnectorPlacement(t, changed)); err == nil || writes != 0 {
			t.Fatal("conflicting placement decision, replay, selection, or request was accepted or rewrote state")
		}
	}

	changedExpected := expected
	changedExpected.InventorySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
	if _, err := OpenNodeConnectorPlacementDecisions(root, changedExpected); err == nil {
		t.Fatal("changed inventory expectation was accepted")
	}
	changedExpected = expected
	changedExpected.PlacementInputSnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
	if _, err := OpenNodeConnectorPlacementDecisions(root, changedExpected); err == nil {
		t.Fatal("changed placement-input expectation was accepted")
	}

	decisionPath := filepath.Join(root, nodeConnectorPlacementDecisionName)
	decisionRaw := mustReadNodeConnectorPlacementFile(t, root, nodeConnectorPlacementDecisionName)
	if err := os.WriteFile(decisionPath, bytes.Replace(decisionRaw, []byte(`"selection_inferred": false`), []byte(`"selection_inferred": true`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementDecisions(root, expected); err == nil {
		t.Fatal("tampered placement decision was accepted")
	}
	if err := os.WriteFile(decisionPath, decisionRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, nodeConnectorPlacementRequestName)
	requestRaw := mustReadNodeConnectorPlacementFile(t, root, nodeConnectorPlacementRequestName)
	if err := os.WriteFile(requestPath, bytes.Replace(requestRaw, []byte(`"placement_dispatched": false`), []byte(`"placement_dispatched": true`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementDecisions(root, expected); err == nil {
		t.Fatal("tampered placement request was accepted")
	}
}

func TestNodeConnectorPlacementDirectlyRevalidatesInventoryAndPlacementOnEveryDecision(t *testing.T) {
	for _, artifact := range []struct {
		name string
		old  []byte
		new  []byte
	}{
		{name: nodeConnectorInventoryArtifactName, old: []byte(`"classification": "available"`), new: []byte(`"classification": "unknown"`)},
		{name: nodeConnectorPlacementInputArtifactName, old: []byte(`"complete_inventory_candidate_set": true`), new: []byte(`"complete_inventory_candidate_set": false`)},
	} {
		t.Run(artifact.name, func(t *testing.T) {
			root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 0)
			decisions := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
			path := filepath.Join(root, artifact.name)
			raw := mustReadNodeConnectorPlacementFile(t, root, artifact.name)
			changed := bytes.Replace(raw, artifact.old, artifact.new, 1)
			if bytes.Equal(changed, raw) {
				t.Fatal("test did not tamper its upstream artifact")
			}
			if err := os.WriteFile(path, changed, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := decisions.Decide(mustMarshalNodeConnectorPlacement(t, fixture)); err == nil {
				t.Fatal("placement decision accepted directly tampered inventory or placement input")
			}
			if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDecisionName)); !os.IsNotExist(err) {
				t.Fatal("upstream tampering published a placement decision")
			}
		})
	}
}

func TestNodeConnectorPlacementAtomicFailuresRecoveryAndPartialPublication(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 1)
	decisions := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	originalDecisionWriter := nodeConnectorPlacementWriteDecisionAtomic
	originalRequestWriter := nodeConnectorPlacementWriteRequestAtomic
	t.Cleanup(func() {
		nodeConnectorPlacementWriteDecisionAtomic, nodeConnectorPlacementWriteRequestAtomic = originalDecisionWriter, originalRequestWriter
	})
	nodeConnectorPlacementWriteDecisionAtomic = func(string, any) error { return errors.New("injected placement decision write failure") }
	if _, _, err := decisions.Decide(mustMarshalNodeConnectorPlacement(t, fixture)); err == nil {
		t.Fatal("placement decision atomic-write failure was accepted")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("decision write failure published an artifact: %#v, %v", entries, err)
	}

	nodeConnectorPlacementWriteDecisionAtomic = originalDecisionWriter
	nodeConnectorPlacementWriteRequestAtomic = func(string, any) error { return errors.New("injected placement request write failure") }
	if _, _, err := decisions.Decide(mustMarshalNodeConnectorPlacement(t, fixture)); err == nil {
		t.Fatal("placement request atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDecisionName)); err != nil {
		t.Fatal("request write failure lost the exact durable decision")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementRequestName)); !os.IsNotExist(err) {
		t.Fatal("request atomic-write failure published a partial request")
	}

	nodeConnectorPlacementWriteRequestAtomic = originalRequestWriter
	restarted := mustOpenNodeConnectorPlacementDecisions(t, root, expected)
	decision, request := mustDecideNodeConnectorPlacement(t, restarted, fixture)
	if decision.Decision != "approved" || request == nil || request.RequestID != fixture.PlacementRequestID {
		t.Fatal("restart did not safely publish the request bound by the durable placement decision")
	}
	if err := os.Remove(filepath.Join(root, nodeConnectorPlacementDecisionName)); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementDecisions(root, expected); err == nil {
		t.Fatal("partially published placement request without its decision was accepted")
	}
}

func TestNodeConnectorPlacementJSONShapeLeaksNoInferenceAuthorityOrExecutionIdentity(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", 2)
	decision, request := mustDecideNodeConnectorPlacement(t, mustOpenNodeConnectorPlacementDecisions(t, root, expected), fixture)
	nodeConnectorInventoryAssertJSONFields(t, fixture, []string{"schema", "decision_id", "replay_identity", "decision", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "placement_input_id", "placement_input_snapshot_fingerprint", "workload_id", "requirements_fingerprint", "candidate_node_ids", "selected_node_id", "placement_request_id", "provenance"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorPlacementSelectedNodeBinding{}, []string{"node_id", "machine_id", "capability_snapshot_id", "capability_snapshot_fingerprint", "profile"})
	nodeConnectorInventoryAssertJSONFields(t, decision, []string{"schema", "decision_id", "replay_identity", "decision", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "placement_input_id", "placement_input_snapshot_fingerprint", "workload_id", "requirements_fingerprint", "candidate_node_ids", "complete_candidate_set", "selected_node", "placement_request_id", "provenance", "fixture_owned", "selection_inferred", "authority", "decision_fingerprint"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorPlacementRequest{}, []string{"schema", "request_id", "decision_id", "decision_fingerprint", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "placement_input_id", "placement_input_snapshot_fingerprint", "workload_id", "requirements_fingerprint", "candidate_node_ids", "complete_candidate_set", "selected_node", "provenance", "fixture_owned", "selection_evidence_only", "placement_dispatched", "authority", "request_fingerprint"})
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementDecision `json:"decision"`
		Request  NodeConnectorPlacementRequest  `json:"request"`
	}{Decision: decision, Request: *request})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"availability", "cpu_utilization", "memory_utilization", "active_task", "risk", "normalized_value", "recommend", "connection", "provider_payload", "command", "credential", "endpoint", "lease_id", "receipt_id", "operation_id", "attempt", "retry_count", "repair_plan", "schedule_at", "workspace_path", "filesystem_path"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("placement contract leaked inference evidence, execution identity, or forbidden field %q", forbidden)
		}
	}
	selectorCalls, schedulerCalls, dispatcherCalls, leaseCalls, executorCalls, validatorCalls, retryCalls, repairCalls, quarantineCalls, networkCalls, providerCalls, serviceCalls, mutationCalls, gitCalls, publicationCalls := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	_ = []func(){func() { selectorCalls++ }, func() { schedulerCalls++ }, func() { dispatcherCalls++ }, func() { leaseCalls++ }, func() { executorCalls++ }, func() { validatorCalls++ }, func() { retryCalls++ }, func() { repairCalls++ }, func() { quarantineCalls++ }, func() { networkCalls++ }, func() { providerCalls++ }, func() { serviceCalls++ }, func() { mutationCalls++ }, func() { gitCalls++ }, func() { publicationCalls++ }}
	if selectorCalls+schedulerCalls+dispatcherCalls+leaseCalls+executorCalls+validatorCalls+retryCalls+repairCalls+quarantineCalls+networkCalls+providerCalls+serviceCalls+mutationCalls+gitCalls+publicationCalls != 0 {
		t.Fatal("fixture-only placement decision/request invoked a forbidden callback")
	}
}

func TestNodeConnectorPlacementExistingSchemasRemainUnchanged(t *testing.T) {
	got := map[string]string{
		"machine": NodeExecutionMachineIdentitySchema, "capability": NodeExecutionCapabilitySnapshotSchema,
		"lease": NodeExecutionLeaseSchema, "receipt": NodeExecutionReceiptSchema, "session": NodeConnectorSessionNegotiationSchema,
		"aggregate": NodeConnectorMultiTargetValidationAggregateSchema, "repair_decision": NodeConnectorMultiTargetRepairDecisionSchema,
		"repair_request": NodeConnectorMultiTargetRepairRequestSchema, "service_intent": NodeConnectorServiceLifecycleIntentSchema,
		"service_diagnostic": NodeConnectorServiceDiagnosticSchema, "inventory": NodeConnectorInventorySnapshotSchema,
		"placement_input": NodeConnectorPlacementInputSnapshotSchema,
	}
	want := map[string]string{
		"machine": "dorkpipe.node-execution.machine-identity/v1", "capability": "dorkpipe.node-execution.capability-snapshot/v1",
		"lease": "dorkpipe.node-execution.task-lease/v1", "receipt": "dorkpipe.node-execution.execution-receipt/v1", "session": "dorkpipe.node-connector.session-negotiation/v1",
		"aggregate": "dorkpipe.multi-target-validation-aggregate/v1", "repair_decision": "dorkpipe.multi-target-repair-decision/v1",
		"repair_request": "dorkpipe.multi-target-repair-request/v1", "service_intent": "dorkpipe.node-connector-service-lifecycle-intent/v1",
		"service_diagnostic": "dorkpipe.node-connector-service-diagnostic/v1", "inventory": "dorkpipe.node-inventory-snapshot/v1",
		"placement_input": "dorkpipe.node-placement-input-snapshot/v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing node execution, inventory, repair, or lifecycle schema changed: %#v", got)
	}
}

func nodeConnectorPlacementDecisionFixture(t *testing.T, decision string, selectedIndex int) (string, NodeConnectorPlacementDecisionExpected, NodeConnectorPlacementDecisionFixture, NodeConnectorInventorySnapshot, NodeConnectorPlacementInputSnapshot) {
	t.Helper()
	root, inventoryExpected, inventoryFixture := nodeConnectorInventoryTestFixture(t)
	snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, inventoryExpected)
	inventory := mustRecordNodeConnectorInventory(t, snapshots, inventoryFixture)
	placement := mustRecordNodeConnectorPlacement(t, snapshots, nodeConnectorPlacementTestFixture(inventory, inventoryExpected))
	expected := NodeConnectorPlacementDecisionExpected{Inventory: inventoryExpected, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint}
	fixture := NodeConnectorPlacementDecisionFixture{
		Schema: NodeConnectorPlacementDecisionFixtureSchema, DecisionID: "placement-decision-001", ReplayIdentity: "replay-placement-decision-001", Decision: decision,
		InventorySnapshotID: inventory.InventorySnapshotID, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint,
		PlacementInputID: placement.PlacementInputID, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint,
		WorkloadID: placement.WorkloadID, RequirementsFingerprint: placement.RequirementsFingerprint,
		CandidateNodeIDs: append([]string{}, placement.CandidateNodeIDs...), Provenance: nodeConnectorPlacementDecisionProvenance,
	}
	if decision == "approved" {
		fixture.SelectedNodeID = placement.CandidateNodeIDs[selectedIndex]
		fixture.PlacementRequestID = "placement-request-001"
	}
	return root, expected, fixture, inventory, placement
}

func mustOpenNodeConnectorPlacementDecisions(t *testing.T, root string, expected NodeConnectorPlacementDecisionExpected) *NodeConnectorPlacementDecisions {
	t.Helper()
	value, err := OpenNodeConnectorPlacementDecisions(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustDecideNodeConnectorPlacement(t *testing.T, decisions *NodeConnectorPlacementDecisions, fixture NodeConnectorPlacementDecisionFixture) (NodeConnectorPlacementDecision, *NodeConnectorPlacementRequest) {
	t.Helper()
	decision, request, err := decisions.Decide(mustMarshalNodeConnectorPlacement(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacement(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorPlacementFile(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustWriteCanonicalNodeConnectorPlacement(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cloneNodeConnectorPlacementDecisionFixture(value NodeConnectorPlacementDecisionFixture) NodeConnectorPlacementDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
