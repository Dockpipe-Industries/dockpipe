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

type nodeConnectorPlacementExecutionReconciliationTestFixture struct {
	deliveryValue *nodeConnectorPlacementExecutionDeliveryTestFixture
	delivery      NodeConnectorPlacementExecutionDelivery
	expected      NodeConnectorPlacementExecutionReconciliationExpected
	fixture       NodeConnectorPlacementExecutionReconciliationDecisionFixture
}

func TestNodeConnectorPlacementExecutionReconciliationApprovedExactDecisionEmitsOneOpaqueRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	beforeRoot := nodeConnectorPlacementExecutionReconciliationRootBytes(t, value.deliveryValue.handoff.base.root)
	beforeBroker := nodeConnectorStateBytes(t, value.deliveryValue.handoff.base.brokerRoot)
	beforeOperation := value.deliveryValue.handoff.base.broker.state.Operations[value.delivery.ExecutionRequest.OperationID]
	decision, request := mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, value), value.fixture)
	if decision.Decision != "approved" || request == nil || request.RequestID != value.fixture.ReconciliationRequestID || !decision.CompleteChainRevalidated || decision.ApprovalInferred {
		t.Fatal("approved exact local decision did not emit its one reconciliation request")
	}
	if decision.Authority != (NodeConnectorPlacementExecutionReconciliationAuthority{}) || request.Authority != (NodeConnectorPlacementExecutionReconciliationAuthority{LocalGraphReconciliationRequest: true}) {
		t.Fatal("decision or request gained authority beyond requesting future local graph reconciliation")
	}
	if !request.OneTimeRequest || request.AuthorizationConsumed || !request.TerminalOutcomeOpaque || request.TerminalOutcomeInterpreted || request.GraphReconciliationPerformed || request.GraphCompletionClaimed || request.GraphFailurePropagated || request.NextTaskScheduled {
		t.Fatal("reconciliation request interpreted terminal evidence or advanced the graph")
	}
	if !nodeExecutionEqual(request.Delivery, value.delivery) || !nodeExecutionEqual(decision.Delivery, value.delivery) || !nodeExecutionEqual(request.Delivery.Events, value.delivery.Events) || !nodeExecutionEqual(request.Delivery.Receipt, value.delivery.Receipt) {
		t.Fatal("reconciliation artifacts did not preserve the exact opaque terminal delivery, events, and receipt")
	}
	if request.ConnectorInvoked || request.PreparedValidationInvoked || request.BrokerExecutorInvoked || request.BrokerOperationCreated || request.LeaseCreated || request.AttemptCreated || request.ConnectionCreated || request.SessionCreated || request.EnrollmentCreated || request.CredentialCreated || request.EventCreated || request.ReceiptCreated || request.DeliveryCreated {
		t.Fatal("reconciliation request claimed a connector, validator, broker, or protocol mutation")
	}
	_, deliveryConnectionErr := value.deliveryValue.handoff.base.broker.connectedMachine(value.deliveryValue.negotiation.ConnectionID)
	if *value.deliveryValue.validationCalls != 1 || deliveryConnectionErr == nil || !nodeConnectorStateBytesEqual(beforeBroker, nodeConnectorStateBytes(t, value.deliveryValue.handoff.base.brokerRoot)) || !reflect.DeepEqual(beforeOperation, value.deliveryValue.handoff.base.broker.state.Operations[value.delivery.ExecutionRequest.OperationID]) {
		t.Fatal("reconciliation changed terminal broker evidence or reinvoked prepared validation")
	}
	for name, raw := range beforeRoot {
		if strings.Contains(name, "reconciliation") {
			continue
		}
		if got := mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, name); !bytes.Equal(raw, got) {
			t.Fatalf("reconciliation changed upstream artifact %s", name)
		}
	}
}

func TestNodeConnectorPlacementExecutionReconciliationRejectedOrMissingDecisionEmitsNoRequest(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
		reconciliations := mustOpenNodeConnectorPlacementExecutionReconciliations(t, value)
		decision, request := reconciliations.Artifacts()
		if decision != nil || request != nil {
			t.Fatal("opening terminal evidence inferred a reconciliation decision or request")
		}
		assertNodeConnectorPlacementExecutionReconciliationArtifactsAbsent(t, value.deliveryValue.handoff.base.root)
	})

	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "rejected")
		decision, request := mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, value), value.fixture)
		if decision.Decision != "rejected" || decision.ReconciliationRequestID != "" || request != nil {
			t.Fatal("rejected reconciliation decision emitted or bound a request")
		}
		if _, err := os.Lstat(filepath.Join(value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)); !os.IsNotExist(err) {
			t.Fatal("rejected reconciliation decision published a request")
		}
	})
}

