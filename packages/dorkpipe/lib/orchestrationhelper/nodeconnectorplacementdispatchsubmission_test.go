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

type nodeConnectorPlacementDispatchSubmissionTestFixture struct {
	root       string
	expected   NodeConnectorPlacementDispatchSubmissionExpected
	fixture    NodeConnectorPlacementDispatchSubmissionFixture
	brokerRoot string
	broker     *NodeExecutionFakeBroker
	machine    NodeExecutionMachineIdentity
	capability NodeExecutionCapabilitySnapshot
	connection string
}

func TestNodeConnectorPlacementDispatchSubmissionApprovedExactBrokerTransition(t *testing.T) {
	value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
	before := nodeExecutionStateArtifacts(t, value.brokerRoot)
	submission := mustSubmitNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchSubmissions(t, value), value.connection, value.fixture)
	if len(before) != 1 || len(nodeExecutionStateArtifacts(t, value.brokerRoot)) != 2 || len(value.broker.state.Operations) != 1 {
		t.Fatal("approved submission did not produce exactly one durable broker generation")
	}
	operation := value.broker.state.Operations[value.fixture.ExecutionRequest.OperationID]
	if !reflect.DeepEqual(submission.TaskLease, operation.Lease) || !reflect.DeepEqual(operation.Request, value.fixture.ExecutionRequest) {
		t.Fatal("submission did not preserve the exact canonical request and broker-issued lease")
	}
	if !submission.AuthorizationConsumed || !submission.BrokerInvoked || !submission.LeaseIssued || submission.ExecutorInvoked || submission.ExecutionStarted || submission.ConnectionCreated || submission.Authority != (NodeConnectorPlacementDispatchSubmissionAuthority{}) {
		t.Fatal("submission evidence misstated the narrow broker transition or gained adjacent authority")
	}
	if submission.SelectedNode.MachineID != value.machine.MachineID || submission.SelectedNode.CapabilitySnapshotID != value.capability.SnapshotID || submission.ExecutionRequestFingerprint != value.fixture.ExecutionRequest.RequestFingerprint {
		t.Fatal("submission lost the exact selected machine, capability, or execution-request binding")
	}
	raw := mustReadNodeConnectorPlacementDispatchSubmissionFile(t, value.root)
	if len(raw) > nodeConnectorPlacementDispatchMaxSubmissionArtifactBytes || raw[len(raw)-1] != '\n' {
		t.Fatal("submission artifact is not bounded canonical newline-terminated JSON")
	}
	var decoded NodeConnectorPlacementDispatchSubmission
	if err := decodeNodeConnectorPlacementDispatchSubmissionArtifact(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, submission) {
		t.Fatal("durable submission bytes are not the exact canonical returned artifact")
	}
}

func TestNodeConnectorPlacementDispatchSubmissionRequiresApprovedUnconsumedRequestAndConnection(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementDispatchSubmissionTestFixture)
	}{
		{name: "placement-and-connection-only", mutate: func(value *nodeConnectorPlacementDispatchSubmissionTestFixture) {
			mustRemoveNodeConnectorPlacementDispatchSubmissionFile(t, filepath.Join(value.root, nodeConnectorPlacementDispatchDecisionName))
			mustRemoveNodeConnectorPlacementDispatchSubmissionFile(t, filepath.Join(value.root, nodeConnectorPlacementDispatchRequestName))
		}},
		{name: "missing-request", mutate: func(value *nodeConnectorPlacementDispatchSubmissionTestFixture) {
			mustRemoveNodeConnectorPlacementDispatchSubmissionFile(t, filepath.Join(value.root, nodeConnectorPlacementDispatchRequestName))
		}},
		{name: "orphaned-request", mutate: func(value *nodeConnectorPlacementDispatchSubmissionTestFixture) {
			mustRemoveNodeConnectorPlacementDispatchSubmissionFile(t, filepath.Join(value.root, nodeConnectorPlacementDispatchDecisionName))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
			test.mutate(value)
			before := nodeExecutionStateArtifacts(t, value.brokerRoot)
			if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker); err == nil {
				t.Fatal("missing or orphaned dispatch authorization was accepted")
			}
			assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, value, before)
		})
	}

	t.Run("rejected-decision", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "rejected", true)
		before := nodeExecutionStateArtifacts(t, value.brokerRoot)
		if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker); err == nil {
			t.Fatal("rejected placement dispatch decision was accepted for submission")
		}
		assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, value, before)
	})

	t.Run("missing-connection", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", false)
		submissions := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
		before := nodeExecutionStateArtifacts(t, value.brokerRoot)
		if _, err := submissions.Submit(value.connection, mustMarshalNodeConnectorPlacementDispatchSubmission(t, value.fixture)); err == nil {
			t.Fatal("placement evidence without connection presence invoked the broker")
		}
		assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, value, before)
	})
}

