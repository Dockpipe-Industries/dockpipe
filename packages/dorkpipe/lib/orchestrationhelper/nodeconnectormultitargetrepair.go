package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	NodeConnectorMultiTargetRepairDecisionFixtureSchema = "dorkpipe.multi-target-repair-decision-fixture/v1"
	NodeConnectorMultiTargetRepairDecisionSchema        = "dorkpipe.multi-target-repair-decision/v1"
	NodeConnectorMultiTargetRepairRequestSchema         = "dorkpipe.multi-target-repair-request/v1"

	nodeConnectorMultiTargetRepairDecisionProvenance = "fixture_only_local_repair_decision"
	nodeConnectorMultiTargetRepairRequestProvenance  = "fixture_only_requested_follow_up"
	nodeConnectorMultiTargetRepairDecisionName       = "multi-target-repair-decision.json"
	nodeConnectorMultiTargetRepairRequestName        = "multi-target-repair-request.json"
	nodeConnectorMultiTargetRepairMaxDecisionBytes   = 16 << 10
	nodeConnectorMultiTargetRepairMaxArtifactBytes   = 64 << 10
)

var (
	nodeConnectorMultiTargetRepairWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorMultiTargetRepairWriteRequestAtomic  = writeJSONFileAtomic
)

type NodeConnectorMultiTargetRepairExpected struct {
	Aggregate            NodeConnectorMultiTargetValidationExpected `json:"aggregate"`
	AggregateFingerprint string                                     `json:"aggregate_fingerprint"`
	FailedTargetIDs      []string                                   `json:"failed_target_ids"`
}

// NodeConnectorMultiTargetRepairDecisionFixture is a local, fixture-owned
// decision. No aggregate, provider, connection, availability, or receipt claim
// can create or imply it.
type NodeConnectorMultiTargetRepairDecisionFixture struct {
	Schema               string   `json:"schema"`
	DecisionID           string   `json:"decision_id"`
	ReplayID             string   `json:"replay_id"`
	Decision             string   `json:"decision"`
	AggregateID          string   `json:"aggregate_id"`
	AggregateFingerprint string   `json:"aggregate_fingerprint"`
	FailedTargetIDs      []string `json:"failed_target_ids"`
	RepairRequestID      string   `json:"repair_request_id,omitempty"`
	Provenance           string   `json:"provenance"`
}

type NodeConnectorMultiTargetRepairDecision struct {
	Schema               string                                      `json:"schema"`
	DecisionID           string                                      `json:"decision_id"`
	ReplayID             string                                      `json:"replay_id"`
	Decision             string                                      `json:"decision"`
	AggregateID          string                                      `json:"aggregate_id"`
	AggregateFingerprint string                                      `json:"aggregate_fingerprint"`
	FailedTargetIDs      []string                                    `json:"failed_target_ids"`
	RepairRequestID      string                                      `json:"repair_request_id,omitempty"`
	Provenance           string                                      `json:"provenance"`
	Authority            NodeConnectorMultiTargetValidationAuthority `json:"authority"`
	DecisionFingerprint  string                                      `json:"decision_fingerprint"`
}

// NodeConnectorMultiTargetRepairRequest contains only immutable bindings from
// the failed aggregate. It is evidence of requested follow-up, not instructions
// or authority to dispatch or perform a repair.
type NodeConnectorMultiTargetRepairRequest struct {
	Schema               string                                             `json:"schema"`
	RequestID            string                                             `json:"request_id"`
	DecisionID           string                                             `json:"decision_id"`
	DecisionFingerprint  string                                             `json:"decision_fingerprint"`
	AggregateID          string                                             `json:"aggregate_id"`
	AggregateFingerprint string                                             `json:"aggregate_fingerprint"`
	Targets              []NodeConnectorMultiTargetValidationReceiptBinding `json:"targets"`
	Provenance           string                                             `json:"provenance"`
	RepairDispatched     bool                                               `json:"repair_dispatched"`
	Authority            NodeConnectorMultiTargetValidationAuthority        `json:"authority"`
	RequestFingerprint   string                                             `json:"request_fingerprint"`
}

type NodeConnectorMultiTargetRepair struct {
	root      string
	expected  NodeConnectorMultiTargetRepairExpected
	aggregate NodeConnectorMultiTargetValidationAggregate
	decision  *NodeConnectorMultiTargetRepairDecision
	request   *NodeConnectorMultiTargetRepairRequest
	mu        sync.Mutex
}

