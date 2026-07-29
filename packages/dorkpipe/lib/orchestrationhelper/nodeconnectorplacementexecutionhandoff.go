package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	NodeConnectorPlacementExecutionHandoffDecisionFixtureSchema = "dorkpipe.node-placement-execution-handoff-decision-fixture/v1"
	NodeConnectorPlacementExecutionHandoffDecisionSchema        = "dorkpipe.node-placement-execution-handoff-decision/v1"
	NodeConnectorPlacementExecutionHandoffRequestSchema         = "dorkpipe.node-placement-execution-handoff-request/v1"

	nodeConnectorPlacementExecutionHandoffDecisionProvenance = "fixture_only_local_placement_execution_handoff_decision"
	nodeConnectorPlacementExecutionHandoffRequestProvenance  = "fixture_only_placement_bound_execution_handoff_request"
	nodeConnectorPlacementExecutionHandoffScope              = "exact_broker_accepted_request_lease_to_connector_session_once"
	nodeConnectorPlacementExecutionHandoffDecisionName       = "node-placement-execution-handoff-decision.json"
	nodeConnectorPlacementExecutionHandoffRequestName        = "node-placement-execution-handoff-request.json"
	nodeConnectorPlacementExecutionHandoffMaxDecisionBytes   = 512 << 10
	nodeConnectorPlacementExecutionHandoffMaxArtifactBytes   = 1 << 20
	nodeConnectorPlacementExecutionHandoffMaxReasonBytes     = 512
)

var (
	nodeConnectorPlacementExecutionHandoffWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionHandoffWriteRequestAtomic  = writeJSONFileAtomic
)

type NodeConnectorPlacementExecutionHandoffExpected struct {
	Submission            NodeConnectorPlacementDispatchSubmissionExpected `json:"submission"`
	SubmissionFingerprint string                                           `json:"submission_fingerprint"`
}

// NodeConnectorPlacementExecutionHandoffAuthority permits only a future,
// separately implemented one-time call through the existing in-process
// connector-session seam. It grants no invocation or adjacent authority.
type NodeConnectorPlacementExecutionHandoffAuthority struct {
	FixtureConnectorHandoff bool `json:"fixture_connector_handoff"`
	ConnectorInvocation     bool `json:"connector_invocation"`
	ExecutorInvocation      bool `json:"executor_invocation"`
	ExecutionStart          bool `json:"execution_start"`
	Event                   bool `json:"event"`
	Receipt                 bool `json:"receipt"`
	Cancellation            bool `json:"cancellation"`
	Network                 bool `json:"network"`
	Provider                bool `json:"provider"`
	Retry                   bool `json:"retry"`
	Repair                  bool `json:"repair"`
	Quarantine              bool `json:"quarantine"`
	Service                 bool `json:"service"`
	Validation              bool `json:"validation"`
	Mutation                bool `json:"mutation"`
	Git                     bool `json:"git"`
	Apply                   bool `json:"apply"`
	Checkpoint              bool `json:"checkpoint"`
	Commit                  bool `json:"commit"`
	Push                    bool `json:"push"`
	Publication             bool `json:"publication"`
	Completion              bool `json:"completion"`
	Lifecycle               bool `json:"lifecycle"`
	NextTask                bool `json:"next_task"`
}