func TestNodeConnectorPlacementDispatchSubmissionRejectsChangedBindingsAndIssuanceBeforeBrokerMutation(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementDispatchSubmissionFixture)
	}{
		{"node", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.SelectedNode.NodeID = "node-substituted-001"
		}},
		{"machine", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.SelectedNode.MachineID = "machine-substituted-001"
		}},
		{"capability", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.SelectedNode.CapabilitySnapshotID = nodeConnectorInventoryFingerprint("0")
		}},
		{"operation", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.ExecutionRequest.OperationID = "operation-substituted-001"
			value.ExecutionRequest, _ = FinalizeNodeExecutionRequest(value.ExecutionRequest)
			value.ExecutionRequestFingerprint = value.ExecutionRequest.RequestFingerprint
		}},
		{"request-fingerprint", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.ExecutionRequestFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"upstream-fingerprint", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.PlacementDispatchRequestFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"issuance-before-request", func(value *NodeConnectorPlacementDispatchSubmissionFixture) { value.IssuedAt = "2026-07-28T20:59:59Z" }},
		{"zero-duration", func(value *NodeConnectorPlacementDispatchSubmissionFixture) { value.LeaseDurationSeconds = 0 }},
		{"oversized-duration", func(value *NodeConnectorPlacementDispatchSubmissionFixture) {
			value.LeaseDurationSeconds = nodeConnectorPlacementDispatchMaxLeaseDurationSeconds + 1
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
			submissions := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
			changed := cloneNodeConnectorPlacementDispatchSubmissionFixture(value.fixture)
			test.mutate(&changed)
			before := nodeExecutionStateArtifacts(t, value.brokerRoot)
			if _, err := submissions.Submit(value.connection, mustMarshalNodeConnectorPlacementDispatchSubmission(t, changed)); err == nil {
				t.Fatal("changed binding or invalid issuance policy was accepted")
			}
			assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, value, before)
		})
	}
}

func TestNodeConnectorPlacementDispatchSubmissionRejectsExecutorBeforeMutation(t *testing.T) {
	value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
	executorCalls := 0
	value.broker.executor = func(NodeExecutionRequest, NodeExecutionTaskLease) { executorCalls++ }
	submissions := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
	before := nodeExecutionStateArtifacts(t, value.brokerRoot)
	if _, err := submissions.Submit(value.connection, mustMarshalNodeConnectorPlacementDispatchSubmission(t, value.fixture)); err == nil {
		t.Fatal("fake broker with an injected executor was accepted")
	}
	if executorCalls != 0 {
		t.Fatal("rejected executor-bearing broker invoked its executor")
	}
	assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, value, before)
}

