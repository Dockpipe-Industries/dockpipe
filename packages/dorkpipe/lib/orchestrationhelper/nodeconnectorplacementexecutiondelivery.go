package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	NodeConnectorPlacementExecutionDeliveryFixtureSchema = "dorkpipe.node-placement-execution-delivery-fixture/v1"
	NodeConnectorPlacementExecutionDeliverySchema        = "dorkpipe.node-placement-execution-delivery/v1"

	nodeConnectorPlacementExecutionDeliveryProvenance       = "fixture_only_placement_bound_connector_session_delivery"
	nodeConnectorPlacementExecutionDeliveryName             = "node-placement-execution-delivery.json"
	nodeConnectorPlacementExecutionDeliveryMaxFixtureBytes  = 1 << 20
	nodeConnectorPlacementExecutionDeliveryMaxArtifactBytes = 2 << 20
)

var nodeConnectorPlacementExecutionDeliveryWriteAtomic = writeJSONFileAtomic

type NodeConnectorPlacementExecutionDeliveryExpected struct {
	Handoff                    NodeConnectorPlacementExecutionHandoffExpected `json:"handoff"`
	HandoffDecisionFingerprint string                                         `json:"handoff_decision_fingerprint"`
	HandoffRequestFingerprint  string                                         `json:"handoff_request_fingerprint"`
	SessionStateFingerprint    string                                         `json:"session_state_fingerprint"`
	NegotiationFingerprint     string                                         `json:"negotiation_fingerprint"`
}

type NodeConnectorPlacementExecutionDeliveryAuthority struct {
	Cancellation bool `json:"cancellation"`
	Retry        bool `json:"retry"`
	Repair       bool `json:"repair"`
	Quarantine   bool `json:"quarantine"`
	Service      bool `json:"service"`
	Network      bool `json:"network"`
	Provider     bool `json:"provider"`
	Mutation     bool `json:"mutation"`
	Git          bool `json:"git"`
	Apply        bool `json:"apply"`
	Checkpoint   bool `json:"checkpoint"`
	Commit       bool `json:"commit"`
	Push         bool `json:"push"`
	Publication  bool `json:"publication"`
	Completion   bool `json:"completion"`
	Lifecycle    bool `json:"lifecycle"`
	NextTask     bool `json:"next_task"`
}

// NodeConnectorPlacementExecutionDeliveryFixture binds the exact expected
// output of one deterministic prepared validation. Receipt-shaped fixture
// evidence is never authority: the approved handoff request is revalidated
// independently before this fixture may invoke anything.
type NodeConnectorPlacementExecutionDeliveryFixture struct {
	Schema                        string                                    `json:"schema"`
	DeliveryID                    string                                    `json:"delivery_id"`
	ReplayIdentity                string                                    `json:"replay_identity"`
	DeliveredAt                   string                                    `json:"delivered_at"`
	HandoffDecisionID             string                                    `json:"handoff_decision_id"`
	HandoffDecisionFingerprint    string                                    `json:"handoff_decision_fingerprint"`
	HandoffRequestID              string                                    `json:"handoff_request_id"`
	HandoffRequestFingerprint     string                                    `json:"handoff_request_fingerprint"`
	SubmissionID                  string                                    `json:"submission_id"`
	SubmissionReplayIdentity      string                                    `json:"submission_replay_identity"`
	SubmissionFingerprint         string                                    `json:"submission_fingerprint"`
	SubmissionProvenance          string                                    `json:"submission_provenance"`
	BrokerStateFingerprintBefore  string                                    `json:"broker_state_fingerprint_before"`
	SessionStateFingerprint       string                                    `json:"session_state_fingerprint"`
	EnrollmentID                  string                                    `json:"enrollment_id"`
	EnrollmentFingerprint         string                                    `json:"enrollment_fingerprint"`
	CredentialID                  string                                    `json:"credential_id"`
	SessionID                     string                                    `json:"session_id"`
	ConnectionID                  string                                    `json:"connection_id"`
	NegotiationID                 string                                    `json:"negotiation_id"`
	NegotiationFingerprint        string                                    `json:"negotiation_fingerprint"`
	MachineID                     string                                    `json:"machine_id"`
	CapabilitySnapshotID          string                                    `json:"capability_snapshot_id"`
	CapabilitySnapshotFingerprint string                                    `json:"capability_snapshot_fingerprint"`
	WorkloadID                    string                                    `json:"workload_id"`
	CandidateNodeIDs              []string                                  `json:"candidate_node_ids"`
	SelectedNode                  NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	ExecutionRequest              NodeExecutionRequest                      `json:"execution_request"`
	ExecutionRequestFingerprint   string                                    `json:"execution_request_fingerprint"`
	TaskLease                     NodeExecutionTaskLease                    `json:"task_lease"`
	ConnectorWorkflow             NodeExecutionWorkflowReference            `json:"connector_workflow"`
	ConnectorSourceRevision       string                                    `json:"connector_source_revision"`
	ExpectedEvents                []NodeExecutionEventEnvelope              `json:"expected_events"`
	ExpectedReceipt               NodeExecutionReceipt                      `json:"expected_receipt"`
	ExpectedReceiptFingerprint    string                                    `json:"expected_receipt_fingerprint"`
	Provenance                    string                                    `json:"provenance"`
}

