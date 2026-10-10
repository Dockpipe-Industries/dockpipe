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

type nodeConnectorPlacementExecutionDeliveryTestFixture struct {
	handoff         *nodeConnectorPlacementExecutionHandoffTestFixture
	decision        NodeConnectorPlacementExecutionHandoffDecision
	request         NodeConnectorPlacementExecutionHandoffRequest
	session         *NodeConnectorSessionFake
	negotiation     NodeConnectorSessionNegotiation
	expected        NodeConnectorPlacementExecutionDeliveryExpected
	fixture         NodeConnectorPlacementExecutionDeliveryFixture
	connector       *NodeValidationConnector
	validationCalls *int
	transportCalls  *int
}

func TestNodeConnectorPlacementExecutionDeliveryApprovedExactTransitionInvokesOnce(t *testing.T) {
	value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
	before := cloneNodeExecutionState(value.handoff.base.broker.state)
	connectionsBefore := len(value.handoff.base.broker.connections)
	sessionBefore := cloneNodeConnectorSessionState(value.session.state)
	delivery := mustDeliverNodeConnectorPlacementExecution(t, mustOpenNodeConnectorPlacementExecutionDeliveries(t, value), value.fixture, value.connector)
	operationBefore := before.Operations[value.request.ExecutionRequest.OperationID]
	operationAfter := value.handoff.base.broker.state.Operations[value.request.ExecutionRequest.OperationID]
	if *value.validationCalls != 1 || *value.transportCalls != 1 || delivery.BrokerExecutorInvocations != 0 || len(value.handoff.base.broker.connections) != connectionsBefore {
		t.Fatalf("unexpected invocation counts: session_transport=%d validation=%d executor=%d", *value.transportCalls, *value.validationCalls, delivery.BrokerExecutorInvocations)
	}
	if !delivery.AuthorizationConsumed || !delivery.ConnectorSessionInvoked || !delivery.ValidationConnectorInvoked || !delivery.PreparedValidationInvoked || !delivery.EventsPublished || !delivery.ReceiptPublished || !delivery.ReceiptMaterialized {
		t.Fatal("accepted delivery did not record only the exact connector-session validation transition")
	}
	if delivery.Authority != (NodeConnectorPlacementExecutionDeliveryAuthority{}) || delivery.NewBrokerOperations != 0 || delivery.NewLeases != 0 || delivery.NewAttempts != 0 || delivery.NewConnections != 0 || delivery.NewSessions != 0 || delivery.NewEnrollments != 0 || delivery.NewCredentials != 0 {
		t.Fatal("delivery gained adjacent authority or claimed a new protocol identity")
	}
	if len(value.handoff.base.broker.state.Operations) != 1 || !reflect.DeepEqual(operationBefore.Request, operationAfter.Request) || !reflect.DeepEqual(operationBefore.Lease, operationAfter.Lease) || operationAfter.ExecutionCount != operationBefore.ExecutionCount || operationAfter.Receipt == nil {
		t.Fatal("delivery changed the original broker operation, request, lease, or attempt")
	}
	if !reflect.DeepEqual(sessionBefore, value.session.state) || !reflect.DeepEqual(delivery.Events, operationAfter.Events) || !reflect.DeepEqual(delivery.Receipt, *operationAfter.Receipt) || delivery.ReceiptFingerprint != operationAfter.Receipt.ReceiptFingerprint {
		t.Fatal("delivery did not bind the unchanged session and exact seam events and receipt")
	}
	raw := mustReadNodeConnectorPlacementExecutionDeliveryFile(t, value.handoff.base.root)
	var decoded NodeConnectorPlacementExecutionDelivery
	if len(raw) > nodeConnectorPlacementExecutionDeliveryMaxArtifactBytes {
		t.Fatal("durable delivery artifact exceeds its encoded bound")
	}
	if err := decodeNodeConnectorPlacementExecutionDeliveryArtifact(raw, &decoded); err != nil {
		t.Fatalf("durable delivery artifact is not strict canonical JSON: %v", err)
	}
	if !nodeExecutionEqual(decoded, delivery) {
		t.Fatal("durable delivery artifact is not the exact returned artifact")
	}
}

