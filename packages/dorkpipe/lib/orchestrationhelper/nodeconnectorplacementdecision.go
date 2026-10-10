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
	NodeConnectorPlacementDecisionFixtureSchema = "dorkpipe.node-placement-decision-fixture/v1"
	NodeConnectorPlacementDecisionSchema        = "dorkpipe.node-placement-decision/v1"
	NodeConnectorPlacementRequestSchema         = "dorkpipe.node-placement-request/v1"

	nodeConnectorPlacementDecisionProvenance = "fixture_only_local_node_placement_decision"
	nodeConnectorPlacementRequestProvenance  = "fixture_only_explicit_node_placement_request"
	nodeConnectorPlacementDecisionName       = "node-placement-decision.json"
	nodeConnectorPlacementRequestName        = "node-placement-request.json"
	nodeConnectorPlacementMaxDecisionBytes   = 64 << 10
	nodeConnectorPlacementMaxArtifactBytes   = 256 << 10
)

var (
	nodeConnectorPlacementWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementWriteRequestAtomic  = writeJSONFileAtomic
)

type NodeConnectorPlacementDecisionExpected struct {
	Inventory                         NodeConnectorInventorySnapshotExpected `json:"inventory"`
	InventorySnapshotFingerprint      string                                 `json:"inventory_snapshot_fingerprint"`
	PlacementInputSnapshotFingerprint string                                 `json:"placement_input_snapshot_fingerprint"`
}

type NodeConnectorPlacementSelectedNodeBinding struct {
	NodeID                        string                              `json:"node_id"`
	MachineID                     string                              `json:"machine_id"`
	CapabilitySnapshotID          string                              `json:"capability_snapshot_id"`
	CapabilitySnapshotFingerprint string                              `json:"capability_snapshot_fingerprint"`
	Profile                       NodeConnectorInventoryTargetProfile `json:"profile"`
}

// NodeConnectorPlacementDecisionFixture is a separate local decision. The
// inventory ordering, availability, load, risk, cost, connection presence, or
// any derived score cannot create or imply this decision.
type NodeConnectorPlacementDecisionFixture struct {
	Schema                            string   `json:"schema"`
	DecisionID                        string   `json:"decision_id"`
	ReplayIdentity                    string   `json:"replay_identity"`
	Decision                          string   `json:"decision"`
	InventorySnapshotID               string   `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string   `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                  string   `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint string   `json:"placement_input_snapshot_fingerprint"`
	WorkloadID                        string   `json:"workload_id"`
	RequirementsFingerprint           string   `json:"requirements_fingerprint"`
	CandidateNodeIDs                  []string `json:"candidate_node_ids"`
	SelectedNodeID                    string   `json:"selected_node_id,omitempty"`
	PlacementRequestID                string   `json:"placement_request_id,omitempty"`
	Provenance                        string   `json:"provenance"`
}

type NodeConnectorPlacementDecision struct {
	Schema                            string                                     `json:"schema"`
	DecisionID                        string                                     `json:"decision_id"`
	ReplayIdentity                    string                                     `json:"replay_identity"`
	Decision                          string                                     `json:"decision"`
	InventorySnapshotID               string                                     `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string                                     `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                  string                                     `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint string                                     `json:"placement_input_snapshot_fingerprint"`
	WorkloadID                        string                                     `json:"workload_id"`
	RequirementsFingerprint           string                                     `json:"requirements_fingerprint"`
	CandidateNodeIDs                  []string                                   `json:"candidate_node_ids"`
	CompleteCandidateSet              bool                                       `json:"complete_candidate_set"`
	SelectedNode                      *NodeConnectorPlacementSelectedNodeBinding `json:"selected_node,omitempty"`
	PlacementRequestID                string                                     `json:"placement_request_id,omitempty"`
	Provenance                        string                                     `json:"provenance"`
	FixtureOwned                      bool                                       `json:"fixture_owned"`
	SelectionInferred                 bool                                       `json:"selection_inferred"`
	Authority                         NodeConnectorInventoryAuthority            `json:"authority"`
	DecisionFingerprint               string                                     `json:"decision_fingerprint"`
}

