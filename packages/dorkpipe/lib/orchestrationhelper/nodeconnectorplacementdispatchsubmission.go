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
	NodeConnectorPlacementDispatchSubmissionFixtureSchema = "dorkpipe.node-placement-dispatch-submission-fixture/v1"
	NodeConnectorPlacementDispatchSubmissionSchema        = "dorkpipe.node-placement-dispatch-submission/v1"

	nodeConnectorPlacementDispatchSubmissionProvenance       = "fixture_only_placement_dispatch_submission"
	nodeConnectorPlacementDispatchSubmissionName             = "node-placement-dispatch-submission.json"
	nodeConnectorPlacementDispatchMaxSubmissionBytes         = 512 << 10
	nodeConnectorPlacementDispatchMaxSubmissionArtifactBytes = 1 << 20
	nodeConnectorPlacementDispatchMaxLeaseDurationSeconds    = int64(24 * 60 * 60)
)

var nodeConnectorPlacementDispatchSubmissionWriteAtomic = writeJSONFileAtomic

type NodeConnectorPlacementDispatchSubmissionExpected struct {
	Dispatch                             NodeConnectorPlacementDispatchExpected `json:"dispatch"`
	PlacementDispatchDecisionFingerprint string                                 `json:"placement_dispatch_decision_fingerprint"`
	PlacementDispatchRequestFingerprint  string                                 `json:"placement_dispatch_request_fingerprint"`
}

