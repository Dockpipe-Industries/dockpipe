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
	"time"
)

func TestNodeConnectorPlacementDispatchApprovedBindsExactRequestWithoutSubmission(t *testing.T) {
	root, expected, fixture, placementDecision, placementRequest := nodeConnectorPlacementDispatchFixture(t, "approved", 1)
	brokerEvidence := newNodeExecutionTestFixture(t)
	brokerBefore := nodeExecutionStateArtifacts(t, brokerEvidence.root)
	decision, request := mustDecideNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected), fixture)
	if decision.Decision != "approved" || request == nil || decision.DispatchInferred {
		t.Fatal("approved dispatch decision did not emit exactly one explicit request")
	}
	if !reflect.DeepEqual(decision.ExecutionRequest, fixture.ExecutionRequest) || !reflect.DeepEqual(request.ExecutionRequest, fixture.ExecutionRequest) || request.ExecutionRequest.RequestFingerprint != expected.ExecutionRequestFingerprint {
		t.Fatal("dispatch artifacts did not bind the complete exact execution request")
	}
	if decision.WorkloadID != placementRequest.WorkloadID || decision.ExecutionTaskID != fixture.ExecutionRequest.TaskID || decision.WorkloadID == decision.ExecutionTaskID {
		t.Fatal("workload and execution-task identities were not preserved as separate explicit bindings")
	}
	if !reflect.DeepEqual(decision.SelectedNode, placementRequest.SelectedNode) || !reflect.DeepEqual(request.SelectedNode, placementRequest.SelectedNode) || request.ExecutionRequest.CapabilitySnapshotID != request.SelectedNode.CapabilitySnapshotID {
		t.Fatal("selected node, machine, capability, fingerprint, profile, or execution capability binding was lost")
	}
	if decision.PlacementDecisionFingerprint != placementDecision.DecisionFingerprint || decision.PlacementRequestFingerprint != placementRequest.RequestFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint {
		t.Fatal("dispatch artifacts omitted an exact upstream or decision fingerprint binding")
	}
	if !reflect.DeepEqual(request.CandidateNodeIDs, placementRequest.CandidateNodeIDs) || !decision.CompleteCandidateSet || !request.CompleteCandidateSet {
		t.Fatal("dispatch artifacts did not preserve the complete exact candidate set")
	}
	wantAuthority := NodeConnectorPlacementDispatchAuthority{FixtureBrokerSubmission: true}
	if decision.Authority != (NodeConnectorPlacementDispatchAuthority{}) || request.Authority != wantAuthority || request.SubmissionScope != nodeConnectorPlacementDispatchSubmissionScope || !request.InProcessFixtureBrokerOnly || !request.OneTimeSubmission || request.AuthorizationConsumed || request.BrokerInvoked || request.LeaseIssued || request.ExecutionStarted {
		t.Fatal("dispatch request authority is broader than one future unconsumed fixture-broker submission")
	}
	if len(brokerEvidence.broker.state.Operations) != 0 || *brokerEvidence.calls != 0 || !nodeExecutionStringSlicesEqual(brokerBefore, nodeExecutionStateArtifacts(t, brokerEvidence.root)) {
		t.Fatal("placement dispatch decision invoked the fake broker, issued a lease, executed work, or published broker state")
	}
	for _, name := range []string{nodeConnectorPlacementDispatchDecisionName, nodeConnectorPlacementDispatchRequestName} {
		raw := mustReadNodeConnectorPlacementDispatchFile(t, root, name)
		if len(raw) > nodeConnectorPlacementDispatchMaxArtifactBytes || raw[len(raw)-1] != '\n' {
			t.Fatal("placement dispatch artifact is not bounded canonical newline-terminated JSON")
		}
	}
}