func TestNodeConnectorPlacementDispatchSubmissionReplayRestartAndFreshConnectionAreIdempotent(t *testing.T) {
	value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
	submissions := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
	first := mustSubmitNodeConnectorPlacementDispatch(t, submissions, value.connection, value.fixture)
	firstRaw := mustReadNodeConnectorPlacementDispatchSubmissionFile(t, value.root)
	brokerArtifacts := nodeExecutionStateArtifacts(t, value.brokerRoot)
	second := mustSubmitNodeConnectorPlacementDispatch(t, submissions, value.connection, value.fixture)
	if !reflect.DeepEqual(first, second) || !bytes.Equal(firstRaw, mustReadNodeConnectorPlacementDispatchSubmissionFile(t, value.root)) || !nodeExecutionStringSlicesEqual(brokerArtifacts, nodeExecutionStateArtifacts(t, value.brokerRoot)) {
		t.Fatal("exact replay changed submission evidence or generated another broker lease")
	}

	restartedBroker, err := NewNodeExecutionFakeBroker(value.brokerRoot, value.machine, []NodeExecutionCapabilitySnapshot{value.capability}, nil)
	if err != nil {
		t.Fatal(err)
	}
	freshConnection := "connection-placement-submission-restarted-001"
	if err := restartedBroker.Connect(value.machine.MachineID, freshConnection); err != nil {
		t.Fatal(err)
	}
	value.broker, value.connection = restartedBroker, freshConnection
	restarted := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
	third := mustSubmitNodeConnectorPlacementDispatch(t, restarted, freshConnection, value.fixture)
	if !reflect.DeepEqual(first, third) || !bytes.Equal(firstRaw, mustReadNodeConnectorPlacementDispatchSubmissionFile(t, value.root)) || !nodeExecutionStringSlicesEqual(brokerArtifacts, nodeExecutionStateArtifacts(t, value.brokerRoot)) {
		t.Fatal("restart or fresh transient connection changed machine, capability, lease, or submission authority")
	}
}

func TestNodeConnectorPlacementDispatchSubmissionRecoversAfterLocalAtomicWriteFailure(t *testing.T) {
	value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
	submissions := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
	originalWriter := nodeConnectorPlacementDispatchSubmissionWriteAtomic
	nodeConnectorPlacementDispatchSubmissionWriteAtomic = func(string, any) error { return errors.New("injected submission write failure") }
	t.Cleanup(func() { nodeConnectorPlacementDispatchSubmissionWriteAtomic = originalWriter })
	if _, err := submissions.Submit(value.connection, mustMarshalNodeConnectorPlacementDispatchSubmission(t, value.fixture)); err == nil {
		t.Fatal("post-broker submission write failure was accepted")
	}
	if len(value.broker.state.Operations) != 1 || len(nodeExecutionStateArtifacts(t, value.brokerRoot)) != 2 {
		t.Fatal("local publication failure rewound or duplicated accepted broker history")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementDispatchSubmissionName)); !os.IsNotExist(err) {
		t.Fatal("local atomic-write failure left a partial submission artifact")
	}
	brokerArtifacts := nodeExecutionStateArtifacts(t, value.brokerRoot)
	nodeConnectorPlacementDispatchSubmissionWriteAtomic = originalWriter
	restartedBroker, err := NewNodeExecutionFakeBroker(value.brokerRoot, value.machine, []NodeExecutionCapabilitySnapshot{value.capability}, nil)
	if err != nil {
		t.Fatal(err)
	}
	freshConnection := "connection-placement-submission-recovery-001"
	if err := restartedBroker.Connect(value.machine.MachineID, freshConnection); err != nil {
		t.Fatal(err)
	}
	value.broker, value.connection = restartedBroker, freshConnection
	restarted := mustOpenNodeConnectorPlacementDispatchSubmissions(t, value)
	submission := mustSubmitNodeConnectorPlacementDispatch(t, restarted, freshConnection, value.fixture)
	if submission.TaskLease.LeaseID != restartedBroker.state.Operations[value.fixture.ExecutionRequest.OperationID].Lease.LeaseID || !nodeExecutionStringSlicesEqual(brokerArtifacts, nodeExecutionStateArtifacts(t, value.brokerRoot)) {
		t.Fatal("exact retry did not recover the existing lease without another broker generation")
	}
}