// NodeConnectorPlacementDispatchSubmissionAuthority is deliberately all false.
// The artifact proves one broker transition; it is not a second lease, an
// execution receipt, or authority for any adjacent operation.
type NodeConnectorPlacementDispatchSubmissionAuthority struct {
	Executor     bool `json:"executor"`
	Connector    bool `json:"connector"`
	Event        bool `json:"event"`
	Receipt      bool `json:"receipt"`
	Cancellation bool `json:"cancellation"`
	Network      bool `json:"network"`
	Provider     bool `json:"provider"`
	Retry        bool `json:"retry"`
	Repair       bool `json:"repair"`
	Quarantine   bool `json:"quarantine"`
	Service      bool `json:"service"`
	Mutation     bool `json:"mutation"`
	Validation   bool `json:"validation"`
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

type NodeConnectorPlacementDispatchSubmissionFixture struct {
	Schema                               string                                    `json:"schema"`
	SubmissionID                         string                                    `json:"submission_id"`
	ReplayIdentity                       string                                    `json:"replay_identity"`
	InventorySnapshotID                  string                                    `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint         string                                    `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                     string                                    `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint    string                                    `json:"placement_input_snapshot_fingerprint"`
	PlacementDecisionID                  string                                    `json:"placement_decision_id"`
	PlacementDecisionFingerprint         string                                    `json:"placement_decision_fingerprint"`
	PlacementRequestID                   string                                    `json:"placement_request_id"`
	PlacementRequestFingerprint          string                                    `json:"placement_request_fingerprint"`
	PlacementDispatchDecisionID          string                                    `json:"placement_dispatch_decision_id"`
	PlacementDispatchDecisionFingerprint string                                    `json:"placement_dispatch_decision_fingerprint"`
	PlacementDispatchRequestID           string                                    `json:"placement_dispatch_request_id"`
	PlacementDispatchRequestFingerprint  string                                    `json:"placement_dispatch_request_fingerprint"`
	WorkloadID                           string                                    `json:"workload_id"`
	CandidateNodeIDs                     []string                                  `json:"candidate_node_ids"`
	SelectedNode                         NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	ExecutionTaskID                      string                                    `json:"execution_task_id"`
	ExecutionRequest                     NodeExecutionRequest                      `json:"execution_request"`
	ExecutionRequestFingerprint          string                                    `json:"execution_request_fingerprint"`
	IssuedAt                             string                                    `json:"issued_at"`
	LeaseDurationSeconds                 int64                                     `json:"lease_duration_seconds"`
	Provenance                           string                                    `json:"provenance"`
}

type NodeConnectorPlacementDispatchSubmission struct {
	Schema                               string                                            `json:"schema"`
	SubmissionID                         string                                            `json:"submission_id"`
	ReplayIdentity                       string                                            `json:"replay_identity"`
	InventorySnapshotID                  string                                            `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint         string                                            `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                     string                                            `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint    string                                            `json:"placement_input_snapshot_fingerprint"`
	PlacementDecisionID                  string                                            `json:"placement_decision_id"`
	PlacementDecisionFingerprint         string                                            `json:"placement_decision_fingerprint"`
	PlacementRequestID                   string                                            `json:"placement_request_id"`
	PlacementRequestFingerprint          string                                            `json:"placement_request_fingerprint"`
	PlacementDispatchDecisionID          string                                            `json:"placement_dispatch_decision_id"`
	PlacementDispatchDecisionFingerprint string                                            `json:"placement_dispatch_decision_fingerprint"`
	PlacementDispatchRequestID           string                                            `json:"placement_dispatch_request_id"`
	PlacementDispatchRequestFingerprint  string                                            `json:"placement_dispatch_request_fingerprint"`
	WorkloadID                           string                                            `json:"workload_id"`
	CandidateNodeIDs                     []string                                          `json:"candidate_node_ids"`
	CompleteCandidateSet                 bool                                              `json:"complete_candidate_set"`
	SelectedNode                         NodeConnectorPlacementSelectedNodeBinding         `json:"selected_node"`
	ExecutionTaskID                      string                                            `json:"execution_task_id"`
	ExecutionRequest                     NodeExecutionRequest                              `json:"execution_request"`
	ExecutionRequestFingerprint          string                                            `json:"execution_request_fingerprint"`
	IssuedAt                             string                                            `json:"issued_at"`
	LeaseDurationSeconds                 int64                                             `json:"lease_duration_seconds"`
	BrokerStateFingerprint               string                                            `json:"broker_state_fingerprint"`
	TaskLease                            NodeExecutionTaskLease                            `json:"task_lease"`
	AuthorizationConsumed                bool                                              `json:"authorization_consumed"`
	BrokerInvoked                        bool                                              `json:"broker_invoked"`
	LeaseIssued                          bool                                              `json:"lease_issued"`
	ExecutorInvoked                      bool                                              `json:"executor_invoked"`
	ExecutionStarted                     bool                                              `json:"execution_started"`
	ConnectionCreated                    bool                                              `json:"connection_created"`
	Provenance                           string                                            `json:"provenance"`
	FixtureOwned                         bool                                              `json:"fixture_owned"`
	Authority                            NodeConnectorPlacementDispatchSubmissionAuthority `json:"authority"`
	SubmissionFingerprint                string                                            `json:"submission_fingerprint"`
}

type nodeConnectorPlacementDispatchSubmissionInputs struct {
	dispatchInputs   nodeConnectorPlacementDispatchInputs
	dispatchDecision NodeConnectorPlacementDispatchDecision
	dispatchRequest  NodeConnectorPlacementDispatchRequest
}

type NodeConnectorPlacementDispatchSubmissions struct {
	root       string
	expected   NodeConnectorPlacementDispatchSubmissionExpected
	inputs     nodeConnectorPlacementDispatchSubmissionInputs
	broker     *NodeExecutionFakeBroker
	submission *NodeConnectorPlacementDispatchSubmission
	mu         sync.Mutex
}

func OpenNodeConnectorPlacementDispatchSubmissions(root string, expected NodeConnectorPlacementDispatchSubmissionExpected, broker *NodeExecutionFakeBroker) (*NodeConnectorPlacementDispatchSubmissions, error) {
	normalized, err := normalizeNodeConnectorPlacementDispatchSubmissionExpected(expected)
	if err != nil {
		return nil, err
	}
	inputs, err := loadNodeConnectorPlacementDispatchSubmissionInputs(root, normalized)
	if err != nil {
		return nil, err
	}
	brokerState, operation, err := revalidateNodeConnectorPlacementDispatchSubmissionBroker(broker, inputs.dispatchRequest)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementDispatchSubmissions{root: root, expected: normalized, inputs: inputs, broker: broker}
	submission, exists, err := loadNodeConnectorPlacementDispatchSubmission(root, inputs, normalized, brokerState, operation)
	if err != nil {
		return nil, err
	}
	if exists {
		value.submission = &submission
	}
	return value, nil
}

func (submissions *NodeConnectorPlacementDispatchSubmissions) Submit(connectionID string, raw []byte) (NodeConnectorPlacementDispatchSubmission, error) {
	submissions.mu.Lock()
	defer submissions.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementDispatchMaxSubmissionBytes {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementDispatchSubmissionFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission fixture is not strict canonical JSON")
	}
	inputs, err := loadNodeConnectorPlacementDispatchSubmissionInputs(submissions.root, submissions.expected)
	if err != nil || !nodeExecutionEqual(inputs, submissions.inputs) {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission could not directly revalidate its complete immutable authorization chain")
	}
	issuedAt, leaseDuration, err := validateNodeConnectorPlacementDispatchSubmissionFixture(fixture, inputs)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	brokerState, operation, err := revalidateNodeConnectorPlacementDispatchSubmissionBroker(submissions.broker, inputs.dispatchRequest)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	if submissions.submission != nil {
		if operation == nil || validateNodeConnectorPlacementDispatchSubmission(*submissions.submission, inputs, submissions.expected, brokerState, operation) != nil || !nodeConnectorPlacementDispatchSubmissionMatchesFixture(*submissions.submission, fixture) {
			return NodeConnectorPlacementDispatchSubmission{}, errors.New("changed submission replay or broker evidence is rejected")
		}
		return cloneNodeConnectorPlacementDispatchSubmission(*submissions.submission), nil
	}
	if submissions.broker.executor != nil {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission requires an executorless fake broker")
	}
	connectedMachine, err := submissions.broker.connectedMachine(connectionID)
	if err != nil || connectedMachine != inputs.dispatchRequest.SelectedNode.MachineID {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission requires an existing exact selected-machine connection")
	}
	requestRaw, err := json.Marshal(inputs.dispatchRequest.ExecutionRequest)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	lease, err := submissions.broker.Dispatch(connectionID, requestRaw, issuedAt, leaseDuration)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission broker transition failed")
	}
	brokerState, operation, err = revalidateNodeConnectorPlacementDispatchSubmissionBroker(submissions.broker, inputs.dispatchRequest)
	if err != nil || operation == nil || !nodeExecutionEqual(lease, operation.Lease) {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission could not revalidate the exact broker-issued lease")
	}
	submission, err := deriveNodeConnectorPlacementDispatchSubmission(inputs, fixture, brokerState, lease)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	path := filepath.Join(submissions.root, nodeConnectorPlacementDispatchSubmissionName)
	if err := requireNodeConnectorPlacementDispatchSubmissionAbsent(path); err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	if err := nodeConnectorPlacementDispatchSubmissionWriteAtomic(path, submission); err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, errors.New("placement dispatch submission artifact could not be published after broker acceptance")
	}
	submissions.submission = &submission
	return cloneNodeConnectorPlacementDispatchSubmission(submission), nil
}