func TestNodeConnectorPlacementDispatchRequiresSeparateDecisionAndRejectedEmitsNoRequest(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 0)
	decisions := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	if decision, request := decisions.Artifacts(); decision != nil || request != nil {
		t.Fatal("placement request alone created placement dispatch authority")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchRequestName)); !os.IsNotExist(err) {
		t.Fatal("placement request alone published a dispatch request")
	}

	root, expected, fixture, _, _ = nodeConnectorPlacementDispatchFixture(t, "rejected", 2)
	decision, request := mustDecideNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected), fixture)
	if decision.Decision != "rejected" || decision.PlacementDispatchRequestID != "" || request != nil {
		t.Fatal("rejected placement dispatch decision emitted a request")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchRequestName)); !os.IsNotExist(err) {
		t.Fatal("rejected placement dispatch decision published a request artifact")
	}
	restarted := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	replayedDecision, replayedRequest := mustDecideNodeConnectorPlacementDispatch(t, restarted, fixture)
	if !reflect.DeepEqual(decision, replayedDecision) || replayedRequest != nil {
		t.Fatal("rejected placement dispatch decision changed or created a request after restart")
	}
}

func TestNodeConnectorPlacementDispatchRejectsChangedPlacementAndExecutionBindings(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementDispatchDecisionFixture)
	}{
		{"inventory-id", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.InventorySnapshotID = "inventory-substituted-001"
		}},
		{"inventory-fingerprint", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.InventorySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"placement-id", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.PlacementInputID = "placement-input-substituted-001"
		}},
		{"placement-fingerprint", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.PlacementInputSnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"placement-decision", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.PlacementDecisionFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"placement-request", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.PlacementRequestFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"workload", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.WorkloadID = "workload-substituted-001"
		}},
		{"execution-task", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.ExecutionTaskID = "task-substituted-001"
		}},
		{"candidate-reordered", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.CandidateNodeIDs[0], value.CandidateNodeIDs[1] = value.CandidateNodeIDs[1], value.CandidateNodeIDs[0]
		}},
		{"selected-node", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.SelectedNode.NodeID = value.CandidateNodeIDs[0]
		}},
		{"selected-machine", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.SelectedNode.MachineID = "machine-substituted-001"
		}},
		{"selected-capability", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.SelectedNode.CapabilitySnapshotID = nodeConnectorInventoryFingerprint("0")
		}},
		{"selected-capability-fingerprint", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.SelectedNode.CapabilitySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"selected-profile", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.SelectedNode.Profile.Runtime = "host"
		}},
		{"request-capability", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.ExecutionRequest.CapabilitySnapshotID = nodeConnectorInventoryFingerprint("0")
			value.ExecutionRequest, _ = FinalizeNodeExecutionRequest(value.ExecutionRequest)
		}},
		{"request-operation", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.ExecutionRequest.OperationID = "operation-substituted-001"
			value.ExecutionRequest, _ = FinalizeNodeExecutionRequest(value.ExecutionRequest)
		}},
		{"request-fingerprint", func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.ExecutionRequest.RequestFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 1)
			test.mutate(&fixture)
			if _, _, err := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected).Decide(mustMarshalNodeConnectorPlacementDispatch(t, fixture)); err == nil {
				t.Fatal("changed placement, selected-node, workload, task, or execution-request binding was accepted")
			}
			if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchDecisionName)); !os.IsNotExist(err) {
				t.Fatal("rejected dispatch binding published a decision")
			}
		})
	}
}

func TestNodeConnectorPlacementDispatchRejectsMalformedNoncanonicalUnknownOversizedAndInferenceClaims(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 1)
	valid := mustMarshalNodeConnectorPlacementDispatch(t, fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{[]byte("{not-json"), pretty.Bytes(), append(append([]byte{}, valid...), []byte(" trailing")...), make([]byte, nodeConnectorPlacementDispatchMaxDecisionBytes+1)}
	for _, field := range []string{"unknown", "availability", "load", "risk", "cost", "ordering", "rank", "recommendation", "matching", "connection", "provider", "lease_id", "receipt_id", "retry", "repair", "quarantine", "schedule_at"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":{"authoritative":true}}`)...))
	}
	nestedUnknown := bytes.Replace(valid, []byte(`"operation_id":`), []byte(`"command":"forbidden","operation_id":`), 1)
	inputs = append(inputs, nestedUnknown)
	decisions := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	for index, raw := range inputs {
		if _, _, err := decisions.Decide(raw); err == nil {
			t.Fatalf("malformed, noncanonical, unknown, oversized, or inference-bearing decision %d was accepted", index)
		}
	}
	if decision, request := decisions.Artifacts(); decision != nil || request != nil {
		t.Fatal("rejected placement dispatch input published durable state")
	}
}