type NodeConnectorPlacementExecutionDelivery struct {
	Schema                        string                                           `json:"schema"`
	DeliveryID                    string                                           `json:"delivery_id"`
	ReplayIdentity                string                                           `json:"replay_identity"`
	DeliveredAt                   string                                           `json:"delivered_at"`
	HandoffDecisionID             string                                           `json:"handoff_decision_id"`
	HandoffDecisionFingerprint    string                                           `json:"handoff_decision_fingerprint"`
	HandoffRequestID              string                                           `json:"handoff_request_id"`
	HandoffRequestFingerprint     string                                           `json:"handoff_request_fingerprint"`
	SubmissionID                  string                                           `json:"submission_id"`
	SubmissionReplayIdentity      string                                           `json:"submission_replay_identity"`
	SubmissionFingerprint         string                                           `json:"submission_fingerprint"`
	SubmissionProvenance          string                                           `json:"submission_provenance"`
	BrokerStateFingerprintBefore  string                                           `json:"broker_state_fingerprint_before"`
	BrokerStateFingerprintAfter   string                                           `json:"broker_state_fingerprint_after"`
	SessionStateFingerprint       string                                           `json:"session_state_fingerprint"`
	EnrollmentID                  string                                           `json:"enrollment_id"`
	EnrollmentFingerprint         string                                           `json:"enrollment_fingerprint"`
	CredentialID                  string                                           `json:"credential_id"`
	SessionID                     string                                           `json:"session_id"`
	ConnectionID                  string                                           `json:"connection_id"`
	NegotiationID                 string                                           `json:"negotiation_id"`
	NegotiationFingerprint        string                                           `json:"negotiation_fingerprint"`
	MachineID                     string                                           `json:"machine_id"`
	CapabilitySnapshotID          string                                           `json:"capability_snapshot_id"`
	CapabilitySnapshotFingerprint string                                           `json:"capability_snapshot_fingerprint"`
	WorkloadID                    string                                           `json:"workload_id"`
	CandidateNodeIDs              []string                                         `json:"candidate_node_ids"`
	CompleteCandidateSet          bool                                             `json:"complete_candidate_set"`
	SelectedNode                  NodeConnectorPlacementSelectedNodeBinding        `json:"selected_node"`
	ExecutionRequest              NodeExecutionRequest                             `json:"execution_request"`
	ExecutionRequestFingerprint   string                                           `json:"execution_request_fingerprint"`
	TaskLease                     NodeExecutionTaskLease                           `json:"task_lease"`
	ConnectorWorkflow             NodeExecutionWorkflowReference                   `json:"connector_workflow"`
	ConnectorSourceRevision       string                                           `json:"connector_source_revision"`
	Events                        []NodeExecutionEventEnvelope                     `json:"events"`
	Receipt                       NodeExecutionReceipt                             `json:"receipt"`
	ReceiptFingerprint            string                                           `json:"receipt_fingerprint"`
	AuthorizationConsumed         bool                                             `json:"authorization_consumed"`
	ConnectorSessionInvoked       bool                                             `json:"connector_session_invoked"`
	ValidationConnectorInvoked    bool                                             `json:"validation_connector_invoked"`
	PreparedValidationInvoked     bool                                             `json:"prepared_validation_invoked"`
	EventsPublished               bool                                             `json:"events_published"`
	ReceiptPublished              bool                                             `json:"receipt_published"`
	ReceiptMaterialized           bool                                             `json:"receipt_materialized"`
	BrokerExecutorInvocations     int                                              `json:"broker_executor_invocations"`
	NewBrokerOperations           int                                              `json:"new_broker_operations"`
	NewLeases                     int                                              `json:"new_leases"`
	NewAttempts                   int                                              `json:"new_attempts"`
	NewConnections                int                                              `json:"new_connections"`
	NewSessions                   int                                              `json:"new_sessions"`
	NewEnrollments                int                                              `json:"new_enrollments"`
	NewCredentials                int                                              `json:"new_credentials"`
	Provenance                    string                                           `json:"provenance"`
	FixtureOwned                  bool                                             `json:"fixture_owned"`
	Authority                     NodeConnectorPlacementExecutionDeliveryAuthority `json:"authority"`
	DeliveryFingerprint           string                                           `json:"delivery_fingerprint"`
}