func TestNodeConnectorPlacementDispatchSubmissionTamperAndOrphanedLeaseFailClosed(t *testing.T) {
	t.Run("upstream", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
		path := filepath.Join(value.root, nodeConnectorPlacementDispatchRequestName)
		raw := mustReadNodeConnectorPlacementDispatchFile(t, value.root, nodeConnectorPlacementDispatchRequestName)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"authorization_consumed": false`), []byte(`"authorization_consumed": true`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		before := nodeExecutionStateArtifacts(t, value.brokerRoot)
		if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker); err == nil {
			t.Fatal("tampered upstream dispatch request was accepted")
		}
		assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, value, before)
	})

	t.Run("broker-state", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
		latest := filepath.Join(value.brokerRoot, nodeExecutionStateFileName(value.broker.state.Generation))
		raw, err := os.ReadFile(latest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(latest, bytes.Replace(raw, []byte(value.machine.MachineID), []byte("machine-substituted-001"), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker); err == nil {
			t.Fatal("tampered durable broker state was accepted")
		}
	})

	t.Run("lease-evidence", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
		submission := mustSubmitNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchSubmissions(t, value), value.connection, value.fixture)
		state := cloneNodeExecutionState(value.broker.state)
		operation := state.Operations[value.fixture.ExecutionRequest.OperationID]
		operation.Lease.LeaseID = "lease-tampered-000000001"
		operation.Lease.CancellationID = "cancellation-tampered-001"
		state.Operations[value.fixture.ExecutionRequest.OperationID] = operation
		state.StateFingerprint = ""
		if err := finalizeNodeExecutionState(&state); err != nil {
			t.Fatal(err)
		}
		mustWriteCanonicalNodeConnectorPlacementDispatchSubmission(t, filepath.Join(value.brokerRoot, nodeExecutionStateFileName(state.Generation)), state)
		value.broker.state = state
		if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker); err == nil {
			t.Fatal("changed broker lease evidence was accepted")
		}
		if submission.TaskLease.LeaseID == operation.Lease.LeaseID {
			t.Fatal("lease tamper test did not change the accepted lease")
		}
	})

	t.Run("existing-submission", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
		mustSubmitNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchSubmissions(t, value), value.connection, value.fixture)
		path := filepath.Join(value.root, nodeConnectorPlacementDispatchSubmissionName)
		raw := mustReadNodeConnectorPlacementDispatchSubmissionFile(t, value.root)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"executor_invoked": false`), []byte(`"executor_invoked": true`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker); err == nil {
			t.Fatal("tampered existing submission artifact was accepted or repaired")
		}
	})

	t.Run("submission-without-broker-operation", func(t *testing.T) {
		value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
		mustSubmitNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchSubmissions(t, value), value.connection, value.fixture)
		emptyBrokerRoot := t.TempDir()
		emptyBroker, err := NewNodeExecutionFakeBroker(emptyBrokerRoot, value.machine, []NodeExecutionCapabilitySnapshot{value.capability}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, emptyBroker); err == nil {
			t.Fatal("durable submission without its exact broker operation and lease was accepted")
		}
	})
}

