package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const (
	NodeConnectorPlacementExecutionReconciliationDecisionFixtureSchema = "dorkpipe.node-placement-execution-reconciliation-decision-fixture/v1"
	NodeConnectorPlacementExecutionReconciliationDecisionSchema        = "dorkpipe.node-placement-execution-reconciliation-decision/v1"
	NodeConnectorPlacementExecutionReconciliationRequestSchema         = "dorkpipe.node-placement-execution-reconciliation-request/v1"

	nodeConnectorPlacementExecutionReconciliationDecisionProvenance = "fixture_only_local_placement_execution_reconciliation_decision"
	nodeConnectorPlacementExecutionReconciliationRequestProvenance  = "fixture_only_placement_bound_graph_reconciliation_request"
	nodeConnectorPlacementExecutionReconciliationScope              = "exact_terminal_delivery_to_local_graph_reconciliation_once"
	nodeConnectorPlacementExecutionReconciliationDecisionName       = "node-placement-execution-reconciliation-decision.json"
	nodeConnectorPlacementExecutionReconciliationRequestName        = "node-placement-execution-reconciliation-request.json"
	nodeConnectorPlacementExecutionReconciliationMaxDecisionBytes   = 4 << 20
	nodeConnectorPlacementExecutionReconciliationMaxArtifactBytes   = 8 << 20
)

var (
	nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic  = writeJSONFileAtomic
)

type NodeConnectorPlacementExecutionReconciliationExpected struct {
	Delivery            NodeConnectorPlacementExecutionDeliveryExpected `json:"delivery"`
	DeliveryFingerprint string                                          `json:"delivery_fingerprint"`
}

// NodeConnectorPlacementExecutionReconciliationAuthority permits only a
// future one-time local graph-reconciliation request. It cannot interpret the
// terminal result or authorize reconciliation, completion, scheduling, or any
// other adjacent action by itself.
type NodeConnectorPlacementExecutionReconciliationAuthority struct {
	LocalGraphReconciliationRequest bool `json:"local_graph_reconciliation_request"`
	GraphReconciliation             bool `json:"graph_reconciliation"`
	GraphCompletion                 bool `json:"graph_completion"`
	GraphFailure                    bool `json:"graph_failure"`
	NextTask                        bool `json:"next_task"`
	Connector                       bool `json:"connector"`
	Validation                      bool `json:"validation"`
	Executor                        bool `json:"executor"`
	Broker                          bool `json:"broker"`
	Dispatch                        bool `json:"dispatch"`
	Lease                           bool `json:"lease"`
	Cancellation                    bool `json:"cancellation"`
	Retry                           bool `json:"retry"`
	Repair                          bool `json:"repair"`
	Quarantine                      bool `json:"quarantine"`
	Service                         bool `json:"service"`
	Network                         bool `json:"network"`
	Provider                        bool `json:"provider"`
	Mutation                        bool `json:"mutation"`
	Git                             bool `json:"git"`
	Apply                           bool `json:"apply"`
	Checkpoint                      bool `json:"checkpoint"`
	Commit                          bool `json:"commit"`
	Push                            bool `json:"push"`
	Publication                     bool `json:"publication"`
	Lifecycle                       bool `json:"lifecycle"`
}

// NodeConnectorPlacementExecutionReconciliationDecisionFixture is the only
// source of approval. The embedded delivery must be the exact revalidated
// durable terminal evidence; no receipt, event, provider, broker, validation,
// availability, or connection claim can synthesize this decision.
type NodeConnectorPlacementExecutionReconciliationDecisionFixture struct {
	Schema                  string                                  `json:"schema"`
	DecisionID              string                                  `json:"decision_id"`
	ReplayIdentity          string                                  `json:"replay_identity"`
	Decision                string                                  `json:"decision"`
	Delivery                NodeConnectorPlacementExecutionDelivery `json:"delivery"`
	ReconciliationRequestID string                                  `json:"reconciliation_request_id,omitempty"`
	Provenance              string                                  `json:"provenance"`
}

