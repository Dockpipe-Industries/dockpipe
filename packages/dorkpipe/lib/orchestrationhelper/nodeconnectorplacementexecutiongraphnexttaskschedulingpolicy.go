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
	NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-scheduling-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-scheduling-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-scheduling-policy-request/v1"

	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionName     = "node-placement-execution-graph-next-task-scheduling-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName      = "node-placement-execution-graph-next-task-scheduling-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteRequestAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyLocks               sync.Map
)

// NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate binds one
// independently selectable next task to the exact released dependency postimage.
type NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate struct {
	TaskID                       string `json:"task_id"`
	DependencyRecordID           string `json:"dependency_record_id"`
	ReleasedPostimageFingerprint string `json:"released_postimage_fingerprint"`
	ReleasedPostimageVersion     uint64 `json:"released_postimage_version"`
}

// NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority grants
// only one future local executor attempt. It performs and implies no action.
type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority struct {
	NextTaskSchedulingExecutorAttempt bool `json:"next_task_scheduling_executor_attempt"`
	QueueMutation                     bool `json:"queue_mutation"`
	SchedulingMutation                bool `json:"scheduling_mutation"`
	TaskLaunch                        bool `json:"task_launch"`
	NodeExecution                     bool `json:"node_execution"`
	Retry                             bool `json:"retry"`
	Repair                            bool `json:"repair"`
	Cancellation                      bool `json:"cancellation"`
	Publication                       bool `json:"publication"`
	Broker                            bool `json:"broker"`
	Provider                          bool `json:"provider"`
	ForgePipe                         bool `json:"forgepipe"`
	RemoteExecution                   bool `json:"remote_execution"`
	Network                           bool `json:"network"`
	Validation                        bool `json:"validation"`
	Checkout                          bool `json:"checkout"`
	Git                               bool `json:"git"`
	Commit                            bool `json:"commit"`
	Push                              bool `json:"push"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected struct {
	Executor                     NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected `json:"executor"`
	TransitionReceiptFingerprint string                                                                   `json:"transition_receipt_fingerprint"`
	TerminalTaskID               string                                                                   `json:"terminal_task_id"`
	Candidates                   []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate        `json:"candidates"`
	DecisionAuthenticationID     string                                                                   `json:"decision_authentication_id"`
	DecisionAuthenticationDigest string                                                                   `json:"decision_authentication_digest"`
	SchedulingRequestID          string                                                                   `json:"scheduling_request_id,omitempty"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture struct {
	Schema                       string                                                                `json:"schema"`
	DecisionID                   string                                                                `json:"decision_id"`
	ReplayIdentity               string                                                                `json:"replay_identity"`
	AuthenticationID             string                                                                `json:"authentication_id"`
	AuthenticationDigest         string                                                                `json:"authentication_digest"`
	Decision                     string                                                                `json:"decision"`
	TransitionReceiptID          string                                                                `json:"transition_receipt_id"`
	TransitionReceiptFingerprint string                                                                `json:"transition_receipt_fingerprint"`
	GraphRunID                   string                                                                `json:"graph_run_id"`
	TerminalTaskID               string                                                                `json:"terminal_task_id"`
	Route                        string                                                                `json:"route"`
	Candidates                   []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate     `json:"candidates"`
	CandidatesFingerprint        string                                                                `json:"candidates_fingerprint"`
	SelectedTaskID               string                                                                `json:"selected_task_id,omitempty"`
	SchedulingRequestID          string                                                                `json:"scheduling_request_id,omitempty"`
	ApprovalInferred             bool                                                                  `json:"approval_inferred"`
	InferenceSource              string                                                                `json:"inference_source,omitempty"`
	Authority                    NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority `json:"authority"`
	Provenance                   string                                                                `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding struct {
	TransitionReceiptID          string                                                                   `json:"transition_receipt_id"`
	TransitionReceiptFingerprint string                                                                   `json:"transition_receipt_fingerprint"`
	PolicyBinding                NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding    `json:"policy_binding"`
	PolicyDecisionID             string                                                                   `json:"policy_decision_id"`
	PolicyDecisionFingerprint    string                                                                   `json:"policy_decision_fingerprint"`
	PolicyRequestID              string                                                                   `json:"policy_request_id"`
	PolicyRequestFingerprint     string                                                                   `json:"policy_request_fingerprint"`
	Route                        string                                                                   `json:"route"`
	DependencyTargetsFingerprint string                                                                   `json:"dependency_targets_fingerprint"`
	Transitions                  []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence `json:"transitions"`
	TransitionsFingerprint       string                                                                   `json:"transitions_fingerprint"`
	GraphRunID                   string                                                                   `json:"graph_run_id"`
	TerminalTaskID               string                                                                   `json:"terminal_task_id"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision struct {
	Schema                     string                                                                `json:"schema"`
	DecisionID                 string                                                                `json:"decision_id"`
	ReplayIdentity             string                                                                `json:"replay_identity"`
	AuthenticationID           string                                                                `json:"authentication_id"`
	AuthenticationDigest       string                                                                `json:"authentication_digest"`
	Decision                   string                                                                `json:"decision"`
	Binding                    NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding   `json:"binding"`
	Candidates                 []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate     `json:"candidates"`
	CandidatesFingerprint      string                                                                `json:"candidates_fingerprint"`
	SelectedTaskID             string                                                                `json:"selected_task_id,omitempty"`
	ApprovalInferred           bool                                                                  `json:"approval_inferred"`
	InferenceSource            string                                                                `json:"inference_source,omitempty"`
	IndependentlyAuthenticated bool                                                                  `json:"independently_authenticated"`
	FixtureOwned               bool                                                                  `json:"fixture_owned"`
	Authority                  NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority `json:"authority"`
	DecisionFingerprint        string                                                                `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest struct {
	Schema                 string                                                                `json:"schema"`
	RequestID              string                                                                `json:"request_id"`
	DecisionID             string                                                                `json:"decision_id"`
	DecisionFingerprint    string                                                                `json:"decision_fingerprint"`
	AuthenticationID       string                                                                `json:"authentication_id"`
	AuthenticationDigest   string                                                                `json:"authentication_digest"`
	Binding                NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding   `json:"binding"`
	Candidates             []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate     `json:"candidates"`
	CandidatesFingerprint  string                                                                `json:"candidates_fingerprint"`
	SelectedTaskID         string                                                                `json:"selected_task_id"`
	OneTimeRequest         bool                                                                  `json:"one_time_request"`
	AuthorizationConsumed  bool                                                                  `json:"authorization_consumed"`
	SchedulingInvoked      bool                                                                  `json:"scheduling_invoked"`
	TaskLaunched           bool                                                                  `json:"task_launched"`
	CallbacksInvoked       bool                                                                  `json:"callbacks_invoked"`
	ExternalActionsInvoked bool                                                                  `json:"external_actions_invoked"`
	FixtureOwned           bool                                                                  `json:"fixture_owned"`
	Authority              NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority `json:"authority"`
	RequestFingerprint     string                                                                `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected
	receipt  NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt
	decision *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision
	request  *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies, error) {
	normalized, receipt, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies{root: root, expected: normalized, receipt: receipt}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(root, normalized, receipt)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(root, normalized, receipt, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("next-task scheduling policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(policies.expected, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "next-task scheduling policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy rejection conflicts with an accepted request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "next-task scheduling policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs(root, value.Executor)
	if err != nil || !inputs.receiptExists {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{}, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("next-task scheduling policy requires the complete durable dependency-transition predecessor chain")
	}
	value.Executor = inputs.expected
	if value.TransitionReceiptFingerprint != inputs.receipt.ReceiptFingerprint || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TerminalTaskID) || !nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyHasTerminalTask(inputs.receipt, value.TerminalTaskID) {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{}, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("next-task scheduling policy receipt, graph, or terminal task binding is stale or conflicting")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{}, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("next-task scheduling policy requires exact fixture authentication")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(inputs.receipt, value.Candidates); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{}, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, err
	}
	if inputs.receipt.Route == "dependency_release_transition" {
		if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.SchedulingRequestID) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{}, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("release scheduling policy requires one intended scheduling request identity")
		}
	} else if value.SchedulingRequestID != "" {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{}, NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("failure propagation cannot carry a scheduling request identity")
	}
	value.Candidates = cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(value.Candidates)
	return value, inputs.receipt, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest, error) {
	candidates := cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(expected.Candidates)
	candidateFingerprint, err := nodeExecutionFingerprintValue(candidates)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	binding, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding(expected, receipt)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || fixture.TransitionReceiptID != receipt.TransitionReceiptID || fixture.TransitionReceiptFingerprint != receipt.ReceiptFingerprint || fixture.GraphRunID != receipt.PolicyBinding.GraphRunID || fixture.TerminalTaskID != expected.TerminalTaskID || fixture.Route != receipt.Route || !nodeExecutionEqual(fixture.Candidates, candidates) || fixture.CandidatesFingerprint != candidateFingerprint || fixture.ApprovalInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_forgepipe_local_graph_next_task_scheduling_policy_decision" {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy fixture identity, authentication, receipt, route, terminal task, candidate set, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("next-task scheduling policy decision is invalid")
	}
	if fixture.Decision == "rejected" {
		if fixture.SelectedTaskID != "" || fixture.SchedulingRequestID != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("rejected next-task scheduling policy cannot select a task, name a request, or grant authority")
		}
	} else if receipt.Route != "dependency_release_transition" || !nodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidateContains(candidates, fixture.SelectedTaskID) || fixture.SchedulingRequestID != expected.SchedulingRequestID || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{NextTaskSchedulingExecutorAttempt: true}) {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, errors.New("approved next-task scheduling policy requires an exact release candidate and narrow request authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity,
		AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Binding: binding,
		Candidates: candidates, CandidatesFingerprint: candidateFingerprint, SelectedTaskID: fixture.SelectedTaskID, IndependentlyAuthenticated: true, FixtureOwned: true,
	}
	decision.DecisionFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(decision, expected, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestSchema, RequestID: fixture.SchedulingRequestID, DecisionID: decision.DecisionID,
		DecisionFingerprint: decision.DecisionFingerprint, AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest,
		Binding: binding, Candidates: cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(candidates), CandidatesFingerprint: candidateFingerprint, SelectedTaskID: fixture.SelectedTaskID,
		OneTimeRequest: true, FixtureOwned: true, Authority: NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{NextTaskSchedulingExecutorAttempt: true},
	}
	request.RequestFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(decision, expected, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(*request, expected, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt) error {
	binding, bindingErr := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding(expected, receipt)
	candidateFingerprint, candidateErr := nodeExecutionFingerprintValue(expected.Candidates)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFingerprint(value)
	selectionValid := value.Decision == "rejected" && value.SelectedTaskID == "" || value.Decision == "approved" && receipt.Route == "dependency_release_transition" && nodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidateContains(expected.Candidates, value.SelectedTaskID)
	if err != nil || bindingErr != nil || candidateErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(value.Binding, binding) || !nodeExecutionEqual(value.Candidates, expected.Candidates) || value.CandidatesFingerprint != candidateFingerprint || !selectionValid || value.ApprovalInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{}) || fingerprint != value.DecisionFingerprint {
		return errors.New("next-task scheduling policy decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision) error {
	binding, bindingErr := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding(expected, receipt)
	candidateFingerprint, candidateErr := nodeExecutionFingerprintValue(expected.Candidates)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestFingerprint(value)
	expectedAuthority := NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{NextTaskSchedulingExecutorAttempt: true}
	if err != nil || bindingErr != nil || candidateErr != nil || receipt.Route != "dependency_release_transition" || decision.Decision != "approved" || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestSchema || value.RequestID != expected.SchedulingRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || !nodeExecutionEqual(value.Binding, binding) || !nodeExecutionEqual(value.Candidates, expected.Candidates) || value.CandidatesFingerprint != candidateFingerprint || value.SelectedTaskID != decision.SelectedTaskID || !nodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidateContains(expected.Candidates, value.SelectedTaskID) || !value.OneTimeRequest || value.AuthorizationConsumed || value.SchedulingInvoked || value.TaskLaunched || value.CallbacksInvoked || value.ExternalActionsInvoked || !value.FixtureOwned || value.Authority != expectedAuthority || fingerprint != value.RequestFingerprint {
		return errors.New("next-task scheduling policy request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyCanonicalArtifact(path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, false, errors.New("next-task scheduling policy decision is malformed, noncanonical, oversized, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(value, expected, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyCanonicalArtifact(path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest{}, false, errors.New("next-task scheduling policy request is malformed, noncanonical, oversized, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(value, expected, receipt, decision) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest{}, false, errors.New("next-task scheduling policy request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyCanonicalArtifact(path string, target any, allowMissing bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return err
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactMaxBytes {
		return errors.New("next-task scheduling policy artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("next-task scheduling policy artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("next-task scheduling policy artifact is noncanonical")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, values []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate) error {
	if receipt.Route == "failure_propagation_transition" {
		if len(values) != 0 {
			return errors.New("failure-propagation evidence cannot create scheduling candidates")
		}
		return nil
	}
	if receipt.Route != "dependency_release_transition" || len(values) == 0 || len(values) > 256 || len(values) != len(receipt.Transitions) {
		return errors.New("release scheduling policy requires the exact bounded nonempty released candidate set")
	}
	last := ""
	for index, value := range values {
		transition := receipt.Transitions[index]
		if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TaskID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DependencyRecordID) || last != "" && value.TaskID <= last || value.TaskID != transition.Target.DependencyID || value.DependencyRecordID != transition.Target.DependencyRecordID || value.ReleasedPostimageFingerprint != transition.PostimageFingerprint || value.ReleasedPostimageVersion != transition.PostimageVersion || transition.Postimage.State != "dependency_released" || transition.Postimage.Route != "dependency_release_transition" {
			return errors.New("next-task scheduling candidates must exactly and ordinally bind every released dependency postimage")
		}
		last = value.TaskID
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyHasTerminalTask(receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, terminalTaskID string) bool {
	count := 0
	for _, binding := range receipt.PolicyBinding.TaskBindings {
		if binding.TaskID == terminalTaskID {
			count++
		}
	}
	return count == 1
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidateContains(values []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate, taskID string) bool {
	for _, value := range values {
		if value.TaskID == taskID {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding(expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding, error) {
	transitions := cloneNodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence(receipt.Transitions)
	fingerprint, err := nodeExecutionFingerprintValue(transitions)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding{}, err
	}
	return NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding{
		TransitionReceiptID: receipt.TransitionReceiptID, TransitionReceiptFingerprint: receipt.ReceiptFingerprint, PolicyBinding: receipt.PolicyBinding,
		PolicyDecisionID: receipt.PolicyDecisionID, PolicyDecisionFingerprint: receipt.PolicyDecisionFingerprint, PolicyRequestID: receipt.PolicyRequestID, PolicyRequestFingerprint: receipt.PolicyRequestFingerprint,
		Route: receipt.Route, DependencyTargetsFingerprint: receipt.DependencyTargetsFingerprint, Transitions: transitions, TransitionsFingerprint: fingerprint,
		GraphRunID: receipt.PolicyBinding.GraphRunID, TerminalTaskID: expected.TerminalTaskID,
	}, nil
}

func cloneNodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence(values []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence) []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence {
	raw, _ := json.Marshal(values)
	var cloned []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(values []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate) []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate {
	cloned := append([]NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate(nil), values...)
	sort.SliceStable(cloned, func(i, j int) bool { return cloned[i].TaskID < cloned[j].TaskID })
	return cloned
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