func TestNodeConnectorPlacementExecutionDeliveryRejectedOrMissingHandoffInvokesNothing(t *testing.T) {
	t.Run("missing-request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		if err := os.Remove(filepath.Join(value.handoff.base.root, nodeConnectorPlacementExecutionHandoffRequestName)); err != nil {
			t.Fatal(err)
		}
		before := nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)
		if _, err := OpenNodeConnectorPlacementExecutionDeliveries(value.handoff.base.root, value.expected, value.handoff.base.broker, value.session, value.negotiation); err == nil {
			t.Fatal("missing handoff request was accepted")
		}
		assertNodeConnectorPlacementExecutionDeliveryNotInvoked(t, value, before)
	})

	t.Run("rejected-decision", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "rejected")
		decision, request := mustDecideNodeConnectorPlacementExecutionHandoff(t, mustOpenNodeConnectorPlacementExecutionHandoffs(t, value), value.fixture)
		if request != nil || decision.Decision != "rejected" {
			t.Fatal("rejected handoff fixture did not establish the rejection precondition")
		}
		fingerprint := nodeConnectorInventoryFingerprint("rejected-delivery-placeholder")
		expected := NodeConnectorPlacementExecutionDeliveryExpected{Handoff: value.expected, HandoffDecisionFingerprint: decision.DecisionFingerprint, HandoffRequestFingerprint: fingerprint, SessionStateFingerprint: fingerprint, NegotiationFingerprint: fingerprint}
		before := nodeConnectorStateBytes(t, value.base.brokerRoot)
		if _, err := OpenNodeConnectorPlacementExecutionDeliveries(value.base.root, expected, value.base.broker, nil, NodeConnectorSessionNegotiation{}); err == nil {
			t.Fatal("rejected handoff decision was accepted for delivery")
		}
		if !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, value.base.brokerRoot)) {
			t.Fatal("rejected handoff mutated broker evidence")
		}
	})
}

func TestNodeConnectorPlacementExecutionDeliveryRevalidatesCompleteBindings(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionDeliveryFixture)
	}{
		{"handoff-decision", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.HandoffDecisionFingerprint = nodeConnectorInventoryFingerprint("changed-decision")
		}},
		{"handoff-request", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.HandoffRequestID = "placement-execution-handoff-request-changed-001"
		}},
		{"submission", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.SubmissionReplayIdentity = "replay-placement-submission-changed-001"
		}},
		{"broker-state", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.BrokerStateFingerprintBefore = nodeConnectorInventoryFingerprint("changed-broker")
		}},
		{"session-state", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.SessionStateFingerprint = nodeConnectorInventoryFingerprint("changed-session")
		}},
		{"enrollment", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.EnrollmentID = "enrollment-delivery-changed-001"
		}},
		{"credential", func(v *NodeConnectorPlacementExecutionDeliveryFixture) { v.CredentialID = "cred-delivery-changed-001" }},
		{"session", func(v *NodeConnectorPlacementExecutionDeliveryFixture) { v.SessionID = "session-delivery-changed-001" }},
		{"connection", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.ConnectionID = "connection-delivery-changed-001"
		}},
		{"negotiation", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.NegotiationFingerprint = nodeConnectorInventoryFingerprint("changed-negotiation")
		}},
		{"machine", func(v *NodeConnectorPlacementExecutionDeliveryFixture) { v.MachineID = "machine-delivery-changed-001" }},
		{"capability", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.CapabilitySnapshotID = "capability-delivery-changed-001"
		}},
		{"candidate-set", func(v *NodeConnectorPlacementExecutionDeliveryFixture) { v.CandidateNodeIDs = v.CandidateNodeIDs[1:] }},
		{"selected-node", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.SelectedNode.NodeID = "node-delivery-changed-001"
		}},
		{"execution-request", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.ExecutionRequest.TaskID = "task-delivery-changed-001"
		}},
		{"lease", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.TaskLease.LeaseID = "lease-delivery-changed-001"
		}},
		{"workflow", func(v *NodeConnectorPlacementExecutionDeliveryFixture) { v.ConnectorWorkflow.Name = "validate.changed" }},
		{"revision", func(v *NodeConnectorPlacementExecutionDeliveryFixture) {
			v.ConnectorSourceRevision = strings.Repeat("c", 40)
		}},
		{"delivery-time", func(v *NodeConnectorPlacementExecutionDeliveryFixture) { v.DeliveredAt = v.TaskLease.ExpiresAt }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
			changed := cloneNodeConnectorPlacementExecutionDeliveryFixture(value.fixture)
			test.mutate(&changed)
			assertNodeConnectorPlacementExecutionDeliveryRejected(t, value, mustOpenNodeConnectorPlacementExecutionDeliveries(t, value), changed, value.connector)
		})
	}
}