func OpenNodeConnectorMultiTargetRepair(root string, expected NodeConnectorMultiTargetRepairExpected) (*NodeConnectorMultiTargetRepair, error) {
	normalized, err := normalizeNodeConnectorMultiTargetRepairExpected(expected)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("multi-target repair root must be an existing regular directory")
	}
	aggregate, err := loadNodeConnectorMultiTargetRepairAggregate(root, normalized)
	if err != nil {
		return nil, err
	}
	repair := &NodeConnectorMultiTargetRepair{root: root, expected: normalized, aggregate: aggregate}
	decision, decisionExists, err := loadNodeConnectorMultiTargetRepairDecision(root, aggregate)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorMultiTargetRepairRequest(root, aggregate, decision, decisionExists)
	if err != nil {
		return nil, err
	}
	if requestExists && !decisionExists {
		return nil, errors.New("multi-target repair request exists without its durable decision")
	}
	if decisionExists {
		repair.decision = &decision
	}
	if requestExists {
		repair.request = &request
	}
	return repair, nil
}

func (repair *NodeConnectorMultiTargetRepair) Decide(raw []byte) (NodeConnectorMultiTargetRepairDecision, *NodeConnectorMultiTargetRepairRequest, error) {
	repair.mu.Lock()
	defer repair.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorMultiTargetRepairMaxDecisionBytes {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair decision fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorMultiTargetRepairDecisionFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair decision fixture is not strict canonical JSON")
	}
	aggregate, err := loadNodeConnectorMultiTargetRepairAggregate(repair.root, repair.expected)
	if err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	decision, request, err := deriveNodeConnectorMultiTargetRepairArtifacts(aggregate, fixture)
	if err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	if repair.decision != nil {
		if !nodeExecutionEqual(*repair.decision, decision) {
			return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("changed or conflicting multi-target repair decision replay is rejected")
		}
	} else {
		path := filepath.Join(repair.root, nodeConnectorMultiTargetRepairDecisionName)
		if err := requireNodeConnectorMultiTargetRepairArtifactAbsent(path, "decision"); err != nil {
			return NodeConnectorMultiTargetRepairDecision{}, nil, err
		}
		if err := nodeConnectorMultiTargetRepairWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair decision could not be published")
		}
		repair.decision = &decision
	}
	if request == nil {
		if repair.request != nil {
			return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("rejected multi-target repair decision conflicts with a durable request")
		}
		return cloneNodeConnectorMultiTargetRepairDecision(*repair.decision), nil, nil
	}
	if repair.request != nil {
		if !nodeExecutionEqual(*repair.request, *request) {
			return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("changed or conflicting multi-target repair request replay is rejected")
		}
		cloned := cloneNodeConnectorMultiTargetRepairRequest(*repair.request)
		return cloneNodeConnectorMultiTargetRepairDecision(*repair.decision), &cloned, nil
	}
	path := filepath.Join(repair.root, nodeConnectorMultiTargetRepairRequestName)
	if err := requireNodeConnectorMultiTargetRepairArtifactAbsent(path, "request"); err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	if err := nodeConnectorMultiTargetRepairWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair request could not be published after the durable decision")
	}
	repair.request = request
	cloned := cloneNodeConnectorMultiTargetRepairRequest(*request)
	return cloneNodeConnectorMultiTargetRepairDecision(*repair.decision), &cloned, nil
}

func (repair *NodeConnectorMultiTargetRepair) Artifacts() (*NodeConnectorMultiTargetRepairDecision, *NodeConnectorMultiTargetRepairRequest) {
	repair.mu.Lock()
	defer repair.mu.Unlock()
	var decision *NodeConnectorMultiTargetRepairDecision
	var request *NodeConnectorMultiTargetRepairRequest
	if repair.decision != nil {
		cloned := cloneNodeConnectorMultiTargetRepairDecision(*repair.decision)
		decision = &cloned
	}
	if repair.request != nil {
		cloned := cloneNodeConnectorMultiTargetRepairRequest(*repair.request)
		request = &cloned
	}
	return decision, request
}

