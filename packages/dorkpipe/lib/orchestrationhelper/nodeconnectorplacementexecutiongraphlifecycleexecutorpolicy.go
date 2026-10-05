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
	NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-lifecycle-executor-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-lifecycle-executor-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-lifecycle-executor-policy-request/v1"

	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName     = "node-placement-execution-graph-lifecycle-executor-policy-decision.json"
	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName      = "node-placement-execution-graph-lifecycle-executor-policy-request.json"
	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteRequestAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyLocks               sync.Map
)

// NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority grants
// only a future local graph-state projection executor attempt. It does not
// perform or authorize any other graph, execution, ForgePipe, or Git action.
type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority struct {
	LocalGraphStateProjectionExecutorAttempt bool `json:"local_graph_state_projection_executor_attempt"`
	GraphMutation                            bool `json:"graph_mutation"`
	GraphCompletion                          bool `json:"graph_completion"`
	GraphFailure                             bool `json:"graph_failure"`
	DependencyRelease                        bool `json:"dependency_release"`
	NextTask                                 bool `json:"next_task"`
	Retry                                    bool `json:"retry"`
	Repair                                   bool `json:"repair"`
	Cancellation                             bool `json:"cancellation"`
	Execution                                bool `json:"execution"`
	Broker                                   bool `json:"broker"`
	ForgePipe                                bool `json:"forgepipe"`
	Provider                                 bool `json:"provider"`
	Validation                               bool `json:"validation"`
	Checkout                                 bool `json:"checkout"`
	Git                                      bool `json:"git"`
	Publication                              bool `json:"publication"`
	Lifecycle                                bool `json:"lifecycle"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition struct {
	GraphStoreID                string `json:"graph_store_id"`
	GraphRecordID               string `json:"graph_record_id"`
	ExpectedPreimageFingerprint string `json:"expected_preimage_fingerprint"`
	ExpectedPreimageVersion     uint64 `json:"expected_preimage_version"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequirements struct {
	CompareAndSwapRequired                bool `json:"compare_and_swap_required"`
	OneRecordAtomicityRequired            bool `json:"one_record_atomicity_required"`
	ExactReplayIdempotencyRequired        bool `json:"exact_replay_idempotency_required"`
	CrashRecoveryRequired                 bool `json:"crash_recovery_required"`
	SeparatelyDurableAuditReceiptRequired bool `json:"separately_durable_audit_receipt_required"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected struct {
	Projection                    NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected             `json:"projection"`
	ProjectionDecisionFingerprint string                                                                       `json:"projection_decision_fingerprint"`
	ProjectionRequestFingerprint  string                                                                       `json:"projection_request_fingerprint"`
	StorePrecondition             NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition `json:"store_precondition"`
}

// NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture
// is the only executor-policy approval source. The accepted projection and all
// provider-like evidence remain inputs only and cannot infer policy approval.
type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture struct {
	Schema                          string                                                                       `json:"schema"`
	DecisionID                      string                                                                       `json:"decision_id"`
	ReplayIdentity                  string                                                                       `json:"replay_identity"`
	Decision                        string                                                                       `json:"decision"`
	ProjectedTerminalPostState      string                                                                       `json:"projected_terminal_post_state"`
	StorePrecondition               NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition `json:"store_precondition"`
	GraphRunID                      string                                                                       `json:"graph_run_id"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding        `json:"task_bindings"`
	FinalizationDecisionID          string                                                                       `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                       `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                       `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                       `json:"finalization_request_fingerprint"`
	ProjectionDecisionID            string                                                                       `json:"projection_decision_id"`
	ProjectionDecisionFingerprint   string                                                                       `json:"projection_decision_fingerprint"`
	ProjectionRequestID             string                                                                       `json:"projection_request_id"`
	ProjectionRequestFingerprint    string                                                                       `json:"projection_request_fingerprint"`
	Requirements                    NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequirements      `json:"requirements"`
	ExecutorRequestID               string                                                                       `json:"executor_request_id,omitempty"`
	Provenance                      string                                                                       `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision struct {
	Schema                          string                                                                       `json:"schema"`
	DecisionID                      string                                                                       `json:"decision_id"`
	ReplayIdentity                  string                                                                       `json:"replay_identity"`
	Decision                        string                                                                       `json:"decision"`
	ProjectedTerminalPostState      string                                                                       `json:"projected_terminal_post_state"`
	StorePrecondition               NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition `json:"store_precondition"`
	StorePreconditionFingerprint    string                                                                       `json:"store_precondition_fingerprint"`
	GraphRunID                      string                                                                       `json:"graph_run_id"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding        `json:"task_bindings"`
	TaskBindingsFingerprint         string                                                                       `json:"task_bindings_fingerprint"`
	FinalizationDecisionID          string                                                                       `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                       `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                       `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                       `json:"finalization_request_fingerprint"`
	ProjectionDecisionID            string                                                                       `json:"projection_decision_id"`
	ProjectionDecisionFingerprint   string                                                                       `json:"projection_decision_fingerprint"`
	ProjectionRequestID             string                                                                       `json:"projection_request_id"`
	ProjectionRequestFingerprint    string                                                                       `json:"projection_request_fingerprint"`
	ProjectionAuthority             NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority            `json:"projection_authority"`
	ProjectionAuthorityAccepted     bool                                                                         `json:"projection_authority_accepted"`
	Requirements                    NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequirements      `json:"requirements"`
	ApprovalInferred                bool                                                                         `json:"approval_inferred"`
	FixtureOwned                    bool                                                                         `json:"fixture_owned"`
	Authority                       NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority         `json:"authority"`
	DecisionFingerprint             string                                                                       `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest struct {
	Schema                          string                                                                       `json:"schema"`
	RequestID                       string                                                                       `json:"request_id"`
	DecisionID                      string                                                                       `json:"decision_id"`
	DecisionFingerprint             string                                                                       `json:"decision_fingerprint"`
	ProjectedTerminalPostState      string                                                                       `json:"projected_terminal_post_state"`
	StorePrecondition               NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition `json:"store_precondition"`
	StorePreconditionFingerprint    string                                                                       `json:"store_precondition_fingerprint"`
	GraphRunID                      string                                                                       `json:"graph_run_id"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding        `json:"task_bindings"`
	TaskBindingsFingerprint         string                                                                       `json:"task_bindings_fingerprint"`
	FinalizationDecisionID          string                                                                       `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                       `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                       `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                       `json:"finalization_request_fingerprint"`
	ProjectionDecisionID            string                                                                       `json:"projection_decision_id"`
	ProjectionDecisionFingerprint   string                                                                       `json:"projection_decision_fingerprint"`
	ProjectionRequestID             string                                                                       `json:"projection_request_id"`
	ProjectionRequestFingerprint    string                                                                       `json:"projection_request_fingerprint"`
	ProjectionAuthority             NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority            `json:"projection_authority"`
	Requirements                    NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequirements      `json:"requirements"`
	OneTimeRequest                  bool                                                                         `json:"one_time_request"`
	AuthorizationConsumed           bool                                                                         `json:"authorization_consumed"`
	ExecutorInvoked                 bool                                                                         `json:"executor_invoked"`
	FixtureOwned                    bool                                                                         `json:"fixture_owned"`
	Authority                       NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority         `json:"authority"`
	RequestFingerprint              string                                                                       `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies struct {
	root               string
	expected           NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected
	projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision
	projectionRequest  NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest
	decision           *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision
	request            *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest
	mu                 sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(root string, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected) (*NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies, error) {
	normalized, projectionDecision, projectionRequest, err := normalizeNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies{root: root, expected: normalized, projectionDecision: projectionDecision, projectionRequest: projectionRequest}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(root, normalized, projectionDecision, projectionRequest)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(root, normalized, projectionDecision, projectionRequest, decision, decisionExists)
	if err != nil || (requestExists && !decisionExists) {
		return nil, errors.New("graph lifecycle executor policy request is orphaned or invalid")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("graph lifecycle executor policy fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("graph lifecycle executor policy fixture is not strict canonical JSON")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(policies.expected, policies.projectionDecision, policies.projectionRequest, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("changed or conflicting graph lifecycle executor policy decision replay is rejected")
		}
	} else {
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName), "graph lifecycle executor policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteDecisionAtomic(filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName), decision); err != nil {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("graph lifecycle executor policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("rejected graph lifecycle executor policy conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(*policies.decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("changed or conflicting graph lifecycle executor policy request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(*policies.decision), &cloned, nil
	}
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName), "graph lifecycle executor policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteRequestAtomic(filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName), *request); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("graph lifecycle executor policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(*policies.decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected) (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected, NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, error) {
	projection, finalizationDecision, finalizationRequest, err := normalizeNodeConnectorPlacementExecutionGraphFinalStateProjectionExpected(root, value.Projection)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, errors.New("graph lifecycle executor policy requires the exact immutable finalization chain")
	}
	value.Projection = projection
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphFinalStateProjectionDecision(root, projection, finalizationDecision, finalizationRequest)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != value.ProjectionDecisionFingerprint {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, errors.New("graph lifecycle executor policy requires the exact accepted projection decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphFinalStateProjectionRequest(root, projection, finalizationDecision, finalizationRequest, decision, true)
	if err != nil || !requestExists || request.AuthorizationConsumed || request.RequestFingerprint != value.ProjectionRequestFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint || request.Authority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{LocalFinalStateProjection: true}) {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, errors.New("graph lifecycle executor policy requires the exact accepted unconsumed projection request")
	}
	if !validNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition(value.StorePrecondition) {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision{}, NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest{}, errors.New("graph lifecycle executor policy requires one exact logical graph-store precondition")
	}
	return value, decision, request, nil
}

func deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(expected NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected, projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, projectionRequest NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, fixture NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest, error) {
	bindings := cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(projectionRequest.TaskBindings)
	bindingsFingerprint, err := nodeExecutionFingerprintValue(bindings)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	preconditionFingerprint, err := nodeExecutionFingerprintValue(expected.StorePrecondition)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	requirements := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees()
	if fixture.Schema != NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.ProjectedTerminalPostState != projectionRequest.FinalState || !nodeExecutionEqual(fixture.StorePrecondition, expected.StorePrecondition) || fixture.GraphRunID != projectionRequest.GraphRunID || !nodeExecutionEqual(fixture.TaskBindings, bindings) || fixture.FinalizationDecisionID != projectionRequest.FinalizationDecisionID || fixture.FinalizationDecisionFingerprint != projectionRequest.FinalizationDecisionFingerprint || fixture.FinalizationRequestID != projectionRequest.FinalizationRequestID || fixture.FinalizationRequestFingerprint != projectionRequest.FinalizationRequestFingerprint || fixture.ProjectionDecisionID != projectionDecision.DecisionID || fixture.ProjectionDecisionFingerprint != projectionDecision.DecisionFingerprint || fixture.ProjectionRequestID != projectionRequest.RequestID || fixture.ProjectionRequestFingerprint != projectionRequest.RequestFingerprint || !nodeExecutionEqual(fixture.Requirements, requirements) || fixture.Provenance != "fixture_only_forgepipe_local_graph_lifecycle_executor_policy_decision" {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("graph lifecycle executor policy fixture identity, store precondition, terminal state, or immutable predecessor binding is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("graph lifecycle executor policy decision is invalid")
	}
	if fixture.Decision == "rejected" && fixture.ExecutorRequestID != "" {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("rejected graph lifecycle executor policy cannot name an executor request")
	}
	if fixture.Decision == "approved" && !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ExecutorRequestID) {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, errors.New("approved graph lifecycle executor policy must explicitly name one executor request")
	}
	decision := NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, Decision: fixture.Decision,
		ProjectedTerminalPostState: projectionRequest.FinalState, StorePrecondition: expected.StorePrecondition, StorePreconditionFingerprint: preconditionFingerprint,
		GraphRunID: projectionRequest.GraphRunID, TaskBindings: bindings, TaskBindingsFingerprint: bindingsFingerprint,
		FinalizationDecisionID: projectionRequest.FinalizationDecisionID, FinalizationDecisionFingerprint: projectionRequest.FinalizationDecisionFingerprint, FinalizationRequestID: projectionRequest.FinalizationRequestID, FinalizationRequestFingerprint: projectionRequest.FinalizationRequestFingerprint,
		ProjectionDecisionID: projectionDecision.DecisionID, ProjectionDecisionFingerprint: projectionDecision.DecisionFingerprint, ProjectionRequestID: projectionRequest.RequestID, ProjectionRequestFingerprint: projectionRequest.RequestFingerprint,
		ProjectionAuthority: projectionRequest.Authority, ProjectionAuthorityAccepted: true, Requirements: requirements, FixtureOwned: true,
	}
	decisionFingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	decision.DecisionFingerprint = decisionFingerprint
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(decision, expected, projectionDecision, projectionRequest)
	}
	request := &NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestSchema, RequestID: fixture.ExecutorRequestID, DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint,
		ProjectedTerminalPostState: projectionRequest.FinalState, StorePrecondition: expected.StorePrecondition, StorePreconditionFingerprint: preconditionFingerprint,
		GraphRunID: projectionRequest.GraphRunID, TaskBindings: cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(bindings), TaskBindingsFingerprint: bindingsFingerprint,
		FinalizationDecisionID: projectionRequest.FinalizationDecisionID, FinalizationDecisionFingerprint: projectionRequest.FinalizationDecisionFingerprint, FinalizationRequestID: projectionRequest.FinalizationRequestID, FinalizationRequestFingerprint: projectionRequest.FinalizationRequestFingerprint,
		ProjectionDecisionID: projectionDecision.DecisionID, ProjectionDecisionFingerprint: projectionDecision.DecisionFingerprint, ProjectionRequestID: projectionRequest.RequestID, ProjectionRequestFingerprint: projectionRequest.RequestFingerprint,
		ProjectionAuthority: projectionRequest.Authority, Requirements: requirements, OneTimeRequest: true, FixtureOwned: true,
		Authority: NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority{LocalGraphStateProjectionExecutorAttempt: true},
	}
	requestFingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(decision, expected, projectionDecision, projectionRequest); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(*request, expected, projectionDecision, projectionRequest, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected, projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, projectionRequest NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest) error {
	bindings := projectionRequest.TaskBindings
	bindingsFingerprint, bindingsErr := nodeExecutionFingerprintValue(bindings)
	preconditionFingerprint, preconditionErr := nodeExecutionFingerprintValue(expected.StorePrecondition)
	fingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFingerprint(value)
	if err != nil || bindingsErr != nil || preconditionErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionSchema || (value.Decision != "approved" && value.Decision != "rejected") || value.ProjectedTerminalPostState != projectionRequest.FinalState || !nodeExecutionEqual(value.StorePrecondition, expected.StorePrecondition) || value.StorePreconditionFingerprint != preconditionFingerprint || value.GraphRunID != projectionRequest.GraphRunID || !nodeExecutionEqual(value.TaskBindings, bindings) || value.TaskBindingsFingerprint != bindingsFingerprint || value.FinalizationDecisionID != projectionRequest.FinalizationDecisionID || value.FinalizationDecisionFingerprint != projectionRequest.FinalizationDecisionFingerprint || value.FinalizationRequestID != projectionRequest.FinalizationRequestID || value.FinalizationRequestFingerprint != projectionRequest.FinalizationRequestFingerprint || value.ProjectionDecisionID != projectionDecision.DecisionID || value.ProjectionDecisionFingerprint != projectionDecision.DecisionFingerprint || value.ProjectionRequestID != projectionRequest.RequestID || value.ProjectionRequestFingerprint != projectionRequest.RequestFingerprint || value.ProjectionAuthority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{LocalFinalStateProjection: true}) || !value.ProjectionAuthorityAccepted || !nodeExecutionEqual(value.Requirements, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees()) || value.ApprovalInferred || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority{}) || fingerprint != value.DecisionFingerprint {
		return errors.New("graph lifecycle executor policy decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected, projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, projectionRequest NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, decision NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision) error {
	bindings := projectionRequest.TaskBindings
	bindingsFingerprint, bindingsErr := nodeExecutionFingerprintValue(bindings)
	preconditionFingerprint, preconditionErr := nodeExecutionFingerprintValue(expected.StorePrecondition)
	fingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestFingerprint(value)
	if err != nil || bindingsErr != nil || preconditionErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestSchema || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.ProjectedTerminalPostState != projectionRequest.FinalState || !nodeExecutionEqual(value.StorePrecondition, expected.StorePrecondition) || value.StorePreconditionFingerprint != preconditionFingerprint || value.GraphRunID != projectionRequest.GraphRunID || !nodeExecutionEqual(value.TaskBindings, bindings) || value.TaskBindingsFingerprint != bindingsFingerprint || value.FinalizationDecisionID != projectionRequest.FinalizationDecisionID || value.FinalizationDecisionFingerprint != projectionRequest.FinalizationDecisionFingerprint || value.FinalizationRequestID != projectionRequest.FinalizationRequestID || value.FinalizationRequestFingerprint != projectionRequest.FinalizationRequestFingerprint || value.ProjectionDecisionID != projectionDecision.DecisionID || value.ProjectionDecisionFingerprint != projectionDecision.DecisionFingerprint || value.ProjectionRequestID != projectionRequest.RequestID || value.ProjectionRequestFingerprint != projectionRequest.RequestFingerprint || value.ProjectionAuthority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{LocalFinalStateProjection: true}) || !nodeExecutionEqual(value.Requirements, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees()) || !value.OneTimeRequest || value.AuthorizationConsumed || value.ExecutorInvoked || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority{LocalGraphStateProjectionExecutorAttempt: true}) || fingerprint != value.RequestFingerprint {
		return errors.New("graph lifecycle executor policy request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected, projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, projectionRequest NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest) (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyArtifactMaxBytes {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, false, errors.New("graph lifecycle executor policy decision cannot be read")
	}
	var value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision
	if decodeNodeExecutionStrict(raw, &value) != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, false, errors.New("graph lifecycle executor policy decision is malformed")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(value, expected, projectionDecision, projectionRequest) != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision{}, false, errors.New("graph lifecycle executor policy decision is noncanonical, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected, projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, projectionRequest NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest, decision NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest{}, false, nil
	}
	if err != nil || !decisionExists || decision.Decision != "approved" || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyArtifactMaxBytes {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest{}, false, errors.New("graph lifecycle executor policy request cannot be read")
	}
	var value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest
	if decodeNodeExecutionStrict(raw, &value) != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest{}, false, errors.New("graph lifecycle executor policy request is malformed")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(value, expected, projectionDecision, projectionRequest, decision) != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest{}, false, errors.New("graph lifecycle executor policy request is noncanonical, tampered, or conflicting")
	}
	return value, true, nil
}

func validNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition) bool {
	return nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.GraphStoreID) && nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.GraphRecordID) && value.GraphStoreID != value.GraphRecordID && nodeExecutionFingerprint.MatchString(value.ExpectedPreimageFingerprint) && value.ExpectedPreimageVersion > 0
}

func nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees() NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequirements {
	return NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequirements{
		CompareAndSwapRequired: true, OneRecordAtomicityRequired: true, ExactReplayIdempotencyRequired: true,
		CrashRecoveryRequired: true, SeparatelyDurableAuditReceiptRequired: true,
	}
}

func nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision) NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest) NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