func TestNodeConnectorPlacementExecutionDeliveryEvidenceCannotImplyApproval(t *testing.T) {
	for _, field := range []string{"connection_present", "healthy", "available", "load", "risk", "cost", "ordering", "ranking", "provider_evidence", "broker_acceptance", "lease_exists", "receipt_authority"} {
		t.Run(field, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
			valid := mustMarshalNodeConnectorPlacementExecutionDelivery(t, value.fixture)
			raw := append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...)
			assertNodeConnectorPlacementExecutionDeliveryRawRejected(t, value, mustOpenNodeConnectorPlacementExecutionDeliveries(t, value), raw, value.connector)
		})
	}
}

func TestNodeConnectorPlacementExecutionDeliveryReplayRestartAndAtomicRecoveryAreIdempotent(t *testing.T) {
	value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
	deliveries := mustOpenNodeConnectorPlacementExecutionDeliveries(t, value)
	original := nodeConnectorPlacementExecutionDeliveryWriteAtomic
	nodeConnectorPlacementExecutionDeliveryWriteAtomic = func(string, any) error { return errors.New("injected delivery write failure") }
	t.Cleanup(func() { nodeConnectorPlacementExecutionDeliveryWriteAtomic = original })
	if _, err := deliveries.Deliver(mustMarshalNodeConnectorPlacementExecutionDelivery(t, value.fixture), value.connector); err == nil {
		t.Fatal("injected delivery artifact failure was accepted")
	}
	if *value.validationCalls != 1 {
		t.Fatalf("terminal broker publication did not invoke prepared validation exactly once: %d", *value.validationCalls)
	}
	if _, err := os.Lstat(filepath.Join(value.handoff.base.root, nodeConnectorPlacementExecutionDeliveryName)); !os.IsNotExist(err) {
		t.Fatal("atomic delivery write failure left a partial local artifact")
	}
	terminalBroker := nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)
	terminalSession := cloneNodeConnectorSessionState(value.session.state)
	nodeConnectorPlacementExecutionDeliveryWriteAtomic = original
	recovered := mustDeliverNodeConnectorPlacementExecution(t, deliveries, value.fixture, value.connector)
	if *value.validationCalls != 1 || !nodeConnectorStateBytesEqual(terminalBroker, nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)) || !reflect.DeepEqual(terminalSession, value.session.state) {
		t.Fatal("exact retry reinvoked the seam or validation or published duplicate broker/session evidence")
	}
	restartCalls := 0
	restartConnector, err := NewNodeValidationConnector(value.fixture.ConnectorWorkflow, value.fixture.ConnectorSourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		restartCalls++
		return NodeValidationEvidence{}, errors.New("terminal restart must not invoke validation")
	})
	if err != nil {
		t.Fatal(err)
	}
	restarted := mustOpenNodeConnectorPlacementExecutionDeliveries(t, value)
	replayed := mustDeliverNodeConnectorPlacementExecution(t, restarted, value.fixture, restartConnector)
	if restartCalls != 0 || !reflect.DeepEqual(recovered, replayed) || !nodeConnectorStateBytesEqual(terminalBroker, nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)) {
		t.Fatal("restart replay did not recover the identical delivery without reinvocation")
	}
	changed := cloneNodeConnectorPlacementExecutionDeliveryFixture(value.fixture)
	changed.ReplayIdentity = "replay-placement-execution-delivery-changed-001"
	assertNodeConnectorPlacementExecutionDeliveryRejected(t, value, restarted, changed, restartConnector)
}