func TestNodeConnectorPlacementExecutionReconciliationRequiresEveryExactTerminalBinding(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionReconciliationDecisionFixture)
	}{
		{"delivery", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.DeliveryID = "placement-execution-delivery-changed-001"
		}},
		{"handoff-decision", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.HandoffDecisionFingerprint = nodeConnectorInventoryFingerprint("changed-handoff")
		}},
		{"handoff-request", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.HandoffRequestID = "placement-execution-handoff-request-changed-001"
		}},
		{"submission", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.SubmissionID = "submission-placement-changed-001"
		}},
		{"broker-operation", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.ExecutionRequest.OperationID = "operation-placement-changed-001"
		}},
		{"request", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.ExecutionRequest.TaskID = "task-placement-changed-001"
		}},
		{"lease", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.TaskLease.LeaseID = "lease-placement-changed-001"
		}},
		{"attempt", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) { v.Delivery.TaskLease.Attempt++ }},
		{"events", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.Events[0], v.Delivery.Events[1] = v.Delivery.Events[1], v.Delivery.Events[0]
		}},
		{"receipt", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.Receipt.Result = "failed"
		}},
		{"workflow", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.ConnectorWorkflow.Name = "validate.changed"
		}},
		{"revision", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.ConnectorSourceRevision = strings.Repeat("c", 40)
		}},
		{"selected-node", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.SelectedNode.NodeID = "node-placement-changed-001"
		}},
		{"machine", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.MachineID = "machine-placement-changed-001"
		}},
		{"capability", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.CapabilitySnapshotFingerprint = nodeConnectorInventoryFingerprint("changed-capability")
		}},
		{"profile", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.Delivery.SelectedNode.Profile.Runtime = "docker"
		}},
		{"decision-id", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.DecisionID = ""
		}},
		{"replay", func(v *NodeConnectorPlacementExecutionReconciliationDecisionFixture) {
			v.ReplayIdentity = ""
		}},
	}
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	reconciliations := mustOpenNodeConnectorPlacementExecutionReconciliations(t, value)
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionReconciliationDecisionFixture(value.fixture)
			test.mutate(&changed)
			assertNodeConnectorPlacementExecutionReconciliationRejected(t, value, reconciliations, mustMarshalNodeConnectorPlacementExecutionReconciliation(t, changed))
		})
	}
}

func TestNodeConnectorPlacementExecutionReconciliationEvidenceCannotImplyApproval(t *testing.T) {
	fields := []string{"terminal_success", "receipt_authority", "availability", "load", "risk", "cost", "ordering", "ranking", "recommendation", "connection", "presence", "health", "validation", "provider", "broker_acceptance", "lease_exists"}
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	valid := mustMarshalNodeConnectorPlacementExecutionReconciliation(t, value.fixture)
	reconciliations := mustOpenNodeConnectorPlacementExecutionReconciliations(t, value)
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			raw := append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...)
			assertNodeConnectorPlacementExecutionReconciliationRejected(t, value, reconciliations, raw)
		})
	}
}

func TestNodeConnectorPlacementExecutionReconciliationDisconnectedSessionPreservesDurableAuthority(t *testing.T) {
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	if _, err := value.deliveryValue.handoff.base.broker.connectedMachine(value.deliveryValue.negotiation.ConnectionID); err == nil {
		t.Fatal("test precondition requires a disconnected current session")
	}
	decision, request := mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, value), value.fixture)
	if decision.Decision != "approved" || request == nil || !nodeExecutionEqual(request.Delivery.Receipt, value.delivery.Receipt) {
		t.Fatal("disconnect erased or replaced the durable terminal result")
	}
}