func (submissions *NodeConnectorPlacementDispatchSubmissions) Artifact() *NodeConnectorPlacementDispatchSubmission {
	submissions.mu.Lock()
	defer submissions.mu.Unlock()
	if submissions.submission == nil {
		return nil
	}
	value := cloneNodeConnectorPlacementDispatchSubmission(*submissions.submission)
	return &value
}

func normalizeNodeConnectorPlacementDispatchSubmissionExpected(value NodeConnectorPlacementDispatchSubmissionExpected) (NodeConnectorPlacementDispatchSubmissionExpected, error) {
	dispatch, err := normalizeNodeConnectorPlacementDispatchExpected(value.Dispatch)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.PlacementDispatchDecisionFingerprint) || !nodeExecutionFingerprint.MatchString(value.PlacementDispatchRequestFingerprint) {
		return NodeConnectorPlacementDispatchSubmissionExpected{}, errors.New("placement dispatch submission expected binding is invalid")
	}
	value.Dispatch = dispatch
	return value, nil
}

func loadNodeConnectorPlacementDispatchSubmissionInputs(root string, expected NodeConnectorPlacementDispatchSubmissionExpected) (nodeConnectorPlacementDispatchSubmissionInputs, error) {
	dispatchInputs, err := loadNodeConnectorPlacementDispatchInputs(root, expected.Dispatch)
	if err != nil {
		return nodeConnectorPlacementDispatchSubmissionInputs{}, errors.New("placement dispatch submission could not revalidate the inventory and placement chain")
	}
	decision, decisionExists, err := loadNodeConnectorPlacementDispatchDecision(root, dispatchInputs, expected.Dispatch)
	if err != nil {
		return nodeConnectorPlacementDispatchSubmissionInputs{}, err
	}
	request, requestExists, err := loadNodeConnectorPlacementDispatchRequest(root, dispatchInputs, expected.Dispatch, decision, decisionExists)
	if err != nil || !decisionExists || !requestExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PlacementDispatchDecisionFingerprint || request.RequestFingerprint != expected.PlacementDispatchRequestFingerprint {
		return nodeConnectorPlacementDispatchSubmissionInputs{}, errors.New("placement dispatch submission requires one exact approved unconsumed dispatch request")
	}
	if request.AuthorizationConsumed || request.BrokerInvoked || request.LeaseIssued || request.ExecutionStarted || request.Authority != (NodeConnectorPlacementDispatchAuthority{FixtureBrokerSubmission: true}) || validateNodeExecutionRequest(request.ExecutionRequest) != nil {
		return nodeConnectorPlacementDispatchSubmissionInputs{}, errors.New("placement dispatch request is consumed, mutated, or no longer grants only fixture-broker submission")
	}
	return nodeConnectorPlacementDispatchSubmissionInputs{dispatchInputs: dispatchInputs, dispatchDecision: decision, dispatchRequest: request}, nil
}