func TestNodeConnectorPlacementDispatchReplayRestartConflictTamperAndOrphanFailClosed(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 1)
	decisions := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	acceptedDecision, acceptedRequest := mustDecideNodeConnectorPlacementDispatch(t, decisions, fixture)
	originalDecisionWriter := nodeConnectorPlacementDispatchWriteDecisionAtomic
	originalRequestWriter := nodeConnectorPlacementDispatchWriteRequestAtomic
	writes := 0
	nodeConnectorPlacementDispatchWriteDecisionAtomic = func(path string, value any) error { writes++; return originalDecisionWriter(path, value) }
	nodeConnectorPlacementDispatchWriteRequestAtomic = func(path string, value any) error { writes++; return originalRequestWriter(path, value) }
	t.Cleanup(func() {
		nodeConnectorPlacementDispatchWriteDecisionAtomic = originalDecisionWriter
		nodeConnectorPlacementDispatchWriteRequestAtomic = originalRequestWriter
	})
	if decision, request := mustDecideNodeConnectorPlacementDispatch(t, decisions, fixture); !reflect.DeepEqual(decision, acceptedDecision) || !reflect.DeepEqual(request, acceptedRequest) || writes != 0 {
		t.Fatal("exact placement dispatch replay rewrote durable artifacts")
	}
	restarted := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	if decision, request := mustDecideNodeConnectorPlacementDispatch(t, restarted, fixture); !reflect.DeepEqual(decision, acceptedDecision) || !reflect.DeepEqual(request, acceptedRequest) || writes != 0 {
		t.Fatal("restart replay changed or rewrote durable placement dispatch artifacts")
	}
	for _, mutate := range []func(*NodeConnectorPlacementDispatchDecisionFixture){
		func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.DecisionID = "placement-dispatch-decision-conflict-001"
		},
		func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.ReplayIdentity = "replay-placement-dispatch-conflict-001"
		},
		func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.PlacementDispatchRequestID = "placement-dispatch-request-conflict-001"
		},
		func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.Decision, value.PlacementDispatchRequestID = "rejected", ""
		},
		func(value *NodeConnectorPlacementDispatchDecisionFixture) {
			value.ExecutionRequest.RunID = "run-conflict-001"
			value.ExecutionRequest, _ = FinalizeNodeExecutionRequest(value.ExecutionRequest)
			value.ExecutionTaskID = value.ExecutionRequest.TaskID
		},
	} {
		changed := cloneNodeConnectorPlacementDispatchDecisionFixture(fixture)
		mutate(&changed)
		if _, _, err := restarted.Decide(mustMarshalNodeConnectorPlacementDispatch(t, changed)); err == nil || writes != 0 {
			t.Fatal("conflicting dispatch decision, replay, request, or execution request was accepted or rewrote state")
		}
	}

	decisionPath := filepath.Join(root, nodeConnectorPlacementDispatchDecisionName)
	decisionRaw := mustReadNodeConnectorPlacementDispatchFile(t, root, nodeConnectorPlacementDispatchDecisionName)
	tamperedDecision := bytes.Replace(decisionRaw, []byte(`"dispatch_inferred": false`), []byte(`"dispatch_inferred": true`), 1)
	if err := os.WriteFile(decisionPath, tamperedDecision, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementDispatchDecisions(root, expected); err == nil {
		t.Fatal("tampered placement dispatch decision was accepted")
	}
	if err := os.WriteFile(decisionPath, decisionRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, nodeConnectorPlacementDispatchRequestName)
	requestRaw := mustReadNodeConnectorPlacementDispatchFile(t, root, nodeConnectorPlacementDispatchRequestName)
	tamperedRequest := bytes.Replace(requestRaw, []byte(`"authorization_consumed": false`), []byte(`"authorization_consumed": true`), 1)
	if err := os.WriteFile(requestPath, tamperedRequest, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementDispatchDecisions(root, expected); err == nil {
		t.Fatal("tampered placement dispatch request was accepted")
	}
	if err := os.WriteFile(requestPath, requestRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(decisionPath); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementDispatchDecisions(root, expected); err == nil {
		t.Fatal("orphan placement dispatch request was accepted")
	}
}

func TestNodeConnectorPlacementDispatchDirectlyRevalidatesUpstreamOnEveryDecision(t *testing.T) {
	for _, artifact := range []struct {
		name string
		old  []byte
		new  []byte
	}{
		{name: nodeConnectorInventoryArtifactName, old: []byte(`"classification": "available"`), new: []byte(`"classification": "unknown"`)},
		{name: nodeConnectorPlacementInputArtifactName, old: []byte(`"complete_inventory_candidate_set": true`), new: []byte(`"complete_inventory_candidate_set": false`)},
		{name: nodeConnectorPlacementDecisionName, old: []byte(`"selection_inferred": false`), new: []byte(`"selection_inferred": true`)},
		{name: nodeConnectorPlacementRequestName, old: []byte(`"placement_dispatched": false`), new: []byte(`"placement_dispatched": true`)},
	} {
		t.Run(artifact.name, func(t *testing.T) {
			root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 0)
			decisions := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
			path := filepath.Join(root, artifact.name)
			raw := mustReadNodeConnectorPlacementDispatchFile(t, root, artifact.name)
			changed := bytes.Replace(raw, artifact.old, artifact.new, 1)
			if bytes.Equal(changed, raw) {
				t.Fatal("test did not tamper its upstream artifact")
			}
			if err := os.WriteFile(path, changed, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := decisions.Decide(mustMarshalNodeConnectorPlacementDispatch(t, fixture)); err == nil {
				t.Fatal("placement dispatch accepted tampered inventory or placement evidence")
			}
			if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchDecisionName)); !os.IsNotExist(err) {
				t.Fatal("upstream tampering published a placement dispatch decision")
			}
		})
	}
}

