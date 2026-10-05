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
	NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-final-state-projection-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionSchema        = "dorkpipe.node-placement-execution-graph-final-state-projection-decision/v1"
	NodeConnectorPlacementExecutionGraphFinalStateProjectionRequestSchema         = "dorkpipe.node-placement-execution-graph-final-state-projection-request/v1"

	nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName     = "node-placement-execution-graph-final-state-projection-decision.json"
	nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName      = "node-placement-execution-graph-final-state-projection-request.json"
	nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphFinalStateProjectionArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteRequestAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphFinalStateProjectionLocks               sync.Map
)

// NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority grants
// only a future local projection request. It grants no graph lifecycle action
// to ForgePipe, a broker, a provider, or any execution boundary.
type NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority struct {
	LocalFinalStateProjection bool `json:"local_final_state_projection"`
	GraphCompletion           bool `json:"graph_completion"`
	GraphFailure              bool `json:"graph_failure"`
	DependencyRelease         bool `json:"dependency_release"`
	NextTask                  bool `json:"next_task"`
	Retry                     bool `json:"retry"`
	Repair                    bool `json:"repair"`
	Cancellation              bool `json:"cancellation"`
	Execution                 bool `json:"execution"`
	Broker                    bool `json:"broker"`
	ForgePipe                 bool `json:"forgepipe"`
	Provider                  bool `json:"provider"`
	Validation                bool `json:"validation"`
	Mutation                  bool `json:"mutation"`
	Git                       bool `json:"git"`
	Publication               bool `json:"publication"`
	Lifecycle                 bool `json:"lifecycle"`
}

type NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding struct {
	GraphRunID         string `json:"graph_run_id"`
	RunID              string `json:"run_id"`
	TaskID             string `json:"task_id"`
	OperationID        string `json:"operation_id"`
	ReceiptID          string `json:"receipt_id"`
	ReceiptFingerprint string `json:"receipt_fingerprint"`
	TaskOutcome        string `json:"task_outcome"`
	OutcomeFingerprint string `json:"outcome_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected struct {
	Finalization                    NodeConnectorPlacementExecutionGraphFinalizationExpected `json:"finalization"`
	FinalizationDecisionFingerprint string                                                   `json:"finalization_decision_fingerprint"`
	FinalizationRequestFingerprint  string                                                   `json:"finalization_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture is
// the only projection approval source. Provider-like evidence, events,
// connections, machines, capability snapshots, leases, and receipts cannot
// substitute for the accepted local graph-finalization authority.
type NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture struct {
	Schema                          string                                                                `json:"schema"`
	DecisionID                      string                                                                `json:"decision_id"`
	ReplayIdentity                  string                                                                `json:"replay_identity"`
	Decision                        string                                                                `json:"decision"`
	FinalState                      string                                                                `json:"final_state,omitempty"`
	GraphRunID                      string                                                                `json:"graph_run_id"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding `json:"task_bindings"`
	FinalizationDecisionID          string                                                                `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                `json:"finalization_request_fingerprint"`
	ProjectionRequestID             string                                                                `json:"projection_request_id,omitempty"`
	Provenance                      string                                                                `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision struct {
	Schema                          string                                                                `json:"schema"`
	DecisionID                      string                                                                `json:"decision_id"`
	ReplayIdentity                  string                                                                `json:"replay_identity"`
	Decision                        string                                                                `json:"decision"`
	FinalState                      string                                                                `json:"final_state,omitempty"`
	GraphRunID                      string                                                                `json:"graph_run_id"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding `json:"task_bindings"`
	TaskBindingsFingerprint         string                                                                `json:"task_bindings_fingerprint"`
	FinalizationDecisionID          string                                                                `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                `json:"finalization_request_fingerprint"`
	FinalizationAuthority           NodeConnectorPlacementExecutionGraphFinalizationAuthority             `json:"finalization_authority"`
	FinalizationAuthorityAccepted   bool                                                                  `json:"finalization_authority_accepted"`
	ApprovalInferred                bool                                                                  `json:"approval_inferred"`
	FixtureOwned                    bool                                                                  `json:"fixture_owned"`
	Authority                       NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority     `json:"authority"`
	DecisionFingerprint             string                                                                `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest struct {
	Schema                          string                                                                `json:"schema"`
	RequestID                       string                                                                `json:"request_id"`
	DecisionID                      string                                                                `json:"decision_id"`
	DecisionFingerprint             string                                                                `json:"decision_fingerprint"`
	FinalState                      string                                                                `json:"final_state"`
	GraphRunID                      string                                                                `json:"graph_run_id"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding `json:"task_bindings"`
	TaskBindingsFingerprint         string                                                                `json:"task_bindings_fingerprint"`
	FinalizationDecisionID          string                                                                `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                `json:"finalization_request_fingerprint"`
	FinalizationAuthority           NodeConnectorPlacementExecutionGraphFinalizationAuthority             `json:"finalization_authority"`
	OneTimeRequest                  bool                                                                  `json:"one_time_request"`
	AuthorizationConsumed           bool                                                                  `json:"authorization_consumed"`
	FixtureOwned                    bool                                                                  `json:"fixture_owned"`
	Authority                       NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority     `json:"authority"`
	RequestFingerprint              string                                                                `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphFinalStateProjections struct {
	root                 string
	expected             NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected
	finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision
	finalizationRequest  NodeConnectorPlacementExecutionGraphFinalizationRequest
	decision             *NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision
	request              *NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest
	mu                   sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphFinalStateProjections(root string, expected NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected) (*NodeConnectorPlacementExecutionGraphFinalStateProjections, error) {
	normalized, finalizationDecision, finalizationRequest, err := normalizeNodeConnectorPlacementExecutionGraphFinalStateProjectionExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphFinalStateProjections{root: root, expected: normalized, finalizationDecision: finalizationDecision, finalizationRequest: finalizationRequest}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(root, normalized, finalizationDecision, finalizationRequest)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(root, normalized, finalizationDecision, finalizationRequest, decision, decisionExists)
	if err != nil || (requestExists && !decisionExists) {
		return nil, errors.New("graph final-state projection request is orphaned or invalid")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (projections *NodeConnectorPlacementExecutionGraphFinalStateProjections) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, *NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, error) {
	projections.mu.Lock()
	defer projections.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("graph final-state projection fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("graph final-state projection fixture is not strict canonical JSON")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphFinalStateProjection(projections.expected, projections.finalizationDecision, projections.finalizationRequest, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphFinalStateProjectionLocks.LoadOrStore(projections.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if projections.decision != nil {
		if !nodeExecutionEqual(*projections.decision, decision) {
			return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("changed or conflicting graph final-state projection decision replay is rejected")
		}
	} else {
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(filepath.Join(projections.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName), "graph final-state projection decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteDecisionAtomic(filepath.Join(projections.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName), decision); err != nil {
			return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("graph final-state projection decision could not be published")
		}
		projections.decision = &decision
	}
	if request == nil {
		if projections.request != nil {
			return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("rejected graph final-state projection conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(*projections.decision), nil, nil
	}
	if projections.request != nil {
		if !nodeExecutionEqual(*projections.request, *request) {
			return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("changed or conflicting graph final-state projection request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(*projections.request)
		return cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(*projections.decision), &cloned, nil
	}
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(filepath.Join(projections.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName), "graph final-state projection request"); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteRequestAtomic(filepath.Join(projections.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName), *request); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("graph final-state projection request could not be published")
	}
	projections.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(*projections.decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphFinalStateProjectionExpected(root string, value NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected) (NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected, NodeConnectorPlacementExecutionGraphFinalizationDecision, NodeConnectorPlacementExecutionGraphFinalizationRequest, error) {
	finalization, err := normalizeNodeConnectorPlacementExecutionGraphFinalizationExpected(value.Finalization)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected{}, NodeConnectorPlacementExecutionGraphFinalizationDecision{}, NodeConnectorPlacementExecutionGraphFinalizationRequest{}, errors.New("graph final-state projection requires exact canonical terminal outcomes")
	}
	value.Finalization = finalization
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphFinalizationDecision(root, finalization)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != value.FinalizationDecisionFingerprint {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected{}, NodeConnectorPlacementExecutionGraphFinalizationDecision{}, NodeConnectorPlacementExecutionGraphFinalizationRequest{}, errors.New("graph final-state projection requires the exact accepted graph-finalization decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphFinalizationRequest(root, finalization, decision, true)
	if err != nil || !requestExists || request.AuthorizationConsumed || request.RequestFingerprint != value.FinalizationRequestFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint || request.Authority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}) {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected{}, NodeConnectorPlacementExecutionGraphFinalizationDecision{}, NodeConnectorPlacementExecutionGraphFinalizationRequest{}, errors.New("graph final-state projection requires the exact accepted unconsumed graph-finalization request")
	}
	return value, decision, request, nil
}

func deriveNodeConnectorPlacementExecutionGraphFinalStateProjection(expected NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected, finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision, finalizationRequest NodeConnectorPlacementExecutionGraphFinalizationRequest, fixture NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture) (NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, *NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, error) {
	bindings := nodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(expected.Finalization.Outcomes)
	bindingsFingerprint, err := nodeExecutionFingerprintValue(bindings)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	if fixture.Schema != NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.GraphRunID != expected.Finalization.GraphRunID || !nodeExecutionEqual(fixture.TaskBindings, bindings) || fixture.FinalizationDecisionID != finalizationDecision.DecisionID || fixture.FinalizationDecisionFingerprint != finalizationDecision.DecisionFingerprint || fixture.FinalizationRequestID != finalizationRequest.RequestID || fixture.FinalizationRequestFingerprint != finalizationRequest.RequestFingerprint || fixture.Provenance != "fixture_only_forgepipe_local_graph_final_state_projection_decision" {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("graph final-state projection fixture identity or immutable finalization binding is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("graph final-state projection decision is invalid")
	}
	if fixture.Decision == "rejected" && (fixture.FinalState != "" || fixture.ProjectionRequestID != "") {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("rejected graph final-state projection cannot name a final state or request")
	}
	if fixture.Decision == "approved" && (!nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ProjectionRequestID) || fixture.FinalState != finalizationRequest.Finalization) {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, errors.New("approved graph final-state projection must explicitly match the accepted terminal finalization")
	}
	decision := NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{
		Schema: NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, Decision: fixture.Decision, FinalState: fixture.FinalState,
		GraphRunID: expected.Finalization.GraphRunID, TaskBindings: cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(bindings), TaskBindingsFingerprint: bindingsFingerprint,
		FinalizationDecisionID: finalizationDecision.DecisionID, FinalizationDecisionFingerprint: finalizationDecision.DecisionFingerprint, FinalizationRequestID: finalizationRequest.RequestID, FinalizationRequestFingerprint: finalizationRequest.RequestFingerprint,
		FinalizationAuthority: finalizationRequest.Authority, FinalizationAuthorityAccepted: true, FixtureOwned: true,
	}
	decisionFingerprint, err := nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	decision.DecisionFingerprint = decisionFingerprint
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(decision, expected, finalizationDecision, finalizationRequest)
	}
	request := &NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{
		Schema: NodeConnectorPlacementExecutionGraphFinalStateProjectionRequestSchema, RequestID: fixture.ProjectionRequestID, DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint, FinalState: fixture.FinalState,
		GraphRunID: decision.GraphRunID, TaskBindings: cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(bindings), TaskBindingsFingerprint: bindingsFingerprint,
		FinalizationDecisionID: finalizationDecision.DecisionID, FinalizationDecisionFingerprint: finalizationDecision.DecisionFingerprint, FinalizationRequestID: finalizationRequest.RequestID, FinalizationRequestFingerprint: finalizationRequest.RequestFingerprint,
		FinalizationAuthority: finalizationRequest.Authority, OneTimeRequest: true, FixtureOwned: true, Authority: NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{LocalFinalStateProjection: true},
	}
	requestFingerprint, err := nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(decision, expected, finalizationDecision, finalizationRequest); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(*request, expected, finalizationDecision, finalizationRequest, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(value NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, expected NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected, finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision, finalizationRequest NodeConnectorPlacementExecutionGraphFinalizationRequest) error {
	bindings := nodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(expected.Finalization.Outcomes)
	bindingsFingerprint, bindingsErr := nodeExecutionFingerprintValue(bindings)
	fingerprint, err := nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFingerprint(value)
	if err != nil || bindingsErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionSchema || (value.Decision != "approved" && value.Decision != "rejected") || value.GraphRunID != expected.Finalization.GraphRunID || !nodeExecutionEqual(value.TaskBindings, bindings) || value.TaskBindingsFingerprint != bindingsFingerprint || value.FinalizationDecisionID != finalizationDecision.DecisionID || value.FinalizationDecisionFingerprint != finalizationDecision.DecisionFingerprint || value.FinalizationRequestID != finalizationRequest.RequestID || value.FinalizationRequestFingerprint != finalizationRequest.RequestFingerprint || value.FinalizationAuthority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}) || !value.FinalizationAuthorityAccepted || value.ApprovalInferred || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{}) || fingerprint != value.DecisionFingerprint {
		return errors.New("graph final-state projection decision is invalid or escalates authority")
	}
	if (value.Decision == "approved" && value.FinalState != finalizationRequest.Finalization) || (value.Decision == "rejected" && value.FinalState != "") {
		return errors.New("graph final-state projection decision does not preserve the terminal finalization")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(value NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, expected NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected, finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision, finalizationRequest NodeConnectorPlacementExecutionGraphFinalizationRequest, decision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision) error {
	bindings := nodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(expected.Finalization.Outcomes)
	bindingsFingerprint, bindingsErr := nodeExecutionFingerprintValue(bindings)
	fingerprint, err := nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestFingerprint(value)
	if err != nil || bindingsErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphFinalStateProjectionRequestSchema || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.FinalState != finalizationRequest.Finalization || value.GraphRunID != expected.Finalization.GraphRunID || !nodeExecutionEqual(value.TaskBindings, bindings) || value.TaskBindingsFingerprint != bindingsFingerprint || value.FinalizationDecisionID != finalizationDecision.DecisionID || value.FinalizationDecisionFingerprint != finalizationDecision.DecisionFingerprint || value.FinalizationRequestID != finalizationRequest.RequestID || value.FinalizationRequestFingerprint != finalizationRequest.RequestFingerprint || value.FinalizationAuthority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}) || !value.OneTimeRequest || value.AuthorizationConsumed || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{LocalFinalStateProjection: true}) || fingerprint != value.RequestFingerprint {
		return errors.New("graph final-state projection request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(root string, expected NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected, finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision, finalizationRequest NodeConnectorPlacementExecutionGraphFinalizationRequest) (NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphFinalStateProjectionArtifactMaxBytes {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, false, errors.New("graph final-state projection decision cannot be read")
	}
	var value NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision
	if decodeNodeExecutionStrict(raw, &value) != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, false, errors.New("graph final-state projection decision is malformed")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(value, expected, finalizationDecision, finalizationRequest) != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, false, errors.New("graph final-state projection decision is noncanonical, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(root string, expected NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected, finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision, finalizationRequest NodeConnectorPlacementExecutionGraphFinalizationRequest, decision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, false, nil
	}
	if err != nil || !decisionExists || decision.Decision != "approved" || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphFinalStateProjectionArtifactMaxBytes {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, false, errors.New("graph final-state projection request cannot be read")
	}
	var value NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest
	if decodeNodeExecutionStrict(raw, &value) != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, false, errors.New("graph final-state projection request is malformed")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(value, expected, finalizationDecision, finalizationRequest, decision) != nil {
		return NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, false, errors.New("graph final-state projection request is noncanonical, tampered, or conflicting")
	}
	return value, true, nil
}

func nodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(outcomes []NodeConnectorPlacementExecutionGraphReconciliation) []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding {
	bindings := make([]NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding, len(outcomes))
	for i, outcome := range outcomes {
		bindings[i] = NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding{
			GraphRunID: outcome.GraphRunID, RunID: outcome.RunID, TaskID: outcome.TaskID, OperationID: outcome.OperationID,
			ReceiptID: outcome.ReceiptID, ReceiptFingerprint: outcome.ReceiptFingerprint, TaskOutcome: outcome.TaskOutcome, OutcomeFingerprint: outcome.ArtifactFingerprint,
		}
	}
	return bindings
}

func nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFingerprint(value NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestFingerprint(value NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(value []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding) []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding {
	raw, _ := json.Marshal(value)
	var cloned []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(value NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision) NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(value NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest) NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