func TestNodeConnectorPlacementExecutionDeliveryRejectsMalformedUnknownTrailingOversizedAndNoncanonicalJSON(t *testing.T) {
	value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
	valid := mustMarshalNodeConnectorPlacementExecutionDelivery(t, value.fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{[]byte("{not-json"), pretty.Bytes(), append(append([]byte{}, valid...), []byte(" trailing")...), make([]byte, nodeConnectorPlacementExecutionDeliveryMaxFixtureBytes+1)}
	for _, field := range []string{"unknown", "cancellation", "retry", "repair", "service", "network", "provider", "mutation", "git", "publication", "completion", "next_task"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...))
	}
	for index, raw := range inputs {
		caseValue := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		assertNodeConnectorPlacementExecutionDeliveryRawRejected(t, caseValue, mustOpenNodeConnectorPlacementExecutionDeliveries(t, caseValue), raw, caseValue.connector)
		if _, err := os.Lstat(filepath.Join(caseValue.handoff.base.root, nodeConnectorPlacementExecutionDeliveryName)); !os.IsNotExist(err) {
			t.Fatalf("invalid input %d published partial delivery evidence", index)
		}
	}
}

func TestNodeConnectorPlacementExecutionDeliveryRejectsStaleSessionTamperAndConflictingEvidence(t *testing.T) {
	t.Run("stale-health", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		deliveries := mustOpenNodeConnectorPlacementExecutionDeliveries(t, value)
		evidence := mustFinalizeNodeConnectorSessionEvidence(t, NodeConnectorSessionEvidence{Sequence: int64(len(value.session.state.Transitions) + 1), EvidenceID: "evidence-delivery-unhealthy-001", ReplayIdentity: "replay-delivery-unhealthy-001", Kind: "health", Status: "unhealthy", SessionID: value.negotiation.SessionID, ConnectionID: value.negotiation.ConnectionID, EnrollmentID: value.negotiation.EnrollmentID, MachineID: value.negotiation.MachineID, CredentialID: value.negotiation.CredentialID, CapabilitySnapshotID: value.negotiation.CapabilitySnapshotID, ObservedAt: nodeExecutionTime(time.Date(2026, 7, 28, 21, 2, 10, 0, time.UTC))})
		mustRecordNodeConnectorEvidence(t, value.session, evidence)
		assertNodeConnectorPlacementExecutionDeliveryRejected(t, value, deliveries, value.fixture, value.connector)
	})

	t.Run("upstream-tamper", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		path := filepath.Join(value.handoff.base.root, nodeConnectorPlacementExecutionHandoffRequestName)
		raw := mustReadNodeConnectorPlacementExecutionHandoffFile(t, value.handoff.base.root, nodeConnectorPlacementExecutionHandoffRequestName)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"connector_invoked": false`), []byte(`"connector_invoked": true`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionDeliveries(value.handoff.base.root, value.expected, value.handoff.base.broker, value.session, value.negotiation); err == nil {
			t.Fatal("tampered handoff request was accepted or repaired")
		}
	})

	t.Run("terminal-evidence-substitution", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		changed := cloneNodeConnectorPlacementExecutionDeliveryFixture(value.fixture)
		changed.ExpectedEvents[0].EnvelopeFingerprint = nodeConnectorInventoryFingerprint("changed-event")
		assertNodeConnectorPlacementExecutionDeliveryRejected(t, value, mustOpenNodeConnectorPlacementExecutionDeliveries(t, value), changed, value.connector)
	})

	t.Run("existing-artifact-tamper", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		mustDeliverNodeConnectorPlacementExecution(t, mustOpenNodeConnectorPlacementExecutionDeliveries(t, value), value.fixture, value.connector)
		path := filepath.Join(value.handoff.base.root, nodeConnectorPlacementExecutionDeliveryName)
		raw := mustReadNodeConnectorPlacementExecutionDeliveryFile(t, value.handoff.base.root)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"receipt_materialized": true`), []byte(`"receipt_materialized": false`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionDeliveries(value.handoff.base.root, value.expected, value.handoff.base.broker, value.session, value.negotiation); err == nil {
			t.Fatal("tampered existing delivery artifact was accepted or repaired")
		}
	})
}

