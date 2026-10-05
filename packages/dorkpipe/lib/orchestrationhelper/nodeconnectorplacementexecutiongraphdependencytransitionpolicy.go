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
	NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-dependency-transition-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-dependency-transition-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-dependency-transition-policy-request/v1"

	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionName     = "node-placement-execution-graph-dependency-transition-policy-decision.json"
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName      = "node-placement-execution-graph-dependency-transition-policy-request.json"
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteRequestAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyLocks               sync.Map
)

type NodeConnectorPlacementExecutionGraphDependencyTransitionTarget struct {
	DependencyID                string `json:"dependency_id"`
	DependencyRecordID          string `json:"dependency_record_id"`
	ExpectedPreimageFingerprint string `json:"expected_preimage_fingerprint"`
	ExpectedPreimageVersion     uint64 `json:"expected_preimage_version"`
}

// NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority grants
// exactly one future local route attempt. It never grants the transition itself
// or any scheduling, execution, external, checkout, or Git authority.
type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority struct {
	DependencyReleaseTransitionAttempt  bool `json:"dependency_release_transition_attempt"`
	FailurePropagationTransitionAttempt bool `json:"failure_propagation_transition_attempt"`
	DependencyRelease                   bool `json:"dependency_release"`
	FailurePropagation                  bool `json:"failure_propagation"`
	NextTaskScheduling                  bool `json:"next_task_scheduling"`
	NewGraphExecution                   bool `json:"new_graph_execution"`
	Retry                               bool `json:"retry"`
	Repair                              bool `json:"repair"`
	Cancellation                        bool `json:"cancellation"`
	Validation                          bool `json:"validation"`
	CheckoutMutation                    bool `json:"checkout_mutation"`
	Git                                 bool `json:"git"`
	Commit                              bool `json:"commit"`
	Push                                bool `json:"push"`
	Publication                         bool `json:"publication"`
	Broker                              bool `json:"broker"`
	Provider                            bool `json:"provider"`
	ForgePipe                           bool `json:"forgepipe"`
	Remote                              bool `json:"remote"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected struct {
	Executor                     NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected    `json:"executor"`
	AuditReceiptFingerprint      string                                                           `json:"audit_receipt_fingerprint"`
	DependencyTargets            []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget `json:"dependency_targets"`
	DecisionAuthenticationID     string                                                           `json:"decision_authentication_id"`
	DecisionAuthenticationDigest string                                                           `json:"decision_authentication_digest"`
	TransitionRequestID          string                                                           `json:"transition_request_id"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture struct {
	Schema                       string                                                                  `json:"schema"`
	DecisionID                   string                                                                  `json:"decision_id"`
	ReplayIdentity               string                                                                  `json:"replay_identity"`
	AuthenticationID             string                                                                  `json:"authentication_id"`
	AuthenticationDigest         string                                                                  `json:"authentication_digest"`
	Decision                     string                                                                  `json:"decision"`
	AuditReceiptID               string                                                                  `json:"audit_receipt_id"`
	AuditReceiptFingerprint      string                                                                  `json:"audit_receipt_fingerprint"`
	GraphStoreID                 string                                                                  `json:"graph_store_id"`
	GraphRecordID                string                                                                  `json:"graph_record_id"`
	GraphRunID                   string                                                                  `json:"graph_run_id"`
	TerminalGraphState           string                                                                  `json:"terminal_graph_state"`
	PostimageFingerprint         string                                                                  `json:"postimage_fingerprint"`
	PostimageVersion             uint64                                                                  `json:"postimage_version"`
	Route                        string                                                                  `json:"route,omitempty"`
	DependencyTargets            []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget        `json:"dependency_targets"`
	DependencyTargetsFingerprint string                                                                  `json:"dependency_targets_fingerprint"`
	TransitionRequestID          string                                                                  `json:"transition_request_id,omitempty"`
	Authority                    NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority `json:"authority"`
	Provenance                   string                                                                  `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionReconciliationBinding struct {
	GraphRunID                        string `json:"graph_run_id"`
	RunID                             string `json:"run_id"`
	TaskID                            string `json:"task_id"`
	ReconciliationRequestID           string `json:"reconciliation_request_id"`
	ReconciliationRequestFingerprint  string `json:"reconciliation_request_fingerprint"`
	ReconciliationDecisionID          string `json:"reconciliation_decision_id"`
	ReconciliationDecisionFingerprint string `json:"reconciliation_decision_fingerprint"`
	OutcomeFingerprint                string `json:"outcome_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding struct {
	AuditReceiptID                        string                                                                          `json:"audit_receipt_id"`
	AuditReceiptFingerprint               string                                                                          `json:"audit_receipt_fingerprint"`
	GraphStoreID                          string                                                                          `json:"graph_store_id"`
	GraphRecordID                         string                                                                          `json:"graph_record_id"`
	GraphRunID                            string                                                                          `json:"graph_run_id"`
	LifecyclePolicyDecisionID             string                                                                          `json:"lifecycle_policy_decision_id"`
	LifecyclePolicyDecisionFingerprint    string                                                                          `json:"lifecycle_policy_decision_fingerprint"`
	LifecycleTransitionRequestID          string                                                                          `json:"lifecycle_transition_request_id"`
	LifecycleTransitionRequestFingerprint string                                                                          `json:"lifecycle_transition_request_fingerprint"`
	ProjectionDecisionID                  string                                                                          `json:"projection_decision_id"`
	ProjectionDecisionFingerprint         string                                                                          `json:"projection_decision_fingerprint"`
	ProjectionRequestID                   string                                                                          `json:"projection_request_id"`
	ProjectionRequestFingerprint          string                                                                          `json:"projection_request_fingerprint"`
	FinalizationDecisionID                string                                                                          `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint       string                                                                          `json:"finalization_decision_fingerprint"`
	FinalizationRequestID                 string                                                                          `json:"finalization_request_id"`
	FinalizationRequestFingerprint        string                                                                          `json:"finalization_request_fingerprint"`
	ReconciliationBindings                []NodeConnectorPlacementExecutionGraphDependencyTransitionReconciliationBinding `json:"reconciliation_bindings"`
	ReconciliationBindingsFingerprint     string                                                                          `json:"reconciliation_bindings_fingerprint"`
	TaskBindings                          []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding           `json:"task_bindings"`
	TaskBindingsFingerprint               string                                                                          `json:"task_bindings_fingerprint"`
	TerminalGraphState                    string                                                                          `json:"terminal_graph_state"`
	PostimageFingerprint                  string                                                                          `json:"postimage_fingerprint"`
	PostimageVersion                      uint64                                                                          `json:"postimage_version"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision struct {
	Schema                       string                                                                  `json:"schema"`
	DecisionID                   string                                                                  `json:"decision_id"`
	ReplayIdentity               string                                                                  `json:"replay_identity"`
	AuthenticationID             string                                                                  `json:"authentication_id"`
	AuthenticationDigest         string                                                                  `json:"authentication_digest"`
	Decision                     string                                                                  `json:"decision"`
	Binding                      NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding   `json:"binding"`
	Route                        string                                                                  `json:"route,omitempty"`
	DependencyTargets            []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget        `json:"dependency_targets"`
	DependencyTargetsFingerprint string                                                                  `json:"dependency_targets_fingerprint"`
	ApprovalInferred             bool                                                                    `json:"approval_inferred"`
	IndependentlyAuthenticated   bool                                                                    `json:"independently_authenticated"`
	FixtureOwned                 bool                                                                    `json:"fixture_owned"`
	Authority                    NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority `json:"authority"`
	DecisionFingerprint          string                                                                  `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest struct {
	Schema                       string                                                                  `json:"schema"`
	RequestID                    string                                                                  `json:"request_id"`
	DecisionID                   string                                                                  `json:"decision_id"`
	DecisionFingerprint          string                                                                  `json:"decision_fingerprint"`
	AuthenticationID             string                                                                  `json:"authentication_id"`
	AuthenticationDigest         string                                                                  `json:"authentication_digest"`
	Binding                      NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding   `json:"binding"`
	Route                        string                                                                  `json:"route"`
	DependencyTargets            []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget        `json:"dependency_targets"`
	DependencyTargetsFingerprint string                                                                  `json:"dependency_targets_fingerprint"`
	OneTimeRequest               bool                                                                    `json:"one_time_request"`
	AuthorizationConsumed        bool                                                                    `json:"authorization_consumed"`
	TransitionInvoked            bool                                                                    `json:"transition_invoked"`
	CallbacksInvoked             bool                                                                    `json:"callbacks_invoked"`
	FixtureOwned                 bool                                                                    `json:"fixture_owned"`
	Authority                    NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority `json:"authority"`
	RequestFingerprint           string                                                                  `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionPolicies struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected
	receipt  NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt
	decision *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision
	request  *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(root string, expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) (*NodeConnectorPlacementExecutionGraphDependencyTransitionPolicies, error) {
	normalized, receipt, err := normalizeNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphDependencyTransitionPolicies{root: root, expected: normalized, receipt: receipt}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(root, normalized, receipt)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(root, normalized, receipt, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("graph dependency-transition policy request is orphaned or invalid")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("graph dependency-transition policy fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("graph dependency-transition policy fixture is not strict canonical JSON")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(policies.expected, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("changed or conflicting graph dependency-transition policy decision replay is rejected")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "graph dependency-transition policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("graph dependency-transition policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("rejected graph dependency-transition policy conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(*policies.decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("changed or conflicting graph dependency-transition policy request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(*policies.decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "graph dependency-transition policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("graph dependency-transition policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(*policies.decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorInputs(root, value.Executor)
	if err != nil || !inputs.receiptExists || !inputs.recordIsPost {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected{}, NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, errors.New("graph dependency-transition policy requires the exact durable executor audit receipt and persisted postimage")
	}
	value.Executor = inputs.expected
	if value.AuditReceiptFingerprint != inputs.receipt.ReceiptFingerprint {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected{}, NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, errors.New("graph dependency-transition policy audit receipt fingerprint is stale or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(value.DependencyTargets); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected{}, NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, err
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TransitionRequestID) {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected{}, NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, errors.New("graph dependency-transition policy requires exact authentication and intended request identities")
	}
	value.DependencyTargets = cloneNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(value.DependencyTargets)
	return value, inputs.receipt, nil
}

func deriveNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, fixture NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest, error) {
	targets := cloneNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(expected.DependencyTargets)
	targetFingerprint, err := nodeExecutionFingerprintValue(targets)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	binding, err := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding(expected, receipt)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	if fixture.Schema != NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || fixture.AuditReceiptID != receipt.AuditReceiptID || fixture.AuditReceiptFingerprint != receipt.ReceiptFingerprint || fixture.GraphStoreID != receipt.GraphStoreID || fixture.GraphRecordID != receipt.GraphRecordID || fixture.GraphRunID != receipt.GraphRunID || fixture.TerminalGraphState != receipt.ProjectedTerminalPostState || fixture.PostimageFingerprint != receipt.PostimageFingerprint || fixture.PostimageVersion != receipt.PostimageVersion || !nodeExecutionEqual(fixture.DependencyTargets, targets) || fixture.DependencyTargetsFingerprint != targetFingerprint || fixture.Provenance != "fixture_only_forgepipe_local_graph_dependency_transition_policy_decision" {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("graph dependency-transition policy fixture identity, authentication, target set, receipt, or postimage binding is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("graph dependency-transition policy decision is invalid")
	}
	expectedAuthority, routeValid := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRouteAuthority(receipt.ProjectedTerminalPostState, fixture.Route)
	if fixture.Decision == "rejected" {
		if fixture.Route != "" || fixture.TransitionRequestID != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("rejected graph dependency-transition policy cannot name a route, request, or authority")
		}
	} else if !routeValid || fixture.TransitionRequestID != expected.TransitionRequestID || fixture.Authority != expectedAuthority {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, errors.New("approved graph dependency-transition policy route or authority is invalid")
	}
	decision := NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{
		Schema: NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity,
		AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Binding: binding, Route: fixture.Route,
		DependencyTargets: targets, DependencyTargetsFingerprint: targetFingerprint, IndependentlyAuthenticated: true, FixtureOwned: true,
	}
	decision.DecisionFingerprint, err = nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(decision, expected, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest{
		Schema: NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestSchema, RequestID: fixture.TransitionRequestID, DecisionID: decision.DecisionID,
		DecisionFingerprint: decision.DecisionFingerprint, AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest,
		Binding: binding, Route: fixture.Route, DependencyTargets: cloneNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(targets), DependencyTargetsFingerprint: targetFingerprint,
		OneTimeRequest: true, FixtureOwned: true, Authority: expectedAuthority,
	}
	request.RequestFingerprint, err = nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(decision, expected, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(*request, expected, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt) error {
	targetFingerprint, targetErr := nodeExecutionFingerprintValue(expected.DependencyTargets)
	fingerprint, err := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFingerprint(value)
	expectedAuthority, routeValid := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRouteAuthority(receipt.ProjectedTerminalPostState, value.Route)
	if value.Decision == "rejected" {
		routeValid = value.Route == "" && value.Authority == (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{})
	} else {
		routeValid = value.Decision == "approved" && routeValid && value.Authority == (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{})
		_ = expectedAuthority
	}
	binding, bindingErr := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding(expected, receipt)
	if err != nil || targetErr != nil || bindingErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(value.Binding, binding) || !nodeExecutionEqual(value.DependencyTargets, expected.DependencyTargets) || value.DependencyTargetsFingerprint != targetFingerprint || !routeValid || value.ApprovalInferred || !value.IndependentlyAuthenticated || !value.FixtureOwned || fingerprint != value.DecisionFingerprint {
		return errors.New("graph dependency-transition policy decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest, expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, decision NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision) error {
	targetFingerprint, targetErr := nodeExecutionFingerprintValue(expected.DependencyTargets)
	expectedAuthority, routeValid := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRouteAuthority(receipt.ProjectedTerminalPostState, value.Route)
	fingerprint, err := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestFingerprint(value)
	binding, bindingErr := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding(expected, receipt)
	if err != nil || targetErr != nil || bindingErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestSchema || value.RequestID != expected.TransitionRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || !nodeExecutionEqual(value.Binding, binding) || !nodeExecutionEqual(value.DependencyTargets, expected.DependencyTargets) || value.DependencyTargetsFingerprint != targetFingerprint || !routeValid || value.Authority != expectedAuthority || !value.OneTimeRequest || value.AuthorizationConsumed || value.TransitionInvoked || value.CallbacksInvoked || !value.FixtureOwned || fingerprint != value.RequestFingerprint {
		return errors.New("graph dependency-transition policy request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionName)
	var value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyCanonicalArtifact(path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, false, errors.New("graph dependency-transition policy decision is malformed, noncanonical, oversized, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(value, expected, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, decision NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
	var value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyCanonicalArtifact(path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest{}, false, errors.New("graph dependency-transition policy request is malformed, noncanonical, oversized, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(value, expected, receipt, decision) != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest{}, false, errors.New("graph dependency-transition policy request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyCanonicalArtifact(path string, target any, allowMissing bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return err
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactMaxBytes {
		return errors.New("graph dependency-transition policy artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("graph dependency-transition policy artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("graph dependency-transition policy artifact is noncanonical")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(values []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget) error {
	if len(values) == 0 || len(values) > 256 {
		return errors.New("graph dependency-transition policy requires a bounded nonempty target set")
	}
	last := ""
	for _, value := range values {
		if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DependencyID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DependencyRecordID) || value.DependencyID == value.DependencyRecordID || !nodeExecutionFingerprint.MatchString(value.ExpectedPreimageFingerprint) || value.ExpectedPreimageVersion == 0 || last != "" && value.DependencyID <= last {
			return errors.New("graph dependency-transition targets must be exact, unique, ordinally sorted, and fingerprinted")
		}
		last = value.DependencyID
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRouteAuthority(terminalState, route string) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority, bool) {
	switch {
	case terminalState == "succeeded" && route == "dependency_release_transition":
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{DependencyReleaseTransitionAttempt: true}, true
	case terminalState == "failed" && route == "failure_propagation_transition":
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{FailurePropagationTransitionAttempt: true}, true
	default:
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{}, false
	}
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding(expected NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding, error) {
	outcomes := expected.Executor.Policy.Projection.Finalization.Outcomes
	reconciliations := make([]NodeConnectorPlacementExecutionGraphDependencyTransitionReconciliationBinding, len(outcomes))
	for index, outcome := range outcomes {
		reconciliations[index] = NodeConnectorPlacementExecutionGraphDependencyTransitionReconciliationBinding{
			GraphRunID: outcome.GraphRunID, RunID: outcome.RunID, TaskID: outcome.TaskID,
			ReconciliationRequestID: outcome.ReconciliationRequestID, ReconciliationRequestFingerprint: outcome.ReconciliationRequestFingerprint,
			ReconciliationDecisionID: outcome.ReconciliationDecisionID, ReconciliationDecisionFingerprint: outcome.ReconciliationDecisionFingerprint,
			OutcomeFingerprint: outcome.ArtifactFingerprint,
		}
	}
	reconciliationFingerprint, err := nodeExecutionFingerprintValue(reconciliations)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding{}, err
	}
	return NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding{
		AuditReceiptID: receipt.AuditReceiptID, AuditReceiptFingerprint: receipt.ReceiptFingerprint, GraphStoreID: receipt.GraphStoreID, GraphRecordID: receipt.GraphRecordID, GraphRunID: receipt.GraphRunID,
		LifecyclePolicyDecisionID: receipt.PolicyDecisionID, LifecyclePolicyDecisionFingerprint: receipt.PolicyDecisionFingerprint, LifecycleTransitionRequestID: receipt.PolicyRequestID, LifecycleTransitionRequestFingerprint: receipt.PolicyRequestFingerprint,
		ProjectionDecisionID: receipt.ProjectionDecisionID, ProjectionDecisionFingerprint: receipt.ProjectionDecisionFingerprint, ProjectionRequestID: receipt.ProjectionRequestID, ProjectionRequestFingerprint: receipt.ProjectionRequestFingerprint,
		FinalizationDecisionID: receipt.FinalizationDecisionID, FinalizationDecisionFingerprint: receipt.FinalizationDecisionFingerprint, FinalizationRequestID: receipt.FinalizationRequestID, FinalizationRequestFingerprint: receipt.FinalizationRequestFingerprint,
		ReconciliationBindings: reconciliations, ReconciliationBindingsFingerprint: reconciliationFingerprint,
		TaskBindings: cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(receipt.TaskBindings), TaskBindingsFingerprint: receipt.TaskBindingsFingerprint,
		TerminalGraphState: receipt.ProjectedTerminalPostState, PostimageFingerprint: receipt.PostimageFingerprint, PostimageVersion: receipt.PostimageVersion,
	}, nil
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(values []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget) []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget {
	cloned := append([]NodeConnectorPlacementExecutionGraphDependencyTransitionTarget(nil), values...)
	sort.SliceStable(cloned, func(i, j int) bool { return cloned[i].DependencyID < cloned[j].DependencyID })
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision) NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