func TestNodeConnectorPlacementDispatchAtomicFailuresRecoverWithoutPartialRequest(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 1)
	decisions := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	originalDecisionWriter := nodeConnectorPlacementDispatchWriteDecisionAtomic
	originalRequestWriter := nodeConnectorPlacementDispatchWriteRequestAtomic
	t.Cleanup(func() {
		nodeConnectorPlacementDispatchWriteDecisionAtomic = originalDecisionWriter
		nodeConnectorPlacementDispatchWriteRequestAtomic = originalRequestWriter
	})
	nodeConnectorPlacementDispatchWriteDecisionAtomic = func(string, any) error { return errors.New("injected placement dispatch decision write failure") }
	if _, _, err := decisions.Decide(mustMarshalNodeConnectorPlacementDispatch(t, fixture)); err == nil {
		t.Fatal("placement dispatch decision atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchDecisionName)); !os.IsNotExist(err) {
		t.Fatal("decision write failure published an artifact")
	}

	nodeConnectorPlacementDispatchWriteDecisionAtomic = originalDecisionWriter
	nodeConnectorPlacementDispatchWriteRequestAtomic = func(string, any) error { return errors.New("injected placement dispatch request write failure") }
	if _, _, err := decisions.Decide(mustMarshalNodeConnectorPlacementDispatch(t, fixture)); err == nil {
		t.Fatal("placement dispatch request atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchDecisionName)); err != nil {
		t.Fatal("request write failure lost the exact durable decision")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementDispatchRequestName)); !os.IsNotExist(err) {
		t.Fatal("request write failure published a partial request")
	}

	nodeConnectorPlacementDispatchWriteRequestAtomic = originalRequestWriter
	restarted := mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected)
	decision, request := mustDecideNodeConnectorPlacementDispatch(t, restarted, fixture)
	if decision.Decision != "approved" || request == nil || request.RequestID != fixture.PlacementDispatchRequestID {
		t.Fatal("restart did not safely publish the request bound by the durable decision")
	}
}