func TestNodeConnectorPlacementExecutionDeliveryExistingSchemasAndAuthorityRemainUnchanged(t *testing.T) {
	got := map[string]string{"request": NodeExecutionRequestSchema, "lease": NodeExecutionLeaseSchema, "receipt": NodeExecutionReceiptSchema, "session": NodeConnectorSessionNegotiationSchema, "submission": NodeConnectorPlacementDispatchSubmissionSchema, "handoff": NodeConnectorPlacementExecutionHandoffRequestSchema, "delivery_fixture": NodeConnectorPlacementExecutionDeliveryFixtureSchema, "delivery": NodeConnectorPlacementExecutionDeliverySchema}
	want := map[string]string{"request": "dorkpipe.node-execution.execution-request/v1", "lease": "dorkpipe.node-execution.task-lease/v1", "receipt": "dorkpipe.node-execution.execution-receipt/v1", "session": "dorkpipe.node-connector.session-negotiation/v1", "submission": "dorkpipe.node-placement-dispatch-submission/v1", "handoff": "dorkpipe.node-placement-execution-handoff-request/v1", "delivery_fixture": "dorkpipe.node-placement-execution-delivery-fixture/v1", "delivery": "dorkpipe.node-placement-execution-delivery/v1"}
	if !reflect.DeepEqual(got, want) || (NodeConnectorPlacementExecutionDeliveryAuthority{}) != (NodeConnectorPlacementExecutionDeliveryAuthority{}) {
		t.Fatalf("an existing TASK-015 schema or delivery authority changed unexpectedly: %#v", got)
	}
}