func deriveNodeConnectorMultiTargetRepairArtifacts(aggregate NodeConnectorMultiTargetValidationAggregate, fixture NodeConnectorMultiTargetRepairDecisionFixture) (NodeConnectorMultiTargetRepairDecision, *NodeConnectorMultiTargetRepairRequest, error) {
	if fixture.Schema != NodeConnectorMultiTargetRepairDecisionFixtureSchema || fixture.Provenance != nodeConnectorMultiTargetRepairDecisionProvenance ||
		fixture.AggregateID != aggregate.AggregateID || fixture.AggregateFingerprint != aggregate.AggregateFingerprint ||
		!nodeExecutionEqual(fixture.FailedTargetIDs, aggregate.FailedTargetIDs) {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair decision does not exactly bind the failed aggregate")
	}
	if err := validateNodeExecutionTypedID("decision", fixture.DecisionID); err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair decision identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayID); err != nil || fixture.ReplayID == fixture.DecisionID {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair replay identity is invalid or colliding")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("multi-target repair decision must be approved or rejected")
	}
	if fixture.Decision == "approved" {
		if err := validateNodeExecutionTypedID("repair-request", fixture.RepairRequestID); err != nil || fixture.RepairRequestID == fixture.DecisionID || fixture.RepairRequestID == fixture.ReplayID {
			return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("approved multi-target repair decision requires one distinct request identity")
		}
	} else if fixture.RepairRequestID != "" {
		return NodeConnectorMultiTargetRepairDecision{}, nil, errors.New("rejected multi-target repair decision cannot bind a request identity")
	}
	decision := NodeConnectorMultiTargetRepairDecision{
		Schema: NodeConnectorMultiTargetRepairDecisionSchema, DecisionID: fixture.DecisionID, ReplayID: fixture.ReplayID,
		Decision: fixture.Decision, AggregateID: aggregate.AggregateID, AggregateFingerprint: aggregate.AggregateFingerprint,
		FailedTargetIDs: append([]string{}, aggregate.FailedTargetIDs...), RepairRequestID: fixture.RepairRequestID,
		Provenance: nodeConnectorMultiTargetRepairDecisionProvenance,
	}
	fingerprint, err := nodeConnectorMultiTargetRepairDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	decision.DecisionFingerprint = fingerprint
	if err := validateNodeConnectorMultiTargetRepairDecision(decision, aggregate); err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, nil
	}
	bindings := make([]NodeConnectorMultiTargetValidationReceiptBinding, 0, len(aggregate.FailedTargetIDs))
	failed := make(map[string]bool, len(aggregate.FailedTargetIDs))
	for _, targetID := range aggregate.FailedTargetIDs {
		failed[targetID] = true
	}
	for _, binding := range aggregate.ReceiptBindings {
		if failed[binding.TargetID] {
			bindings = append(bindings, binding)
		}
	}
	request := NodeConnectorMultiTargetRepairRequest{
		Schema: NodeConnectorMultiTargetRepairRequestSchema, RequestID: fixture.RepairRequestID,
		DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint,
		AggregateID: aggregate.AggregateID, AggregateFingerprint: aggregate.AggregateFingerprint,
		Targets: bindings, Provenance: nodeConnectorMultiTargetRepairRequestProvenance,
	}
	requestFingerprint, err := nodeConnectorMultiTargetRepairRequestFingerprint(request)
	if err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorMultiTargetRepairRequest(request, decision, aggregate); err != nil {
		return NodeConnectorMultiTargetRepairDecision{}, nil, err
	}
	return decision, &request, nil
}

func normalizeNodeConnectorMultiTargetRepairExpected(value NodeConnectorMultiTargetRepairExpected) (NodeConnectorMultiTargetRepairExpected, error) {
	aggregate, err := normalizeNodeConnectorMultiTargetValidationExpected(value.Aggregate)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.AggregateFingerprint) {
		return NodeConnectorMultiTargetRepairExpected{}, errors.New("multi-target repair aggregate expectation is invalid")
	}
	value.Aggregate = aggregate
	value.FailedTargetIDs = append([]string{}, value.FailedTargetIDs...)
	if len(value.FailedTargetIDs) == 0 || len(value.FailedTargetIDs) > nodeConnectorMultiTargetValidationTargetCount {
		return NodeConnectorMultiTargetRepairExpected{}, errors.New("multi-target repair requires one to three failed targets")
	}
	known := map[string]bool{}
	for _, target := range aggregate.Targets {
		known[target.TargetID] = true
	}
	if !sort.StringsAreSorted(value.FailedTargetIDs) {
		return NodeConnectorMultiTargetRepairExpected{}, errors.New("multi-target repair failed targets must be ordinally sorted")
	}
	last := ""
	for _, targetID := range value.FailedTargetIDs {
		if targetID <= last || !known[targetID] {
			return NodeConnectorMultiTargetRepairExpected{}, errors.New("multi-target repair failed targets are duplicate or unknown")
		}
		last = targetID
	}
	return value, nil
}