func TestNodeConnectorPlacementDispatchSubmissionStrictInputShapeAndSchemas(t *testing.T) {
	value := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
	valid := mustMarshalNodeConnectorPlacementDispatchSubmission(t, value.fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{[]byte("{not-json"), pretty.Bytes(), append(append([]byte{}, valid...), []byte(" trailing")...), make([]byte, nodeConnectorPlacementDispatchMaxSubmissionBytes+1)}
	for _, field := range []string{"unknown", "connection_id", "executor", "connector", "event", "receipt", "cancellation", "network", "provider", "retry", "repair", "service", "git", "lifecycle"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...))
	}
	for index, raw := range inputs {
		caseValue := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
		submissions := mustOpenNodeConnectorPlacementDispatchSubmissions(t, caseValue)
		before := nodeExecutionStateArtifacts(t, caseValue.brokerRoot)
		if _, err := submissions.Submit(caseValue.connection, raw); err == nil {
			t.Fatalf("malformed, noncanonical, unknown, trailing, or oversized input %d was accepted", index)
		}
		assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t, caseValue, before)
	}

	got := map[string]string{
		"machine": NodeExecutionMachineIdentitySchema, "capability": NodeExecutionCapabilitySnapshotSchema,
		"request": NodeExecutionRequestSchema, "lease": NodeExecutionLeaseSchema, "receipt": NodeExecutionReceiptSchema,
		"session": NodeConnectorSessionNegotiationSchema, "inventory": NodeConnectorInventorySnapshotSchema,
		"placement": NodeConnectorPlacementDecisionSchema, "placement_request": NodeConnectorPlacementRequestSchema,
		"dispatch": NodeConnectorPlacementDispatchDecisionSchema, "dispatch_request": NodeConnectorPlacementDispatchRequestSchema,
		"repair": NodeConnectorMultiTargetRepairDecisionSchema, "service": NodeConnectorServiceLifecycleIntentSchema,
	}
	want := map[string]string{
		"machine": "dorkpipe.node-execution.machine-identity/v1", "capability": "dorkpipe.node-execution.capability-snapshot/v1",
		"request": "dorkpipe.node-execution.execution-request/v1", "lease": "dorkpipe.node-execution.task-lease/v1", "receipt": "dorkpipe.node-execution.execution-receipt/v1",
		"session": "dorkpipe.node-connector.session-negotiation/v1", "inventory": "dorkpipe.node-inventory-snapshot/v1",
		"placement": "dorkpipe.node-placement-decision/v1", "placement_request": "dorkpipe.node-placement-request/v1",
		"dispatch": "dorkpipe.node-placement-dispatch-decision/v1", "dispatch_request": "dorkpipe.node-placement-dispatch-request/v1",
		"repair": "dorkpipe.multi-target-repair-decision/v1", "service": "dorkpipe.node-connector-service-lifecycle-intent/v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing node-execution, session, inventory, placement, dispatch, repair, or lifecycle schema changed: %#v", got)
	}
}