// NodeConnectorPlacementExecutionHandoffDecisionFixture is an independent
// strict local decision. Placement, connection, availability, load, risk,
// cost, ordering, provider, broker, and lease evidence cannot imply approval.
type NodeConnectorPlacementExecutionHandoffDecisionFixture struct {
	Schema                       string                                    `json:"schema"`
	DecisionID                   string                                    `json:"decision_id"`
	ReplayIdentity               string                                    `json:"replay_identity"`
	Decision                     string                                    `json:"decision"`
	Reason                       string                                    `json:"reason"`
	IssuedAt                     string                                    `json:"issued_at"`
	SubmissionID                 string                                    `json:"submission_id"`
	SubmissionReplayIdentity     string                                    `json:"submission_replay_identity"`
	SubmissionFingerprint        string                                    `json:"submission_fingerprint"`
	SubmissionProvenance         string                                    `json:"submission_provenance"`
	InventorySnapshotID          string                                    `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint string                                    `json:"inventory_snapshot_fingerprint"`
	PlacementInputID             string                                    `json:"placement_input_id"`
	PlacementInputFingerprint    string                                    `json:"placement_input_fingerprint"`
	PlacementDecisionID          string                                    `json:"placement_decision_id"`
	PlacementDecisionFingerprint string                                    `json:"placement_decision_fingerprint"`
	PlacementRequestID           string                                    `json:"placement_request_id"`
	PlacementRequestFingerprint  string                                    `json:"placement_request_fingerprint"`
	DispatchDecisionID           string                                    `json:"dispatch_decision_id"`
	DispatchDecisionFingerprint  string                                    `json:"dispatch_decision_fingerprint"`
	DispatchRequestID            string                                    `json:"dispatch_request_id"`
	DispatchRequestFingerprint   string                                    `json:"dispatch_request_fingerprint"`
	WorkloadID                   string                                    `json:"workload_id"`
	CandidateNodeIDs             []string                                  `json:"candidate_node_ids"`
	SelectedNode                 NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	ExecutionTaskID              string                                    `json:"execution_task_id"`
	ExecutionRequest             NodeExecutionRequest                      `json:"execution_request"`
	ExecutionRequestFingerprint  string                                    `json:"execution_request_fingerprint"`
	BrokerStateFingerprint       string                                    `json:"broker_state_fingerprint"`
	TaskLease                    NodeExecutionTaskLease                    `json:"task_lease"`
	ExecutionHandoffRequestID    string                                    `json:"execution_handoff_request_id,omitempty"`
	Provenance                   string                                    `json:"provenance"`
}

type NodeConnectorPlacementExecutionHandoffDecision struct {
	Schema                       string                                          `json:"schema"`
	DecisionID                   string                                          `json:"decision_id"`
	ReplayIdentity               string                                          `json:"replay_identity"`
	Decision                     string                                          `json:"decision"`
	Reason                       string                                          `json:"reason"`
	IssuedAt                     string                                          `json:"issued_at"`
	SubmissionID                 string                                          `json:"submission_id"`
	SubmissionReplayIdentity     string                                          `json:"submission_replay_identity"`
	SubmissionFingerprint        string                                          `json:"submission_fingerprint"`
	SubmissionProvenance         string                                          `json:"submission_provenance"`
	InventorySnapshotID          string                                          `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint string                                          `json:"inventory_snapshot_fingerprint"`
	PlacementInputID             string                                          `json:"placement_input_id"`
	PlacementInputFingerprint    string                                          `json:"placement_input_fingerprint"`
	PlacementDecisionID          string                                          `json:"placement_decision_id"`
	PlacementDecisionFingerprint string                                          `json:"placement_decision_fingerprint"`
	PlacementRequestID           string                                          `json:"placement_request_id"`
	PlacementRequestFingerprint  string                                          `json:"placement_request_fingerprint"`
	DispatchDecisionID           string                                          `json:"dispatch_decision_id"`
	DispatchDecisionFingerprint  string                                          `json:"dispatch_decision_fingerprint"`
	DispatchRequestID            string                                          `json:"dispatch_request_id"`
	DispatchRequestFingerprint   string                                          `json:"dispatch_request_fingerprint"`
	WorkloadID                   string                                          `json:"workload_id"`
	CandidateNodeIDs             []string                                        `json:"candidate_node_ids"`
	CompleteCandidateSet         bool                                            `json:"complete_candidate_set"`
	SelectedNode                 NodeConnectorPlacementSelectedNodeBinding       `json:"selected_node"`
	ExecutionTaskID              string                                          `json:"execution_task_id"`
	ExecutionRequest             NodeExecutionRequest                            `json:"execution_request"`
	ExecutionRequestFingerprint  string                                          `json:"execution_request_fingerprint"`
	BrokerStateFingerprint       string                                          `json:"broker_state_fingerprint"`
	TaskLease                    NodeExecutionTaskLease                          `json:"task_lease"`
	ExecutionHandoffRequestID    string                                          `json:"execution_handoff_request_id,omitempty"`
	Provenance                   string                                          `json:"provenance"`
	FixtureOwned                 bool                                            `json:"fixture_owned"`
	ApprovalInferred             bool                                            `json:"approval_inferred"`
	Authority                    NodeConnectorPlacementExecutionHandoffAuthority `json:"authority"`
	DecisionFingerprint          string                                          `json:"decision_fingerprint"`
}