type nodeConnectorPlacementExecutionDeliveryInputs struct {
	submission            NodeConnectorPlacementDispatchSubmission
	decision              NodeConnectorPlacementExecutionHandoffDecision
	request               NodeConnectorPlacementExecutionHandoffRequest
	brokerState           nodeExecutionBrokerState
	operation             nodeExecutionOperationState
	sessionState          nodeConnectorSessionState
	negotiation           NodeConnectorSessionNegotiation
	capabilityFingerprint string
}

type NodeConnectorPlacementExecutionDeliveries struct {
	root        string
	expected    NodeConnectorPlacementExecutionDeliveryExpected
	broker      *NodeExecutionFakeBroker
	session     *NodeConnectorSessionFake
	negotiation NodeConnectorSessionNegotiation
	delivery    *NodeConnectorPlacementExecutionDelivery
	mu          sync.Mutex
}

func OpenNodeConnectorPlacementExecutionDeliveries(root string, expected NodeConnectorPlacementExecutionDeliveryExpected, broker *NodeExecutionFakeBroker, session *NodeConnectorSessionFake, negotiation NodeConnectorSessionNegotiation) (*NodeConnectorPlacementExecutionDeliveries, error) {
	normalized, err := normalizeNodeConnectorPlacementExecutionDeliveryExpected(expected)
	if err != nil {
		return nil, err
	}
	inputs, err := loadNodeConnectorPlacementExecutionDeliveryInputs(root, normalized, broker, session, negotiation)
	if err != nil {
		return nil, err
	}
	deliveries := &NodeConnectorPlacementExecutionDeliveries{root: root, expected: normalized, broker: broker, session: session, negotiation: negotiation}
	delivery, exists, err := loadNodeConnectorPlacementExecutionDelivery(root, inputs)
	if err != nil {
		return nil, err
	}
	if exists {
		deliveries.delivery = &delivery
	}
	return deliveries, nil
}

func (deliveries *NodeConnectorPlacementExecutionDeliveries) Deliver(raw []byte, connector *NodeValidationConnector) (NodeConnectorPlacementExecutionDelivery, error) {
	deliveries.mu.Lock()
	defer deliveries.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionDeliveryMaxFixtureBytes {
		return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionDeliveryFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery fixture is not strict canonical JSON")
	}
	inputs, err := loadNodeConnectorPlacementExecutionDeliveryInputs(deliveries.root, deliveries.expected, deliveries.broker, deliveries.session, deliveries.negotiation)
	if err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	deliveredAt, err := validateNodeConnectorPlacementExecutionDeliveryFixture(fixture, inputs, connector)
	if err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	if deliveries.delivery != nil {
		if !nodeConnectorPlacementExecutionDeliveryMatchesFixture(*deliveries.delivery, fixture) {
			return NodeConnectorPlacementExecutionDelivery{}, errors.New("changed or conflicting placement execution delivery replay is rejected")
		}
		return cloneNodeConnectorPlacementExecutionDelivery(*deliveries.delivery), nil
	}
	if deliveries.broker.executor != nil {
		return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery requires the unchanged executorless fake broker")
	}
	sessionBefore := cloneNodeConnectorSessionState(inputs.sessionState)
	operationBefore := inputs.operation
	if operationBefore.Receipt == nil {
		if inputs.brokerState.StateFingerprint != fixture.BrokerStateFingerprintBefore || len(operationBefore.Events) != 0 || operationBefore.Cancellation != nil || operationBefore.CancellationAck != nil || operationBefore.ExecutionCount != 1 {
			return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery requires the exact untouched accepted broker operation")
		}
		receipt, dispatchErr := deliveries.session.DispatchAcceptedValidation(connector, deliveries.negotiation, inputs.request.ExecutionRequest, inputs.request.TaskLease, deliveredAt)
		if dispatchErr != nil || !nodeExecutionEqual(receipt, fixture.ExpectedReceipt) {
			return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery connector-session transition did not return the exact expected receipt")
		}
	}
	after, err := loadNodeConnectorPlacementExecutionDeliveryInputs(deliveries.root, deliveries.expected, deliveries.broker, deliveries.session, deliveries.negotiation)
	if err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	terminalErr := validateNodeConnectorPlacementExecutionDeliveryTerminal(after, fixture)
	if !nodeExecutionEqual(sessionBefore, after.sessionState) || terminalErr != nil {
		return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery could not revalidate the exact terminal broker receipt and unchanged session state")
	}
	delivery, err := deriveNodeConnectorPlacementExecutionDelivery(fixture, after)
	if err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	path := filepath.Join(deliveries.root, nodeConnectorPlacementExecutionDeliveryName)
	if err := requireNodeConnectorPlacementExecutionDeliveryAbsent(path); err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	if err := nodeConnectorPlacementExecutionDeliveryWriteAtomic(path, delivery); err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, errors.New("placement execution delivery artifact could not be published after terminal receipt acceptance")
	}
	deliveries.delivery = &delivery
	return cloneNodeConnectorPlacementExecutionDelivery(delivery), nil
}