func TestNodeConnectorPlacementDispatchJSONShapeAndExistingSchemasRemainUnchanged(t *testing.T) {
	root, expected, fixture, _, _ := nodeConnectorPlacementDispatchFixture(t, "approved", 2)
	decision, request := mustDecideNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchDecisions(t, root, expected), fixture)
	nodeConnectorInventoryAssertJSONFields(t, fixture, []string{"schema", "decision_id", "replay_identity", "decision", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "placement_input_id", "placement_input_snapshot_fingerprint", "placement_decision_id", "placement_decision_fingerprint", "placement_request_id", "placement_request_fingerprint", "workload_id", "candidate_node_ids", "selected_node", "execution_task_id", "execution_request", "placement_dispatch_request_id", "provenance"})
	nodeConnectorInventoryAssertJSONFields(t, decision, []string{"schema", "decision_id", "replay_identity", "decision", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "placement_input_id", "placement_input_snapshot_fingerprint", "placement_decision_id", "placement_decision_fingerprint", "placement_request_id", "placement_request_fingerprint", "workload_id", "candidate_node_ids", "complete_candidate_set", "selected_node", "execution_task_id", "execution_request", "placement_dispatch_request_id", "provenance", "fixture_owned", "dispatch_inferred", "authority", "decision_fingerprint"})
	nodeConnectorInventoryAssertJSONFields(t, *request, []string{"schema", "request_id", "decision_id", "decision_fingerprint", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "placement_input_id", "placement_input_snapshot_fingerprint", "placement_decision_id", "placement_decision_fingerprint", "placement_request_id", "placement_request_fingerprint", "workload_id", "candidate_node_ids", "complete_candidate_set", "selected_node", "execution_task_id", "execution_request", "submission_scope", "in_process_fixture_broker_only", "one_time_submission", "authorization_consumed", "broker_invoked", "lease_issued", "execution_started", "provenance", "fixture_owned", "authority", "request_fingerprint"})
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementDispatchDecision `json:"decision"`
		Request  NodeConnectorPlacementDispatchRequest  `json:"request"`
	}{Decision: decision, Request: *request})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"availability", "cpu_utilization", "memory_utilization", "active_task", "risk", "normalized_value", "recommend", "connection", "provider_payload", "command", "credential", "endpoint", "lease_id", "receipt_id", "attempt", "retry_count", "repair_plan", "schedule_at", "workspace_path", "filesystem_path"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("placement dispatch contract leaked inference evidence, lease/receipt identity, or forbidden field %q", forbidden)
		}
	}
	got := map[string]string{
		"machine": NodeExecutionMachineIdentitySchema, "capability": NodeExecutionCapabilitySnapshotSchema,
		"execution_request": NodeExecutionRequestSchema, "lease": NodeExecutionLeaseSchema, "receipt": NodeExecutionReceiptSchema,
		"session": NodeConnectorSessionNegotiationSchema, "inventory": NodeConnectorInventorySnapshotSchema,
		"placement_input": NodeConnectorPlacementInputSnapshotSchema, "placement_decision": NodeConnectorPlacementDecisionSchema,
		"placement_request": NodeConnectorPlacementRequestSchema, "repair_decision": NodeConnectorMultiTargetRepairDecisionSchema,
		"repair_request": NodeConnectorMultiTargetRepairRequestSchema, "service_intent": NodeConnectorServiceLifecycleIntentSchema,
	}
	want := map[string]string{
		"machine": "dorkpipe.node-execution.machine-identity/v1", "capability": "dorkpipe.node-execution.capability-snapshot/v1",
		"execution_request": "dorkpipe.node-execution.execution-request/v1", "lease": "dorkpipe.node-execution.task-lease/v1", "receipt": "dorkpipe.node-execution.execution-receipt/v1",
		"session": "dorkpipe.node-connector.session-negotiation/v1", "inventory": "dorkpipe.node-inventory-snapshot/v1",
		"placement_input": "dorkpipe.node-placement-input-snapshot/v1", "placement_decision": "dorkpipe.node-placement-decision/v1",
		"placement_request": "dorkpipe.node-placement-request/v1", "repair_decision": "dorkpipe.multi-target-repair-decision/v1",
		"repair_request": "dorkpipe.multi-target-repair-request/v1", "service_intent": "dorkpipe.node-connector-service-lifecycle-intent/v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing node-execution, session, inventory, placement, repair, or lifecycle schema changed: %#v", got)
	}
}