type NodeConnectorPlacementExecutionReconciliationDecision struct {
	Schema                   string                                                 `json:"schema"`
	DecisionID               string                                                 `json:"decision_id"`
	ReplayIdentity           string                                                 `json:"replay_identity"`
	Decision                 string                                                 `json:"decision"`
	Delivery                 NodeConnectorPlacementExecutionDelivery                `json:"delivery"`
	CompleteChainRevalidated bool                                                   `json:"complete_chain_revalidated"`
	ApprovalInferred         bool                                                   `json:"approval_inferred"`
	ReconciliationRequestID  string                                                 `json:"reconciliation_request_id,omitempty"`
	Provenance               string                                                 `json:"provenance"`
	FixtureOwned             bool                                                   `json:"fixture_owned"`
	Authority                NodeConnectorPlacementExecutionReconciliationAuthority `json:"authority"`
	DecisionFingerprint      string                                                 `json:"decision_fingerprint"`
}

// NodeConnectorPlacementExecutionReconciliationRequest preserves the exact
// delivery, including its ordered terminal events and receipt, as opaque bound
// evidence. A later graph owner must consume and interpret it separately.
type NodeConnectorPlacementExecutionReconciliationRequest struct {
	Schema                       string                                                 `json:"schema"`
	RequestID                    string                                                 `json:"request_id"`
	DecisionID                   string                                                 `json:"decision_id"`
	DecisionFingerprint          string                                                 `json:"decision_fingerprint"`
	Delivery                     NodeConnectorPlacementExecutionDelivery                `json:"delivery"`
	ReconciliationScope          string                                                 `json:"reconciliation_scope"`
	OneTimeRequest               bool                                                   `json:"one_time_request"`
	AuthorizationConsumed        bool                                                   `json:"authorization_consumed"`
	TerminalOutcomeOpaque        bool                                                   `json:"terminal_outcome_opaque"`
	TerminalOutcomeInterpreted   bool                                                   `json:"terminal_outcome_interpreted"`
	GraphReconciliationPerformed bool                                                   `json:"graph_reconciliation_performed"`
	GraphCompletionClaimed       bool                                                   `json:"graph_completion_claimed"`
	GraphFailurePropagated       bool                                                   `json:"graph_failure_propagated"`
	NextTaskScheduled            bool                                                   `json:"next_task_scheduled"`
	ConnectorInvoked             bool                                                   `json:"connector_invoked"`
	PreparedValidationInvoked    bool                                                   `json:"prepared_validation_invoked"`
	BrokerExecutorInvoked        bool                                                   `json:"broker_executor_invoked"`
	BrokerOperationCreated       bool                                                   `json:"broker_operation_created"`
	LeaseCreated                 bool                                                   `json:"lease_created"`
	AttemptCreated               bool                                                   `json:"attempt_created"`
	ConnectionCreated            bool                                                   `json:"connection_created"`
	SessionCreated               bool                                                   `json:"session_created"`
	EnrollmentCreated            bool                                                   `json:"enrollment_created"`
	CredentialCreated            bool                                                   `json:"credential_created"`
	EventCreated                 bool                                                   `json:"event_created"`
	ReceiptCreated               bool                                                   `json:"receipt_created"`
	DeliveryCreated              bool                                                   `json:"delivery_created"`
	Provenance                   string                                                 `json:"provenance"`
	FixtureOwned                 bool                                                   `json:"fixture_owned"`
	Authority                    NodeConnectorPlacementExecutionReconciliationAuthority `json:"authority"`
	RequestFingerprint           string                                                 `json:"request_fingerprint"`
}

type nodeConnectorPlacementExecutionReconciliationInputs struct {
	submission NodeConnectorPlacementDispatchSubmission
	handoff    NodeConnectorPlacementExecutionHandoffDecision
	request    NodeConnectorPlacementExecutionHandoffRequest
	delivery   NodeConnectorPlacementExecutionDelivery
	broker     nodeExecutionBrokerState
	operation  nodeExecutionOperationState
}