// NodeConnectorPlacementRequest records the one explicitly selected immutable
// node binding. It is evidence only and cannot dispatch, lease, execute, retry,
// repair, mutate, validate, publish, or advance any lifecycle.
type NodeConnectorPlacementRequest struct {
	Schema                            string                                    `json:"schema"`
	RequestID                         string                                    `json:"request_id"`
	DecisionID                        string                                    `json:"decision_id"`
	DecisionFingerprint               string                                    `json:"decision_fingerprint"`
	InventorySnapshotID               string                                    `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string                                    `json:"inventory_snapshot_fingerprint"`
	PlacementInputID                  string                                    `json:"placement_input_id"`
	PlacementInputSnapshotFingerprint string                                    `json:"placement_input_snapshot_fingerprint"`
	WorkloadID                        string                                    `json:"workload_id"`
	RequirementsFingerprint           string                                    `json:"requirements_fingerprint"`
	CandidateNodeIDs                  []string                                  `json:"candidate_node_ids"`
	CompleteCandidateSet              bool                                      `json:"complete_candidate_set"`
	SelectedNode                      NodeConnectorPlacementSelectedNodeBinding `json:"selected_node"`
	Provenance                        string                                    `json:"provenance"`
	FixtureOwned                      bool                                      `json:"fixture_owned"`
	SelectionEvidenceOnly             bool                                      `json:"selection_evidence_only"`
	PlacementDispatched               bool                                      `json:"placement_dispatched"`
	Authority                         NodeConnectorInventoryAuthority           `json:"authority"`
	RequestFingerprint                string                                    `json:"request_fingerprint"`
}

type NodeConnectorPlacementDecisions struct {
	root      string
	expected  NodeConnectorPlacementDecisionExpected
	inventory NodeConnectorInventorySnapshot
	placement NodeConnectorPlacementInputSnapshot
	decision  *NodeConnectorPlacementDecision
	request   *NodeConnectorPlacementRequest
	mu        sync.Mutex
}

func OpenNodeConnectorPlacementDecisions(root string, expected NodeConnectorPlacementDecisionExpected) (*NodeConnectorPlacementDecisions, error) {
	normalized, err := normalizeNodeConnectorPlacementDecisionExpected(expected)
	if err != nil {
		return nil, err
	}
	inventory, placement, err := loadNodeConnectorPlacementInputs(root, normalized)
	if err != nil {
		return nil, err
	}
	decisions := &NodeConnectorPlacementDecisions{root: root, expected: normalized, inventory: inventory, placement: placement}
	decision, decisionExists, err := loadNodeConnectorPlacementDecision(root, inventory, placement)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementRequest(root, inventory, placement, decision, decisionExists)
	if err != nil {
		return nil, err
	}
	if requestExists && !decisionExists {
		return nil, errors.New("node placement request exists without its exact durable decision")
	}
	if decisionExists {
		decisions.decision = &decision
	}
	if requestExists {
		decisions.request = &request
	}
	return decisions, nil
}

func (decisions *NodeConnectorPlacementDecisions) Decide(raw []byte) (NodeConnectorPlacementDecision, *NodeConnectorPlacementRequest, error) {
	decisions.mu.Lock()
	defer decisions.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementMaxDecisionBytes {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementDecisionFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision fixture is not strict canonical JSON")
	}
	inventory, placement, err := loadNodeConnectorPlacementInputs(decisions.root, decisions.expected)
	if err != nil || !nodeExecutionEqual(inventory, decisions.inventory) || !nodeExecutionEqual(placement, decisions.placement) {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision could not directly revalidate its exact inventory and placement-input snapshots")
	}
	decision, request, err := deriveNodeConnectorPlacementArtifacts(inventory, placement, fixture)
	if err != nil {
		return NodeConnectorPlacementDecision{}, nil, err
	}
	if decisions.decision != nil {
		if !nodeExecutionEqual(*decisions.decision, decision) {
			return NodeConnectorPlacementDecision{}, nil, errors.New("changed or conflicting node placement decision replay is rejected")
		}
	} else {
		path := filepath.Join(decisions.root, nodeConnectorPlacementDecisionName)
		if err := requireNodeConnectorPlacementArtifactAbsent(path, "decision"); err != nil {
			return NodeConnectorPlacementDecision{}, nil, err
		}
		if err := nodeConnectorPlacementWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision could not be published")
		}
		decisions.decision = &decision
	}
	if request == nil {
		if decisions.request != nil {
			return NodeConnectorPlacementDecision{}, nil, errors.New("rejected node placement decision conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementDecision(*decisions.decision), nil, nil
	}
	if decisions.request != nil {
		if !nodeExecutionEqual(*decisions.request, *request) {
			return NodeConnectorPlacementDecision{}, nil, errors.New("changed or conflicting node placement request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementRequest(*decisions.request)
		return cloneNodeConnectorPlacementDecision(*decisions.decision), &cloned, nil
	}
	path := filepath.Join(decisions.root, nodeConnectorPlacementRequestName)
	if err := requireNodeConnectorPlacementArtifactAbsent(path, "request"); err != nil {
		return NodeConnectorPlacementDecision{}, nil, err
	}
	if err := nodeConnectorPlacementWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement request could not be published after the durable decision")
	}
	decisions.request = request
	cloned := cloneNodeConnectorPlacementRequest(*request)
	return cloneNodeConnectorPlacementDecision(*decisions.decision), &cloned, nil
}

func (decisions *NodeConnectorPlacementDecisions) Artifacts() (*NodeConnectorPlacementDecision, *NodeConnectorPlacementRequest) {
	decisions.mu.Lock()
	defer decisions.mu.Unlock()
	var decision *NodeConnectorPlacementDecision
	var request *NodeConnectorPlacementRequest
	if decisions.decision != nil {
		value := cloneNodeConnectorPlacementDecision(*decisions.decision)
		decision = &value
	}
	if decisions.request != nil {
		value := cloneNodeConnectorPlacementRequest(*decisions.request)
		request = &value
	}
	return decision, request
}

func deriveNodeConnectorPlacementArtifacts(inventory NodeConnectorInventorySnapshot, placement NodeConnectorPlacementInputSnapshot, fixture NodeConnectorPlacementDecisionFixture) (NodeConnectorPlacementDecision, *NodeConnectorPlacementRequest, error) {
	if fixture.Schema != NodeConnectorPlacementDecisionFixtureSchema || fixture.Provenance != nodeConnectorPlacementDecisionProvenance ||
		fixture.InventorySnapshotID != inventory.InventorySnapshotID || fixture.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint ||
		fixture.PlacementInputID != placement.PlacementInputID || fixture.PlacementInputSnapshotFingerprint != placement.PlacementInputSnapshotFingerprint ||
		fixture.WorkloadID != placement.WorkloadID || fixture.RequirementsFingerprint != placement.RequirementsFingerprint ||
		!nodeExecutionEqual(fixture.CandidateNodeIDs, placement.CandidateNodeIDs) {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision does not exactly bind the inventory and complete placement-input snapshot")
	}
	if err := validateNodeExecutionTypedID("placement-decision", fixture.DecisionID); err != nil {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayIdentity); err != nil || fixture.ReplayIdentity == fixture.DecisionID || fixture.ReplayIdentity == inventory.ReplayIdentity || fixture.ReplayIdentity == placement.ReplayIdentity {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision replay identity is invalid or colliding")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementDecision{}, nil, errors.New("node placement decision must be approved or rejected")
	}
	var selected *NodeConnectorPlacementSelectedNodeBinding
	if fixture.Decision == "approved" {
		if err := validateNodeExecutionTypedID("placement-request", fixture.PlacementRequestID); err != nil || fixture.PlacementRequestID == fixture.DecisionID || fixture.PlacementRequestID == fixture.ReplayIdentity {
			return NodeConnectorPlacementDecision{}, nil, errors.New("approved node placement decision requires one distinct placement request identity")
		}
		binding, ok := findNodeConnectorPlacementSelectedNode(inventory, fixture.SelectedNodeID)
		if !ok || !nodeConnectorPlacementContainsCandidate(placement.CandidateNodeIDs, fixture.SelectedNodeID) {
			return NodeConnectorPlacementDecision{}, nil, errors.New("approved node placement decision must explicitly select exactly one node in the complete candidate set")
		}
		selected = &binding
	} else if fixture.SelectedNodeID != "" || fixture.PlacementRequestID != "" {
		return NodeConnectorPlacementDecision{}, nil, errors.New("rejected node placement decision cannot select a node or bind a request")
	}
	decision := NodeConnectorPlacementDecision{
		Schema: NodeConnectorPlacementDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity,
		Decision: fixture.Decision, InventorySnapshotID: inventory.InventorySnapshotID, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint,
		PlacementInputID: placement.PlacementInputID, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint,
		WorkloadID: placement.WorkloadID, RequirementsFingerprint: placement.RequirementsFingerprint,
		CandidateNodeIDs: append([]string{}, placement.CandidateNodeIDs...), CompleteCandidateSet: true,
		SelectedNode: selected, PlacementRequestID: fixture.PlacementRequestID,
		Provenance: nodeConnectorPlacementDecisionProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementDecision{}, nil, err
	}
	decision.DecisionFingerprint = fingerprint
	if err := validateNodeConnectorPlacementDecision(decision, inventory, placement); err != nil {
		return NodeConnectorPlacementDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, nil
	}
	request := NodeConnectorPlacementRequest{
		Schema: NodeConnectorPlacementRequestSchema, RequestID: fixture.PlacementRequestID,
		DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint,
		InventorySnapshotID: inventory.InventorySnapshotID, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint,
		PlacementInputID: placement.PlacementInputID, PlacementInputSnapshotFingerprint: placement.PlacementInputSnapshotFingerprint,
		WorkloadID: placement.WorkloadID, RequirementsFingerprint: placement.RequirementsFingerprint,
		CandidateNodeIDs: append([]string{}, placement.CandidateNodeIDs...), CompleteCandidateSet: true,
		SelectedNode: *selected, Provenance: nodeConnectorPlacementRequestProvenance, FixtureOwned: true, SelectionEvidenceOnly: true,
	}
	requestFingerprint, err := nodeConnectorPlacementRequestFingerprint(request)
	if err != nil {
		return NodeConnectorPlacementDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementRequest(request, decision, inventory, placement); err != nil {
		return NodeConnectorPlacementDecision{}, nil, err
	}
	return decision, &request, nil
}

func normalizeNodeConnectorPlacementDecisionExpected(value NodeConnectorPlacementDecisionExpected) (NodeConnectorPlacementDecisionExpected, error) {
	inventory, err := normalizeNodeConnectorInventoryExpected(value.Inventory)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.InventorySnapshotFingerprint) || !nodeExecutionFingerprint.MatchString(value.PlacementInputSnapshotFingerprint) {
		return NodeConnectorPlacementDecisionExpected{}, errors.New("node placement decision expected inventory or placement-input binding is invalid")
	}
	value.Inventory = inventory
	return value, nil
}

func loadNodeConnectorPlacementInputs(root string, expected NodeConnectorPlacementDecisionExpected) (NodeConnectorInventorySnapshot, NodeConnectorPlacementInputSnapshot, error) {
	snapshots, err := OpenNodeConnectorInventorySnapshots(root, expected.Inventory)
	if err != nil {
		return NodeConnectorInventorySnapshot{}, NodeConnectorPlacementInputSnapshot{}, errors.New("node placement decision could not revalidate its immutable inventory snapshots")
	}
	inventory, placement := snapshots.Artifacts()
	if inventory == nil || placement == nil || inventory.InventorySnapshotFingerprint != expected.InventorySnapshotFingerprint || placement.PlacementInputSnapshotFingerprint != expected.PlacementInputSnapshotFingerprint ||
		placement.InventorySnapshotID != inventory.InventorySnapshotID || placement.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint {
		return NodeConnectorInventorySnapshot{}, NodeConnectorPlacementInputSnapshot{}, errors.New("node placement decision requires the exact expected inventory and placement-input snapshots")
	}
	return *inventory, *placement, nil
}

func validateNodeConnectorPlacementDecision(value NodeConnectorPlacementDecision, inventory NodeConnectorInventorySnapshot, placement NodeConnectorPlacementInputSnapshot) error {
	if value.Schema != NodeConnectorPlacementDecisionSchema || value.Provenance != nodeConnectorPlacementDecisionProvenance || !value.FixtureOwned || value.SelectionInferred || value.Authority != (NodeConnectorInventoryAuthority{}) || !value.CompleteCandidateSet ||
		value.InventorySnapshotID != inventory.InventorySnapshotID || value.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint ||
		value.PlacementInputID != placement.PlacementInputID || value.PlacementInputSnapshotFingerprint != placement.PlacementInputSnapshotFingerprint ||
		value.WorkloadID != placement.WorkloadID || value.RequirementsFingerprint != placement.RequirementsFingerprint || !nodeExecutionEqual(value.CandidateNodeIDs, placement.CandidateNodeIDs) {
		return errors.New("node placement decision contract, authority, or immutable input binding is invalid")
	}
	if validateNodeExecutionTypedID("placement-decision", value.DecisionID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || value.ReplayIdentity == value.DecisionID || value.ReplayIdentity == inventory.ReplayIdentity || value.ReplayIdentity == placement.ReplayIdentity {
		return errors.New("node placement decision identity or replay identity is invalid")
	}
	if value.Decision == "approved" {
		if value.SelectedNode == nil || validateNodeExecutionTypedID("placement-request", value.PlacementRequestID) != nil || value.PlacementRequestID == value.DecisionID || value.PlacementRequestID == value.ReplayIdentity {
			return errors.New("approved node placement decision is missing its exact selected node or request identity")
		}
		expected, ok := findNodeConnectorPlacementSelectedNode(inventory, value.SelectedNode.NodeID)
		if !ok || !nodeExecutionEqual(*value.SelectedNode, expected) || !nodeConnectorPlacementContainsCandidate(placement.CandidateNodeIDs, value.SelectedNode.NodeID) {
			return errors.New("node placement decision selected-node binding is substituted or outside the complete candidate set")
		}
	} else if value.Decision != "rejected" || value.SelectedNode != nil || value.PlacementRequestID != "" {
		return errors.New("node placement decision value, selected node, or request identity is invalid")
	}
	expectedFingerprint, err := nodeConnectorPlacementDecisionFingerprint(value)
	if err != nil || expectedFingerprint != value.DecisionFingerprint {
		return errors.New("node placement decision fingerprint is invalid")
	}
	return validateNodeConnectorPlacementEncodedBound(value)
}

func validateNodeConnectorPlacementRequest(value NodeConnectorPlacementRequest, decision NodeConnectorPlacementDecision, inventory NodeConnectorInventorySnapshot, placement NodeConnectorPlacementInputSnapshot) error {
	if decision.Decision != "approved" || decision.SelectedNode == nil || value.Schema != NodeConnectorPlacementRequestSchema || value.RequestID != decision.PlacementRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint ||
		value.InventorySnapshotID != inventory.InventorySnapshotID || value.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint || value.PlacementInputID != placement.PlacementInputID || value.PlacementInputSnapshotFingerprint != placement.PlacementInputSnapshotFingerprint ||
		value.WorkloadID != placement.WorkloadID || value.RequirementsFingerprint != placement.RequirementsFingerprint || !nodeExecutionEqual(value.CandidateNodeIDs, placement.CandidateNodeIDs) || !value.CompleteCandidateSet ||
		!nodeExecutionEqual(value.SelectedNode, *decision.SelectedNode) || value.Provenance != nodeConnectorPlacementRequestProvenance || !value.FixtureOwned || !value.SelectionEvidenceOnly || value.PlacementDispatched || value.Authority != (NodeConnectorInventoryAuthority{}) {
		return errors.New("node placement request contract, authority, decision, or immutable input binding is invalid")
	}
	expectedSelected, ok := findNodeConnectorPlacementSelectedNode(inventory, value.SelectedNode.NodeID)
	if !ok || !nodeExecutionEqual(value.SelectedNode, expectedSelected) || !nodeConnectorPlacementContainsCandidate(placement.CandidateNodeIDs, value.SelectedNode.NodeID) {
		return errors.New("node placement request selected-node binding is substituted or outside the complete candidate set")
	}
	expectedFingerprint, err := nodeConnectorPlacementRequestFingerprint(value)
	if err != nil || expectedFingerprint != value.RequestFingerprint {
		return errors.New("node placement request fingerprint is invalid")
	}
	return validateNodeConnectorPlacementEncodedBound(value)
}

func loadNodeConnectorPlacementDecision(root string, inventory NodeConnectorInventorySnapshot, placement NodeConnectorPlacementInputSnapshot) (NodeConnectorPlacementDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementDecisionName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementMaxArtifactBytes {
		return NodeConnectorPlacementDecision{}, false, errors.New("durable node placement decision cannot be read within its bound")
	}
	var decision NodeConnectorPlacementDecision
	if decodeNodeConnectorPlacementArtifact(raw, &decision) != nil || validateNodeConnectorPlacementDecision(decision, inventory, placement) != nil {
		return NodeConnectorPlacementDecision{}, false, errors.New("durable node placement decision is malformed, noncanonical, or tampered")
	}
	return decision, true, nil
}

func loadNodeConnectorPlacementRequest(root string, inventory NodeConnectorInventorySnapshot, placement NodeConnectorPlacementInputSnapshot, decision NodeConnectorPlacementDecision, decisionExists bool) (NodeConnectorPlacementRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementRequestName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementRequest{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementMaxArtifactBytes || !decisionExists || decision.Decision != "approved" {
		return NodeConnectorPlacementRequest{}, false, errors.New("node placement request cannot exist without its exact approved decision")
	}
	var request NodeConnectorPlacementRequest
	if decodeNodeConnectorPlacementArtifact(raw, &request) != nil || validateNodeConnectorPlacementRequest(request, decision, inventory, placement) != nil {
		return NodeConnectorPlacementRequest{}, false, errors.New("durable node placement request is malformed, noncanonical, or tampered")
	}
	return request, true, nil
}

func findNodeConnectorPlacementSelectedNode(inventory NodeConnectorInventorySnapshot, nodeID string) (NodeConnectorPlacementSelectedNodeBinding, bool) {
	for _, node := range inventory.Nodes {
		if node.NodeID == nodeID {
			return NodeConnectorPlacementSelectedNodeBinding{NodeID: node.NodeID, MachineID: node.MachineID, CapabilitySnapshotID: node.CapabilitySnapshotID, CapabilitySnapshotFingerprint: node.CapabilitySnapshotFingerprint, Profile: node.Profile}, true
		}
	}
	return NodeConnectorPlacementSelectedNodeBinding{}, false
}

func nodeConnectorPlacementContainsCandidate(candidates []string, nodeID string) bool {
	for _, candidate := range candidates {
		if candidate == nodeID {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementDecisionFingerprint(value NodeConnectorPlacementDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementRequestFingerprint(value NodeConnectorPlacementRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorPlacementEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementMaxArtifactBytes {
		return errors.New("node placement decision or request exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorPlacementArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("node placement decision or request is not canonical")
	}
	return nil
}

func requireNodeConnectorPlacementArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("node placement " + kind + " already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("node placement " + kind + " path cannot be inspected")
	}
	return nil
}

func cloneNodeConnectorPlacementDecision(value NodeConnectorPlacementDecision) NodeConnectorPlacementDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementRequest(value NodeConnectorPlacementRequest) NodeConnectorPlacementRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorPlacementDecisionFixture(value NodeConnectorPlacementDecisionFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorPlacementDecisionFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorPlacementDecisionProvenance
	}
	return json.Marshal(value)
}