func validateNodeConnectorPlacementDispatchSubmissionFixture(value NodeConnectorPlacementDispatchSubmissionFixture, inputs nodeConnectorPlacementDispatchSubmissionInputs) (time.Time, time.Duration, error) {
	request, decision := inputs.dispatchRequest, inputs.dispatchDecision
	if value.Schema != NodeConnectorPlacementDispatchSubmissionFixtureSchema || value.Provenance != nodeConnectorPlacementDispatchSubmissionProvenance ||
		value.InventorySnapshotID != request.InventorySnapshotID || value.InventorySnapshotFingerprint != request.InventorySnapshotFingerprint ||
		value.PlacementInputID != request.PlacementInputID || value.PlacementInputSnapshotFingerprint != request.PlacementInputSnapshotFingerprint ||
		value.PlacementDecisionID != request.PlacementDecisionID || value.PlacementDecisionFingerprint != request.PlacementDecisionFingerprint ||
		value.PlacementRequestID != request.PlacementRequestID || value.PlacementRequestFingerprint != request.PlacementRequestFingerprint ||
		value.PlacementDispatchDecisionID != decision.DecisionID || value.PlacementDispatchDecisionFingerprint != decision.DecisionFingerprint ||
		value.PlacementDispatchRequestID != request.RequestID || value.PlacementDispatchRequestFingerprint != request.RequestFingerprint ||
		value.WorkloadID != request.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, request.CandidateNodeIDs) || !nodeExecutionEqual(value.SelectedNode, request.SelectedNode) ||
		value.ExecutionTaskID != request.ExecutionTaskID || !nodeExecutionEqual(value.ExecutionRequest, request.ExecutionRequest) || value.ExecutionRequestFingerprint != request.ExecutionRequest.RequestFingerprint {
		return time.Time{}, 0, errors.New("placement dispatch submission fixture does not exactly bind the complete authorization and execution request")
	}
	if validateNodeExecutionTypedID("submission", value.SubmissionID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || placementDispatchIdentityCollides(value.SubmissionID, value.ReplayIdentity, decision.DecisionID, decision.ReplayIdentity, request.RequestID) || placementDispatchIdentityCollides(value.ReplayIdentity, value.SubmissionID, request.DecisionID, request.RequestID) {
		return time.Time{}, 0, errors.New("placement dispatch submission or replay identity is invalid or colliding")
	}
	issuedAt, err := parseNodeExecutionTime(value.IssuedAt)
	if err != nil || value.LeaseDurationSeconds <= 0 || value.LeaseDurationSeconds > nodeConnectorPlacementDispatchMaxLeaseDurationSeconds {
		return time.Time{}, 0, errors.New("placement dispatch submission issuance time or lease duration is invalid or unbounded")
	}
	requestedAt, _ := parseNodeExecutionTime(request.ExecutionRequest.RequestedAt)
	if issuedAt.Before(requestedAt) {
		return time.Time{}, 0, errors.New("placement dispatch submission cannot issue a lease before the execution request")
	}
	return issuedAt, time.Duration(value.LeaseDurationSeconds) * time.Second, nil
}