func (deliveries *NodeConnectorPlacementExecutionDeliveries) Artifact() *NodeConnectorPlacementExecutionDelivery {
	deliveries.mu.Lock()
	defer deliveries.mu.Unlock()
	if deliveries.delivery == nil {
		return nil
	}
	value := cloneNodeConnectorPlacementExecutionDelivery(*deliveries.delivery)
	return &value
}

func normalizeNodeConnectorPlacementExecutionDeliveryExpected(value NodeConnectorPlacementExecutionDeliveryExpected) (NodeConnectorPlacementExecutionDeliveryExpected, error) {
	handoff, err := normalizeNodeConnectorPlacementExecutionHandoffExpected(value.Handoff)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.HandoffDecisionFingerprint) || !nodeExecutionFingerprint.MatchString(value.HandoffRequestFingerprint) || !nodeExecutionFingerprint.MatchString(value.SessionStateFingerprint) || !nodeExecutionFingerprint.MatchString(value.NegotiationFingerprint) {
		return NodeConnectorPlacementExecutionDeliveryExpected{}, errors.New("placement execution delivery expected binding is invalid")
	}
	value.Handoff = handoff
	return value, nil
}

func loadNodeConnectorPlacementExecutionDeliveryInputs(root string, expected NodeConnectorPlacementExecutionDeliveryExpected, broker *NodeExecutionFakeBroker, session *NodeConnectorSessionFake, negotiation NodeConnectorSessionNegotiation) (nodeConnectorPlacementExecutionDeliveryInputs, error) {
	submission, state, operation, err := loadNodeConnectorPlacementExecutionDeliverySubmission(root, expected.Handoff.Submission, broker)
	if err != nil {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery could not revalidate the complete inventory, placement, dispatch, and submission chain")
	}
	if submission.SubmissionFingerprint != expected.Handoff.SubmissionFingerprint {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery requires the exact dispatch submission")
	}
	handoffInputs := nodeConnectorPlacementExecutionHandoffInputs{submission: submission}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionHandoffDecision(root, handoffInputs, expected.Handoff)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.HandoffDecisionFingerprint {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery requires the exact approved handoff decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionHandoffRequest(root, handoffInputs, expected.Handoff, decision, decisionExists)
	if err != nil || !requestExists || request.RequestFingerprint != expected.HandoffRequestFingerprint || request.AuthorizationConsumed || request.ConnectorInvoked || request.ExecutorInvoked || request.ExecutionStarted || request.EventsPublished || request.ReceiptPublished || request.Authority != (NodeConnectorPlacementExecutionHandoffAuthority{FixtureConnectorHandoff: true}) {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery requires one exact approved and unconsumed handoff request")
	}
	if operation == nil || !nodeExecutionEqual(operation.Request, request.ExecutionRequest) || !nodeExecutionEqual(operation.Lease, request.TaskLease) || operation.ExecutionCount != 1 || operation.Cancellation != nil || operation.CancellationAck != nil {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery could not revalidate the exact broker operation, request, and lease")
	}
	if session == nil || session.broker != broker || !nodeExecutionEqual(negotiation.Authority, NodeConnectorSessionAuthority{}) || negotiation.NegotiationFingerprint != expected.NegotiationFingerprint {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery requires the exact existing in-process connector session and negotiation")
	}
	sessionStates, err := loadNodeConnectorSessionStates(session.root, broker)
	if err != nil || len(sessionStates) == 0 || !nodeExecutionEqual(sessionStates[len(sessionStates)-1], session.state) || session.state.StateFingerprint != expected.SessionStateFingerprint {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery could not revalidate the exact durable connector-session state")
	}
	derived, err := deriveNodeConnectorSessionState(session.state, broker)
	if err != nil {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, err
	}
	record, ok := derived.negotiations[negotiation.NegotiationID]
	runtimeSession, sessionOK := derived.sessions[negotiation.SessionID]
	connectedMachine, connectionErr := broker.connectedMachine(negotiation.ConnectionID)
	if !ok || !nodeExecutionEqual(record.Negotiation, negotiation) || !sessionOK || !runtimeSession.Present || runtimeSession.Health != "healthy" || runtimeSession.ConnectionID != negotiation.ConnectionID || runtimeSession.CredentialID != negotiation.CredentialID || runtimeSession.CapabilitySnapshot != negotiation.CapabilitySnapshotID || negotiation.EnrollmentID != session.state.Enrollment.EnrollmentID || negotiation.CredentialID != derived.currentCredential || derived.revoked[negotiation.CredentialID] || connectionErr != nil || connectedMachine != negotiation.MachineID {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery requires current exact enrollment, credential, session, connection, health, presence, and negotiation bindings")
	}
	capability, ok := broker.capability(request.SelectedNode.CapabilitySnapshotID)
	capabilityFingerprint, fingerprintErr := nodeExecutionFingerprintValue(capability)
	if !ok || fingerprintErr != nil || capabilityFingerprint != request.SelectedNode.CapabilitySnapshotFingerprint || capability.MachineID != request.SelectedNode.MachineID || negotiation.MachineID != request.SelectedNode.MachineID || negotiation.CapabilitySnapshotID != request.SelectedNode.CapabilitySnapshotID {
		return nodeConnectorPlacementExecutionDeliveryInputs{}, errors.New("placement execution delivery selected machine and capability binding is invalid")
	}
	return nodeConnectorPlacementExecutionDeliveryInputs{submission: submission, decision: decision, request: request, brokerState: state, operation: *operation, sessionState: session.state, negotiation: negotiation, capabilityFingerprint: capabilityFingerprint}, nil
}

func loadNodeConnectorPlacementExecutionDeliverySubmission(root string, expected NodeConnectorPlacementDispatchSubmissionExpected, broker *NodeExecutionFakeBroker) (NodeConnectorPlacementDispatchSubmission, nodeExecutionBrokerState, *nodeExecutionOperationState, error) {
	inputs, err := loadNodeConnectorPlacementDispatchSubmissionInputs(root, expected)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, err
	}
	currentState, currentOperation, err := revalidateNodeConnectorPlacementDispatchSubmissionBroker(broker, inputs.dispatchRequest)
	if err != nil || currentOperation == nil {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, errors.New("placement execution delivery could not revalidate the current broker operation")
	}
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementDispatchSubmissionName))
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementDispatchMaxSubmissionArtifactBytes {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, errors.New("placement execution delivery could not read the bounded dispatch submission")
	}
	var encoded NodeConnectorPlacementDispatchSubmission
	if decodeNodeConnectorPlacementDispatchSubmissionArtifact(raw, &encoded) != nil {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, errors.New("placement execution delivery requires a strict canonical dispatch submission")
	}
	states, err := loadNodeExecutionStates(broker.root)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, errors.New("placement execution delivery could not revalidate the durable broker history")
	}
	var issuanceState nodeExecutionBrokerState
	var issuanceOperation nodeExecutionOperationState
	found := false
	for _, state := range states {
		if state.StateFingerprint != encoded.BrokerStateFingerprint {
			continue
		}
		operation, ok := state.Operations[inputs.dispatchRequest.ExecutionRequest.OperationID]
		if !ok {
			break
		}
		issuanceState = state
		issuanceOperation = operation
		found = true
		break
	}
	if !found {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, errors.New("placement execution delivery submission is orphaned from its exact broker issuance state")
	}
	submission, exists, err := loadNodeConnectorPlacementDispatchSubmission(root, inputs, expected, issuanceState, &issuanceOperation)
	if err != nil || !exists {
		return NodeConnectorPlacementDispatchSubmission{}, nodeExecutionBrokerState{}, nil, errors.New("placement execution delivery could not revalidate the exact dispatch submission at issuance")
	}
	return submission, currentState, currentOperation, nil
}