type NodeConnectorPlacementExecutionReconciliations struct {
	root     string
	expected NodeConnectorPlacementExecutionReconciliationExpected
	broker   *NodeExecutionFakeBroker
	inputs   nodeConnectorPlacementExecutionReconciliationInputs
	decision *NodeConnectorPlacementExecutionReconciliationDecision
	request  *NodeConnectorPlacementExecutionReconciliationRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionReconciliations(root string, expected NodeConnectorPlacementExecutionReconciliationExpected, broker *NodeExecutionFakeBroker) (*NodeConnectorPlacementExecutionReconciliations, error) {
	normalized, err := normalizeNodeConnectorPlacementExecutionReconciliationExpected(expected)
	if err != nil {
		return nil, err
	}
	inputs, err := loadNodeConnectorPlacementExecutionReconciliationInputs(root, normalized, broker)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionReconciliations{root: root, expected: normalized, broker: broker, inputs: inputs}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionReconciliationDecision(root, inputs)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionReconciliationRequest(root, inputs, decision, decisionExists)
	if err != nil {
		return nil, err
	}
	if requestExists && !decisionExists {
		return nil, errors.New("placement execution reconciliation request exists without its exact durable decision")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (reconciliations *NodeConnectorPlacementExecutionReconciliations) Decide(raw []byte) (NodeConnectorPlacementExecutionReconciliationDecision, *NodeConnectorPlacementExecutionReconciliationRequest, error) {
	reconciliations.mu.Lock()
	defer reconciliations.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionReconciliationMaxDecisionBytes {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionReconciliationDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision fixture is not strict canonical JSON")
	}
	inputs, err := loadNodeConnectorPlacementExecutionReconciliationInputs(reconciliations.root, reconciliations.expected, reconciliations.broker)
	if err != nil || !nodeExecutionEqual(inputs, reconciliations.inputs) {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision could not directly revalidate the immutable terminal delivery chain")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionReconciliationArtifacts(inputs, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
	}
	if reconciliations.decision != nil {
		if !nodeExecutionEqual(*reconciliations.decision, decision) {
			return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("changed or conflicting placement execution reconciliation decision replay is rejected")
		}
	} else {
		path := filepath.Join(reconciliations.root, nodeConnectorPlacementExecutionReconciliationDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "decision"); err != nil {
			return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionReconciliationWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision could not be published")
		}
		reconciliations.decision = &decision
	}
	if request == nil {
		if reconciliations.request != nil {
			return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("rejected placement execution reconciliation decision conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementExecutionReconciliationDecision(*reconciliations.decision), nil, nil
	}
	if reconciliations.request != nil {
		if !nodeExecutionEqual(*reconciliations.request, *request) {
			return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("changed or conflicting placement execution reconciliation request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementExecutionReconciliationRequest(*reconciliations.request)
		return cloneNodeConnectorPlacementExecutionReconciliationDecision(*reconciliations.decision), &cloned, nil
	}
	path := filepath.Join(reconciliations.root, nodeConnectorPlacementExecutionReconciliationRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "request"); err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionReconciliationWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation request could not be published after the durable decision")
	}
	reconciliations.request = request
	cloned := cloneNodeConnectorPlacementExecutionReconciliationRequest(*request)
	return cloneNodeConnectorPlacementExecutionReconciliationDecision(*reconciliations.decision), &cloned, nil
}

func (reconciliations *NodeConnectorPlacementExecutionReconciliations) Artifacts() (*NodeConnectorPlacementExecutionReconciliationDecision, *NodeConnectorPlacementExecutionReconciliationRequest) {
	reconciliations.mu.Lock()
	defer reconciliations.mu.Unlock()
	var decision *NodeConnectorPlacementExecutionReconciliationDecision
	var request *NodeConnectorPlacementExecutionReconciliationRequest
	if reconciliations.decision != nil {
		cloned := cloneNodeConnectorPlacementExecutionReconciliationDecision(*reconciliations.decision)
		decision = &cloned
	}
	if reconciliations.request != nil {
		cloned := cloneNodeConnectorPlacementExecutionReconciliationRequest(*reconciliations.request)
		request = &cloned
	}
	return decision, request
}

func normalizeNodeConnectorPlacementExecutionReconciliationExpected(value NodeConnectorPlacementExecutionReconciliationExpected) (NodeConnectorPlacementExecutionReconciliationExpected, error) {
	delivery, err := normalizeNodeConnectorPlacementExecutionDeliveryExpected(value.Delivery)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.DeliveryFingerprint) {
		return NodeConnectorPlacementExecutionReconciliationExpected{}, errors.New("placement execution reconciliation expected binding is invalid")
	}
	value.Delivery = delivery
	return value, nil
}

func loadNodeConnectorPlacementExecutionReconciliationInputs(root string, expected NodeConnectorPlacementExecutionReconciliationExpected, broker *NodeExecutionFakeBroker) (nodeConnectorPlacementExecutionReconciliationInputs, error) {
	submission, state, operation, err := loadNodeConnectorPlacementExecutionDeliverySubmission(root, expected.Delivery.Handoff.Submission, broker)
	if err != nil || operation == nil || submission.SubmissionFingerprint != expected.Delivery.Handoff.SubmissionFingerprint {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation could not revalidate the complete inventory, placement, dispatch, submission, lease, and broker chain")
	}
	handoffInputs := nodeConnectorPlacementExecutionHandoffInputs{submission: submission}
	handoff, handoffExists, err := loadNodeConnectorPlacementExecutionHandoffDecision(root, handoffInputs, expected.Delivery.Handoff)
	if err != nil || !handoffExists || handoff.Decision != "approved" || handoff.DecisionFingerprint != expected.Delivery.HandoffDecisionFingerprint {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation requires the exact approved execution-handoff decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionHandoffRequest(root, handoffInputs, expected.Delivery.Handoff, handoff, handoffExists)
	if err != nil || !requestExists || request.RequestFingerprint != expected.Delivery.HandoffRequestFingerprint {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation requires the exact execution-handoff request")
	}
	if !nodeExecutionEqual(operation.Request, request.ExecutionRequest) || !nodeExecutionEqual(operation.Lease, request.TaskLease) || operation.Receipt == nil || operation.ExecutionCount != 1 || operation.Cancellation != nil || operation.CancellationAck != nil {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation requires the exact terminal broker operation, request, lease, attempt, and receipt")
	}
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionDeliveryName))
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionDeliveryMaxArtifactBytes {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation could not read the bounded terminal delivery")
	}
	var delivery NodeConnectorPlacementExecutionDelivery
	if decodeNodeConnectorPlacementExecutionDeliveryArtifact(raw, &delivery) != nil {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation requires a strict canonical terminal delivery")
	}
	deliveryInputs := nodeConnectorPlacementExecutionDeliveryInputs{
		submission: submission, decision: handoff, request: request, brokerState: state, operation: *operation,
		sessionState: nodeConnectorSessionState{StateFingerprint: expected.Delivery.SessionStateFingerprint},
		negotiation:  NodeConnectorSessionNegotiation{NegotiationFingerprint: expected.Delivery.NegotiationFingerprint},
	}
	if delivery.DeliveryFingerprint != expected.DeliveryFingerprint || validateNodeConnectorPlacementExecutionDeliveryTerminal(deliveryInputs, NodeConnectorPlacementExecutionDeliveryFixture{ExpectedEvents: delivery.Events, ExpectedReceipt: delivery.Receipt, BrokerStateFingerprintBefore: delivery.BrokerStateFingerprintBefore}) != nil || validateNodeConnectorPlacementExecutionDelivery(delivery, deliveryInputs) != nil {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation terminal delivery is tampered, substituted, or orphaned from its exact broker outcome")
	}
	states, err := loadNodeExecutionStates(broker.root)
	if err != nil {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation could not revalidate durable broker history")
	}
	foundTerminal := false
	for _, historical := range states {
		if historical.StateFingerprint != delivery.BrokerStateFingerprintAfter {
			continue
		}
		historicalOperation, ok := historical.Operations[request.ExecutionRequest.OperationID]
		if ok && nodeExecutionEqual(historicalOperation, *operation) {
			foundTerminal = true
		}
		break
	}
	if !foundTerminal || !nodeExecutionEqual(delivery.Events, operation.Events) || !nodeExecutionEqual(delivery.Receipt, *operation.Receipt) || delivery.ReceiptFingerprint != operation.Receipt.ReceiptFingerprint || !nodeExecutionEqual(delivery.ExecutionRequest.Workflow, request.ExecutionRequest.Workflow) || delivery.ConnectorSourceRevision != request.ExecutionRequest.SourceRevision || !nodeExecutionEqual(delivery.SelectedNode, request.SelectedNode) || delivery.TaskLease.Attempt != operation.Lease.Attempt {
		return nodeConnectorPlacementExecutionReconciliationInputs{}, errors.New("placement execution reconciliation could not revalidate exact terminal history, events, receipt, workflow, revision, selected node, capability, profile, lease, and attempt")
	}
	return nodeConnectorPlacementExecutionReconciliationInputs{submission: submission, handoff: handoff, request: request, delivery: delivery, broker: state, operation: *operation}, nil
}

func deriveNodeConnectorPlacementExecutionReconciliationArtifacts(inputs nodeConnectorPlacementExecutionReconciliationInputs, fixture NodeConnectorPlacementExecutionReconciliationDecisionFixture) (NodeConnectorPlacementExecutionReconciliationDecision, *NodeConnectorPlacementExecutionReconciliationRequest, error) {
	if fixture.Schema != NodeConnectorPlacementExecutionReconciliationDecisionFixtureSchema || fixture.Provenance != nodeConnectorPlacementExecutionReconciliationDecisionProvenance || !nodeExecutionEqual(fixture.Delivery, inputs.delivery) {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision does not exactly bind the revalidated terminal delivery")
	}
	if validateNodeExecutionTypedID("placement-execution-reconciliation-decision", fixture.DecisionID) != nil || validateNodeExecutionTypedID("replay", fixture.ReplayIdentity) != nil || nodeConnectorPlacementExecutionReconciliationIdentityCollides(fixture.DecisionID, inputs, fixture.ReplayIdentity) || nodeConnectorPlacementExecutionReconciliationIdentityCollides(fixture.ReplayIdentity, inputs, fixture.DecisionID) {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision or replay identity is invalid or colliding")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("placement execution reconciliation decision must be independently approved or rejected")
	}
	if fixture.Decision == "approved" {
		if validateNodeExecutionTypedID("placement-execution-reconciliation-request", fixture.ReconciliationRequestID) != nil || fixture.ReconciliationRequestID == fixture.DecisionID || fixture.ReconciliationRequestID == fixture.ReplayIdentity || nodeConnectorPlacementExecutionReconciliationIdentityCollides(fixture.ReconciliationRequestID, inputs) {
			return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("approved placement execution reconciliation decision requires one distinct request identity")
		}
	} else if fixture.ReconciliationRequestID != "" {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, errors.New("rejected placement execution reconciliation decision cannot bind a request")
	}
	decision := NodeConnectorPlacementExecutionReconciliationDecision{
		Schema: NodeConnectorPlacementExecutionReconciliationDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, Decision: fixture.Decision,
		Delivery: cloneNodeConnectorPlacementExecutionDelivery(inputs.delivery), CompleteChainRevalidated: true, ReconciliationRequestID: fixture.ReconciliationRequestID,
		Provenance: nodeConnectorPlacementExecutionReconciliationDecisionProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementExecutionReconciliationDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
	}
	decision.DecisionFingerprint = fingerprint
	if err := validateNodeConnectorPlacementExecutionReconciliationDecision(decision, inputs); err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, nil
	}
	request := NodeConnectorPlacementExecutionReconciliationRequest{
		Schema: NodeConnectorPlacementExecutionReconciliationRequestSchema, RequestID: fixture.ReconciliationRequestID, DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint,
		Delivery: cloneNodeConnectorPlacementExecutionDelivery(inputs.delivery), ReconciliationScope: nodeConnectorPlacementExecutionReconciliationScope, OneTimeRequest: true, TerminalOutcomeOpaque: true,
		Provenance: nodeConnectorPlacementExecutionReconciliationRequestProvenance, FixtureOwned: true, Authority: NodeConnectorPlacementExecutionReconciliationAuthority{LocalGraphReconciliationRequest: true},
	}
	requestFingerprint, err := nodeConnectorPlacementExecutionReconciliationRequestFingerprint(request)
	if err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementExecutionReconciliationRequest(request, decision, inputs); err != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, nil, err
	}
	return decision, &request, nil
}

func validateNodeConnectorPlacementExecutionReconciliationDecision(value NodeConnectorPlacementExecutionReconciliationDecision, inputs nodeConnectorPlacementExecutionReconciliationInputs) error {
	if value.Schema != NodeConnectorPlacementExecutionReconciliationDecisionSchema || value.Provenance != nodeConnectorPlacementExecutionReconciliationDecisionProvenance || !value.FixtureOwned || !value.CompleteChainRevalidated || value.ApprovalInferred || value.Authority != (NodeConnectorPlacementExecutionReconciliationAuthority{}) || !nodeExecutionEqual(value.Delivery, inputs.delivery) {
		return errors.New("placement execution reconciliation decision contract, authority, or delivery binding is invalid")
	}
	if validateNodeExecutionTypedID("placement-execution-reconciliation-decision", value.DecisionID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || nodeConnectorPlacementExecutionReconciliationIdentityCollides(value.DecisionID, inputs, value.ReplayIdentity) || nodeConnectorPlacementExecutionReconciliationIdentityCollides(value.ReplayIdentity, inputs, value.DecisionID) {
		return errors.New("placement execution reconciliation decision identity is invalid or colliding")
	}
	if value.Decision == "approved" {
		if validateNodeExecutionTypedID("placement-execution-reconciliation-request", value.ReconciliationRequestID) != nil || value.ReconciliationRequestID == value.DecisionID || value.ReconciliationRequestID == value.ReplayIdentity || nodeConnectorPlacementExecutionReconciliationIdentityCollides(value.ReconciliationRequestID, inputs) {
			return errors.New("approved placement execution reconciliation decision request identity is invalid")
		}
	} else if value.Decision != "rejected" || value.ReconciliationRequestID != "" {
		return errors.New("placement execution reconciliation decision value or request identity is invalid")
	}
	fingerprint, err := nodeConnectorPlacementExecutionReconciliationDecisionFingerprint(value)
	if err != nil || fingerprint != value.DecisionFingerprint {
		return errors.New("placement execution reconciliation decision fingerprint is invalid")
	}
	return validateNodeConnectorPlacementExecutionReconciliationEncodedBound(value)
}

func validateNodeConnectorPlacementExecutionReconciliationRequest(value NodeConnectorPlacementExecutionReconciliationRequest, decision NodeConnectorPlacementExecutionReconciliationDecision, inputs nodeConnectorPlacementExecutionReconciliationInputs) error {
	wantAuthority := NodeConnectorPlacementExecutionReconciliationAuthority{LocalGraphReconciliationRequest: true}
	if decision.Decision != "approved" || value.Schema != NodeConnectorPlacementExecutionReconciliationRequestSchema || value.RequestID != decision.ReconciliationRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || !nodeExecutionEqual(value.Delivery, inputs.delivery) || !nodeExecutionEqual(value.Delivery, decision.Delivery) ||
		value.ReconciliationScope != nodeConnectorPlacementExecutionReconciliationScope || !value.OneTimeRequest || value.AuthorizationConsumed || !value.TerminalOutcomeOpaque || value.TerminalOutcomeInterpreted || value.GraphReconciliationPerformed || value.GraphCompletionClaimed || value.GraphFailurePropagated || value.NextTaskScheduled ||
		value.ConnectorInvoked || value.PreparedValidationInvoked || value.BrokerExecutorInvoked || value.BrokerOperationCreated || value.LeaseCreated || value.AttemptCreated || value.ConnectionCreated || value.SessionCreated || value.EnrollmentCreated || value.CredentialCreated || value.EventCreated || value.ReceiptCreated || value.DeliveryCreated ||
		value.Provenance != nodeConnectorPlacementExecutionReconciliationRequestProvenance || !value.FixtureOwned || value.Authority != wantAuthority {
		return errors.New("placement execution reconciliation request contract, authority, or immutable terminal binding is invalid")
	}
	fingerprint, err := nodeConnectorPlacementExecutionReconciliationRequestFingerprint(value)
	if err != nil || fingerprint != value.RequestFingerprint {
		return errors.New("placement execution reconciliation request fingerprint is invalid")
	}
	return validateNodeConnectorPlacementExecutionReconciliationEncodedBound(value)
}

func loadNodeConnectorPlacementExecutionReconciliationDecision(root string, inputs nodeConnectorPlacementExecutionReconciliationInputs) (NodeConnectorPlacementExecutionReconciliationDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionReconciliationDecisionName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionReconciliationMaxArtifactBytes {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, false, errors.New("durable placement execution reconciliation decision cannot be read within its bound")
	}
	var decision NodeConnectorPlacementExecutionReconciliationDecision
	if decodeNodeConnectorPlacementExecutionReconciliationArtifact(raw, &decision) != nil || validateNodeConnectorPlacementExecutionReconciliationDecision(decision, inputs) != nil {
		return NodeConnectorPlacementExecutionReconciliationDecision{}, false, errors.New("durable placement execution reconciliation decision is malformed, noncanonical, tampered, or orphaned")
	}
	return decision, true, nil
}

func loadNodeConnectorPlacementExecutionReconciliationRequest(root string, inputs nodeConnectorPlacementExecutionReconciliationInputs, decision NodeConnectorPlacementExecutionReconciliationDecision, decisionExists bool) (NodeConnectorPlacementExecutionReconciliationRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionReconciliationRequestName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionReconciliationRequest{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionReconciliationMaxArtifactBytes || !decisionExists || decision.Decision != "approved" {
		return NodeConnectorPlacementExecutionReconciliationRequest{}, false, errors.New("placement execution reconciliation request cannot exist without its exact approved decision")
	}
	var request NodeConnectorPlacementExecutionReconciliationRequest
	if decodeNodeConnectorPlacementExecutionReconciliationArtifact(raw, &request) != nil || validateNodeConnectorPlacementExecutionReconciliationRequest(request, decision, inputs) != nil {
		return NodeConnectorPlacementExecutionReconciliationRequest{}, false, errors.New("durable placement execution reconciliation request is malformed, noncanonical, tampered, or orphaned")
	}
	return request, true, nil
}

func nodeConnectorPlacementExecutionReconciliationIdentityCollides(value string, inputs nodeConnectorPlacementExecutionReconciliationInputs, additional ...string) bool {
	delivery, request, lease, receipt := inputs.delivery, inputs.request.ExecutionRequest, inputs.request.TaskLease, inputs.delivery.Receipt
	identities := []string{
		delivery.DeliveryID, delivery.ReplayIdentity, delivery.HandoffDecisionID, delivery.HandoffRequestID, delivery.SubmissionID, delivery.SubmissionReplayIdentity,
		delivery.EnrollmentID, delivery.CredentialID, delivery.SessionID, delivery.ConnectionID, delivery.NegotiationID, delivery.MachineID, delivery.WorkloadID, delivery.SelectedNode.NodeID,
		request.OperationID, request.GraphRunID, request.RunID, request.TaskID, lease.LeaseID, lease.CancellationID, receipt.ReceiptID, receipt.LocalRunID,
	}
	identities = append(identities, additional...)
	for _, identity := range identities {
		if value == identity {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionReconciliationDecisionFingerprint(value NodeConnectorPlacementExecutionReconciliationDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionReconciliationRequestFingerprint(value NodeConnectorPlacementExecutionReconciliationRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorPlacementExecutionReconciliationEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementExecutionReconciliationMaxArtifactBytes {
		return errors.New("placement execution reconciliation decision or request exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorPlacementExecutionReconciliationArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("placement execution reconciliation decision or request is not canonical")
	}
	return nil
}

func requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("placement execution reconciliation " + kind + " already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("placement execution reconciliation " + kind + " path cannot be inspected")
	}
	return nil
}

func cloneNodeConnectorPlacementExecutionReconciliationDecision(value NodeConnectorPlacementExecutionReconciliationDecision) NodeConnectorPlacementExecutionReconciliationDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionReconciliationDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionReconciliationRequest(value NodeConnectorPlacementExecutionReconciliationRequest) NodeConnectorPlacementExecutionReconciliationRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionReconciliationRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorPlacementExecutionReconciliationDecisionFixture(value NodeConnectorPlacementExecutionReconciliationDecisionFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorPlacementExecutionReconciliationDecisionFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorPlacementExecutionReconciliationDecisionProvenance
	}
	return json.Marshal(value)
}