func loadNodeConnectorMultiTargetRepairAggregate(root string, expected NodeConnectorMultiTargetRepairExpected) (NodeConnectorMultiTargetValidationAggregate, error) {
	validation, err := OpenNodeConnectorMultiTargetValidation(root, expected.Aggregate)
	if err != nil {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target repair could not revalidate the immutable aggregate")
	}
	aggregate, ok := validation.Aggregate()
	if !ok || aggregate.Status != "failed" || len(aggregate.FailedTargetIDs) == 0 ||
		aggregate.AggregateFingerprint != expected.AggregateFingerprint || !nodeExecutionEqual(aggregate.FailedTargetIDs, expected.FailedTargetIDs) {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target repair requires the exact expected failed aggregate")
	}
	return aggregate, nil
}

func validateNodeConnectorMultiTargetRepairDecision(value NodeConnectorMultiTargetRepairDecision, aggregate NodeConnectorMultiTargetValidationAggregate) error {
	if value.Schema != NodeConnectorMultiTargetRepairDecisionSchema || value.Provenance != nodeConnectorMultiTargetRepairDecisionProvenance ||
		value.Authority != (NodeConnectorMultiTargetValidationAuthority{}) || value.AggregateID != aggregate.AggregateID ||
		value.AggregateFingerprint != aggregate.AggregateFingerprint || !nodeExecutionEqual(value.FailedTargetIDs, aggregate.FailedTargetIDs) {
		return errors.New("multi-target repair decision contract or aggregate binding is invalid")
	}
	fixture := NodeConnectorMultiTargetRepairDecisionFixture{
		Schema: NodeConnectorMultiTargetRepairDecisionFixtureSchema, DecisionID: value.DecisionID, ReplayID: value.ReplayID,
		Decision: value.Decision, AggregateID: value.AggregateID, AggregateFingerprint: value.AggregateFingerprint,
		FailedTargetIDs: value.FailedTargetIDs, RepairRequestID: value.RepairRequestID, Provenance: value.Provenance,
	}
	if _, _, err := deriveNodeConnectorMultiTargetRepairDecisionIdentityOnly(aggregate, fixture); err != nil {
		return err
	}
	expected, err := nodeConnectorMultiTargetRepairDecisionFingerprint(value)
	if err != nil || expected != value.DecisionFingerprint {
		return errors.New("multi-target repair decision fingerprint is invalid")
	}
	return validateNodeConnectorMultiTargetRepairEncodedBound(value)
}

func deriveNodeConnectorMultiTargetRepairDecisionIdentityOnly(aggregate NodeConnectorMultiTargetValidationAggregate, fixture NodeConnectorMultiTargetRepairDecisionFixture) (string, string, error) {
	if fixture.AggregateID != aggregate.AggregateID || fixture.AggregateFingerprint != aggregate.AggregateFingerprint || !nodeExecutionEqual(fixture.FailedTargetIDs, aggregate.FailedTargetIDs) ||
		fixture.Schema != NodeConnectorMultiTargetRepairDecisionFixtureSchema || fixture.Provenance != nodeConnectorMultiTargetRepairDecisionProvenance {
		return "", "", errors.New("multi-target repair decision aggregate binding is invalid")
	}
	if err := validateNodeExecutionTypedID("decision", fixture.DecisionID); err != nil {
		return "", "", err
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayID); err != nil || fixture.ReplayID == fixture.DecisionID {
		return "", "", errors.New("multi-target repair decision replay identity is invalid")
	}
	if fixture.Decision == "approved" {
		if err := validateNodeExecutionTypedID("repair-request", fixture.RepairRequestID); err != nil || fixture.RepairRequestID == fixture.DecisionID || fixture.RepairRequestID == fixture.ReplayID {
			return "", "", errors.New("multi-target repair decision request identity is invalid")
		}
	} else if fixture.Decision != "rejected" || fixture.RepairRequestID != "" {
		return "", "", errors.New("multi-target repair decision value or request identity is invalid")
	}
	return fixture.DecisionID, fixture.RepairRequestID, nil
}