func TestNodeConnectorPlacementExecutionReconciliationReplayRestartAndAtomicRecoveryAreIdempotent(t *testing.T) {
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	reconciliations := mustOpenNodeConnectorPlacementExecutionReconciliations(t, value)
	originalDecisionWriter := nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic
	originalRequestWriter := nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic
	t.Cleanup(func() {
		nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic = originalDecisionWriter
		nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic = originalRequestWriter
	})
	nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic = func(string, any) error { return errors.New("injected decision write failure") }
	if _, _, err := reconciliations.Decide(mustMarshalNodeConnectorPlacementExecutionReconciliation(t, value.fixture)); err == nil {
		t.Fatal("decision atomic-write failure was accepted")
	}
	assertNodeConnectorPlacementExecutionReconciliationArtifactsAbsent(t, value.deliveryValue.handoff.base.root)

	nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic = originalDecisionWriter
	nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic = func(string, any) error { return errors.New("injected request write failure") }
	if _, _, err := reconciliations.Decide(mustMarshalNodeConnectorPlacementExecutionReconciliation(t, value.fixture)); err == nil {
		t.Fatal("request atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationDecisionName)); err != nil {
		t.Fatal("approved decision was not durable before request publication failure")
	}
	if _, err := os.Lstat(filepath.Join(value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)); !os.IsNotExist(err) {
		t.Fatal("request atomic-write failure left a partial request")
	}

	nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic = originalRequestWriter
	restarted := mustOpenNodeConnectorPlacementExecutionReconciliations(t, value)
	decision, request := mustDecideNodeConnectorPlacementExecutionReconciliation(t, restarted, value.fixture)
	if request == nil {
		t.Fatal("restart did not recover the exact request from the durable decision")
	}
	decisionRaw := mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationDecisionName)
	requestRaw := mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)
	writes := 0
	nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic = func(string, any) error { writes++; return nil }
	nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic = func(string, any) error { writes++; return nil }
	replayedDecision, replayedRequest := mustDecideNodeConnectorPlacementExecutionReconciliation(t, restarted, value.fixture)
	if writes != 0 || !reflect.DeepEqual(decision, replayedDecision) || !reflect.DeepEqual(request, replayedRequest) || !bytes.Equal(decisionRaw, mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationDecisionName)) || !bytes.Equal(requestRaw, mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)) {
		t.Fatal("exact replay rewrote or duplicated reconciliation artifacts")
	}
	changed := cloneNodeConnectorPlacementExecutionReconciliationDecisionFixture(value.fixture)
	changed.ReplayIdentity = "replay-placement-reconciliation-conflict-001"
	assertNodeConnectorPlacementExecutionReconciliationRejected(t, value, restarted, mustMarshalNodeConnectorPlacementExecutionReconciliation(t, changed))
	changed = cloneNodeConnectorPlacementExecutionReconciliationDecisionFixture(value.fixture)
	changed.DecisionID = "placement-execution-reconciliation-decision-conflict-001"
	assertNodeConnectorPlacementExecutionReconciliationRejected(t, value, restarted, mustMarshalNodeConnectorPlacementExecutionReconciliation(t, changed))
}

func TestNodeConnectorPlacementExecutionReconciliationRejectsMalformedUnknownTrailingOversizedNoncanonicalAndCollidingInput(t *testing.T) {
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	valid := mustMarshalNodeConnectorPlacementExecutionReconciliation(t, value.fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{[]byte("{not-json"), pretty.Bytes(), append(append([]byte{}, valid...), []byte(" trailing")...), make([]byte, nodeConnectorPlacementExecutionReconciliationMaxDecisionBytes+1)}
	for _, field := range []string{"unknown", "graph_completion", "next_task", "retry", "repair", "provider", "connection", "receipt_authority"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...))
	}
	reconciliations := mustOpenNodeConnectorPlacementExecutionReconciliations(t, value)
	for _, raw := range inputs {
		assertNodeConnectorPlacementExecutionReconciliationRejected(t, value, reconciliations, raw)
		assertNodeConnectorPlacementExecutionReconciliationArtifactsAbsent(t, value.deliveryValue.handoff.base.root)
	}
	for _, collision := range []struct{ decision, replay, request string }{
		{decision: value.delivery.DeliveryID, replay: value.fixture.ReplayIdentity, request: value.fixture.ReconciliationRequestID},
		{decision: value.fixture.DecisionID, replay: value.delivery.ReplayIdentity, request: value.fixture.ReconciliationRequestID},
		{decision: value.fixture.DecisionID, replay: value.fixture.ReplayIdentity, request: value.delivery.Receipt.ReceiptID},
	} {
		changed := cloneNodeConnectorPlacementExecutionReconciliationDecisionFixture(value.fixture)
		changed.DecisionID, changed.ReplayIdentity, changed.ReconciliationRequestID = collision.decision, collision.replay, collision.request
		assertNodeConnectorPlacementExecutionReconciliationRejected(t, value, reconciliations, mustMarshalNodeConnectorPlacementExecutionReconciliation(t, changed))
	}
}