func validateNodeConnectorPlacementExecutionDeliveryFixture(value NodeConnectorPlacementExecutionDeliveryFixture, inputs nodeConnectorPlacementExecutionDeliveryInputs, connector *NodeValidationConnector) (time.Time, error) {
	request, decision, submission, negotiation := inputs.request, inputs.decision, inputs.submission, inputs.negotiation
	if value.Schema != NodeConnectorPlacementExecutionDeliveryFixtureSchema || value.Provenance != nodeConnectorPlacementExecutionDeliveryProvenance ||
		value.HandoffDecisionID != decision.DecisionID || value.HandoffDecisionFingerprint != decision.DecisionFingerprint || value.HandoffRequestID != request.RequestID || value.HandoffRequestFingerprint != request.RequestFingerprint ||
		value.SubmissionID != submission.SubmissionID || value.SubmissionReplayIdentity != submission.ReplayIdentity || value.SubmissionFingerprint != submission.SubmissionFingerprint || value.SubmissionProvenance != submission.Provenance || value.BrokerStateFingerprintBefore != request.BrokerStateFingerprint ||
		value.SessionStateFingerprint != inputs.sessionState.StateFingerprint || value.EnrollmentID != inputs.sessionState.Enrollment.EnrollmentID || value.EnrollmentFingerprint != inputs.sessionState.Enrollment.EnrollmentFingerprint || value.CredentialID != negotiation.CredentialID || value.SessionID != negotiation.SessionID || value.ConnectionID != negotiation.ConnectionID || value.NegotiationID != negotiation.NegotiationID || value.NegotiationFingerprint != negotiation.NegotiationFingerprint ||
		value.MachineID != request.SelectedNode.MachineID || value.CapabilitySnapshotID != request.SelectedNode.CapabilitySnapshotID || value.CapabilitySnapshotFingerprint != inputs.capabilityFingerprint || value.WorkloadID != request.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, request.CandidateNodeIDs) || !nodeExecutionEqual(value.SelectedNode, request.SelectedNode) ||
		!nodeExecutionEqual(value.ExecutionRequest, request.ExecutionRequest) || value.ExecutionRequestFingerprint != request.ExecutionRequestFingerprint || !nodeExecutionEqual(value.TaskLease, request.TaskLease) || !nodeExecutionEqual(value.ConnectorWorkflow, request.ExecutionRequest.Workflow) || value.ConnectorSourceRevision != request.ExecutionRequest.SourceRevision {
		return time.Time{}, errors.New("placement execution delivery fixture does not exactly bind the complete handoff, submission, broker, session, selected-node, request, lease, and connector chain")
	}
	if connector == nil || !nodeExecutionEqual(connector.expectedWorkflow, value.ConnectorWorkflow) || connector.expectedRevision != value.ConnectorSourceRevision {
		return time.Time{}, errors.New("placement execution delivery requires the explicitly supplied exact validation connector")
	}
	if validateNodeExecutionTypedID("placement-execution-delivery", value.DeliveryID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || placementExecutionDeliveryIdentityCollides(value.DeliveryID, value.ReplayIdentity, decision.DecisionID, request.RequestID, submission.SubmissionID, submission.ReplayIdentity, negotiation.NegotiationID, negotiation.SessionID, negotiation.ConnectionID) || placementExecutionDeliveryIdentityCollides(value.ReplayIdentity, value.DeliveryID, decision.ReplayIdentity, decision.DecisionID, request.RequestID, submission.SubmissionID, submission.ReplayIdentity) {
		return time.Time{}, errors.New("placement execution delivery or replay identity is invalid or colliding")
	}
	deliveredAt, err := parseNodeExecutionTime(value.DeliveredAt)
	decisionAt, decisionErr := parseNodeExecutionTime(decision.IssuedAt)
	negotiatedAt, negotiationErr := parseNodeExecutionTime(negotiation.NegotiatedAt)
	leaseExpiresAt, leaseErr := parseNodeExecutionTime(request.TaskLease.ExpiresAt)
	if err != nil || decisionErr != nil || negotiationErr != nil || leaseErr != nil || deliveredAt.Before(decisionAt) || deliveredAt.Before(negotiatedAt) || !deliveredAt.Before(leaseExpiresAt) {
		return time.Time{}, errors.New("placement execution delivery time is invalid or outside the exact bounded authorization and lease interval")
	}
	if len(value.ExpectedEvents) == 0 || len(value.ExpectedEvents) > nodeExecutionMaxArtifacts || value.ExpectedReceiptFingerprint != value.ExpectedReceipt.ReceiptFingerprint {
		return time.Time{}, errors.New("placement execution delivery expected event and receipt evidence is invalid or unbounded")
	}
	for index, event := range value.ExpectedEvents {
		if event.Sequence != int64(index+1) || validateNodeExecutionEvent(event) != nil || event.OperationID != request.ExecutionRequest.OperationID || event.LeaseID != request.TaskLease.LeaseID || event.EnvelopeFingerprint == "" {
			return time.Time{}, errors.New("placement execution delivery expected event evidence is not the exact bounded broker sequence")
		}
	}
	if validateNodeExecutionReceiptShape(value.ExpectedReceipt) != nil || value.ExpectedReceipt.OperationID != request.ExecutionRequest.OperationID || value.ExpectedReceipt.RequestFingerprint != request.ExecutionRequestFingerprint || value.ExpectedReceipt.LeaseID != request.TaskLease.LeaseID || value.ExpectedReceipt.FinalCursor != nodeExecutionCursor(int64(len(value.ExpectedEvents))) {
		return time.Time{}, errors.New("placement execution delivery expected receipt is not exactly bound to the request, lease, and events")
	}
	return deliveredAt, nil
}

