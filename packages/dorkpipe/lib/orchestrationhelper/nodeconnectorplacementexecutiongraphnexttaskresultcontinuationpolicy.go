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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-policy-request/v1"

	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute           = "graph_continuation"
	NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute = "successful_graph_finalization"
	NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute     = "failed_graph_finalization"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionName     = "node-placement-execution-graph-next-task-result-continuation-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestName      = "node-placement-execution-graph-next-task-result-continuation-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyArtifactMaxBytes = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyLocks sync.Map

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority
// grants exactly one future local executor attempt for the explicitly selected
// route. It grants no graph mutation or adjacent authority.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority struct {
	GraphContinuationExecutorAttempt           bool `json:"graph_continuation_executor_attempt"`
	SuccessfulGraphFinalizationExecutorAttempt bool `json:"successful_graph_finalization_executor_attempt"`
	FailedGraphFinalizationExecutorAttempt     bool `json:"failed_graph_finalization_executor_attempt"`
	GraphContinuation                          bool `json:"graph_continuation"`
	GraphFinalization                          bool `json:"graph_finalization"`
	GraphCompletion                            bool `json:"graph_completion"`
	GraphFailurePropagation                    bool `json:"graph_failure_propagation"`
	GraphMutation                              bool `json:"graph_mutation"`
	TaskMutation                               bool `json:"task_mutation"`
	DependencyMutation                         bool `json:"dependency_mutation"`
	LifecycleMutation                          bool `json:"lifecycle_mutation"`
	SchedulingMutation                         bool `json:"scheduling_mutation"`
	ExecutionRecordMutation                    bool `json:"execution_record_mutation"`
	DependencyRelease                          bool `json:"dependency_release"`
	NextTaskScheduling                         bool `json:"next_task_scheduling"`
	TaskLaunch                                 bool `json:"task_launch"`
	NodeExecution                              bool `json:"node_execution"`
	ResultCollection                           bool `json:"result_collection"`
	ResultReconciliation                       bool `json:"result_reconciliation"`
	Placement                                  bool `json:"placement"`
	Dispatch                                   bool `json:"dispatch"`
	Connector                                  bool `json:"connector"`
	Lease                                      bool `json:"lease"`
	Broker                                     bool `json:"broker"`
	Provider                                   bool `json:"provider"`
	ForgePipe                                  bool `json:"forgepipe"`
	Retry                                      bool `json:"retry"`
	Repair                                     bool `json:"repair"`
	Cancellation                               bool `json:"cancellation"`
	Callback                                   bool `json:"callback"`
	GeneralQueueProcessing                     bool `json:"general_queue_processing"`
	ExternalAction                             bool `json:"external_action"`
	Network                                    bool `json:"network"`
	RemoteExecution                            bool `json:"remote_execution"`
	Validation                                 bool `json:"validation"`
	CheckoutMutation                           bool `json:"checkout_mutation"`
	Git                                        bool `json:"git"`
	Checkpoint                                 bool `json:"checkpoint"`
	Commit                                     bool `json:"commit"`
	Push                                       bool `json:"push"`
	Publication                                bool `json:"publication"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding
// preserves the exact accepted result, reconciliation, and selected-task chain
// needed by a future executor boundary.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding struct {
	ReconciliationReceiptID             string                                                          `json:"reconciliation_receipt_id"`
	ReconciliationReceiptFingerprint    string                                                          `json:"reconciliation_receipt_fingerprint"`
	AcceptedResultID                    string                                                          `json:"accepted_result_id"`
	AcceptedResultFingerprint           string                                                          `json:"accepted_result_fingerprint"`
	ObservationID                       string                                                          `json:"observation_id"`
	ObservationReplayIdentity           string                                                          `json:"observation_replay_identity"`
	ObservationFingerprint              string                                                          `json:"observation_fingerprint"`
	ObservationAuthenticationID         string                                                          `json:"observation_authentication_id"`
	ObservationAuthenticationDigest     string                                                          `json:"observation_authentication_digest"`
	ExecutorReceiptID                   string                                                          `json:"executor_receipt_id"`
	ExecutorReceiptFingerprint          string                                                          `json:"executor_receipt_fingerprint"`
	AttemptID                           string                                                          `json:"attempt_id"`
	AttemptRecordFingerprint            string                                                          `json:"attempt_record_fingerprint"`
	AuthorizationDecisionID             string                                                          `json:"authorization_decision_id"`
	AuthorizationDecisionFingerprint    string                                                          `json:"authorization_decision_fingerprint"`
	AuthorizationRequestID              string                                                          `json:"authorization_request_id"`
	AuthorizationRequestFingerprint     string                                                          `json:"authorization_request_fingerprint"`
	SchedulingReceiptID                 string                                                          `json:"scheduling_receipt_id"`
	SchedulingReceiptFingerprint        string                                                          `json:"scheduling_receipt_fingerprint"`
	GraphRunID                          string                                                          `json:"graph_run_id"`
	TerminalTaskID                      string                                                          `json:"terminal_task_id"`
	SelectedTaskID                      string                                                          `json:"selected_task_id"`
	CandidatesFingerprint               string                                                          `json:"candidates_fingerprint"`
	SelectedReleasedDependencyPostimage NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate `json:"selected_released_dependency_postimage"`
	ScheduledRecordPostimage            NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord    `json:"scheduled_record_postimage"`
	ScheduledRecordID                   string                                                          `json:"scheduled_record_id"`
	ScheduledRecordFingerprint          string                                                          `json:"scheduled_record_fingerprint"`
	ScheduledRecordVersion              uint64                                                          `json:"scheduled_record_version"`
	TerminalResult                      string                                                          `json:"terminal_result"`
	TaskOutcome                         string                                                          `json:"task_outcome"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected struct {
	Reconciliation                   NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected `json:"reconciliation"`
	ReconciliationReceiptFingerprint string                                                                   `json:"reconciliation_receipt_fingerprint"`
	AcceptedResultFingerprint        string                                                                   `json:"accepted_result_fingerprint"`
	DecisionAuthenticationID         string                                                                   `json:"decision_authentication_id"`
	DecisionAuthenticationDigest     string                                                                   `json:"decision_authentication_digest"`
	ContinuationRequestID            string                                                                   `json:"continuation_request_id"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture struct {
	Schema                string                                                                        `json:"schema"`
	DecisionID            string                                                                        `json:"decision_id"`
	ReplayIdentity        string                                                                        `json:"replay_identity"`
	AuthenticationID      string                                                                        `json:"authentication_id"`
	AuthenticationDigest  string                                                                        `json:"authentication_digest"`
	Decision              string                                                                        `json:"decision"`
	Route                 string                                                                        `json:"route,omitempty"`
	Binding               NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding   `json:"binding"`
	ContinuationRequestID string                                                                        `json:"continuation_request_id,omitempty"`
	Deterministic         bool                                                                          `json:"deterministic"`
	OneTimeDecision       bool                                                                          `json:"one_time_decision"`
	DecisionConsumed      bool                                                                          `json:"decision_consumed"`
	ApprovalInferred      bool                                                                          `json:"approval_inferred"`
	RouteInferred         bool                                                                          `json:"route_inferred"`
	InferenceSource       string                                                                        `json:"inference_source,omitempty"`
	Authority             NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority `json:"authority"`
	Provenance            string                                                                        `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision struct {
	Schema                     string                                                                        `json:"schema"`
	DecisionID                 string                                                                        `json:"decision_id"`
	ReplayIdentity             string                                                                        `json:"replay_identity"`
	AuthenticationID           string                                                                        `json:"authentication_id"`
	AuthenticationDigest       string                                                                        `json:"authentication_digest"`
	Decision                   string                                                                        `json:"decision"`
	Route                      string                                                                        `json:"route,omitempty"`
	Binding                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding   `json:"binding"`
	ContinuationRequestID      string                                                                        `json:"continuation_request_id,omitempty"`
	Deterministic              bool                                                                          `json:"deterministic"`
	OneTimeDecision            bool                                                                          `json:"one_time_decision"`
	DecisionConsumed           bool                                                                          `json:"decision_consumed"`
	ApprovalInferred           bool                                                                          `json:"approval_inferred"`
	RouteInferred              bool                                                                          `json:"route_inferred"`
	InferenceSource            string                                                                        `json:"inference_source,omitempty"`
	IndependentlyAuthenticated bool                                                                          `json:"independently_authenticated"`
	FixtureOwned               bool                                                                          `json:"fixture_owned"`
	Authority                  NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority `json:"authority"`
	DecisionFingerprint        string                                                                        `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest struct {
	Schema                   string                                                                        `json:"schema"`
	RequestID                string                                                                        `json:"request_id"`
	DecisionID               string                                                                        `json:"decision_id"`
	DecisionReplayIdentity   string                                                                        `json:"decision_replay_identity"`
	DecisionFingerprint      string                                                                        `json:"decision_fingerprint"`
	AuthenticationID         string                                                                        `json:"authentication_id"`
	AuthenticationDigest     string                                                                        `json:"authentication_digest"`
	Route                    string                                                                        `json:"route"`
	Binding                  NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding   `json:"binding"`
	OneTimeRequest           bool                                                                          `json:"one_time_request"`
	AuthorizationConsumed    bool                                                                          `json:"authorization_consumed"`
	GraphContinuationInvoked bool                                                                          `json:"graph_continuation_invoked"`
	GraphFinalizationInvoked bool                                                                          `json:"graph_finalization_invoked"`
	CallbacksInvoked         bool                                                                          `json:"callbacks_invoked"`
	ExternalActionsInvoked   bool                                                                          `json:"external_actions_invoked"`
	FixtureOwned             bool                                                                          `json:"fixture_owned"`
	Authority                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority `json:"authority"`
	RequestFingerprint       string                                                                        `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies struct {
	root          string
	expected      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected
	source        nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource
	accepted      NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult
	receipt       NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
	decision      *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision
	request       *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest
	writeDecision func(string, any) error
	writeRequest  func(string, any) error
	mu            sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies, error) {
	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies{
		root: root, expected: normalized, source: inputs.source, accepted: inputs.accepted, receipt: inputs.receipt,
		writeDecision: writeJSONFileAtomic, writeRequest: writeJSONFileAtomic,
	}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(root, normalized, inputs)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(root, normalized, inputs, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("post-reconciliation continuation policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation policy decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation policy decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(policies.expected, policies.accepted, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	_, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected(policies.root, policies.expected)
	if err != nil || !nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSourceEqual(inputs.source, policies.source) || !nodeExecutionEqual(inputs.accepted, policies.accepted) || !nodeExecutionEqual(inputs.receipt, policies.receipt) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation policy could not revalidate the exact immutable predecessor chain")
	}
	durableDecision, durableDecisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(policies.root, policies.expected, inputs)
	if err != nil || policies.decision != nil && !durableDecisionExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation decision is missing or conflicting")
	}
	durableRequest, durableRequestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(policies.root, policies.expected, inputs, durableDecision, durableDecisionExists)
	if err != nil || durableRequestExists && !durableDecisionExists || policies.request != nil && !durableRequestExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation request is missing, orphaned, or conflicting")
	}
	if durableDecisionExists {
		policies.decision = &durableDecision
	}
	if durableRequestExists {
		policies.request = &durableRequest
	}
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation continuation policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
		}
		if err := policies.writeDecision(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("rejected post-reconciliation continuation decision conflicts with an accepted request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation continuation policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
	}
	if err := policies.writeRequest(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(root, value.Reconciliation)
	if err != nil || !inputs.acceptedExists || !inputs.receiptExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("post-reconciliation continuation policy requires the complete durable result-reconciliation predecessor chain")
	}
	value.Reconciliation = inputs.source.expected
	receipt, accepted := inputs.receipt, inputs.accepted
	outcome, outcomeErr := nodeConnectorPlacementExecutionGraphNextTaskResultOutcome(receipt.TerminalResult)
	if outcomeErr != nil || value.ReconciliationReceiptFingerprint != receipt.ReceiptFingerprint || value.AcceptedResultFingerprint != accepted.AcceptedResultFingerprint || receipt.AcceptedResultID != accepted.AcceptedResultID || receipt.AcceptedResultFingerprint != accepted.AcceptedResultFingerprint || receipt.ObservationID != accepted.ObservationID || receipt.ObservationFingerprint != accepted.ObservationFingerprint || receipt.ExecutorReceiptID != accepted.ExecutorReceiptID || receipt.ExecutorReceiptFingerprint != accepted.ExecutorReceiptFingerprint || receipt.AttemptID != accepted.AttemptID || receipt.AttemptRecordFingerprint != accepted.AttemptRecordFingerprint || receipt.GraphRunID != accepted.GraphRunID || receipt.TerminalTaskID != accepted.TerminalTaskID || receipt.SelectedTaskID != accepted.SelectedTaskID || receipt.ScheduledRecordFingerprint != accepted.ScheduledRecordFingerprint || receipt.ScheduledRecordVersion != accepted.ScheduledRecordVersion || receipt.TerminalResult != accepted.TerminalResult || receipt.TaskOutcome != outcome || receipt.ResultIngestionCount != 1 || receipt.AcceptedResultWriteCount != 1 || receipt.ReconciliationWriteCount != 1 || !receipt.ObservationConsumed || !receipt.CompleteImmutableChainRevalidated || !receipt.TaskLevelResultOutcomeReconciled || receipt.GraphCompletionClaimed || receipt.GraphFailurePropagated || receipt.GraphProgressClaimed || receipt.DependencyReleased || receipt.NextTaskScheduled || receipt.ExecutionInvoked || receipt.CallbackInvoked || receipt.ExternalActionInvoked || !receipt.FixtureOwned || receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) || accepted.ResultIngestionCount != 1 || !accepted.FixtureOwned || accepted.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("post-reconciliation continuation policy result evidence is missing, stale, ambiguous, or escalates authority")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ContinuationRequestID) || value.DecisionAuthenticationID == accepted.AuthenticationID || value.DecisionAuthenticationDigest == accepted.AuthenticationDigest {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("post-reconciliation continuation policy requires separate exact fixture authentication and intended request identity")
	}
	return value, inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, receipt NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest, error) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding(accepted, receipt)
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(fixture.Binding, binding) || !fixture.Deterministic || !fixture.OneTimeDecision || fixture.DecisionConsumed || fixture.ApprovalInferred || fixture.RouteInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_post_reconciliation_graph_continuation_finalization_policy_decision" || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyIdentityCollides(fixture.DecisionID, fixture.ReplayIdentity, binding) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation fixture identity, authentication, result binding, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("post-reconciliation continuation decision is invalid")
	}
	narrowAuthority, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(receipt.TaskOutcome, fixture.Route)
	if fixture.Decision == "rejected" {
		if fixture.Route != "" || fixture.ContinuationRequestID != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("rejected post-reconciliation continuation decision cannot name a route, request, or grant authority")
		}
	} else if !routeValid || fixture.ContinuationRequestID != expected.ContinuationRequestID || fixture.Authority != narrowAuthority {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, errors.New("approved post-reconciliation continuation decision requires an exact outcome-compatible route, intended request, and narrow authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity,
		AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Route: fixture.Route,
		Binding: binding, ContinuationRequestID: fixture.ContinuationRequestID, Deterministic: true, OneTimeDecision: true,
		IndependentlyAuthenticated: true, FixtureOwned: true,
	}
	var err error
	decision.DecisionFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(decision, expected, accepted, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestSchema, RequestID: fixture.ContinuationRequestID,
		DecisionID: decision.DecisionID, DecisionReplayIdentity: decision.ReplayIdentity, DecisionFingerprint: decision.DecisionFingerprint,
		AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest, Route: decision.Route,
		Binding: binding, OneTimeRequest: true, FixtureOwned: true, Authority: narrowAuthority,
	}
	request.RequestFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(decision, expected, accepted, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(*request, expected, accepted, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, receipt NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding(accepted, receipt)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFingerprint(value)
	_, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(receipt.TaskOutcome, value.Route)
	requestValid := value.Decision == "rejected" && value.Route == "" && value.ContinuationRequestID == "" || value.Decision == "approved" && routeValid && value.ContinuationRequestID == expected.ContinuationRequestID
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.DecisionID == value.ReplayIdentity || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || value.Decision != "approved" && value.Decision != "rejected" || !nodeExecutionEqual(value.Binding, binding) || !requestValid || !value.Deterministic || !value.OneTimeDecision || value.DecisionConsumed || value.ApprovalInferred || value.RouteInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{}) || fingerprint != value.DecisionFingerprint || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyIdentityCollides(value.DecisionID, value.ReplayIdentity, binding) {
		return errors.New("post-reconciliation continuation decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, receipt NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding(accepted, receipt)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestFingerprint(value)
	narrowAuthority, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(receipt.TaskOutcome, value.Route)
	if err != nil || decision.Decision != "approved" || !routeValid || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestSchema || value.RequestID != expected.ContinuationRequestID || value.DecisionID != decision.DecisionID || value.DecisionReplayIdentity != decision.ReplayIdentity || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || value.Route != decision.Route || !nodeExecutionEqual(value.Binding, binding) || !value.OneTimeRequest || value.AuthorizationConsumed || value.GraphContinuationInvoked || value.GraphFinalizationInvoked || value.CallbacksInvoked || value.ExternalActionsInvoked || !value.FixtureOwned || value.Authority != narrowAuthority || fingerprint != value.RequestFingerprint {
		return errors.New("post-reconciliation continuation request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, false, errors.New("post-reconciliation continuation decision is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(value, expected, inputs.accepted, inputs.receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest{}, false, errors.New("post-reconciliation continuation request is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(value, expected, inputs.accepted, inputs.receipt, decision) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest{}, false, errors.New("post-reconciliation continuation request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyCanonicalArtifact(root, path string, target any, allowMissing bool) error {
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorPath(root, path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return err
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyArtifactMaxBytes {
		return errors.New("post-reconciliation continuation artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("post-reconciliation continuation artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("post-reconciliation continuation artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding(accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, receipt NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding {
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding{
		ReconciliationReceiptID: receipt.ReconciliationReceiptID, ReconciliationReceiptFingerprint: receipt.ReceiptFingerprint,
		AcceptedResultID: accepted.AcceptedResultID, AcceptedResultFingerprint: accepted.AcceptedResultFingerprint,
		ObservationID: accepted.ObservationID, ObservationReplayIdentity: accepted.ReplayIdentity, ObservationFingerprint: accepted.ObservationFingerprint,
		ObservationAuthenticationID: accepted.AuthenticationID, ObservationAuthenticationDigest: accepted.AuthenticationDigest,
		ExecutorReceiptID: accepted.ExecutorReceiptID, ExecutorReceiptFingerprint: accepted.ExecutorReceiptFingerprint,
		AttemptID: accepted.AttemptID, AttemptRecordFingerprint: accepted.AttemptRecordFingerprint,
		AuthorizationDecisionID: accepted.AuthorizationDecisionID, AuthorizationDecisionFingerprint: accepted.AuthorizationDecisionFingerprint,
		AuthorizationRequestID: accepted.AuthorizationRequestID, AuthorizationRequestFingerprint: accepted.AuthorizationRequestFingerprint,
		SchedulingReceiptID: accepted.SchedulingReceiptID, SchedulingReceiptFingerprint: accepted.SchedulingReceiptFingerprint,
		GraphRunID: accepted.GraphRunID, TerminalTaskID: accepted.TerminalTaskID, SelectedTaskID: accepted.SelectedTaskID,
		CandidatesFingerprint: accepted.CandidatesFingerprint, SelectedReleasedDependencyPostimage: accepted.SelectedReleasedDependencyPostimage,
		ScheduledRecordPostimage: accepted.ScheduledRecordPostimage, ScheduledRecordID: accepted.ScheduledRecordPostimage.TaskID,
		ScheduledRecordFingerprint: accepted.ScheduledRecordFingerprint, ScheduledRecordVersion: accepted.ScheduledRecordVersion,
		TerminalResult: receipt.TerminalResult, TaskOutcome: receipt.TaskOutcome,
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(outcome, route string) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority, bool) {
	switch {
	case outcome == "passed" && route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{GraphContinuationExecutorAttempt: true}, true
	case outcome == "passed" && route == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{SuccessfulGraphFinalizationExecutorAttempt: true}, true
	case outcome == "failed" && route == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{FailedGraphFinalizationExecutorAttempt: true}, true
	default:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{}, false
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyIdentityCollides(decisionID, replayIdentity string, binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding) bool {
	for _, value := range []string{binding.ReconciliationReceiptID, binding.AcceptedResultID, binding.ObservationID, binding.ObservationReplayIdentity, binding.ExecutorReceiptID, binding.AttemptID, binding.AuthorizationDecisionID, binding.AuthorizationRequestID, binding.SchedulingReceiptID, binding.GraphRunID, binding.TerminalTaskID, binding.SelectedTaskID, binding.ScheduledRecordID} {
		if decisionID == value || replayIdentity == value {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