func newNodeConnectorPlacementDispatchSubmissionTestFixture(t *testing.T, dispatchDecision string, connect bool) *nodeConnectorPlacementDispatchSubmissionTestFixture {
	t.Helper()
	root, inventoryExpected, inventoryFixture := nodeConnectorInventoryTestFixture(t)
	selectedIndex := 1
	now := time.Date(2026, 7, 28, 21, 0, 0, 0, time.UTC)
	machine := NodeExecutionMachineIdentity{Schema: NodeExecutionMachineIdentitySchema, MachineID: inventoryFixture.Nodes[selectedIndex].MachineID, EnrolledAt: nodeExecutionTime(now.Add(-time.Hour))}
	capability, err := NewNodeExecutionCapabilitySnapshot(machine.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "linux", Runtime: "qemu", GuestOS: "windows", GuestImageID: "windows-ci", Toolchains: []string{"go1.25"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	capabilityFingerprint, err := nodeExecutionFingerprintValue(capability)
	if err != nil {
		t.Fatal(err)
	}
	inventoryExpected.Nodes[selectedIndex].CapabilitySnapshotID = capability.SnapshotID
	inventoryExpected.Nodes[selectedIndex].CapabilitySnapshotFingerprint = capabilityFingerprint
	inventoryFixture.Nodes[selectedIndex].CapabilitySnapshotID = capability.SnapshotID
	inventoryFixture.Nodes[selectedIndex].CapabilitySnapshotFingerprint = capabilityFingerprint
	snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, inventoryExpected)
	inventory := mustRecordNodeConnectorInventory(t, snapshots, inventoryFixture)
	placement := mustRecordNodeConnectorPlacement(t, snapshots, nodeConnectorPlacementTestFixture(inventory, inventoryExpected))
	placementExpected := NodeConnectorPlacementDecisionExpected{Inventory: inventoryExpected, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint}
	placementFixture := NodeConnectorPlacementDecisionFixture{
		Schema: NodeConnectorPlacementDecisionFixtureSchema, DecisionID: "placement-decision-submission-001", ReplayIdentity: "replay-placement-submission-001", Decision: "approved",
		InventorySnapshotID: inventory.InventorySnapshotID, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint,
		PlacementInputID: placement.PlacementInputID, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint,
		WorkloadID: placement.WorkloadID, RequirementsFingerprint: placement.RequirementsFingerprint, CandidateNodeIDs: append([]string{}, placement.CandidateNodeIDs...),
		SelectedNodeID: placement.CandidateNodeIDs[selectedIndex], PlacementRequestID: "placement-request-submission-001", Provenance: nodeConnectorPlacementDecisionProvenance,
	}
	placementDecision, placementRequest := mustDecideNodeConnectorPlacement(t, mustOpenNodeConnectorPlacementDecisions(t, root, placementExpected), placementFixture)
	executionRequest, err := FinalizeNodeExecutionRequest(NodeExecutionRequest{
		OperationID: "operation-placement-submission-001", GraphRunID: "graph-placement-submission-001", RunID: "run-placement-submission-001", TaskID: "task-placement-submission-001",
		SourceRevision: strings.Repeat("b", 40), Workflow: NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "validate.submission"},
		CapabilitySnapshotID: capability.SnapshotID, Inputs: []NodeExecutionInput{{Name: "mode", Value: "readonly"}}, Artifacts: []NodeExecutionArtifactReference{}, RequestedAt: nodeExecutionTime(now),
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatchExpected := NodeConnectorPlacementDispatchExpected{Placement: placementExpected, PlacementDecisionFingerprint: placementDecision.DecisionFingerprint, PlacementRequestFingerprint: placementRequest.RequestFingerprint, ExecutionRequestFingerprint: executionRequest.RequestFingerprint}
	dispatchFixture := NodeConnectorPlacementDispatchDecisionFixture{
		Schema: NodeConnectorPlacementDispatchDecisionFixtureSchema, DecisionID: "placement-dispatch-decision-submission-001", ReplayIdentity: "replay-placement-dispatch-submission-001", Decision: dispatchDecision,
		InventorySnapshotID: inventory.InventorySnapshotID, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint,
		PlacementInputID: placement.PlacementInputID, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: placementDecision.DecisionID, PlacementDecisionFingerprint: placementDecision.DecisionFingerprint,
		PlacementRequestID: placementRequest.RequestID, PlacementRequestFingerprint: placementRequest.RequestFingerprint,
		WorkloadID: placement.WorkloadID, CandidateNodeIDs: append([]string{}, placement.CandidateNodeIDs...), SelectedNode: placementRequest.SelectedNode,
		ExecutionTaskID: executionRequest.TaskID, ExecutionRequest: executionRequest, Provenance: nodeConnectorPlacementDispatchDecisionProvenance,
	}
	if dispatchDecision == "approved" {
		dispatchFixture.PlacementDispatchRequestID = "placement-dispatch-request-submission-001"
	}
	dispatchDecisionArtifact, dispatchRequest := mustDecideNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchDecisions(t, root, dispatchExpected), dispatchFixture)
	requestFingerprint := nodeConnectorInventoryFingerprint("0")
	if dispatchRequest != nil {
		requestFingerprint = dispatchRequest.RequestFingerprint
	}
	expected := NodeConnectorPlacementDispatchSubmissionExpected{Dispatch: dispatchExpected, PlacementDispatchDecisionFingerprint: dispatchDecisionArtifact.DecisionFingerprint, PlacementDispatchRequestFingerprint: requestFingerprint}
	fixture := NodeConnectorPlacementDispatchSubmissionFixture{}
	if dispatchRequest != nil {
		fixture = NodeConnectorPlacementDispatchSubmissionFixture{
			Schema: NodeConnectorPlacementDispatchSubmissionFixtureSchema, SubmissionID: "submission-placement-dispatch-001", ReplayIdentity: "replay-placement-dispatch-submission-fixture-001",
			InventorySnapshotID: dispatchRequest.InventorySnapshotID, InventorySnapshotFingerprint: dispatchRequest.InventorySnapshotFingerprint,
			PlacementInputID: dispatchRequest.PlacementInputID, PlacementInputSnapshotFingerprint: dispatchRequest.PlacementInputSnapshotFingerprint,
			PlacementDecisionID: dispatchRequest.PlacementDecisionID, PlacementDecisionFingerprint: dispatchRequest.PlacementDecisionFingerprint,
			PlacementRequestID: dispatchRequest.PlacementRequestID, PlacementRequestFingerprint: dispatchRequest.PlacementRequestFingerprint,
			PlacementDispatchDecisionID: dispatchDecisionArtifact.DecisionID, PlacementDispatchDecisionFingerprint: dispatchDecisionArtifact.DecisionFingerprint,
			PlacementDispatchRequestID: dispatchRequest.RequestID, PlacementDispatchRequestFingerprint: dispatchRequest.RequestFingerprint,
			WorkloadID: dispatchRequest.WorkloadID, CandidateNodeIDs: append([]string{}, dispatchRequest.CandidateNodeIDs...), SelectedNode: dispatchRequest.SelectedNode,
			ExecutionTaskID: dispatchRequest.ExecutionTaskID, ExecutionRequest: cloneNodeExecutionRequest(dispatchRequest.ExecutionRequest), ExecutionRequestFingerprint: dispatchRequest.ExecutionRequest.RequestFingerprint,
			IssuedAt: nodeExecutionTime(now.Add(time.Minute)), LeaseDurationSeconds: 30 * 60, Provenance: nodeConnectorPlacementDispatchSubmissionProvenance,
		}
	}
	brokerRoot := t.TempDir()
	broker, err := NewNodeExecutionFakeBroker(brokerRoot, machine, []NodeExecutionCapabilitySnapshot{capability}, nil)
	if err != nil {
		t.Fatal(err)
	}
	connection := "connection-placement-submission-001"
	if connect {
		if err := broker.Connect(machine.MachineID, connection); err != nil {
			t.Fatal(err)
		}
	}
	return &nodeConnectorPlacementDispatchSubmissionTestFixture{root: root, expected: expected, fixture: fixture, brokerRoot: brokerRoot, broker: broker, machine: machine, capability: capability, connection: connection}
}

func mustOpenNodeConnectorPlacementDispatchSubmissions(t *testing.T, value *nodeConnectorPlacementDispatchSubmissionTestFixture) *NodeConnectorPlacementDispatchSubmissions {
	t.Helper()
	submissions, err := OpenNodeConnectorPlacementDispatchSubmissions(value.root, value.expected, value.broker)
	if err != nil {
		t.Fatal(err)
	}
	return submissions
}

func mustSubmitNodeConnectorPlacementDispatch(t *testing.T, submissions *NodeConnectorPlacementDispatchSubmissions, connection string, fixture NodeConnectorPlacementDispatchSubmissionFixture) NodeConnectorPlacementDispatchSubmission {
	t.Helper()
	value, err := submissions.Submit(connection, mustMarshalNodeConnectorPlacementDispatchSubmission(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustMarshalNodeConnectorPlacementDispatchSubmission(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorPlacementDispatchSubmissionFile(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementDispatchSubmissionName))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustRemoveNodeConnectorPlacementDispatchSubmissionFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func mustWriteCanonicalNodeConnectorPlacementDispatchSubmission(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cloneNodeConnectorPlacementDispatchSubmissionFixture(value NodeConnectorPlacementDispatchSubmissionFixture) NodeConnectorPlacementDispatchSubmissionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDispatchSubmissionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func assertNodeConnectorPlacementDispatchSubmissionBrokerUnchanged(t *testing.T, value *nodeConnectorPlacementDispatchSubmissionTestFixture, before []string) {
	t.Helper()
	if len(value.broker.state.Operations) != 0 || !nodeExecutionStringSlicesEqual(before, nodeExecutionStateArtifacts(t, value.brokerRoot)) {
		t.Fatal("rejected submission mutated the broker or issued a lease")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementDispatchSubmissionName)); !os.IsNotExist(err) {
		t.Fatal("rejected submission published local evidence")
	}
}