func validateNodeConnectorPlacementExecutionDeliveryTerminal(inputs nodeConnectorPlacementExecutionDeliveryInputs, fixture NodeConnectorPlacementExecutionDeliveryFixture) error {
	operation := inputs.operation
	if operation.ExecutionCount != 1 || operation.Cancellation != nil || operation.CancellationAck != nil || operation.Receipt == nil || !nodeExecutionEqual(operation.Events, fixture.ExpectedEvents) || !nodeExecutionEqual(*operation.Receipt, fixture.ExpectedReceipt) || inputs.brokerState.StateFingerprint == fixture.BrokerStateFingerprintBefore {
		return errors.New("placement execution delivery terminal broker evidence conflicts with the exact expected seam output")
	}
	if !nodeExecutionEqual(operation.Request, inputs.request.ExecutionRequest) || !nodeExecutionEqual(operation.Lease, inputs.request.TaskLease) {
		return errors.New("placement execution delivery changed the original broker request or lease")
	}
	return nil
}

func deriveNodeConnectorPlacementExecutionDelivery(fixture NodeConnectorPlacementExecutionDeliveryFixture, inputs nodeConnectorPlacementExecutionDeliveryInputs) (NodeConnectorPlacementExecutionDelivery, error) {
	value := NodeConnectorPlacementExecutionDelivery{
		Schema: NodeConnectorPlacementExecutionDeliverySchema, DeliveryID: fixture.DeliveryID, ReplayIdentity: fixture.ReplayIdentity, DeliveredAt: fixture.DeliveredAt,
		HandoffDecisionID: fixture.HandoffDecisionID, HandoffDecisionFingerprint: fixture.HandoffDecisionFingerprint, HandoffRequestID: fixture.HandoffRequestID, HandoffRequestFingerprint: fixture.HandoffRequestFingerprint,
		SubmissionID: fixture.SubmissionID, SubmissionReplayIdentity: fixture.SubmissionReplayIdentity, SubmissionFingerprint: fixture.SubmissionFingerprint, SubmissionProvenance: fixture.SubmissionProvenance,
		BrokerStateFingerprintBefore: fixture.BrokerStateFingerprintBefore, BrokerStateFingerprintAfter: inputs.brokerState.StateFingerprint,
		SessionStateFingerprint: fixture.SessionStateFingerprint, EnrollmentID: fixture.EnrollmentID, EnrollmentFingerprint: fixture.EnrollmentFingerprint, CredentialID: fixture.CredentialID, SessionID: fixture.SessionID, ConnectionID: fixture.ConnectionID, NegotiationID: fixture.NegotiationID, NegotiationFingerprint: fixture.NegotiationFingerprint,
		MachineID: fixture.MachineID, CapabilitySnapshotID: fixture.CapabilitySnapshotID, CapabilitySnapshotFingerprint: fixture.CapabilitySnapshotFingerprint, WorkloadID: fixture.WorkloadID, CandidateNodeIDs: append([]string{}, fixture.CandidateNodeIDs...), CompleteCandidateSet: true, SelectedNode: fixture.SelectedNode,
		ExecutionRequest: cloneNodeExecutionRequest(fixture.ExecutionRequest), ExecutionRequestFingerprint: fixture.ExecutionRequestFingerprint, TaskLease: fixture.TaskLease, ConnectorWorkflow: fixture.ConnectorWorkflow, ConnectorSourceRevision: fixture.ConnectorSourceRevision,
		Events: cloneNodeConnectorPlacementExecutionDeliveryEvents(fixture.ExpectedEvents), Receipt: fixture.ExpectedReceipt, ReceiptFingerprint: fixture.ExpectedReceiptFingerprint,
		AuthorizationConsumed: true, ConnectorSessionInvoked: true, ValidationConnectorInvoked: true, PreparedValidationInvoked: true, EventsPublished: true, ReceiptPublished: true, ReceiptMaterialized: true,
		Provenance: nodeConnectorPlacementExecutionDeliveryProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementExecutionDeliveryFingerprint(value)
	if err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	value.DeliveryFingerprint = fingerprint
	if err := validateNodeConnectorPlacementExecutionDelivery(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionDelivery{}, err
	}
	return value, nil
}

func validateNodeConnectorPlacementExecutionDelivery(value NodeConnectorPlacementExecutionDelivery, inputs nodeConnectorPlacementExecutionDeliveryInputs) error {
	if value.Schema != NodeConnectorPlacementExecutionDeliverySchema || value.Provenance != nodeConnectorPlacementExecutionDeliveryProvenance || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionDeliveryAuthority{}) || !value.AuthorizationConsumed || !value.ConnectorSessionInvoked || !value.ValidationConnectorInvoked || !value.PreparedValidationInvoked || !value.EventsPublished || !value.ReceiptPublished || !value.ReceiptMaterialized || !value.CompleteCandidateSet ||
		value.BrokerExecutorInvocations != 0 || value.NewBrokerOperations != 0 || value.NewLeases != 0 || value.NewAttempts != 0 || value.NewConnections != 0 || value.NewSessions != 0 || value.NewEnrollments != 0 || value.NewCredentials != 0 ||
		value.HandoffDecisionFingerprint != inputs.decision.DecisionFingerprint || value.HandoffRequestFingerprint != inputs.request.RequestFingerprint || value.SubmissionFingerprint != inputs.submission.SubmissionFingerprint || value.BrokerStateFingerprintBefore != inputs.request.BrokerStateFingerprint || value.BrokerStateFingerprintAfter != inputs.brokerState.StateFingerprint || value.SessionStateFingerprint != inputs.sessionState.StateFingerprint || value.NegotiationFingerprint != inputs.negotiation.NegotiationFingerprint ||
		!nodeExecutionEqual(value.ExecutionRequest, inputs.request.ExecutionRequest) || !nodeExecutionEqual(value.TaskLease, inputs.request.TaskLease) || !nodeExecutionEqual(value.SelectedNode, inputs.request.SelectedNode) || !nodeExecutionEqual(value.Events, inputs.operation.Events) || inputs.operation.Receipt == nil || !nodeExecutionEqual(value.Receipt, *inputs.operation.Receipt) || value.ReceiptFingerprint != value.Receipt.ReceiptFingerprint {
		return errors.New("placement execution delivery artifact contract, authority, or immutable binding is invalid")
	}
	fingerprint, err := nodeConnectorPlacementExecutionDeliveryFingerprint(value)
	if err != nil || fingerprint != value.DeliveryFingerprint {
		return errors.New("placement execution delivery artifact fingerprint is invalid")
	}
	return validateNodeConnectorPlacementExecutionDeliveryEncodedBound(value)
}