func TestNodeConnectorPlacementExecutionReconciliationRejectsUpstreamBrokerDeliveryAndExistingArtifactTamper(t *testing.T) {
	tests := []struct {
		name string
		run  func(*nodeConnectorPlacementExecutionReconciliationTestFixture)
	}{
		{"upstream", func(value *nodeConnectorPlacementExecutionReconciliationTestFixture) {
			path := filepath.Join(value.deliveryValue.handoff.base.root, nodeConnectorPlacementRequestName)
			raw := mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementRequestName)
			if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"placement_dispatched": false`), []byte(`"placement_dispatched": true`), 1), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"broker-history", func(value *nodeConnectorPlacementExecutionReconciliationTestFixture) {
			states := nodeExecutionStateArtifacts(t, value.deliveryValue.handoff.base.brokerRoot)
			path := filepath.Join(value.deliveryValue.handoff.base.brokerRoot, states[len(states)-1])
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"result": "succeeded"`), []byte(`"result": "failed"`), 1), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"delivery", func(value *nodeConnectorPlacementExecutionReconciliationTestFixture) {
			path := filepath.Join(value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionDeliveryName)
			raw := mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionDeliveryName)
			if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"receipt_materialized": true`), []byte(`"receipt_materialized": false`), 1), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
			test.run(value)
			if _, err := OpenNodeConnectorPlacementExecutionReconciliations(value.deliveryValue.handoff.base.root, value.expected, value.deliveryValue.handoff.base.broker); err == nil {
				t.Fatal("tampered upstream, broker history, or delivery evidence was accepted or repaired")
			}
			assertNodeConnectorPlacementExecutionReconciliationArtifactsAbsent(t, value.deliveryValue.handoff.base.root)
		})
	}

	t.Run("existing-artifact", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
		mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, value), value.fixture)
		path := filepath.Join(value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)
		raw := mustReadNodeConnectorPlacementExecutionReconciliationFile(t, value.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"terminal_outcome_opaque": true`), []byte(`"terminal_outcome_opaque": false`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionReconciliations(value.deliveryValue.handoff.base.root, value.expected, value.deliveryValue.handoff.base.broker); err == nil {
			t.Fatal("tampered existing reconciliation request was accepted or repaired")
		}
	})
}

func TestNodeConnectorPlacementExecutionReconciliationSchemasCountsAndBoundariesRemainUnchanged(t *testing.T) {
	value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	beforeBroker := nodeConnectorStateBytes(t, value.deliveryValue.handoff.base.brokerRoot)
	decision, request := mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, value), value.fixture)
	got := map[string]string{
		"request": NodeExecutionRequestSchema, "lease": NodeExecutionLeaseSchema, "receipt": NodeExecutionReceiptSchema,
		"delivery": NodeConnectorPlacementExecutionDeliverySchema, "decision_fixture": NodeConnectorPlacementExecutionReconciliationDecisionFixtureSchema,
		"decision": NodeConnectorPlacementExecutionReconciliationDecisionSchema, "reconciliation_request": NodeConnectorPlacementExecutionReconciliationRequestSchema,
	}
	want := map[string]string{
		"request": "dorkpipe.node-execution.execution-request/v1", "lease": "dorkpipe.node-execution.task-lease/v1", "receipt": "dorkpipe.node-execution.execution-receipt/v1",
		"delivery": "dorkpipe.node-placement-execution-delivery/v1", "decision_fixture": "dorkpipe.node-placement-execution-reconciliation-decision-fixture/v1",
		"decision": "dorkpipe.node-placement-execution-reconciliation-decision/v1", "reconciliation_request": "dorkpipe.node-placement-execution-reconciliation-request/v1",
	}
	if !reflect.DeepEqual(got, want) || request == nil || decision.Authority != (NodeConnectorPlacementExecutionReconciliationAuthority{}) || request.Authority != (NodeConnectorPlacementExecutionReconciliationAuthority{LocalGraphReconciliationRequest: true}) {
		t.Fatalf("existing TASK-015 schemas or reconciliation authority changed unexpectedly: %#v", got)
	}
	if *value.deliveryValue.validationCalls != 1 || request.ConnectorInvoked || request.PreparedValidationInvoked || request.BrokerExecutorInvoked || !nodeConnectorStateBytesEqual(beforeBroker, nodeConnectorStateBytes(t, value.deliveryValue.handoff.base.brokerRoot)) {
		t.Fatal("reconciliation invoked connector-session, prepared validation, or fake-broker executor")
	}
	if strings.Contains(strings.ToLower(string(mustMarshalNodeConnectorPlacementExecutionReconciliation(t, request))), "graph_status") {
		t.Fatal("reconciliation request derived a graph status from opaque terminal evidence")
	}
}

