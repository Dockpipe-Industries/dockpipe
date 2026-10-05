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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-policy-request/v1"

	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput             = "continuation_handoff"
	NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization = "successful_terminal_graph_result_materialization"
	NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization     = "failed_terminal_graph_result_materialization"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName     = "node-placement-execution-graph-next-task-result-continuation-output-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName      = "node-placement-execution-graph-next-task-result-continuation-output-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyArtifactMaxBytes = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyLocks sync.Map

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority
// grants one mutually exclusive future route-compatible output attempt. It
// grants no graph, lifecycle, scheduling, execution, or external authority.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority struct {
	GraphContinuationHandoffAttempt                     bool `json:"graph_continuation_handoff_attempt"`
	SuccessfulTerminalGraphResultMaterializationAttempt bool `json:"successful_terminal_graph_result_materialization_attempt"`
	FailedTerminalGraphResultMaterializationAttempt     bool `json:"failed_terminal_graph_result_materialization_attempt"`
	GraphContinuationHandoff                            bool `json:"graph_continuation_handoff"`
	TerminalGraphResultMaterialization                  bool `json:"terminal_graph_result_materialization"`
	GraphMutation                                       bool `json:"graph_mutation"`
	LifecycleMutation                                   bool `json:"lifecycle_mutation"`
	SchedulingMutation                                  bool `json:"scheduling_mutation"`
	TaskLaunch                                          bool `json:"task_launch"`
	NodeExecution                                       bool `json:"node_execution"`
	DependencyRelease                                   bool `json:"dependency_release"`
	Retry                                               bool `json:"retry"`
	Repair                                              bool `json:"repair"`
	Cancellation                                        bool `json:"cancellation"`
	Callback                                            bool `json:"callback"`
	Provider                                            bool `json:"provider"`
	Connector                                           bool `json:"connector"`
	Broker                                              bool `json:"broker"`
	ForgePipe                                           bool `json:"forgepipe"`
	Validation                                          bool `json:"validation"`
	CheckoutMutation                                    bool `json:"checkout_mutation"`
	Git                                                 bool `json:"git"`
	Commit                                              bool `json:"commit"`
	Push                                                bool `json:"push"`
	Publication                                         bool `json:"publication"`
	Network                                             bool `json:"network"`
	ExternalAction                                      bool `json:"external_action"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding
// binds the exact completed transition and its complete executor binding.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding struct {
	TransitionExecutorReceiptID          string                                                                        `json:"transition_executor_receipt_id"`
	TransitionExecutorReceiptFingerprint string                                                                        `json:"transition_executor_receipt_fingerprint"`
	TransitionRecordID                   string                                                                        `json:"transition_record_id"`
	TransitionRecordFingerprint          string                                                                        `json:"transition_record_fingerprint"`
	TransitionRecordVersion              uint64                                                                        `json:"transition_record_version"`
	Route                                string                                                                        `json:"route"`
	PostState                            string                                                                        `json:"post_state"`
	RouteSpecificEffect                  string                                                                        `json:"route_specific_effect"`
	ExecutorBinding                      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding `json:"executor_binding"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected struct {
	Executor                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected `json:"executor"`
	ExecutorReceiptFingerprint   string                                                                         `json:"executor_receipt_fingerprint"`
	TransitionRecordFingerprint  string                                                                         `json:"transition_record_fingerprint"`
	DecisionAuthenticationID     string                                                                         `json:"decision_authentication_id"`
	DecisionAuthenticationDigest string                                                                         `json:"decision_authentication_digest"`
	OutputRequestID              string                                                                         `json:"output_request_id"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture struct {
	Schema               string                                                                              `json:"schema"`
	DecisionID           string                                                                              `json:"decision_id"`
	ReplayIdentity       string                                                                              `json:"replay_identity"`
	AuthenticationID     string                                                                              `json:"authentication_id"`
	AuthenticationDigest string                                                                              `json:"authentication_digest"`
	Decision             string                                                                              `json:"decision"`
	Route                string                                                                              `json:"route,omitempty"`
	OutputType           string                                                                              `json:"output_type,omitempty"`
	Binding              NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding   `json:"binding"`
	OutputRequestID      string                                                                              `json:"output_request_id,omitempty"`
	Deterministic        bool                                                                                `json:"deterministic"`
	OneTimeDecision      bool                                                                                `json:"one_time_decision"`
	DecisionConsumed     bool                                                                                `json:"decision_consumed"`
	ApprovalInferred     bool                                                                                `json:"approval_inferred"`
	RouteInferred        bool                                                                                `json:"route_inferred"`
	OutputTypeInferred   bool                                                                                `json:"output_type_inferred"`
	AuthorityInferred    bool                                                                                `json:"authority_inferred"`
	InferenceSource      string                                                                              `json:"inference_source,omitempty"`
	Authority            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority `json:"authority"`
	Provenance           string                                                                              `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision struct {
	Schema                     string                                                                              `json:"schema"`
	DecisionID                 string                                                                              `json:"decision_id"`
	ReplayIdentity             string                                                                              `json:"replay_identity"`
	AuthenticationID           string                                                                              `json:"authentication_id"`
	AuthenticationDigest       string                                                                              `json:"authentication_digest"`
	Decision                   string                                                                              `json:"decision"`
	Route                      string                                                                              `json:"route,omitempty"`
	OutputType                 string                                                                              `json:"output_type,omitempty"`
	Binding                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding   `json:"binding"`
	OutputRequestID            string                                                                              `json:"output_request_id,omitempty"`
	Deterministic              bool                                                                                `json:"deterministic"`
	OneTimeDecision            bool                                                                                `json:"one_time_decision"`
	DecisionConsumed           bool                                                                                `json:"decision_consumed"`
	ApprovalInferred           bool                                                                                `json:"approval_inferred"`
	RouteInferred              bool                                                                                `json:"route_inferred"`
	OutputTypeInferred         bool                                                                                `json:"output_type_inferred"`
	AuthorityInferred          bool                                                                                `json:"authority_inferred"`
	InferenceSource            string                                                                              `json:"inference_source,omitempty"`
	IndependentlyAuthenticated bool                                                                                `json:"independently_authenticated"`
	FixtureOwned               bool                                                                                `json:"fixture_owned"`
	Authority                  NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority `json:"authority"`
	DecisionFingerprint        string                                                                              `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest struct {
	Schema                                    string                                                                              `json:"schema"`
	RequestID                                 string                                                                              `json:"request_id"`
	DecisionID                                string                                                                              `json:"decision_id"`
	DecisionReplayIdentity                    string                                                                              `json:"decision_replay_identity"`
	DecisionFingerprint                       string                                                                              `json:"decision_fingerprint"`
	AuthenticationID                          string                                                                              `json:"authentication_id"`
	AuthenticationDigest                      string                                                                              `json:"authentication_digest"`
	Route                                     string                                                                              `json:"route"`
	OutputType                                string                                                                              `json:"output_type"`
	Binding                                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding   `json:"binding"`
	OneTimeRequest                            bool                                                                                `json:"one_time_request"`
	AuthorizationConsumed                     bool                                                                                `json:"authorization_consumed"`
	ContinuationHandoffInvoked                bool                                                                                `json:"continuation_handoff_invoked"`
	TerminalGraphResultMaterializationInvoked bool                                                                                `json:"terminal_graph_result_materialization_invoked"`
	CallbacksInvoked                          bool                                                                                `json:"callbacks_invoked"`
	ExternalActionsInvoked                    bool                                                                                `json:"external_actions_invoked"`
	FixtureOwned                              bool                                                                                `json:"fixture_owned"`
	Authority                                 NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority `json:"authority"`
	RequestFingerprint                        string                                                                              `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies struct {
	root          string
	expected      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected
	transition    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
	receipt       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
	decision      *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision
	request       *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest
	writeDecision func(string, any) error
	writeRequest  func(string, any) error
	mu            sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies, error) {
	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies{
		root: root, expected: normalized, transition: inputs.transition, receipt: inputs.receipt,
		writeDecision: writeJSONFileAtomic, writeRequest: writeJSONFileAtomic,
	}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(root, normalized, inputs)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(root, normalized, inputs, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("post-transition graph-output policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output policy decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output policy decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(policies.expected, policies.transition, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	_, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected(policies.root, policies.expected)
	if err != nil || !nodeExecutionEqual(inputs.transition, policies.transition) || !nodeExecutionEqual(inputs.receipt, policies.receipt) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output policy could not revalidate the complete immutable predecessor chain")
	}
	durableDecision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(policies.root, policies.expected, inputs)
	if err != nil || policies.decision != nil && !decisionExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output decision is missing or conflicting")
	}
	durableRequest, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(policies.root, policies.expected, inputs, durableDecision, decisionExists)
	if err != nil || requestExists && !decisionExists || policies.request != nil && !requestExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output request is missing, orphaned, or conflicting")
	}
	if decisionExists {
		policies.decision = &durableDecision
	}
	if requestExists {
		policies.request = &durableRequest
	}
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-transition graph-output policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
		}
		if err := policies.writeDecision(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("rejected post-transition graph-output decision conflicts with an accepted request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-transition graph-output policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
	}
	if err := policies.writeRequest(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs(root, value.Executor)
	if err != nil || !inputs.transitionExists || !inputs.receiptExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-transition graph-output policy requires the complete durable transition predecessor chain")
	}
	value.Executor = inputs.expected
	effect, postState, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationRouteEffect(inputs.reconciliation.TaskOutcome, inputs.transition.Route)
	if !routeValid || value.ExecutorReceiptFingerprint != inputs.receipt.ReceiptFingerprint || value.TransitionRecordFingerprint != inputs.transition.RecordFingerprint || inputs.receipt.TransitionRecordID != inputs.transition.TransitionRecordID || inputs.receipt.TransitionRecordFingerprint != inputs.transition.RecordFingerprint || inputs.receipt.TransitionRecordVersion != inputs.transition.Version || inputs.receipt.Route != inputs.transition.Route || inputs.receipt.ExactPostState != inputs.transition.PostState || inputs.receipt.RouteSpecificEffect != inputs.transition.Effect || inputs.transition.Effect != effect || inputs.transition.PostState != postState || !nodeExecutionEqual(inputs.receipt.Binding, inputs.transition.Binding) || inputs.receipt.TransitionCount != 1 || inputs.receipt.RecordWriteCount != 1 || !inputs.receipt.AuthorizationConsumed || !inputs.receipt.FixtureOwned || inputs.receipt.Evidence != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence{LocalRouteTransitionPerformed: true}) || !inputs.transition.FixtureOwned || inputs.transition.Version != 1 {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-transition graph-output policy transition evidence is missing, stale, conflicting, or escalates authority")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.OutputRequestID) || value.DecisionAuthenticationID == inputs.request.AuthenticationID || value.DecisionAuthenticationDigest == inputs.request.AuthenticationDigest || value.DecisionAuthenticationID == inputs.accepted.AuthenticationID || value.DecisionAuthenticationDigest == inputs.accepted.AuthenticationDigest {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-transition graph-output policy requires separate exact fixture authentication and intended request identity")
	}
	return value, inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected, transition NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest, error) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(transition, receipt)
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(fixture.Binding, binding) || !fixture.Deterministic || !fixture.OneTimeDecision || fixture.DecisionConsumed || fixture.ApprovalInferred || fixture.RouteInferred || fixture.OutputTypeInferred || fixture.AuthorityInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_post_transition_graph_output_policy_decision" || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyIdentityCollides(fixture.DecisionID, fixture.ReplayIdentity, binding) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output fixture identity, authentication, transition binding, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("post-transition graph-output decision is invalid")
	}
	outputType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(transition.Route, transition.PostState, transition.Effect)
	if fixture.Decision == "rejected" {
		if fixture.Route != "" || fixture.OutputType != "" || fixture.OutputRequestID != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("rejected post-transition graph-output decision cannot name a route, output, request, or authority")
		}
	} else if !compatible || fixture.Route != transition.Route || fixture.OutputType != outputType || fixture.OutputRequestID != expected.OutputRequestID || fixture.Authority != authority {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, errors.New("approved post-transition graph-output decision requires the exact route-compatible output and narrow authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{
		Schema:     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionSchema,
		DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest,
		Decision: fixture.Decision, Route: fixture.Route, OutputType: fixture.OutputType, Binding: binding, OutputRequestID: fixture.OutputRequestID,
		Deterministic: true, OneTimeDecision: true, IndependentlyAuthenticated: true, FixtureOwned: true,
	}
	var err error
	decision.DecisionFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(decision, expected, transition, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestSchema, RequestID: fixture.OutputRequestID,
		DecisionID: decision.DecisionID, DecisionReplayIdentity: decision.ReplayIdentity, DecisionFingerprint: decision.DecisionFingerprint,
		AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest, Route: decision.Route, OutputType: decision.OutputType,
		Binding: binding, OneTimeRequest: true, FixtureOwned: true, Authority: authority,
	}
	request.RequestFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(decision, expected, transition, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(*request, expected, transition, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected, transition NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(transition, receipt)
	outputType, _, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(transition.Route, transition.PostState, transition.Effect)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFingerprint(value)
	requestValid := value.Decision == "rejected" && value.Route == "" && value.OutputType == "" && value.OutputRequestID == "" || value.Decision == "approved" && compatible && value.Route == transition.Route && value.OutputType == outputType && value.OutputRequestID == expected.OutputRequestID
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.DecisionID == value.ReplayIdentity || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || value.Decision != "approved" && value.Decision != "rejected" || !nodeExecutionEqual(value.Binding, binding) || !requestValid || !value.Deterministic || !value.OneTimeDecision || value.DecisionConsumed || value.ApprovalInferred || value.RouteInferred || value.OutputTypeInferred || value.AuthorityInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{}) || fingerprint != value.DecisionFingerprint || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyIdentityCollides(value.DecisionID, value.ReplayIdentity, binding) {
		return errors.New("post-transition graph-output decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected, transition NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(transition, receipt)
	outputType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(transition.Route, transition.PostState, transition.Effect)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestFingerprint(value)
	if err != nil || decision.Decision != "approved" || !compatible || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestSchema || value.RequestID != expected.OutputRequestID || value.DecisionID != decision.DecisionID || value.DecisionReplayIdentity != decision.ReplayIdentity || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || value.Route != transition.Route || value.Route != decision.Route || value.OutputType != outputType || value.OutputType != decision.OutputType || !nodeExecutionEqual(value.Binding, binding) || !value.OneTimeRequest || value.AuthorizationConsumed || value.ContinuationHandoffInvoked || value.TerminalGraphResultMaterializationInvoked || value.CallbacksInvoked || value.ExternalActionsInvoked || !value.FixtureOwned || value.Authority != authority || fingerprint != value.RequestFingerprint {
		return errors.New("post-transition graph-output request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, false, errors.New("post-transition graph-output decision is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(value, expected, inputs.transition, inputs.receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest{}, false, errors.New("post-transition graph-output request is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(value, expected, inputs.transition, inputs.receipt, decision) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest{}, false, errors.New("post-transition graph-output request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyArtifactMaxBytes {
		return errors.New("post-transition graph-output artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("post-transition graph-output artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("post-transition graph-output artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(transition NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding {
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding{
		TransitionExecutorReceiptID: receipt.ExecutorReceiptID, TransitionExecutorReceiptFingerprint: receipt.ReceiptFingerprint,
		TransitionRecordID: transition.TransitionRecordID, TransitionRecordFingerprint: transition.RecordFingerprint, TransitionRecordVersion: transition.Version,
		Route: transition.Route, PostState: transition.PostState, RouteSpecificEffect: transition.Effect, ExecutorBinding: transition.Binding,
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(route, postState, effect string) (string, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority, bool) {
	switch {
	case route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute && postState == "continued" && effect == "passed_selected_task_continued_local_graph":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{GraphContinuationHandoffAttempt: true}, true
	case route == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute && postState == "succeeded" && effect == "passed_result_finalized_local_graph_successfully":
		return NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{SuccessfulTerminalGraphResultMaterializationAttempt: true}, true
	case route == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute && postState == "failed" && effect == "failed_result_finalized_local_graph_with_failure_propagation":
		return NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{FailedTerminalGraphResultMaterializationAttempt: true}, true
	default:
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{}, false
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyIdentityCollides(decisionID, replayIdentity string, binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding) bool {
	b := binding.ExecutorBinding
	for _, value := range []string{binding.TransitionExecutorReceiptID, binding.TransitionRecordID, b.PolicyDecisionID, b.PolicyRequestID, b.PolicyAuthenticationID, b.ReconciliationReceiptID, b.AcceptedResultID, b.ObservationID, b.AttemptID, b.ExecutorReceiptID, b.LaunchAuthorizationDecisionID, b.LaunchAuthorizationRequestID, b.SchedulingReceiptID, b.SchedulingPolicyDecisionID, b.SchedulingPolicyRequestID, b.GraphRunID, b.TerminalTaskID, b.SelectedTaskID, b.ScheduledRecordID} {
		if decisionID == value || replayIdentity == value {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
