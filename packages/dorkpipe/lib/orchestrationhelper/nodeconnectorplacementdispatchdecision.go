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
	NodeConnectorPlacementDispatchDecisionFixtureSchema = "dorkpipe.node-placement-dispatch-decision-fixture/v1"
	NodeConnectorPlacementDispatchDecisionSchema        = "dorkpipe.node-placement-dispatch-decision/v1"
	NodeConnectorPlacementDispatchRequestSchema         = "dorkpipe.node-placement-dispatch-request/v1"

	nodeConnectorPlacementDispatchDecisionProvenance = "fixture_only_local_placement_dispatch_decision"
	nodeConnectorPlacementDispatchRequestProvenance  = "fixture_only_placement_bound_dispatch_request"
	nodeConnectorPlacementDispatchSubmissionScope    = "exact_selected_machine_capability_execution_request_once"
	nodeConnectorPlacementDispatchDecisionName       = "node-placement-dispatch-decision.json"
	nodeConnectorPlacementDispatchRequestName        = "node-placement-dispatch-request.json"
	nodeConnectorPlacementDispatchMaxDecisionBytes   = 256 << 10
	nodeConnectorPlacementDispatchMaxArtifactBytes   = 512 << 10
)

var (
	nodeConnectorPlacementDispatchWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementDispatchWriteRequestAtomic  = writeJSONFileAtomic
)

type NodeConnectorPlacementDispatchExpected struct {
	Placement                    NodeConnectorPlacementDecisionExpected `json:"placement"`
	PlacementDecisionFingerprint string                                 `json:"placement_decision_fingerprint"`
	PlacementRequestFingerprint  string                                 `json:"placement_request_fingerprint"`
	ExecutionRequestFingerprint  string                                 `json:"execution_request_fingerprint"`
}