func nodeConnectorPlacementDispatchFixture(t *testing.T, decision string, selectedIndex int) (string, NodeConnectorPlacementDispatchExpected, NodeConnectorPlacementDispatchDecisionFixture, NodeConnectorPlacementDecision, NodeConnectorPlacementRequest) {
	t.Helper()
	root, placementExpected, placementFixture, _, _ := nodeConnectorPlacementDecisionFixture(t, "approved", selectedIndex)
	placementDecision, placementRequest := mustDecideNodeConnectorPlacement(t, mustOpenNodeConnectorPlacementDecisions(t, root, placementExpected), placementFixture)
	executionRequest, err := FinalizeNodeExecutionRequest(NodeExecutionRequest{
		OperationID: "operation-placement-dispatch-001", GraphRunID: "graph-placement-dispatch-001", RunID: "run-placement-dispatch-001", TaskID: "task-placement-dispatch-001",
		SourceRevision: strings.Repeat("a", 40), Workflow: NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "validate.placement"},
		CapabilitySnapshotID: placementRequest.SelectedNode.CapabilitySnapshotID,
		Inputs:               []NodeExecutionInput{{Name: "mode", Value: "readonly"}}, Artifacts: []NodeExecutionArtifactReference{{Name: "source.bundle", MediaType: "application/octet-stream", Digest: nodeConnectorInventoryFingerprint("9"), Bytes: 4096}},
		RequestedAt: nodeExecutionTime(time.Date(2026, 7, 27, 21, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := NodeConnectorPlacementDispatchExpected{
		Placement: placementExpected, PlacementDecisionFingerprint: placementDecision.DecisionFingerprint,
		PlacementRequestFingerprint: placementRequest.RequestFingerprint, ExecutionRequestFingerprint: executionRequest.RequestFingerprint,
	}
	fixture := NodeConnectorPlacementDispatchDecisionFixture{
		Schema: NodeConnectorPlacementDispatchDecisionFixtureSchema, DecisionID: "placement-dispatch-decision-001", ReplayIdentity: "replay-placement-dispatch-decision-001", Decision: decision,
		InventorySnapshotID: placementDecision.InventorySnapshotID, InventorySnapshotFingerprint: placementDecision.InventorySnapshotFingerprint,
		PlacementInputID: placementDecision.PlacementInputID, PlacementInputSnapshotFingerprint: placementDecision.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: placementDecision.DecisionID, PlacementDecisionFingerprint: placementDecision.DecisionFingerprint,
		PlacementRequestID: placementRequest.RequestID, PlacementRequestFingerprint: placementRequest.RequestFingerprint,
		WorkloadID: placementRequest.WorkloadID, CandidateNodeIDs: append([]string{}, placementRequest.CandidateNodeIDs...), SelectedNode: placementRequest.SelectedNode,
		ExecutionTaskID: executionRequest.TaskID, ExecutionRequest: executionRequest, Provenance: nodeConnectorPlacementDispatchDecisionProvenance,
	}
	if decision == "approved" {
		fixture.PlacementDispatchRequestID = "placement-dispatch-request-001"
	}
	return root, expected, fixture, placementDecision, *placementRequest
}

func mustOpenNodeConnectorPlacementDispatchDecisions(t *testing.T, root string, expected NodeConnectorPlacementDispatchExpected) *NodeConnectorPlacementDispatchDecisions {
	t.Helper()
	value, err := OpenNodeConnectorPlacementDispatchDecisions(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustDecideNodeConnectorPlacementDispatch(t *testing.T, decisions *NodeConnectorPlacementDispatchDecisions, fixture NodeConnectorPlacementDispatchDecisionFixture) (NodeConnectorPlacementDispatchDecision, *NodeConnectorPlacementDispatchRequest) {
	t.Helper()
	decision, request, err := decisions.Decide(mustMarshalNodeConnectorPlacementDispatch(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementDispatch(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorPlacementDispatchFile(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementDispatchDecisionFixture(value NodeConnectorPlacementDispatchDecisionFixture) NodeConnectorPlacementDispatchDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDispatchDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