func newNodeConnectorPlacementExecutionReconciliationTestFixture(t *testing.T, decision string) *nodeConnectorPlacementExecutionReconciliationTestFixture {
	t.Helper()
	deliveryValue := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
	delivery := mustDeliverNodeConnectorPlacementExecution(t, mustOpenNodeConnectorPlacementExecutionDeliveries(t, deliveryValue), deliveryValue.fixture, deliveryValue.connector)
	deliveryValue.handoff.base.broker.Disconnect(deliveryValue.negotiation.ConnectionID)
	expected := NodeConnectorPlacementExecutionReconciliationExpected{Delivery: deliveryValue.expected, DeliveryFingerprint: delivery.DeliveryFingerprint}
	fixture := NodeConnectorPlacementExecutionReconciliationDecisionFixture{
		Schema:     NodeConnectorPlacementExecutionReconciliationDecisionFixtureSchema,
		DecisionID: "placement-execution-reconciliation-decision-001", ReplayIdentity: "replay-placement-execution-reconciliation-001", Decision: decision,
		Delivery: cloneNodeConnectorPlacementExecutionDelivery(delivery), Provenance: nodeConnectorPlacementExecutionReconciliationDecisionProvenance,
	}
	if decision == "approved" {
		fixture.ReconciliationRequestID = "placement-execution-reconciliation-request-001"
	}
	return &nodeConnectorPlacementExecutionReconciliationTestFixture{deliveryValue: deliveryValue, delivery: delivery, expected: expected, fixture: fixture}
}

func mustOpenNodeConnectorPlacementExecutionReconciliations(t *testing.T, value *nodeConnectorPlacementExecutionReconciliationTestFixture) *NodeConnectorPlacementExecutionReconciliations {
	t.Helper()
	reconciliations, err := OpenNodeConnectorPlacementExecutionReconciliations(value.deliveryValue.handoff.base.root, value.expected, value.deliveryValue.handoff.base.broker)
	if err != nil {
		t.Fatal(err)
	}
	return reconciliations
}

func mustDecideNodeConnectorPlacementExecutionReconciliation(t *testing.T, reconciliations *NodeConnectorPlacementExecutionReconciliations, fixture NodeConnectorPlacementExecutionReconciliationDecisionFixture) (NodeConnectorPlacementExecutionReconciliationDecision, *NodeConnectorPlacementExecutionReconciliationRequest) {
	t.Helper()
	decision, request, err := reconciliations.Decide(mustMarshalNodeConnectorPlacementExecutionReconciliation(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func assertNodeConnectorPlacementExecutionReconciliationRejected(t *testing.T, value *nodeConnectorPlacementExecutionReconciliationTestFixture, reconciliations *NodeConnectorPlacementExecutionReconciliations, raw []byte) {
	t.Helper()
	beforeBroker := nodeConnectorStateBytes(t, value.deliveryValue.handoff.base.brokerRoot)
	validationBefore := *value.deliveryValue.validationCalls
	if _, _, err := reconciliations.Decide(raw); err == nil {
		t.Fatal("changed, inferred, malformed, stale, replayed, or conflicting reconciliation input was accepted")
	}
	if validationBefore != *value.deliveryValue.validationCalls || !nodeConnectorStateBytesEqual(beforeBroker, nodeConnectorStateBytes(t, value.deliveryValue.handoff.base.brokerRoot)) {
		t.Fatal("rejected reconciliation invoked validation or changed broker evidence")
	}
}

func assertNodeConnectorPlacementExecutionReconciliationArtifactsAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{nodeConnectorPlacementExecutionReconciliationDecisionName, nodeConnectorPlacementExecutionReconciliationRequestName} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("rejected or missing decision left reconciliation artifact %s", name)
		}
	}
}

func nodeConnectorPlacementExecutionReconciliationRootBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		result[entry.Name()] = raw
	}
	return result
}

func mustMarshalNodeConnectorPlacementExecutionReconciliation(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorPlacementExecutionReconciliationFile(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementExecutionReconciliationDecisionFixture(value NodeConnectorPlacementExecutionReconciliationDecisionFixture) NodeConnectorPlacementExecutionReconciliationDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionReconciliationDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