func deriveNodeConnectorPlacementDispatchSubmission(inputs nodeConnectorPlacementDispatchSubmissionInputs, fixture NodeConnectorPlacementDispatchSubmissionFixture, brokerState nodeExecutionBrokerState, lease NodeExecutionTaskLease) (NodeConnectorPlacementDispatchSubmission, error) {
	value := NodeConnectorPlacementDispatchSubmission{
		Schema: NodeConnectorPlacementDispatchSubmissionSchema, SubmissionID: fixture.SubmissionID, ReplayIdentity: fixture.ReplayIdentity,
		InventorySnapshotID: fixture.InventorySnapshotID, InventorySnapshotFingerprint: fixture.InventorySnapshotFingerprint,
		PlacementInputID: fixture.PlacementInputID, PlacementInputSnapshotFingerprint: fixture.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: fixture.PlacementDecisionID, PlacementDecisionFingerprint: fixture.PlacementDecisionFingerprint,
		PlacementRequestID: fixture.PlacementRequestID, PlacementRequestFingerprint: fixture.PlacementRequestFingerprint,
		PlacementDispatchDecisionID: fixture.PlacementDispatchDecisionID, PlacementDispatchDecisionFingerprint: fixture.PlacementDispatchDecisionFingerprint,
		PlacementDispatchRequestID: fixture.PlacementDispatchRequestID, PlacementDispatchRequestFingerprint: fixture.PlacementDispatchRequestFingerprint,
		WorkloadID: fixture.WorkloadID, CandidateNodeIDs: append([]string{}, fixture.CandidateNodeIDs...), CompleteCandidateSet: true,
		SelectedNode: fixture.SelectedNode, ExecutionTaskID: fixture.ExecutionTaskID, ExecutionRequest: cloneNodeExecutionRequest(fixture.ExecutionRequest), ExecutionRequestFingerprint: fixture.ExecutionRequestFingerprint,
		IssuedAt: fixture.IssuedAt, LeaseDurationSeconds: fixture.LeaseDurationSeconds, BrokerStateFingerprint: brokerState.StateFingerprint, TaskLease: lease,
		AuthorizationConsumed: true, BrokerInvoked: true, LeaseIssued: true, Provenance: nodeConnectorPlacementDispatchSubmissionProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementDispatchSubmissionFingerprint(value)
	if err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	value.SubmissionFingerprint = fingerprint
	operation := brokerState.Operations[inputs.dispatchRequest.ExecutionRequest.OperationID]
	if err := validateNodeConnectorPlacementDispatchSubmission(value, inputs, NodeConnectorPlacementDispatchSubmissionExpected{}, brokerState, &operation); err != nil {
		return NodeConnectorPlacementDispatchSubmission{}, err
	}
	return value, nil
}

func revalidateNodeConnectorPlacementDispatchSubmissionBroker(broker *NodeExecutionFakeBroker, request NodeConnectorPlacementDispatchRequest) (nodeExecutionBrokerState, *nodeExecutionOperationState, error) {
	if broker == nil {
		return nodeExecutionBrokerState{}, nil, errors.New("placement dispatch submission requires the existing in-process fake broker")
	}
	states, err := loadNodeExecutionStates(broker.root)
	if err != nil || len(states) == 0 || !nodeExecutionEqual(states[len(states)-1], broker.state) {
		return nodeExecutionBrokerState{}, nil, errors.New("placement dispatch submission could not revalidate the durable fake-broker state")
	}
	state := states[len(states)-1]
	if state.Machine.MachineID != request.SelectedNode.MachineID {
		return nodeExecutionBrokerState{}, nil, errors.New("fake broker machine does not match the exact selected machine")
	}
	capability, ok := broker.capability(request.SelectedNode.CapabilitySnapshotID)
	capabilityFingerprint, fingerprintErr := nodeExecutionFingerprintValue(capability)
	if !ok || fingerprintErr != nil || capability.MachineID != request.SelectedNode.MachineID || capability.SnapshotID != request.SelectedNode.CapabilitySnapshotID || capabilityFingerprint != request.SelectedNode.CapabilitySnapshotFingerprint {
		return nodeExecutionBrokerState{}, nil, errors.New("fake broker does not contain the exact registered selected capability snapshot")
	}
	operation, exists := state.Operations[request.ExecutionRequest.OperationID]
	if !exists {
		return state, nil, nil
	}
	if !nodeExecutionEqual(operation.Request, request.ExecutionRequest) || operation.Request.RequestFingerprint != request.ExecutionRequest.RequestFingerprint || operation.Lease.MachineID != request.SelectedNode.MachineID || operation.Lease.CapabilitySnapshotID != request.SelectedNode.CapabilitySnapshotID {
		return nodeExecutionBrokerState{}, nil, errors.New("fake broker operation conflicts with the exact authorized request or selected binding")
	}
	copy := operation
	return state, &copy, nil
}

func validateNodeConnectorPlacementDispatchSubmission(value NodeConnectorPlacementDispatchSubmission, inputs nodeConnectorPlacementDispatchSubmissionInputs, expected NodeConnectorPlacementDispatchSubmissionExpected, brokerState nodeExecutionBrokerState, operation *nodeExecutionOperationState) error {
	request, decision := inputs.dispatchRequest, inputs.dispatchDecision
	if value.Schema != NodeConnectorPlacementDispatchSubmissionSchema || value.Provenance != nodeConnectorPlacementDispatchSubmissionProvenance || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementDispatchSubmissionAuthority{}) ||
		!value.AuthorizationConsumed || !value.BrokerInvoked || !value.LeaseIssued || value.ExecutorInvoked || value.ExecutionStarted || value.ConnectionCreated || !value.CompleteCandidateSet ||
		value.InventorySnapshotID != request.InventorySnapshotID || value.InventorySnapshotFingerprint != request.InventorySnapshotFingerprint || value.PlacementInputID != request.PlacementInputID || value.PlacementInputSnapshotFingerprint != request.PlacementInputSnapshotFingerprint ||
		value.PlacementDecisionID != request.PlacementDecisionID || value.PlacementDecisionFingerprint != request.PlacementDecisionFingerprint || value.PlacementRequestID != request.PlacementRequestID || value.PlacementRequestFingerprint != request.PlacementRequestFingerprint ||
		value.PlacementDispatchDecisionID != decision.DecisionID || value.PlacementDispatchDecisionFingerprint != decision.DecisionFingerprint || value.PlacementDispatchRequestID != request.RequestID || value.PlacementDispatchRequestFingerprint != request.RequestFingerprint ||
		value.WorkloadID != request.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, request.CandidateNodeIDs) || !nodeExecutionEqual(value.SelectedNode, request.SelectedNode) || value.ExecutionTaskID != request.ExecutionTaskID || !nodeExecutionEqual(value.ExecutionRequest, request.ExecutionRequest) || value.ExecutionRequestFingerprint != request.ExecutionRequest.RequestFingerprint {
		return errors.New("placement dispatch submission contract, authority, or immutable chain binding is invalid")
	}
	if validateNodeExecutionTypedID("submission", value.SubmissionID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || value.SubmissionID == value.ReplayIdentity || value.BrokerStateFingerprint != brokerState.StateFingerprint || operation == nil || !nodeExecutionEqual(value.TaskLease, operation.Lease) {
		return errors.New("placement dispatch submission identity, broker-state, or lease binding is invalid")
	}
	issuedAt, err := parseNodeExecutionTime(value.IssuedAt)
	if err != nil || value.LeaseDurationSeconds <= 0 || value.LeaseDurationSeconds > nodeConnectorPlacementDispatchMaxLeaseDurationSeconds {
		return errors.New("placement dispatch submission issuance policy is invalid")
	}
	expectedLease := newNodeExecutionLease(request.ExecutionRequest, request.SelectedNode.MachineID, issuedAt, issuedAt.Add(time.Duration(value.LeaseDurationSeconds)*time.Second))
	if !nodeExecutionEqual(value.TaskLease, expectedLease) || validateNodeExecutionLease(value.TaskLease) != nil {
		return errors.New("placement dispatch submission lease is not the exact broker-issued lease for its issuance policy")
	}
	if expected.PlacementDispatchDecisionFingerprint != "" && (value.PlacementDispatchDecisionFingerprint != expected.PlacementDispatchDecisionFingerprint || value.PlacementDispatchRequestFingerprint != expected.PlacementDispatchRequestFingerprint) {
		return errors.New("placement dispatch submission does not match the exact expected dispatch artifacts")
	}
	fingerprint, err := nodeConnectorPlacementDispatchSubmissionFingerprint(value)
	if err != nil || fingerprint != value.SubmissionFingerprint {
		return errors.New("placement dispatch submission fingerprint is invalid")
	}
	return validateNodeConnectorPlacementDispatchSubmissionEncodedBound(value)
}