// NodeConnectorPlacementDispatchAuthority permits only a later, separately
// implemented one-time submission to the existing in-process fixture broker.
// It grants no live dispatch or adjacent execution/lifecycle authority.
type NodeConnectorPlacementDispatchAuthority struct {
	FixtureBrokerSubmission bool `json:"fixture_broker_submission"`
	LiveDispatch            bool `json:"live_dispatch"`
	Network                 bool `json:"network"`
	Provider                bool `json:"provider"`
	Lease                   bool `json:"lease"`
	Execution               bool `json:"execution"`
	Retry                   bool `json:"retry"`
	Repair                  bool `json:"repair"`
	Quarantine              bool `json:"quarantine"`
	Service                 bool `json:"service"`
	Mutation                bool `json:"mutation"`
	Validation              bool `json:"validation"`
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

// NodeConnectorPlacementDispatchDecisionFixture is an independent local
// decision. The placement request, inventory evidence, availability, load,
// risk, cost, ordering, recommendation, matching, provider evidence, or
// connection presence cannot create or imply this decision.
type NodeConnectorPlacementDispatchDecisionFixture struct {
	Schema                            string                                    `json:"schema"`
	DecisionID                        string                                    `json:"decision_id"`
	ReplayIdentity                    string                                    `json:"replay_identity"`
	Decision                          string                                    `json:"decision"`
	InventorySnapshotID               string                                    `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string                                    `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                  string                                    `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint string                                    `json:"placement_input_snapshot_fingerprint"`
	PlacementDecisionID               string                                    `json:"placement_decision_id"`
	PlacementDecisionFingerprint      string                                    `json:"placement_decision_fingerprint"`
	PlacementRequestID                string                                    `json:"placement_request_id"`
	PlacementRequestFingerprint       string                                    `json:"placement_request_fingerprint"`
	WorkloadID                        string                                    `json:"workload_id"`
	CandidateNodeIDs                  []string                                  `json:"candidate_node_ids"`
	SelectedNode                      NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	ExecutionTaskID                   string                                    `json:"execution_task_id"`
	ExecutionRequest                  NodeExecutionRequest                      `json:"execution_request"`
	PlacementDispatchRequestID        string                                    `json:"placement_dispatch_request_id,omitempty"`
	Provenance                        string                                    `json:"provenance"`
}

type NodeConnectorPlacementDispatchDecision struct {
	Schema                            string                                    `json:"schema"`
	DecisionID                        string                                    `json:"decision_id"`
	ReplayIdentity                    string                                    `json:"replay_identity"`
	Decision                          string                                    `json:"decision"`
	InventorySnapshotID               string                                    `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string                                    `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                  string                                    `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint string                                    `json:"placement_input_snapshot_fingerprint"`
	PlacementDecisionID               string                                    `json:"placement_decision_id"`
	PlacementDecisionFingerprint      string                                    `json:"placement_decision_fingerprint"`
	PlacementRequestID                string                                    `json:"placement_request_id"`
	PlacementRequestFingerprint       string                                    `json:"placement_request_fingerprint"`
	WorkloadID                        string                                    `json:"workload_id"`
	CandidateNodeIDs                  []string                                  `json:"candidate_node_ids"`
	CompleteCandidateSet              bool                                      `json:"complete_candidate_set"`
	SelectedNode                      NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	ExecutionTaskID                   string                                    `json:"execution_task_id"`
	ExecutionRequest                  NodeExecutionRequest                      `json:"execution_request"`
	PlacementDispatchRequestID        string                                    `json:"placement_dispatch_request_id,omitempty"`
	Provenance                        string                                    `json:"provenance"`
	FixtureOwned                      bool                                      `json:"fixture_owned"`
	DispatchInferred                  bool                                      `json:"dispatch_inferred"`
	Authority                         NodeConnectorPlacementDispatchAuthority   `json:"authority"`
	DecisionFingerprint               string                                    `json:"decision_fingerprint"`
}

// NodeConnectorPlacementDispatchRequest is an unconsumed authorization for a
// future one-time in-process fixture-broker submission. This slice does not
// submit it, create a connection, issue a lease, or execute anything.
type NodeConnectorPlacementDispatchRequest struct {
	Schema                            string                                    `json:"schema"`
	RequestID                         string                                    `json:"request_id"`
	DecisionID                        string                                    `json:"decision_id"`
	DecisionFingerprint               string                                    `json:"decision_fingerprint"`
	InventorySnapshotID               string                                    `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string                                    `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                  string                                    `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint string                                    `json:"placement_input_snapshot_fingerprint"`
	PlacementDecisionID               string                                    `json:"placement_decision_id"`
	PlacementDecisionFingerprint      string                                    `json:"placement_decision_fingerprint"`
	PlacementRequestID                string                                    `json:"placement_request_id"`
	PlacementRequestFingerprint       string                                    `json:"placement_request_fingerprint"`
	WorkloadID                        string                                    `json:"workload_id"`
	CandidateNodeIDs                  []string                                  `json:"candidate_node_ids"`
	CompleteCandidateSet              bool                                      `json:"complete_candidate_set"`
	SelectedNode                      NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	ExecutionTaskID                   string                                    `json:"execution_task_id"`
	ExecutionRequest                  NodeExecutionRequest                      `json:"execution_request"`
	SubmissionScope                   string                                    `json:"submission_scope"`
	InProcessFixtureBrokerOnly        bool                                      `json:"in_process_fixture_broker_only"`
	OneTimeSubmission                 bool                                      `json:"one_time_submission"`
	AuthorizationConsumed             bool                                      `json:"authorization_consumed"`
	BrokerInvoked                     bool                                      `json:"broker_invoked"`
	LeaseIssued                       bool                                      `json:"lease_issued"`
	ExecutionStarted                  bool                                      `json:"execution_started"`
	Provenance                        string                                    `json:"provenance"`
	FixtureOwned                      bool                                      `json:"fixture_owned"`
	Authority                         NodeConnectorPlacementDispatchAuthority   `json:"authority"`
	RequestFingerprint                string                                    `json:"request_fingerprint"`
}

type nodeConnectorPlacementDispatchInputs struct {
	inventory         NodeConnectorInventorySnapshot
	placement         NodeConnectorPlacementInputSnapshot
	placementDecision NodeConnectorPlacementDecision
	placementRequest  NodeConnectorPlacementRequest
}

type NodeConnectorPlacementDispatchDecisions struct {
	root     string
	expected NodeConnectorPlacementDispatchExpected
	inputs   nodeConnectorPlacementDispatchInputs
	decision *NodeConnectorPlacementDispatchDecision
	request  *NodeConnectorPlacementDispatchRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementDispatchDecisions(root string, expected NodeConnectorPlacementDispatchExpected) (*NodeConnectorPlacementDispatchDecisions, error) {
	normalized, err := normalizeNodeConnectorPlacementDispatchExpected(expected)
	if err != nil {
		return nil, err
	}
	inputs, err := loadNodeConnectorPlacementDispatchInputs(root, normalized)
	if err != nil {
		return nil, err
	}
	decisions := &NodeConnectorPlacementDispatchDecisions{root: root, expected: normalized, inputs: inputs}
	decision, decisionExists, err := loadNodeConnectorPlacementDispatchDecision(root, inputs, normalized)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementDispatchRequest(root, inputs, normalized, decision, decisionExists)
	if err != nil {
		return nil, err
	}
	if requestExists && !decisionExists {
		return nil, errors.New("placement dispatch request exists without its exact durable decision")
	}
	if decisionExists {
		decisions.decision = &decision
	}
	if requestExists {
		decisions.request = &request
	}
	return decisions, nil
}

func (decisions *NodeConnectorPlacementDispatchDecisions) Decide(raw []byte) (NodeConnectorPlacementDispatchDecision, *NodeConnectorPlacementDispatchRequest, error) {
	decisions.mu.Lock()
	defer decisions.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementDispatchMaxDecisionBytes {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementDispatchDecisionFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision fixture is not strict canonical JSON")
	}
	inputs, err := loadNodeConnectorPlacementDispatchInputs(decisions.root, decisions.expected)
	if err != nil || !nodeExecutionEqual(inputs, decisions.inputs) {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision could not directly revalidate its exact inventory and placement chain")
	}
	decision, request, err := deriveNodeConnectorPlacementDispatchArtifacts(inputs, decisions.expected, fixture)
	if err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, err
	}
	if decisions.decision != nil {
		if !nodeExecutionEqual(*decisions.decision, decision) {
			return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("changed or conflicting placement dispatch decision replay is rejected")
		}
	} else {
		path := filepath.Join(decisions.root, nodeConnectorPlacementDispatchDecisionName)
		if err := requireNodeConnectorPlacementDispatchArtifactAbsent(path, "decision"); err != nil {
			return NodeConnectorPlacementDispatchDecision{}, nil, err
		}
		if err := nodeConnectorPlacementDispatchWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision could not be published")
		}
		decisions.decision = &decision
	}
	if request == nil {
		if decisions.request != nil {
			return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("rejected placement dispatch decision conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementDispatchDecision(*decisions.decision), nil, nil
	}
	if decisions.request != nil {
		if !nodeExecutionEqual(*decisions.request, *request) {
			return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("changed or conflicting placement dispatch request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementDispatchRequest(*decisions.request)
		return cloneNodeConnectorPlacementDispatchDecision(*decisions.decision), &cloned, nil
	}
	path := filepath.Join(decisions.root, nodeConnectorPlacementDispatchRequestName)
	if err := requireNodeConnectorPlacementDispatchArtifactAbsent(path, "request"); err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, err
	}
	if err := nodeConnectorPlacementDispatchWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch request could not be published after the durable decision")
	}
	decisions.request = request
	cloned := cloneNodeConnectorPlacementDispatchRequest(*request)
	return cloneNodeConnectorPlacementDispatchDecision(*decisions.decision), &cloned, nil
}

func (decisions *NodeConnectorPlacementDispatchDecisions) Artifacts() (*NodeConnectorPlacementDispatchDecision, *NodeConnectorPlacementDispatchRequest) {
	decisions.mu.Lock()
	defer decisions.mu.Unlock()
	var decision *NodeConnectorPlacementDispatchDecision
	var request *NodeConnectorPlacementDispatchRequest
	if decisions.decision != nil {
		value := cloneNodeConnectorPlacementDispatchDecision(*decisions.decision)
		decision = &value
	}
	if decisions.request != nil {
		value := cloneNodeConnectorPlacementDispatchRequest(*decisions.request)
		request = &value
	}
	return decision, request
}

func deriveNodeConnectorPlacementDispatchArtifacts(inputs nodeConnectorPlacementDispatchInputs, expected NodeConnectorPlacementDispatchExpected, fixture NodeConnectorPlacementDispatchDecisionFixture) (NodeConnectorPlacementDispatchDecision, *NodeConnectorPlacementDispatchRequest, error) {
	placementDecision, placementRequest := inputs.placementDecision, inputs.placementRequest
	if fixture.Schema != NodeConnectorPlacementDispatchDecisionFixtureSchema || fixture.Provenance != nodeConnectorPlacementDispatchDecisionProvenance ||
		fixture.InventorySnapshotID != inputs.inventory.InventorySnapshotID || fixture.InventorySnapshotFingerprint != inputs.inventory.InventorySnapshotFingerprint ||
		fixture.PlacementInputID != inputs.placement.PlacementInputID || fixture.PlacementInputSnapshotFingerprint != inputs.placement.PlacementInputSnapshotFingerprint ||
		fixture.PlacementDecisionID != placementDecision.DecisionID || fixture.PlacementDecisionFingerprint != placementDecision.DecisionFingerprint ||
		fixture.PlacementRequestID != placementRequest.RequestID || fixture.PlacementRequestFingerprint != placementRequest.RequestFingerprint ||
		fixture.WorkloadID != inputs.placement.WorkloadID || !nodeExecutionEqual(fixture.CandidateNodeIDs, inputs.placement.CandidateNodeIDs) ||
		!nodeExecutionEqual(fixture.SelectedNode, placementRequest.SelectedNode) {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision does not exactly bind the complete approved placement chain")
	}
	if err := validateNodeExecutionRequest(fixture.ExecutionRequest); err != nil || fixture.ExecutionRequest.RequestFingerprint != expected.ExecutionRequestFingerprint {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision execution request is not the exact finalized request")
	}
	if fixture.ExecutionTaskID != fixture.ExecutionRequest.TaskID || fixture.ExecutionTaskID == fixture.WorkloadID || fixture.ExecutionRequest.CapabilitySnapshotID != fixture.SelectedNode.CapabilitySnapshotID {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision workload, execution task, or selected capability binding is invalid")
	}
	if err := validateNodeExecutionTypedID("placement-dispatch-decision", fixture.DecisionID); err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayIdentity); err != nil || placementDispatchIdentityCollides(fixture.ReplayIdentity, fixture.DecisionID, placementDecision.DecisionID, placementDecision.ReplayIdentity, placementRequest.RequestID) {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch replay identity is invalid or colliding")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("placement dispatch decision must be approved or rejected")
	}
	if fixture.Decision == "approved" {
		if validateNodeExecutionTypedID("placement-dispatch-request", fixture.PlacementDispatchRequestID) != nil || placementDispatchIdentityCollides(fixture.PlacementDispatchRequestID, fixture.DecisionID, fixture.ReplayIdentity, placementDecision.DecisionID, placementRequest.RequestID) {
			return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("approved placement dispatch decision requires one distinct request identity")
		}
	} else if fixture.PlacementDispatchRequestID != "" {
		return NodeConnectorPlacementDispatchDecision{}, nil, errors.New("rejected placement dispatch decision cannot bind a request")
	}
	decision := NodeConnectorPlacementDispatchDecision{
		Schema: NodeConnectorPlacementDispatchDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, Decision: fixture.Decision,
		InventorySnapshotID: inputs.inventory.InventorySnapshotID, InventorySnapshotFingerprint: inputs.inventory.InventorySnapshotFingerprint,
		PlacementInputID: inputs.placement.PlacementInputID, PlacementInputSnapshotFingerprint: inputs.placement.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: placementDecision.DecisionID, PlacementDecisionFingerprint: placementDecision.DecisionFingerprint,
		PlacementRequestID: placementRequest.RequestID, PlacementRequestFingerprint: placementRequest.RequestFingerprint,
		WorkloadID: inputs.placement.WorkloadID, CandidateNodeIDs: append([]string{}, inputs.placement.CandidateNodeIDs...), CompleteCandidateSet: true,
		SelectedNode: placementRequest.SelectedNode, ExecutionTaskID: fixture.ExecutionRequest.TaskID, ExecutionRequest: cloneNodeExecutionRequest(fixture.ExecutionRequest),
		PlacementDispatchRequestID: fixture.PlacementDispatchRequestID, Provenance: nodeConnectorPlacementDispatchDecisionProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementDispatchDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, err
	}
	decision.DecisionFingerprint = fingerprint
	if err := validateNodeConnectorPlacementDispatchDecision(decision, inputs, expected); err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, nil
	}
	request := NodeConnectorPlacementDispatchRequest{
		Schema: NodeConnectorPlacementDispatchRequestSchema, RequestID: fixture.PlacementDispatchRequestID, DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint,
		InventorySnapshotID: decision.InventorySnapshotID, InventorySnapshotFingerprint: decision.InventorySnapshotFingerprint,
		PlacementInputID: decision.PlacementInputID, PlacementInputSnapshotFingerprint: decision.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: decision.PlacementDecisionID, PlacementDecisionFingerprint: decision.PlacementDecisionFingerprint,
		PlacementRequestID: decision.PlacementRequestID, PlacementRequestFingerprint: decision.PlacementRequestFingerprint,
		WorkloadID: decision.WorkloadID, CandidateNodeIDs: append([]string{}, decision.CandidateNodeIDs...), CompleteCandidateSet: true,
		SelectedNode: decision.SelectedNode, ExecutionTaskID: decision.ExecutionTaskID, ExecutionRequest: cloneNodeExecutionRequest(decision.ExecutionRequest),
		SubmissionScope: nodeConnectorPlacementDispatchSubmissionScope, InProcessFixtureBrokerOnly: true, OneTimeSubmission: true,
		Provenance: nodeConnectorPlacementDispatchRequestProvenance, FixtureOwned: true,
		Authority: NodeConnectorPlacementDispatchAuthority{FixtureBrokerSubmission: true},
	}
	requestFingerprint, err := nodeConnectorPlacementDispatchRequestFingerprint(request)
	if err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementDispatchRequest(request, decision, inputs, expected); err != nil {
		return NodeConnectorPlacementDispatchDecision{}, nil, err
	}
	return decision, &request, nil
}

func normalizeNodeConnectorPlacementDispatchExpected(value NodeConnectorPlacementDispatchExpected) (NodeConnectorPlacementDispatchExpected, error) {
	placement, err := normalizeNodeConnectorPlacementDecisionExpected(value.Placement)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.PlacementDecisionFingerprint) || !nodeExecutionFingerprint.MatchString(value.PlacementRequestFingerprint) || !nodeExecutionFingerprint.MatchString(value.ExecutionRequestFingerprint) {
		return NodeConnectorPlacementDispatchExpected{}, errors.New("placement dispatch expected binding is invalid")
	}
	value.Placement = placement
	return value, nil
}

func loadNodeConnectorPlacementDispatchInputs(root string, expected NodeConnectorPlacementDispatchExpected) (nodeConnectorPlacementDispatchInputs, error) {
	placements, err := OpenNodeConnectorPlacementDecisions(root, expected.Placement)
	if err != nil {
		return nodeConnectorPlacementDispatchInputs{}, errors.New("placement dispatch could not revalidate its immutable inventory and placement chain")
	}
	decision, request := placements.Artifacts()
	if decision == nil || request == nil || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PlacementDecisionFingerprint || request.RequestFingerprint != expected.PlacementRequestFingerprint ||
		request.DecisionID != decision.DecisionID || request.DecisionFingerprint != decision.DecisionFingerprint || !nodeExecutionEqual(request.SelectedNode, *decision.SelectedNode) {
		return nodeConnectorPlacementDispatchInputs{}, errors.New("placement dispatch requires the exact approved placement decision and request")
	}
	return nodeConnectorPlacementDispatchInputs{inventory: placements.inventory, placement: placements.placement, placementDecision: *decision, placementRequest: *request}, nil
}

func validateNodeConnectorPlacementDispatchDecision(value NodeConnectorPlacementDispatchDecision, inputs nodeConnectorPlacementDispatchInputs, expected NodeConnectorPlacementDispatchExpected) error {
	placementDecision, placementRequest := inputs.placementDecision, inputs.placementRequest
	if value.Schema != NodeConnectorPlacementDispatchDecisionSchema || value.Provenance != nodeConnectorPlacementDispatchDecisionProvenance || !value.FixtureOwned || value.DispatchInferred || value.Authority != (NodeConnectorPlacementDispatchAuthority{}) || !value.CompleteCandidateSet ||
		value.InventorySnapshotID != inputs.inventory.InventorySnapshotID || value.InventorySnapshotFingerprint != inputs.inventory.InventorySnapshotFingerprint ||
		value.PlacementInputID != inputs.placement.PlacementInputID || value.PlacementInputSnapshotFingerprint != inputs.placement.PlacementInputSnapshotFingerprint ||
		value.PlacementDecisionID != placementDecision.DecisionID || value.PlacementDecisionFingerprint != expected.PlacementDecisionFingerprint ||
		value.PlacementRequestID != placementRequest.RequestID || value.PlacementRequestFingerprint != expected.PlacementRequestFingerprint ||
		value.WorkloadID != inputs.placement.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, inputs.placement.CandidateNodeIDs) || !nodeExecutionEqual(value.SelectedNode, placementRequest.SelectedNode) {
		return errors.New("placement dispatch decision contract, authority, or immutable placement binding is invalid")
	}
	if validateNodeExecutionTypedID("placement-dispatch-decision", value.DecisionID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || placementDispatchIdentityCollides(value.ReplayIdentity, value.DecisionID, placementDecision.DecisionID, placementDecision.ReplayIdentity, placementRequest.RequestID) {
		return errors.New("placement dispatch decision or replay identity is invalid")
	}
	if validateNodeExecutionRequest(value.ExecutionRequest) != nil || value.ExecutionRequest.RequestFingerprint != expected.ExecutionRequestFingerprint || value.ExecutionTaskID != value.ExecutionRequest.TaskID || value.ExecutionTaskID == value.WorkloadID || value.ExecutionRequest.CapabilitySnapshotID != value.SelectedNode.CapabilitySnapshotID {
		return errors.New("placement dispatch decision execution request binding is invalid")
	}
	if value.Decision == "approved" {
		if validateNodeExecutionTypedID("placement-dispatch-request", value.PlacementDispatchRequestID) != nil || placementDispatchIdentityCollides(value.PlacementDispatchRequestID, value.DecisionID, value.ReplayIdentity, placementDecision.DecisionID, placementRequest.RequestID) {
			return errors.New("approved placement dispatch decision request identity is invalid")
		}
	} else if value.Decision != "rejected" || value.PlacementDispatchRequestID != "" {
		return errors.New("placement dispatch decision value or request identity is invalid")
	}
	fingerprint, err := nodeConnectorPlacementDispatchDecisionFingerprint(value)
	if err != nil || fingerprint != value.DecisionFingerprint {
		return errors.New("placement dispatch decision fingerprint is invalid")
	}
	return validateNodeConnectorPlacementDispatchEncodedBound(value)
}

func validateNodeConnectorPlacementDispatchRequest(value NodeConnectorPlacementDispatchRequest, decision NodeConnectorPlacementDispatchDecision, inputs nodeConnectorPlacementDispatchInputs, expected NodeConnectorPlacementDispatchExpected) error {
	wantAuthority := NodeConnectorPlacementDispatchAuthority{FixtureBrokerSubmission: true}
	if decision.Decision != "approved" || value.Schema != NodeConnectorPlacementDispatchRequestSchema || value.RequestID != decision.PlacementDispatchRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint ||
		value.InventorySnapshotID != decision.InventorySnapshotID || value.InventorySnapshotFingerprint != decision.InventorySnapshotFingerprint || value.PlacementInputID != decision.PlacementInputID || value.PlacementInputSnapshotFingerprint != decision.PlacementInputSnapshotFingerprint ||
		value.PlacementDecisionID != decision.PlacementDecisionID || value.PlacementDecisionFingerprint != decision.PlacementDecisionFingerprint || value.PlacementRequestID != decision.PlacementRequestID || value.PlacementRequestFingerprint != decision.PlacementRequestFingerprint ||
		value.WorkloadID != decision.WorkloadID || !nodeExecutionEqual(value.CandidateNodeIDs, decision.CandidateNodeIDs) || !value.CompleteCandidateSet || !nodeExecutionEqual(value.SelectedNode, decision.SelectedNode) ||
		value.ExecutionTaskID != decision.ExecutionTaskID || !nodeExecutionEqual(value.ExecutionRequest, decision.ExecutionRequest) || value.ExecutionRequest.RequestFingerprint != expected.ExecutionRequestFingerprint ||
		value.SubmissionScope != nodeConnectorPlacementDispatchSubmissionScope || !value.InProcessFixtureBrokerOnly || !value.OneTimeSubmission || value.AuthorizationConsumed || value.BrokerInvoked || value.LeaseIssued || value.ExecutionStarted ||
		value.Provenance != nodeConnectorPlacementDispatchRequestProvenance || !value.FixtureOwned || value.Authority != wantAuthority {
		return errors.New("placement dispatch request contract, authority, or immutable binding is invalid")
	}
	if !nodeExecutionEqual(value.SelectedNode, inputs.placementRequest.SelectedNode) || value.ExecutionRequest.CapabilitySnapshotID != value.SelectedNode.CapabilitySnapshotID || value.ExecutionTaskID == value.WorkloadID || validateNodeExecutionRequest(value.ExecutionRequest) != nil {
		return errors.New("placement dispatch request selected node or execution request binding is invalid")
	}
	fingerprint, err := nodeConnectorPlacementDispatchRequestFingerprint(value)
	if err != nil || fingerprint != value.RequestFingerprint {
		return errors.New("placement dispatch request fingerprint is invalid")
	}
	return validateNodeConnectorPlacementDispatchEncodedBound(value)
}

func loadNodeConnectorPlacementDispatchDecision(root string, inputs nodeConnectorPlacementDispatchInputs, expected NodeConnectorPlacementDispatchExpected) (NodeConnectorPlacementDispatchDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementDispatchDecisionName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementDispatchDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementDispatchMaxArtifactBytes {
		return NodeConnectorPlacementDispatchDecision{}, false, errors.New("durable placement dispatch decision cannot be read within its bound")
	}
	var decision NodeConnectorPlacementDispatchDecision
	if decodeNodeConnectorPlacementDispatchArtifact(raw, &decision) != nil || validateNodeConnectorPlacementDispatchDecision(decision, inputs, expected) != nil {
		return NodeConnectorPlacementDispatchDecision{}, false, errors.New("durable placement dispatch decision is malformed, noncanonical, or tampered")
	}
	return decision, true, nil
}

func loadNodeConnectorPlacementDispatchRequest(root string, inputs nodeConnectorPlacementDispatchInputs, expected NodeConnectorPlacementDispatchExpected, decision NodeConnectorPlacementDispatchDecision, decisionExists bool) (NodeConnectorPlacementDispatchRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementDispatchRequestName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementDispatchRequest{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementDispatchMaxArtifactBytes || !decisionExists || decision.Decision != "approved" {
		return NodeConnectorPlacementDispatchRequest{}, false, errors.New("placement dispatch request cannot exist without its exact approved decision")
	}
	var request NodeConnectorPlacementDispatchRequest
	if decodeNodeConnectorPlacementDispatchArtifact(raw, &request) != nil || validateNodeConnectorPlacementDispatchRequest(request, decision, inputs, expected) != nil {
		return NodeConnectorPlacementDispatchRequest{}, false, errors.New("durable placement dispatch request is malformed, noncanonical, or tampered")
	}
	return request, true, nil
}

func placementDispatchIdentityCollides(value string, others ...string) bool {
	for _, other := range others {
		if value == other {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementDispatchDecisionFingerprint(value NodeConnectorPlacementDispatchDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementDispatchRequestFingerprint(value NodeConnectorPlacementDispatchRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorPlacementDispatchEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementDispatchMaxArtifactBytes {
		return errors.New("placement dispatch decision or request exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorPlacementDispatchArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("placement dispatch decision or request is not canonical")
	}
	return nil
}

func requireNodeConnectorPlacementDispatchArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("placement dispatch " + kind + " already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("placement dispatch " + kind + " path cannot be inspected")
	}
	return nil
}

func cloneNodeExecutionRequest(value NodeExecutionRequest) NodeExecutionRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeExecutionRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementDispatchDecision(value NodeConnectorPlacementDispatchDecision) NodeConnectorPlacementDispatchDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDispatchDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementDispatchRequest(value NodeConnectorPlacementDispatchRequest) NodeConnectorPlacementDispatchRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDispatchRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorPlacementDispatchDecisionFixture(value NodeConnectorPlacementDispatchDecisionFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorPlacementDispatchDecisionFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorPlacementDispatchDecisionProvenance
	}
	return json.Marshal(value)
}