func validateNodeConnectorMultiTargetRepairRequest(value NodeConnectorMultiTargetRepairRequest, decision NodeConnectorMultiTargetRepairDecision, aggregate NodeConnectorMultiTargetValidationAggregate) error {
	if decision.Decision != "approved" || value.Schema != NodeConnectorMultiTargetRepairRequestSchema || value.RequestID != decision.RepairRequestID ||
		value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.AggregateID != aggregate.AggregateID ||
		value.AggregateFingerprint != aggregate.AggregateFingerprint || value.Provenance != nodeConnectorMultiTargetRepairRequestProvenance ||
		value.RepairDispatched || value.Authority != (NodeConnectorMultiTargetValidationAuthority{}) || len(value.Targets) != len(aggregate.FailedTargetIDs) {
		return errors.New("multi-target repair request contract, authority, or decision binding is invalid")
	}
	expectedBindings := make([]NodeConnectorMultiTargetValidationReceiptBinding, 0, len(aggregate.FailedTargetIDs))
	failed := make(map[string]bool, len(aggregate.FailedTargetIDs))
	for _, targetID := range aggregate.FailedTargetIDs {
		failed[targetID] = true
	}
	for _, binding := range aggregate.ReceiptBindings {
		if failed[binding.TargetID] {
			expectedBindings = append(expectedBindings, binding)
		}
	}
	if !nodeExecutionEqual(value.Targets, expectedBindings) {
		return errors.New("multi-target repair request failed-target bindings are substituted or incomplete")
	}
	expectedFingerprint, err := nodeConnectorMultiTargetRepairRequestFingerprint(value)
	if err != nil || expectedFingerprint != value.RequestFingerprint {
		return errors.New("multi-target repair request fingerprint is invalid")
	}
	return validateNodeConnectorMultiTargetRepairEncodedBound(value)
}

func loadNodeConnectorMultiTargetRepairDecision(root string, aggregate NodeConnectorMultiTargetValidationAggregate) (NodeConnectorMultiTargetRepairDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorMultiTargetRepairDecisionName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorMultiTargetRepairDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorMultiTargetRepairMaxArtifactBytes {
		return NodeConnectorMultiTargetRepairDecision{}, false, errors.New("multi-target repair decision cannot be read within its bound")
	}
	var decision NodeConnectorMultiTargetRepairDecision
	if err := decodeNodeConnectorMultiTargetRepairArtifact(raw, &decision); err != nil || validateNodeConnectorMultiTargetRepairDecision(decision, aggregate) != nil {
		return NodeConnectorMultiTargetRepairDecision{}, false, errors.New("durable multi-target repair decision is malformed, noncanonical, or tampered")
	}
	return decision, true, nil
}

func loadNodeConnectorMultiTargetRepairRequest(root string, aggregate NodeConnectorMultiTargetValidationAggregate, decision NodeConnectorMultiTargetRepairDecision, decisionExists bool) (NodeConnectorMultiTargetRepairRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorMultiTargetRepairRequestName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorMultiTargetRepairRequest{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorMultiTargetRepairMaxArtifactBytes || !decisionExists || decision.Decision != "approved" {
		return NodeConnectorMultiTargetRepairRequest{}, false, errors.New("multi-target repair request cannot exist without its exact approved decision")
	}
	var request NodeConnectorMultiTargetRepairRequest
	if err := decodeNodeConnectorMultiTargetRepairArtifact(raw, &request); err != nil || validateNodeConnectorMultiTargetRepairRequest(request, decision, aggregate) != nil {
		return NodeConnectorMultiTargetRepairRequest{}, false, errors.New("durable multi-target repair request is malformed, noncanonical, or tampered")
	}
	return request, true, nil
}

func nodeConnectorMultiTargetRepairDecisionFingerprint(value NodeConnectorMultiTargetRepairDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorMultiTargetRepairRequestFingerprint(value NodeConnectorMultiTargetRepairRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorMultiTargetRepairEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorMultiTargetRepairMaxArtifactBytes {
		return errors.New("multi-target repair artifact exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorMultiTargetRepairArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("multi-target repair artifact is not canonical")
	}
	return nil
}

func requireNodeConnectorMultiTargetRepairArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("multi-target repair " + kind + " already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("multi-target repair " + kind + " path cannot be inspected")
	}
	return nil
}

func cloneNodeConnectorMultiTargetRepairDecision(value NodeConnectorMultiTargetRepairDecision) NodeConnectorMultiTargetRepairDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorMultiTargetRepairDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorMultiTargetRepairRequest(value NodeConnectorMultiTargetRepairRequest) NodeConnectorMultiTargetRepairRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorMultiTargetRepairRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorMultiTargetRepairDecisionFixture(value NodeConnectorMultiTargetRepairDecisionFixture) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorMultiTargetRepairDecisionFixtureSchema
	}
	if value.Provenance == "" {
		value.Provenance = nodeConnectorMultiTargetRepairDecisionProvenance
	}
	return json.Marshal(value)
}