func newNodeConnectorPlacementExecutionDeliveryTestFixture(t *testing.T) *nodeConnectorPlacementExecutionDeliveryTestFixture {
	t.Helper()
	handoff := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionHandoff(t, mustOpenNodeConnectorPlacementExecutionHandoffs(t, handoff), handoff.fixture)
	request := *requestPointer
	enrollment, err := FinalizeNodeConnectorEnrollment(NodeConnectorEnrollment{EnrollmentID: "enrollment-placement-delivery-001", MachineID: request.SelectedNode.MachineID, InitialCredentialID: "cred-placement-delivery-001", EnrolledAt: nodeExecutionTime(time.Date(2026, 7, 28, 20, 0, 0, 0, time.UTC))})
	if err != nil {
		t.Fatal(err)
	}
	transportCalls := 0
	transport := func(hello NodeConnectorSessionHello) (NodeConnectorSessionNegotiation, error) {
		transportCalls++
		return FinalizeNodeConnectorSessionNegotiation(NodeConnectorSessionNegotiation{Sequence: hello.Sequence, NegotiationID: hello.NegotiationID, SessionID: "session-placement-delivery-001", ConnectionID: hello.ConnectionID, EnrollmentID: hello.EnrollmentID, MachineID: hello.MachineID, CredentialID: hello.CredentialID, CapabilitySnapshotID: hello.CapabilitySnapshotID, NegotiatedAt: hello.ObservedAt, HelloFingerprint: hello.HelloFingerprint})
	}
	session, err := NewNodeConnectorSessionFake(t.TempDir(), handoff.base.broker, enrollment, transport)
	if err != nil {
		t.Fatal(err)
	}
	hello, err := FinalizeNodeConnectorSessionHello(NodeConnectorSessionHello{Sequence: 1, NegotiationID: "negotiation-placement-delivery-001", ReplayIdentity: "replay-negotiation-placement-delivery-001", EnrollmentID: enrollment.EnrollmentID, MachineID: enrollment.MachineID, CredentialID: enrollment.InitialCredentialID, ConnectionID: "connection-placement-delivery-001", CapabilitySnapshotID: request.SelectedNode.CapabilitySnapshotID, ObservedAt: nodeExecutionTime(time.Date(2026, 7, 28, 21, 1, 10, 0, time.UTC))})
	if err != nil {
		t.Fatal(err)
	}
	negotiation, err := session.Negotiate(mustMarshalNodeConnectorPlacementExecutionDelivery(t, hello))
	if err != nil {
		t.Fatal(err)
	}
	for sequence, spec := range []struct{ id, replay, kind, status string }{{"evidence-placement-delivery-presence-001", "replay-placement-delivery-presence-001", "presence", "connected"}, {"evidence-placement-delivery-health-001", "replay-placement-delivery-health-001", "health", "healthy"}} {
		evidence, evidenceErr := FinalizeNodeConnectorSessionEvidence(NodeConnectorSessionEvidence{Sequence: int64(sequence + 2), EvidenceID: spec.id, ReplayIdentity: spec.replay, Kind: spec.kind, Status: spec.status, SessionID: negotiation.SessionID, ConnectionID: negotiation.ConnectionID, EnrollmentID: negotiation.EnrollmentID, MachineID: negotiation.MachineID, CredentialID: negotiation.CredentialID, CapabilitySnapshotID: negotiation.CapabilitySnapshotID, ObservedAt: nodeExecutionTime(time.Date(2026, 7, 28, 21, 1, 20+sequence*10, 0, time.UTC))})
		if evidenceErr != nil || session.RecordEvidence(mustMarshalNodeConnectorPlacementExecutionDelivery(t, evidence)) != nil {
			t.Fatal(evidenceErr)
		}
	}
	if err := handoff.base.broker.Connect(negotiation.MachineID, negotiation.ConnectionID); err != nil {
		t.Fatal(err)
	}
	validationCalls := 0
	evidenceFixture := &nodeExecutionTestFixture{request: request.ExecutionRequest, now: time.Date(2026, 7, 28, 21, 2, 0, 0, time.UTC)}
	preparedEvidence := nodeConnectorTestEvidence(t, evidenceFixture)
	connector, err := NewNodeValidationConnector(request.ExecutionRequest.Workflow, request.ExecutionRequest.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		validationCalls++
		return preparedEvidence, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareNodeValidationDelivery(request.ExecutionRequest, request.TaskLease, nil, preparedEvidence)
	if err != nil {
		t.Fatal(err)
	}
	expected := NodeConnectorPlacementExecutionDeliveryExpected{Handoff: handoff.expected, HandoffDecisionFingerprint: decision.DecisionFingerprint, HandoffRequestFingerprint: request.RequestFingerprint, SessionStateFingerprint: session.state.StateFingerprint, NegotiationFingerprint: negotiation.NegotiationFingerprint}
	fixture := NodeConnectorPlacementExecutionDeliveryFixture{
		Schema: NodeConnectorPlacementExecutionDeliveryFixtureSchema, DeliveryID: "placement-execution-delivery-001", ReplayIdentity: "replay-placement-execution-delivery-001", DeliveredAt: nodeExecutionTime(time.Date(2026, 7, 28, 21, 2, 30, 0, time.UTC)),
		HandoffDecisionID: decision.DecisionID, HandoffDecisionFingerprint: decision.DecisionFingerprint, HandoffRequestID: request.RequestID, HandoffRequestFingerprint: request.RequestFingerprint,
		SubmissionID: request.SubmissionID, SubmissionReplayIdentity: request.SubmissionReplayIdentity, SubmissionFingerprint: request.SubmissionFingerprint, SubmissionProvenance: request.SubmissionProvenance, BrokerStateFingerprintBefore: request.BrokerStateFingerprint,
		SessionStateFingerprint: session.state.StateFingerprint, EnrollmentID: enrollment.EnrollmentID, EnrollmentFingerprint: enrollment.EnrollmentFingerprint, CredentialID: negotiation.CredentialID, SessionID: negotiation.SessionID, ConnectionID: negotiation.ConnectionID, NegotiationID: negotiation.NegotiationID, NegotiationFingerprint: negotiation.NegotiationFingerprint,
		MachineID: request.SelectedNode.MachineID, CapabilitySnapshotID: request.SelectedNode.CapabilitySnapshotID, CapabilitySnapshotFingerprint: request.SelectedNode.CapabilitySnapshotFingerprint, WorkloadID: request.WorkloadID, CandidateNodeIDs: append([]string{}, request.CandidateNodeIDs...), SelectedNode: request.SelectedNode,
		ExecutionRequest: cloneNodeExecutionRequest(request.ExecutionRequest), ExecutionRequestFingerprint: request.ExecutionRequestFingerprint, TaskLease: request.TaskLease, ConnectorWorkflow: request.ExecutionRequest.Workflow, ConnectorSourceRevision: request.ExecutionRequest.SourceRevision,
		ExpectedEvents: cloneNodeConnectorPlacementExecutionDeliveryEvents(prepared.events), ExpectedReceipt: prepared.receipt, ExpectedReceiptFingerprint: prepared.receipt.ReceiptFingerprint, Provenance: nodeConnectorPlacementExecutionDeliveryProvenance,
	}
	return &nodeConnectorPlacementExecutionDeliveryTestFixture{handoff: handoff, decision: decision, request: request, session: session, negotiation: negotiation, expected: expected, fixture: fixture, connector: connector, validationCalls: &validationCalls, transportCalls: &transportCalls}
}

func mustOpenNodeConnectorPlacementExecutionDeliveries(t *testing.T, value *nodeConnectorPlacementExecutionDeliveryTestFixture) *NodeConnectorPlacementExecutionDeliveries {
	t.Helper()
	deliveries, err := OpenNodeConnectorPlacementExecutionDeliveries(value.handoff.base.root, value.expected, value.handoff.base.broker, value.session, value.negotiation)
	if err != nil {
		t.Fatal(err)
	}
	return deliveries
}

func mustDeliverNodeConnectorPlacementExecution(t *testing.T, deliveries *NodeConnectorPlacementExecutionDeliveries, fixture NodeConnectorPlacementExecutionDeliveryFixture, connector *NodeValidationConnector) NodeConnectorPlacementExecutionDelivery {
	t.Helper()
	value, err := deliveries.Deliver(mustMarshalNodeConnectorPlacementExecutionDelivery(t, fixture), connector)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertNodeConnectorPlacementExecutionDeliveryRejected(t *testing.T, value *nodeConnectorPlacementExecutionDeliveryTestFixture, deliveries *NodeConnectorPlacementExecutionDeliveries, fixture NodeConnectorPlacementExecutionDeliveryFixture, connector *NodeValidationConnector) {
	t.Helper()
	assertNodeConnectorPlacementExecutionDeliveryRawRejected(t, value, deliveries, mustMarshalNodeConnectorPlacementExecutionDelivery(t, fixture), connector)
}

func assertNodeConnectorPlacementExecutionDeliveryRawRejected(t *testing.T, value *nodeConnectorPlacementExecutionDeliveryTestFixture, deliveries *NodeConnectorPlacementExecutionDeliveries, raw []byte, connector *NodeValidationConnector) {
	t.Helper()
	before := nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)
	sessionBefore := cloneNodeConnectorSessionState(value.session.state)
	validationBefore := *value.validationCalls
	if _, err := deliveries.Deliver(raw, connector); err == nil {
		t.Fatal("changed, inferred, malformed, stale, or conflicting delivery input was accepted")
	}
	if *value.validationCalls != validationBefore || !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)) || !reflect.DeepEqual(sessionBefore, value.session.state) {
		t.Fatal("rejected delivery invoked validation or published partial broker/session evidence")
	}
}

func assertNodeConnectorPlacementExecutionDeliveryNotInvoked(t *testing.T, value *nodeConnectorPlacementExecutionDeliveryTestFixture, before map[string][]byte) {
	t.Helper()
	if *value.validationCalls != 0 || !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, value.handoff.base.brokerRoot)) {
		t.Fatal("missing or rejected handoff invoked validation or changed broker evidence")
	}
}

func mustMarshalNodeConnectorPlacementExecutionDelivery(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorPlacementExecutionDeliveryFile(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionDeliveryName))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementExecutionDeliveryFixture(value NodeConnectorPlacementExecutionDeliveryFixture) NodeConnectorPlacementExecutionDeliveryFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionDeliveryFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