// NodeConnectorPlacementExecutionHandoffRequest is an unconsumed request for
// one future fixture connector-session handoff. This slice does not invoke the
// connector-session seam, a connector, an executor, or any broker transition.
type NodeConnectorPlacementExecutionHandoffRequest struct {
	Schema                        string                                          `json:"schema"`
	RequestID                     string                                          `json:"request_id"`
	DecisionID                    string                                          `json:"decision_id"`
	DecisionFingerprint           string                                          `json:"decision_fingerprint"`
	DecisionReason                string                                          `json:"decision_reason"`
	DecisionIssuedAt              string                                          `json:"decision_issued_at"`
	SubmissionID                  string                                          `json:"submission_id"`
	SubmissionReplayIdentity      string                                          `json:"submission_replay_identity"`
	SubmissionFingerprint         string                                          `json:"submission_fingerprint"`
	SubmissionProvenance          string                                          `json:"submission_provenance"`
	InventorySnapshotID           string                                          `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint  string                                          `json:"inventory_snapshot_fingerprint"`
	PlacementInputID              string                                          `json:"placement_input_id"`
	PlacementInputFingerprint     string                                          `json:"placement_input_fingerprint"`
	PlacementDecisionID           string                                          `json:"placement_decision_id"`
	PlacementDecisionFingerprint  string                                          `json:"placement_decision_fingerprint"`
	PlacementRequestID            string                                          `json:"placement_request_id"`
	PlacementRequestFingerprint   string                                          `json:"placement_request_fingerprint"`
	DispatchDecisionID            string                                          `json:"dispatch_decision_id"`
	DispatchDecisionFingerprint   string                                          `json:"dispatch_decision_fingerprint"`
	DispatchRequestID             string                                          `json:"dispatch_request_id"`
	DispatchRequestFingerprint    string                                          `json:"dispatch_request_fingerprint"`
	WorkloadID                    string                                          `json:"workload_id"`
	CandidateNodeIDs              []string                                        `json:"candidate_node_ids"`
	CompleteCandidateSet          bool                                            `json:"complete_candidate_set"`
	SelectedNode                  NodeConnectorPlacementSelectedNodeBinding       `json:"selected_node"`
	ExecutionTaskID               string                                          `json:"execution_task_id"`
	ExecutionRequest              NodeExecutionRequest                            `json:"execution_request"`
	ExecutionRequestFingerprint   string                                          `json:"execution_request_fingerprint"`
	BrokerStateFingerprint        string                                          `json:"broker_state_fingerprint"`
	TaskLease                     NodeExecutionTaskLease                          `json:"task_lease"`
	HandoffScope                  string                                          `json:"handoff_scope"`
	InProcessConnectorSessionOnly bool                                            `json:"in_process_connector_session_only"`
	OneTimeHandoff                bool                                            `json:"one_time_handoff"`
	AuthorizationConsumed         bool                                            `json:"authorization_consumed"`
	ConnectorInvoked              bool                                            `json:"connector_invoked"`
	ExecutorInvoked               bool                                            `json:"executor_invoked"`
	ExecutionStarted              bool                                            `json:"execution_started"`
	EventsPublished               bool                                            `json:"events_published"`
	ReceiptPublished              bool                                            `json:"receipt_published"`
	CancellationRequested         bool                                            `json:"cancellation_requested"`
	NetworkInvoked                bool                                            `json:"network_invoked"`
	ProviderInvoked               bool                                            `json:"provider_invoked"`
	RetryRequested                bool                                            `json:"retry_requested"`
	RepairRequested               bool                                            `json:"repair_requested"`
	ServiceInvoked                bool                                            `json:"service_invoked"`
	ValidationExecuted            bool                                            `json:"validation_executed"`
	MutationApplied               bool                                            `json:"mutation_applied"`
	GitInvoked                    bool                                            `json:"git_invoked"`
	ApplyInvoked                  bool                                            `json:"apply_invoked"`
	CheckpointInvoked             bool                                            `json:"checkpoint_invoked"`
	CommitInvoked                 bool                                            `json:"commit_invoked"`
	PushInvoked                   bool                                            `json:"push_invoked"`
	PublicationInvoked            bool                                            `json:"publication_invoked"`
	CompletionClaimed             bool                                            `json:"completion_claimed"`
	LifecycleAdvanced             bool                                            `json:"lifecycle_advanced"`
	NextTaskAdvanced              bool                                            `json:"next_task_advanced"`
	Provenance                    string                                          `json:"provenance"`
	FixtureOwned                  bool                                            `json:"fixture_owned"`
	Authority                     NodeConnectorPlacementExecutionHandoffAuthority `json:"authority"`
	RequestFingerprint            string                                          `json:"request_fingerprint"`
}

type nodeConnectorPlacementExecutionHandoffInputs struct {
	submission NodeConnectorPlacementDispatchSubmission
}

type NodeConnectorPlacementExecutionHandoffs struct {
	root     string
	expected NodeConnectorPlacementExecutionHandoffExpected
	broker   *NodeExecutionFakeBroker
	inputs   nodeConnectorPlacementExecutionHandoffInputs
	decision *NodeConnectorPlacementExecutionHandoffDecision
	request  *NodeConnectorPlacementExecutionHandoffRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionHandoffs(root string, expected NodeConnectorPlacementExecutionHandoffExpected, broker *NodeExecutionFakeBroker) (*NodeConnectorPlacementExecutionHandoffs, error) {
	normalized, err := normalizeNodeConnectorPlacementExecutionHandoffExpected(expected)
	if err != nil {
		return nil, err
	}
	inputs, err := loadNodeConnectorPlacementExecutionHandoffInputs(root, normalized, broker)
	if err != nil {
		return nil, err
	}
	handoffs := &NodeConnectorPlacementExecutionHandoffs{root: root, expected: normalized, broker: broker, inputs: inputs}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionHandoffDecision(root, inputs, normalized)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionHandoffRequest(root, inputs, normalized, decision, decisionExists)
	if err != nil {
		return nil, err
	}
	if requestExists && !decisionExists {
		return nil, errors.New("placement execution handoff request exists without its exact durable decision")
	}
	if decisionExists {
		handoffs.decision = &decision
	}
	if requestExists {
		handoffs.request = &request
	}
	return handoffs, nil
}

func (handoffs *NodeConnectorPlacementExecutionHandoffs) Decide(raw []byte) (NodeConnectorPlacementExecutionHandoffDecision, *NodeConnectorPlacementExecutionHandoffRequest, error) {
	handoffs.mu.Lock()
	defer handoffs.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionHandoffMaxDecisionBytes {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionHandoffDecisionFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision fixture is not strict canonical JSON")
	}
	inputs, err := loadNodeConnectorPlacementExecutionHandoffInputs(handoffs.root, handoffs.expected, handoffs.broker)
	if err != nil || !nodeExecutionEqual(inputs, handoffs.inputs) {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision could not directly revalidate its immutable submission and broker chain")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionHandoffArtifacts(inputs, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	if handoffs.decision != nil {
		if !nodeExecutionEqual(*handoffs.decision, decision) {
			return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("changed or conflicting placement execution handoff decision replay is rejected")
		}
	} else {
		path := filepath.Join(handoffs.root, nodeConnectorPlacementExecutionHandoffDecisionName)
		if err := requireNodeConnectorPlacementExecutionHandoffArtifactAbsent(path, "decision"); err != nil {
			return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionHandoffWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision could not be published")
		}
		handoffs.decision = &decision
	}
	if request == nil {
		if handoffs.request != nil {
			return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("rejected placement execution handoff decision conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementExecutionHandoffDecision(*handoffs.decision), nil, nil
	}
	if handoffs.request != nil {
		if !nodeExecutionEqual(*handoffs.request, *request) {
			return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("changed or conflicting placement execution handoff request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementExecutionHandoffRequest(*handoffs.request)
		return cloneNodeConnectorPlacementExecutionHandoffDecision(*handoffs.decision), &cloned, nil
	}
	path := filepath.Join(handoffs.root, nodeConnectorPlacementExecutionHandoffRequestName)
	if err := requireNodeConnectorPlacementExecutionHandoffArtifactAbsent(path, "request"); err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionHandoffWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff request could not be published after the durable decision")
	}
	handoffs.request = request
	cloned := cloneNodeConnectorPlacementExecutionHandoffRequest(*request)
	return cloneNodeConnectorPlacementExecutionHandoffDecision(*handoffs.decision), &cloned, nil
}

func (handoffs *NodeConnectorPlacementExecutionHandoffs) Artifacts() (*NodeConnectorPlacementExecutionHandoffDecision, *NodeConnectorPlacementExecutionHandoffRequest) {
	handoffs.mu.Lock()
	defer handoffs.mu.Unlock()
	var decision *NodeConnectorPlacementExecutionHandoffDecision
	var request *NodeConnectorPlacementExecutionHandoffRequest
	if handoffs.decision != nil {
		value := cloneNodeConnectorPlacementExecutionHandoffDecision(*handoffs.decision)
		decision = &value
	}
	if handoffs.request != nil {
		value := cloneNodeConnectorPlacementExecutionHandoffRequest(*handoffs.request)
		request = &value
	}
	return decision, request
}

func normalizeNodeConnectorPlacementExecutionHandoffExpected(value NodeConnectorPlacementExecutionHandoffExpected) (NodeConnectorPlacementExecutionHandoffExpected, error) {
	submission, err := normalizeNodeConnectorPlacementDispatchSubmissionExpected(value.Submission)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.SubmissionFingerprint) {
		return NodeConnectorPlacementExecutionHandoffExpected{}, errors.New("placement execution handoff expected binding is invalid")
	}
	value.Submission = submission
	return value, nil
}

func loadNodeConnectorPlacementExecutionHandoffInputs(root string, expected NodeConnectorPlacementExecutionHandoffExpected, broker *NodeExecutionFakeBroker) (nodeConnectorPlacementExecutionHandoffInputs, error) {
	submissions, err := OpenNodeConnectorPlacementDispatchSubmissions(root, expected.Submission, broker)
	if err != nil {
		return nodeConnectorPlacementExecutionHandoffInputs{}, errors.New("placement execution handoff could not revalidate the complete placement, dispatch, submission, and broker chain")
	}
	submission := submissions.Artifact()
	if submission == nil || submission.SubmissionFingerprint != expected.SubmissionFingerprint || submission.Authority != (NodeConnectorPlacementDispatchSubmissionAuthority{}) || submission.ExecutorInvoked || submission.ExecutionStarted {
		return nodeConnectorPlacementExecutionHandoffInputs{}, errors.New("placement execution handoff requires one exact executorless submission and lease")
	}
	state, operation, err := revalidateNodeConnectorPlacementDispatchSubmissionBroker(broker, submissions.inputs.dispatchRequest)
	if err != nil || operation == nil || state.StateFingerprint != submission.BrokerStateFingerprint || !nodeExecutionEqual(operation.Request, submission.ExecutionRequest) || !nodeExecutionEqual(operation.Lease, submission.TaskLease) || operation.Receipt != nil || operation.Cancellation != nil || operation.CancellationAck != nil || len(operation.Events) != 0 || operation.ExecutionCount != 1 {
		return nodeConnectorPlacementExecutionHandoffInputs{}, errors.New("placement execution handoff requires the exact unchanged broker operation and task lease")
	}
	return nodeConnectorPlacementExecutionHandoffInputs{submission: *submission}, nil
}

func deriveNodeConnectorPlacementExecutionHandoffArtifacts(inputs nodeConnectorPlacementExecutionHandoffInputs, fixture NodeConnectorPlacementExecutionHandoffDecisionFixture) (NodeConnectorPlacementExecutionHandoffDecision, *NodeConnectorPlacementExecutionHandoffRequest, error) {
	submission := inputs.submission
	if fixture.Schema != NodeConnectorPlacementExecutionHandoffDecisionFixtureSchema || fixture.Provenance != nodeConnectorPlacementExecutionHandoffDecisionProvenance ||
		fixture.SubmissionID != submission.SubmissionID || fixture.SubmissionReplayIdentity != submission.ReplayIdentity || fixture.SubmissionFingerprint != submission.SubmissionFingerprint || fixture.SubmissionProvenance != submission.Provenance ||
		fixture.InventorySnapshotID != submission.InventorySnapshotID || fixture.InventorySnapshotFingerprint != submission.InventorySnapshotFingerprint || fixture.PlacementInputID != submission.PlacementInputID || fixture.PlacementInputFingerprint != submission.PlacementInputSnapshotFingerprint ||
		fixture.PlacementDecisionID != submission.PlacementDecisionID || fixture.PlacementDecisionFingerprint != submission.PlacementDecisionFingerprint || fixture.PlacementRequestID != submission.PlacementRequestID || fixture.PlacementRequestFingerprint != submission.PlacementRequestFingerprint ||
		fixture.DispatchDecisionID != submission.PlacementDispatchDecisionID || fixture.DispatchDecisionFingerprint != submission.PlacementDispatchDecisionFingerprint || fixture.DispatchRequestID != submission.PlacementDispatchRequestID || fixture.DispatchRequestFingerprint != submission.PlacementDispatchRequestFingerprint ||
		fixture.WorkloadID != submission.WorkloadID || !nodeExecutionEqual(fixture.CandidateNodeIDs, submission.CandidateNodeIDs) || !nodeExecutionEqual(fixture.SelectedNode, submission.SelectedNode) || fixture.ExecutionTaskID != submission.ExecutionTaskID ||
		!nodeExecutionEqual(fixture.ExecutionRequest, submission.ExecutionRequest) || fixture.ExecutionRequestFingerprint != submission.ExecutionRequestFingerprint || fixture.BrokerStateFingerprint != submission.BrokerStateFingerprint || !nodeExecutionEqual(fixture.TaskLease, submission.TaskLease) {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision does not exactly bind the complete submission, request, selected node, broker state, and lease")
	}
	if validateNodeExecutionTypedID("placement-execution-handoff-decision", fixture.DecisionID) != nil || validateNodeExecutionTypedID("replay", fixture.ReplayIdentity) != nil || placementExecutionHandoffIdentityCollides(fixture.DecisionID, fixture.ReplayIdentity, submission.SubmissionID, submission.ReplayIdentity) || placementExecutionHandoffIdentityCollides(fixture.ReplayIdentity, fixture.DecisionID, submission.SubmissionID, submission.ReplayIdentity) {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision or replay identity is invalid or colliding")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("placement execution handoff decision must be independently approved or rejected")
	}
	if err := validateNodeConnectorPlacementExecutionHandoffPolicy(fixture.Reason, fixture.IssuedAt, submission); err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	if fixture.Decision == "approved" {
		if validateNodeExecutionTypedID("placement-execution-handoff-request", fixture.ExecutionHandoffRequestID) != nil || placementExecutionHandoffIdentityCollides(fixture.ExecutionHandoffRequestID, fixture.DecisionID, fixture.ReplayIdentity, submission.SubmissionID, submission.ReplayIdentity) {
			return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("approved placement execution handoff decision requires one distinct request identity")
		}
	} else if fixture.ExecutionHandoffRequestID != "" {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, errors.New("rejected placement execution handoff decision cannot bind a request")
	}
	decision := NodeConnectorPlacementExecutionHandoffDecision{
		Schema: NodeConnectorPlacementExecutionHandoffDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, Decision: fixture.Decision, Reason: fixture.Reason, IssuedAt: fixture.IssuedAt,
		SubmissionID: submission.SubmissionID, SubmissionReplayIdentity: submission.ReplayIdentity, SubmissionFingerprint: submission.SubmissionFingerprint, SubmissionProvenance: submission.Provenance,
		InventorySnapshotID: submission.InventorySnapshotID, InventorySnapshotFingerprint: submission.InventorySnapshotFingerprint, PlacementInputID: submission.PlacementInputID, PlacementInputFingerprint: submission.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: submission.PlacementDecisionID, PlacementDecisionFingerprint: submission.PlacementDecisionFingerprint, PlacementRequestID: submission.PlacementRequestID, PlacementRequestFingerprint: submission.PlacementRequestFingerprint,
		DispatchDecisionID: submission.PlacementDispatchDecisionID, DispatchDecisionFingerprint: submission.PlacementDispatchDecisionFingerprint, DispatchRequestID: submission.PlacementDispatchRequestID, DispatchRequestFingerprint: submission.PlacementDispatchRequestFingerprint,
		WorkloadID: submission.WorkloadID, CandidateNodeIDs: append([]string{}, submission.CandidateNodeIDs...), CompleteCandidateSet: true, SelectedNode: submission.SelectedNode,
		ExecutionTaskID: submission.ExecutionTaskID, ExecutionRequest: cloneNodeExecutionRequest(submission.ExecutionRequest), ExecutionRequestFingerprint: submission.ExecutionRequestFingerprint,
		BrokerStateFingerprint: submission.BrokerStateFingerprint, TaskLease: submission.TaskLease, ExecutionHandoffRequestID: fixture.ExecutionHandoffRequestID,
		Provenance: nodeConnectorPlacementExecutionHandoffDecisionProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementExecutionHandoffDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	decision.DecisionFingerprint = fingerprint
	if err := validateNodeConnectorPlacementExecutionHandoffDecision(decision, inputs); err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, nil
	}
	request := NodeConnectorPlacementExecutionHandoffRequest{
		Schema: NodeConnectorPlacementExecutionHandoffRequestSchema, RequestID: fixture.ExecutionHandoffRequestID, DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint, DecisionReason: decision.Reason, DecisionIssuedAt: decision.IssuedAt,
		SubmissionID: decision.SubmissionID, SubmissionReplayIdentity: decision.SubmissionReplayIdentity, SubmissionFingerprint: decision.SubmissionFingerprint, SubmissionProvenance: decision.SubmissionProvenance,
		InventorySnapshotID: decision.InventorySnapshotID, InventorySnapshotFingerprint: decision.InventorySnapshotFingerprint, PlacementInputID: decision.PlacementInputID, PlacementInputFingerprint: decision.PlacementInputFingerprint,
		PlacementDecisionID: decision.PlacementDecisionID, PlacementDecisionFingerprint: decision.PlacementDecisionFingerprint, PlacementRequestID: decision.PlacementRequestID, PlacementRequestFingerprint: decision.PlacementRequestFingerprint,
		DispatchDecisionID: decision.DispatchDecisionID, DispatchDecisionFingerprint: decision.DispatchDecisionFingerprint, DispatchRequestID: decision.DispatchRequestID, DispatchRequestFingerprint: decision.DispatchRequestFingerprint,
		WorkloadID: decision.WorkloadID, CandidateNodeIDs: append([]string{}, decision.CandidateNodeIDs...), CompleteCandidateSet: true, SelectedNode: decision.SelectedNode,
		ExecutionTaskID: decision.ExecutionTaskID, ExecutionRequest: cloneNodeExecutionRequest(decision.ExecutionRequest), ExecutionRequestFingerprint: decision.ExecutionRequestFingerprint,
		BrokerStateFingerprint: decision.BrokerStateFingerprint, TaskLease: decision.TaskLease, HandoffScope: nodeConnectorPlacementExecutionHandoffScope, InProcessConnectorSessionOnly: true, OneTimeHandoff: true,
		Provenance: nodeConnectorPlacementExecutionHandoffRequestProvenance, FixtureOwned: true, Authority: NodeConnectorPlacementExecutionHandoffAuthority{FixtureConnectorHandoff: true},
	}
	requestFingerprint, err := nodeConnectorPlacementExecutionHandoffRequestFingerprint(request)
	if err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementExecutionHandoffRequest(request, decision, inputs); err != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, nil, err
	}
	return decision, &request, nil
}

func validateNodeConnectorPlacementExecutionHandoffPolicy(reason, issuedAt string, submission NodeConnectorPlacementDispatchSubmission) error {
	if reason != strings.TrimSpace(reason) || reason == "" || len(reason) > nodeConnectorPlacementExecutionHandoffMaxReasonBytes || !utf8.ValidString(reason) || strings.ContainsAny(reason, "\r\n\x00") {
		return errors.New("placement execution handoff decision reason is invalid or unbounded")
	}
	decisionTime, err := parseNodeExecutionTime(issuedAt)
	leaseIssued, leaseIssuedErr := parseNodeExecutionTime(submission.TaskLease.IssuedAt)
	leaseExpires, leaseExpiresErr := parseNodeExecutionTime(submission.TaskLease.ExpiresAt)
	if err != nil || leaseIssuedErr != nil || leaseExpiresErr != nil || decisionTime.Before(leaseIssued) || !decisionTime.Before(leaseExpires) {
		return errors.New("placement execution handoff decision issuance time is invalid or outside the exact lease bound")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionHandoffDecision(value NodeConnectorPlacementExecutionHandoffDecision, inputs nodeConnectorPlacementExecutionHandoffInputs) error {
	submission := inputs.submission
	if value.Schema != NodeConnectorPlacementExecutionHandoffDecisionSchema || value.Provenance != nodeConnectorPlacementExecutionHandoffDecisionProvenance || !value.FixtureOwned || value.ApprovalInferred || value.Authority != (NodeConnectorPlacementExecutionHandoffAuthority{}) || !value.CompleteCandidateSet ||
		value.SubmissionID != submission.SubmissionID || value.SubmissionReplayIdentity != submission.ReplayIdentity || value.SubmissionFingerprint != submission.SubmissionFingerprint || value.SubmissionProvenance != submission.Provenance ||
		value.InventorySnapshotID != submission.InventorySnapshotID || value.InventorySnapshotFingerprint != submission.InventorySnapshotFingerprint || value.PlacementInputID != submission.PlacementInputID || value.PlacementInputFingerprint != submission.PlacementInputSnapshotFingerprint ||
		value.PlacementDecisionID != submission.PlacementDecisionID || value.PlacementDecisionFingerprint != submission.PlacementDecisionFingerprint || value.PlacementRequestID != submission.PlacementRequestID || value.PlacementRequestFingerprint != submission.PlacementRequestFingerprint ||
		value.DispatchDecisionID != submission.PlacementDispatchDecisionID || value.DispatchDecisionFingerprint != submission.PlacementDispatchDecisionFingerprint || value.DispatchRequestID != submission.PlacementDispatchRequestID || value.DispatchRequestFingerprint != submission.PlacementDispatchRequestFingerprint ||
		value.WorkloadID != submission.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, submission.CandidateNodeIDs) || !nodeExecutionEqual(value.SelectedNode, submission.SelectedNode) || value.ExecutionTaskID != submission.ExecutionTaskID ||
		!nodeExecutionEqual(value.ExecutionRequest, submission.ExecutionRequest) || value.ExecutionRequestFingerprint != submission.ExecutionRequestFingerprint || value.BrokerStateFingerprint != submission.BrokerStateFingerprint || !nodeExecutionEqual(value.TaskLease, submission.TaskLease) {
		return errors.New("placement execution handoff decision contract, authority, or immutable binding is invalid")
	}
	if validateNodeExecutionTypedID("placement-execution-handoff-decision", value.DecisionID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || placementExecutionHandoffIdentityCollides(value.DecisionID, value.ReplayIdentity, submission.SubmissionID, submission.ReplayIdentity) || validateNodeConnectorPlacementExecutionHandoffPolicy(value.Reason, value.IssuedAt, submission) != nil {
		return errors.New("placement execution handoff decision identity or issuance policy is invalid")
	}
	if value.Decision == "approved" {
		if validateNodeExecutionTypedID("placement-execution-handoff-request", value.ExecutionHandoffRequestID) != nil || placementExecutionHandoffIdentityCollides(value.ExecutionHandoffRequestID, value.DecisionID, value.ReplayIdentity, submission.SubmissionID, submission.ReplayIdentity) {
			return errors.New("approved placement execution handoff decision request identity is invalid")
		}
	} else if value.Decision != "rejected" || value.ExecutionHandoffRequestID != "" {
		return errors.New("placement execution handoff decision value or request identity is invalid")
	}
	fingerprint, err := nodeConnectorPlacementExecutionHandoffDecisionFingerprint(value)
	if err != nil || fingerprint != value.DecisionFingerprint {
		return errors.New("placement execution handoff decision fingerprint is invalid")
	}
	return validateNodeConnectorPlacementExecutionHandoffEncodedBound(value)
}

func validateNodeConnectorPlacementExecutionHandoffRequest(value NodeConnectorPlacementExecutionHandoffRequest, decision NodeConnectorPlacementExecutionHandoffDecision, inputs nodeConnectorPlacementExecutionHandoffInputs) error {
	wantAuthority := NodeConnectorPlacementExecutionHandoffAuthority{FixtureConnectorHandoff: true}
	if decision.Decision != "approved" || value.Schema != NodeConnectorPlacementExecutionHandoffRequestSchema || value.RequestID != decision.ExecutionHandoffRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.DecisionReason != decision.Reason || value.DecisionIssuedAt != decision.IssuedAt ||
		value.SubmissionID != decision.SubmissionID || value.SubmissionReplayIdentity != decision.SubmissionReplayIdentity || value.SubmissionFingerprint != decision.SubmissionFingerprint || value.SubmissionProvenance != decision.SubmissionProvenance ||
		value.InventorySnapshotID != decision.InventorySnapshotID || value.InventorySnapshotFingerprint != decision.InventorySnapshotFingerprint || value.PlacementInputID != decision.PlacementInputID || value.PlacementInputFingerprint != decision.PlacementInputFingerprint ||
		value.PlacementDecisionID != decision.PlacementDecisionID || value.PlacementDecisionFingerprint != decision.PlacementDecisionFingerprint || value.PlacementRequestID != decision.PlacementRequestID || value.PlacementRequestFingerprint != decision.PlacementRequestFingerprint ||
		value.DispatchDecisionID != decision.DispatchDecisionID || value.DispatchDecisionFingerprint != decision.DispatchDecisionFingerprint || value.DispatchRequestID != decision.DispatchRequestID || value.DispatchRequestFingerprint != decision.DispatchRequestFingerprint ||
		value.WorkloadID != decision.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, decision.CandidateNodeIDs) || !value.CompleteCandidateSet || !nodeExecutionEqual(value.SelectedNode, decision.SelectedNode) ||
		value.ExecutionTaskID != decision.ExecutionTaskID || !nodeExecutionEqual(value.ExecutionRequest, decision.ExecutionRequest) || value.ExecutionRequestFingerprint != decision.ExecutionRequestFingerprint || value.BrokerStateFingerprint != decision.BrokerStateFingerprint || !nodeExecutionEqual(value.TaskLease, decision.TaskLease) ||
		value.HandoffScope != nodeConnectorPlacementExecutionHandoffScope || !value.InProcessConnectorSessionOnly || !value.OneTimeHandoff || value.AuthorizationConsumed || value.ConnectorInvoked || value.ExecutorInvoked || value.ExecutionStarted || value.EventsPublished || value.ReceiptPublished || value.CancellationRequested || value.NetworkInvoked || value.ProviderInvoked || value.RetryRequested || value.RepairRequested || value.ServiceInvoked || value.ValidationExecuted || value.MutationApplied || value.GitInvoked || value.ApplyInvoked || value.CheckpointInvoked || value.CommitInvoked || value.PushInvoked || value.PublicationInvoked || value.CompletionClaimed || value.LifecycleAdvanced || value.NextTaskAdvanced ||
		value.Provenance != nodeConnectorPlacementExecutionHandoffRequestProvenance || !value.FixtureOwned || value.Authority != wantAuthority {
		return errors.New("placement execution handoff request contract, authority, or immutable binding is invalid")
	}
	if !nodeExecutionEqual(value.SelectedNode, inputs.submission.SelectedNode) || !nodeExecutionEqual(value.ExecutionRequest, inputs.submission.ExecutionRequest) || !nodeExecutionEqual(value.TaskLease, inputs.submission.TaskLease) || validateNodeExecutionRequest(value.ExecutionRequest) != nil || validateNodeExecutionLease(value.TaskLease) != nil {
		return errors.New("placement execution handoff request selected node, request, or lease binding is invalid")
	}
	fingerprint, err := nodeConnectorPlacementExecutionHandoffRequestFingerprint(value)
	if err != nil || fingerprint != value.RequestFingerprint {
		return errors.New("placement execution handoff request fingerprint is invalid")
	}
	return validateNodeConnectorPlacementExecutionHandoffEncodedBound(value)
}

func loadNodeConnectorPlacementExecutionHandoffDecision(root string, inputs nodeConnectorPlacementExecutionHandoffInputs, expected NodeConnectorPlacementExecutionHandoffExpected) (NodeConnectorPlacementExecutionHandoffDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionHandoffDecisionName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionHandoffDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionHandoffMaxArtifactBytes || inputs.submission.SubmissionFingerprint != expected.SubmissionFingerprint {
		return NodeConnectorPlacementExecutionHandoffDecision{}, false, errors.New("durable placement execution handoff decision cannot be read within its bound")
	}
	var decision NodeConnectorPlacementExecutionHandoffDecision
	if decodeNodeConnectorPlacementExecutionHandoffArtifact(raw, &decision) != nil || validateNodeConnectorPlacementExecutionHandoffDecision(decision, inputs) != nil {
		return NodeConnectorPlacementExecutionHandoffDecision{}, false, errors.New("durable placement execution handoff decision is malformed, noncanonical, tampered, or orphaned")
	}
	return decision, true, nil
}

func loadNodeConnectorPlacementExecutionHandoffRequest(root string, inputs nodeConnectorPlacementExecutionHandoffInputs, expected NodeConnectorPlacementExecutionHandoffExpected, decision NodeConnectorPlacementExecutionHandoffDecision, decisionExists bool) (NodeConnectorPlacementExecutionHandoffRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionHandoffRequestName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionHandoffRequest{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionHandoffMaxArtifactBytes || !decisionExists || decision.Decision != "approved" || inputs.submission.SubmissionFingerprint != expected.SubmissionFingerprint {
		return NodeConnectorPlacementExecutionHandoffRequest{}, false, errors.New("placement execution handoff request cannot exist without its exact approved decision")
	}
	var request NodeConnectorPlacementExecutionHandoffRequest
	if decodeNodeConnectorPlacementExecutionHandoffArtifact(raw, &request) != nil || validateNodeConnectorPlacementExecutionHandoffRequest(request, decision, inputs) != nil {
		return NodeConnectorPlacementExecutionHandoffRequest{}, false, errors.New("durable placement execution handoff request is malformed, noncanonical, tampered, or orphaned")
	}
	return request, true, nil
}

func placementExecutionHandoffIdentityCollides(value string, others ...string) bool {
	for _, other := range others {
		if value == other {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionHandoffDecisionFingerprint(value NodeConnectorPlacementExecutionHandoffDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionHandoffRequestFingerprint(value NodeConnectorPlacementExecutionHandoffRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorPlacementExecutionHandoffEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementExecutionHandoffMaxArtifactBytes {
		return errors.New("placement execution handoff decision or request exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorPlacementExecutionHandoffArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("placement execution handoff decision or request is not canonical")
	}
	return nil
}

func requireNodeConnectorPlacementExecutionHandoffArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("placement execution handoff " + kind + " already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("placement execution handoff " + kind + " path cannot be inspected")
	}
	return nil
}

func cloneNodeConnectorPlacementExecutionHandoffDecision(value NodeConnectorPlacementExecutionHandoffDecision) NodeConnectorPlacementExecutionHandoffDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionHandoffDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionHandoffRequest(value NodeConnectorPlacementExecutionHandoffRequest) NodeConnectorPlacementExecutionHandoffRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionHandoffRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorPlacementExecutionHandoffDecisionFixture(value NodeConnectorPlacementExecutionHandoffDecisionFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorPlacementExecutionHandoffDecisionFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorPlacementExecutionHandoffDecisionProvenance
	}
	return json.Marshal(value)
}