func loadNodeConnectorPlacementExecutionDelivery(root string, inputs nodeConnectorPlacementExecutionDeliveryInputs) (NodeConnectorPlacementExecutionDelivery, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionDeliveryName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionDelivery{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionDeliveryMaxArtifactBytes {
		return NodeConnectorPlacementExecutionDelivery{}, false, errors.New("durable placement execution delivery cannot be read within its bound")
	}
	var value NodeConnectorPlacementExecutionDelivery
	if decodeNodeConnectorPlacementExecutionDeliveryArtifact(raw, &value) != nil || validateNodeConnectorPlacementExecutionDeliveryTerminal(inputs, NodeConnectorPlacementExecutionDeliveryFixture{ExpectedEvents: value.Events, ExpectedReceipt: value.Receipt, BrokerStateFingerprintBefore: value.BrokerStateFingerprintBefore}) != nil || validateNodeConnectorPlacementExecutionDelivery(value, inputs) != nil {
		return NodeConnectorPlacementExecutionDelivery{}, false, errors.New("durable placement execution delivery is malformed, noncanonical, tampered, or orphaned from its exact broker receipt")
	}
	return value, true, nil
}

func nodeConnectorPlacementExecutionDeliveryMatchesFixture(value NodeConnectorPlacementExecutionDelivery, fixture NodeConnectorPlacementExecutionDeliveryFixture) bool {
	return value.DeliveryID == fixture.DeliveryID && value.ReplayIdentity == fixture.ReplayIdentity && value.DeliveredAt == fixture.DeliveredAt && value.HandoffDecisionFingerprint == fixture.HandoffDecisionFingerprint && value.HandoffRequestFingerprint == fixture.HandoffRequestFingerprint && value.SubmissionFingerprint == fixture.SubmissionFingerprint && value.SessionStateFingerprint == fixture.SessionStateFingerprint && value.NegotiationFingerprint == fixture.NegotiationFingerprint && nodeExecutionEqual(value.ExecutionRequest, fixture.ExecutionRequest) && nodeExecutionEqual(value.TaskLease, fixture.TaskLease) && nodeExecutionEqual(value.Events, fixture.ExpectedEvents) && nodeExecutionEqual(value.Receipt, fixture.ExpectedReceipt)
}

func placementExecutionDeliveryIdentityCollides(value string, others ...string) bool {
	for _, other := range others {
		if value == other {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionDeliveryFingerprint(value NodeConnectorPlacementExecutionDelivery) (string, error) {
	value.DeliveryFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorPlacementExecutionDeliveryEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementExecutionDeliveryMaxArtifactBytes {
		return errors.New("placement execution delivery durable artifact exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorPlacementExecutionDeliveryArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("placement execution delivery durable artifact is not canonical")
	}
	return nil
}

func requireNodeConnectorPlacementExecutionDeliveryAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("placement execution delivery artifact already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("placement execution delivery artifact path cannot be inspected")
	}
	return nil
}

func cloneNodeConnectorPlacementExecutionDelivery(value NodeConnectorPlacementExecutionDelivery) NodeConnectorPlacementExecutionDelivery {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionDelivery
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionDeliveryEvents(values []NodeExecutionEventEnvelope) []NodeExecutionEventEnvelope {
	raw, _ := json.Marshal(values)
	var cloned []NodeExecutionEventEnvelope
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorPlacementExecutionDeliveryFixture(value NodeConnectorPlacementExecutionDeliveryFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorPlacementExecutionDeliveryFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorPlacementExecutionDeliveryProvenance
	}
	return json.Marshal(value)
}

var _ = time.Second