func loadNodeConnectorPlacementDispatchSubmission(root string, inputs nodeConnectorPlacementDispatchSubmissionInputs, expected NodeConnectorPlacementDispatchSubmissionExpected, brokerState nodeExecutionBrokerState, operation *nodeExecutionOperationState) (NodeConnectorPlacementDispatchSubmission, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementDispatchSubmissionName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementDispatchSubmission{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementDispatchMaxSubmissionArtifactBytes {
		return NodeConnectorPlacementDispatchSubmission{}, false, errors.New("durable placement dispatch submission cannot be read within its bound")
	}
	var value NodeConnectorPlacementDispatchSubmission
	if decodeNodeConnectorPlacementDispatchSubmissionArtifact(raw, &value) != nil || validateNodeConnectorPlacementDispatchSubmission(value, inputs, expected, brokerState, operation) != nil {
		return NodeConnectorPlacementDispatchSubmission{}, false, errors.New("durable placement dispatch submission is malformed, noncanonical, tampered, or orphaned from its broker lease")
	}
	return value, true, nil
}

func nodeConnectorPlacementDispatchSubmissionMatchesFixture(value NodeConnectorPlacementDispatchSubmission, fixture NodeConnectorPlacementDispatchSubmissionFixture) bool {
	return value.SubmissionID == fixture.SubmissionID && value.ReplayIdentity == fixture.ReplayIdentity && value.IssuedAt == fixture.IssuedAt && value.LeaseDurationSeconds == fixture.LeaseDurationSeconds &&
		value.PlacementDispatchRequestFingerprint == fixture.PlacementDispatchRequestFingerprint && value.ExecutionRequestFingerprint == fixture.ExecutionRequestFingerprint && nodeExecutionEqual(value.ExecutionRequest, fixture.ExecutionRequest) && nodeExecutionEqual(value.SelectedNode, fixture.SelectedNode)
}

func nodeConnectorPlacementDispatchSubmissionFingerprint(value NodeConnectorPlacementDispatchSubmission) (string, error) {
	value.SubmissionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorPlacementDispatchSubmissionEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementDispatchMaxSubmissionArtifactBytes {
		return errors.New("placement dispatch submission durable artifact exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorPlacementDispatchSubmissionArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("placement dispatch submission durable artifact is not canonical")
	}
	return nil
}

func requireNodeConnectorPlacementDispatchSubmissionAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("placement dispatch submission artifact already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("placement dispatch submission artifact path cannot be inspected")
	}
	return nil
}

func cloneNodeConnectorPlacementDispatchSubmission(value NodeConnectorPlacementDispatchSubmission) NodeConnectorPlacementDispatchSubmission {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDispatchSubmission
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorPlacementDispatchSubmissionFixture(value NodeConnectorPlacementDispatchSubmissionFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorPlacementDispatchSubmissionFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorPlacementDispatchSubmissionProvenance
	}
	return json.Marshal(value)
}
