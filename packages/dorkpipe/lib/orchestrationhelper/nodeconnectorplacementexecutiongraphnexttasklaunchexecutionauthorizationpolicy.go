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
	NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-launch-execution-authorization-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-launch-execution-authorization-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-launch-execution-authorization-policy-request/v1"

	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName     = "node-placement-execution-graph-next-task-launch-execution-authorization-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName      = "node-placement-execution-graph-next-task-launch-execution-authorization-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteRequestAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyLocks               sync.Map
)

// NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority
// grants only one future local executor attempt. It does not launch or execute a task.
type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority struct {
	TaskLaunchNewNodeExecutionExecutorAttempt bool `json:"task_launch_new_node_execution_executor_attempt"`
	SchedulingMutation                        bool `json:"scheduling_mutation"`
	TaskLaunch                                bool `json:"task_launch"`
	NodeExecution                             bool `json:"node_execution"`
	Placement                                 bool `json:"placement"`
	Dispatch                                  bool `json:"dispatch"`
	Connector                                 bool `json:"connector"`
	Broker                                    bool `json:"broker"`
	Provider                                  bool `json:"provider"`
	ForgePipe                                 bool `json:"forgepipe"`
	Retry                                     bool `json:"retry"`
	Repair                                    bool `json:"repair"`
	Cancellation                              bool `json:"cancellation"`
	Publication                               bool `json:"publication"`
	Callback                                  bool `json:"callback"`
	ExternalAction                            bool `json:"external_action"`
	RemoteExecution                           bool `json:"remote_execution"`
	Network                                   bool `json:"network"`
	Validation                                bool `json:"validation"`
	Checkout                                  bool `json:"checkout"`
	Git                                       bool `json:"git"`
	Commit                                    bool `json:"commit"`
	Push                                      bool `json:"push"`
}

// NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding
// preserves the exact scheduling evidence needed by the future executor boundary.
type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding struct {
	SchedulingReceiptID                  string                                                                   `json:"scheduling_receipt_id"`
	SchedulingReceiptFingerprint         string                                                                   `json:"scheduling_receipt_fingerprint"`
	GraphRunID                           string                                                                   `json:"graph_run_id"`
	TerminalTaskID                       string                                                                   `json:"terminal_task_id"`
	Route                                string                                                                   `json:"route"`
	TransitionReceiptID                  string                                                                   `json:"transition_receipt_id"`
	TransitionReceiptFingerprint         string                                                                   `json:"transition_receipt_fingerprint"`
	Transitions                          []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence `json:"transitions"`
	TransitionsFingerprint               string                                                                   `json:"transitions_fingerprint"`
	Candidates                           []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate        `json:"candidates"`
	CandidatesFingerprint                string                                                                   `json:"candidates_fingerprint"`
	SelectedTaskID                       string                                                                   `json:"selected_task_id"`
	SelectedReleasedDependencyPostimage  NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate          `json:"selected_released_dependency_postimage"`
	SchedulingRecordPostimage            NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord             `json:"scheduling_record_postimage"`
	SchedulingRecordPostimageFingerprint string                                                                   `json:"scheduling_record_postimage_fingerprint"`
	SchedulingRecordPostimageVersion     uint64                                                                   `json:"scheduling_record_postimage_version"`
	SchedulingPolicyDecisionID           string                                                                   `json:"scheduling_policy_decision_id"`
	SchedulingPolicyDecisionFingerprint  string                                                                   `json:"scheduling_policy_decision_fingerprint"`
	SchedulingPolicyRequestID            string                                                                   `json:"scheduling_policy_request_id"`
	SchedulingPolicyRequestFingerprint   string                                                                   `json:"scheduling_policy_request_fingerprint"`
	SchedulingPolicyAuthenticationID     string                                                                   `json:"scheduling_policy_authentication_id"`
	SchedulingPolicyAuthenticationDigest string                                                                   `json:"scheduling_policy_authentication_digest"`
	SchedulingAuthorizationConsumed      bool                                                                     `json:"scheduling_authorization_consumed"`
	SchedulingTransitionCount            uint64                                                                   `json:"scheduling_transition_count"`
	SchedulingRecordWriteCount           uint64                                                                   `json:"scheduling_record_write_count"`
	SchedulingEvidenceFixtureOwned       bool                                                                     `json:"scheduling_evidence_fixture_owned"`
}

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected struct {
	Executor                     NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected `json:"executor"`
	SchedulingReceiptFingerprint string                                                                 `json:"scheduling_receipt_fingerprint"`
	DecisionAuthenticationID     string                                                                 `json:"decision_authentication_id"`
	DecisionAuthenticationDigest string                                                                 `json:"decision_authentication_digest"`
	AuthorizationRequestID       string                                                                 `json:"authorization_request_id"`
}

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture struct {
	Schema                 string                                                                                  `json:"schema"`
	DecisionID             string                                                                                  `json:"decision_id"`
	ReplayIdentity         string                                                                                  `json:"replay_identity"`
	AuthenticationID       string                                                                                  `json:"authentication_id"`
	AuthenticationDigest   string                                                                                  `json:"authentication_digest"`
	Decision               string                                                                                  `json:"decision"`
	Binding                NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding   `json:"binding"`
	AuthorizationRequestID string                                                                                  `json:"authorization_request_id,omitempty"`
	ApprovalInferred       bool                                                                                    `json:"approval_inferred"`
	InferenceSource        string                                                                                  `json:"inference_source,omitempty"`
	Authority              NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority `json:"authority"`
	Provenance             string                                                                                  `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision struct {
	Schema                     string                                                                                  `json:"schema"`
	DecisionID                 string                                                                                  `json:"decision_id"`
	ReplayIdentity             string                                                                                  `json:"replay_identity"`
	AuthenticationID           string                                                                                  `json:"authentication_id"`
	AuthenticationDigest       string                                                                                  `json:"authentication_digest"`
	Decision                   string                                                                                  `json:"decision"`
	Binding                    NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding   `json:"binding"`
	AuthorizationRequestID     string                                                                                  `json:"authorization_request_id,omitempty"`
	ApprovalInferred           bool                                                                                    `json:"approval_inferred"`
	InferenceSource            string                                                                                  `json:"inference_source,omitempty"`
	IndependentlyAuthenticated bool                                                                                    `json:"independently_authenticated"`
	FixtureOwned               bool                                                                                    `json:"fixture_owned"`
	Authority                  NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority `json:"authority"`
	DecisionFingerprint        string                                                                                  `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest struct {
	Schema                 string                                                                                  `json:"schema"`
	RequestID              string                                                                                  `json:"request_id"`
	DecisionID             string                                                                                  `json:"decision_id"`
	DecisionFingerprint    string                                                                                  `json:"decision_fingerprint"`
	AuthenticationID       string                                                                                  `json:"authentication_id"`
	AuthenticationDigest   string                                                                                  `json:"authentication_digest"`
	Binding                NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding   `json:"binding"`
	OneTimeRequest         bool                                                                                    `json:"one_time_request"`
	AuthorizationConsumed  bool                                                                                    `json:"authorization_consumed"`
	TaskLaunchInvoked      bool                                                                                    `json:"task_launch_invoked"`
	NodeExecutionInvoked   bool                                                                                    `json:"node_execution_invoked"`
	CallbacksInvoked       bool                                                                                    `json:"callbacks_invoked"`
	ExternalActionsInvoked bool                                                                                    `json:"external_actions_invoked"`
	FixtureOwned           bool                                                                                    `json:"fixture_owned"`
	Authority              NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority `json:"authority"`
	RequestFingerprint     string                                                                                  `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected
	receipt  NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt
	decision *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
	request  *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies, error) {
	normalized, receipt, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies{root: root, expected: normalized, receipt: receipt}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(root, normalized, receipt)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(root, normalized, receipt, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("task-launch/new-node-execution authorization policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization policy decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization policy decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(policies.expected, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	durableDecision, durableDecisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(policies.root, policies.expected, policies.receipt)
	if err != nil || policies.decision != nil && !durableDecisionExists {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization decision is missing or conflicting")
	}
	durableRequest, durableRequestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(policies.root, policies.expected, policies.receipt, durableDecision, durableDecisionExists)
	if err != nil || durableRequestExists && !durableDecisionExists || policies.request != nil && !durableRequestExists {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization request is missing, orphaned, or conflicting")
	}
	if durableDecisionExists {
		policies.decision = &durableDecision
	}
	if durableRequestExists {
		policies.request = &durableRequest
	}
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "task-launch/new-node-execution authorization policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteDecisionAtomic(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("rejected task-launch/new-node-execution authorization conflicts with an accepted request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "task-launch/new-node-execution authorization policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteRequestAtomic(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected, NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs(root, value.Executor)
	if err != nil || !inputs.receiptExists || !inputs.isPost {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected{}, NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, errors.New("task-launch/new-node-execution authorization policy requires the complete durable scheduling predecessor chain")
	}
	value.Executor = inputs.expected
	receipt := inputs.receipt
	expectedEvidence := NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority{LocalSchedulingTransitionPerformed: true}
	selectedCount := 0
	for _, candidate := range receipt.Candidates {
		if candidate.TaskID == receipt.SelectedTaskID {
			selectedCount++
		}
	}
	if value.SchedulingReceiptFingerprint != receipt.ReceiptFingerprint || receipt.Route != "dependency_release_transition" || len(receipt.Candidates) == 0 || selectedCount != 1 || receipt.Postimage.State != "scheduled" || receipt.PostimageFingerprint != receipt.Postimage.RecordFingerprint || receipt.PostimageVersion != receipt.Postimage.Version || receipt.SelectedTaskID != receipt.Postimage.TaskID || receipt.SelectedCandidate.TaskID != receipt.SelectedTaskID || receipt.SelectedCandidate.ReleasedPostimageFingerprint != receipt.Postimage.ReleasedDependencyPostimageFingerprint || receipt.SelectedCandidate.ReleasedPostimageVersion != receipt.Postimage.ReleasedDependencyPostimageVersion || receipt.SchedulingTransition != "dependency_released_to_scheduled" || receipt.TransitionCount != 1 || receipt.RecordWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.FixtureOwned || receipt.Evidence != expectedEvidence {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected{}, NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, errors.New("task-launch/new-node-execution authorization policy scheduling evidence is missing, stale, changed, or escalates authority")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.AuthorizationRequestID) {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected{}, NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, errors.New("task-launch/new-node-execution authorization policy requires exact fixture authentication and intended request identity")
	}
	return value, receipt, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected, receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest, error) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding(receipt)
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(fixture.Binding, binding) || fixture.ApprovalInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_local_task_launch_new_node_execution_authorization_policy_decision" {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization fixture identity, authentication, scheduling binding, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("task-launch/new-node-execution authorization decision is invalid")
	}
	narrowAuthority := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{TaskLaunchNewNodeExecutionExecutorAttempt: true}
	if fixture.Decision == "rejected" {
		if fixture.AuthorizationRequestID != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("rejected task-launch/new-node-execution authorization cannot name a request or grant authority")
		}
	} else if fixture.AuthorizationRequestID != expected.AuthorizationRequestID || fixture.Authority != narrowAuthority {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, errors.New("approved task-launch/new-node-execution authorization requires the exact intended request and narrow authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity,
		AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Binding: binding,
		AuthorizationRequestID: fixture.AuthorizationRequestID, IndependentlyAuthenticated: true, FixtureOwned: true,
	}
	var err error
	decision.DecisionFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision, expected, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestSchema, RequestID: fixture.AuthorizationRequestID,
		DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint, AuthenticationID: decision.AuthenticationID,
		AuthenticationDigest: decision.AuthenticationDigest, Binding: binding, OneTimeRequest: true, FixtureOwned: true, Authority: narrowAuthority,
	}
	request.RequestFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision, expected, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(*request, expected, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected, receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding(receipt)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFingerprint(value)
	requestIdentityValid := value.Decision == "rejected" && value.AuthorizationRequestID == "" || value.Decision == "approved" && value.AuthorizationRequestID == expected.AuthorizationRequestID
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.DecisionID == value.ReplayIdentity || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || value.Decision != "approved" && value.Decision != "rejected" || !nodeExecutionEqual(value.Binding, binding) || !requestIdentityValid || value.ApprovalInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{}) || fingerprint != value.DecisionFingerprint {
		return errors.New("task-launch/new-node-execution authorization decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected, receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding(receipt)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestFingerprint(value)
	narrowAuthority := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{TaskLaunchNewNodeExecutionExecutorAttempt: true}
	if err != nil || decision.Decision != "approved" || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestSchema || value.RequestID != expected.AuthorizationRequestID || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || !nodeExecutionEqual(value.Binding, binding) || !value.OneTimeRequest || value.AuthorizationConsumed || value.TaskLaunchInvoked || value.NodeExecutionInvoked || value.CallbacksInvoked || value.ExternalActionsInvoked || !value.FixtureOwned || value.Authority != narrowAuthority || fingerprint != value.RequestFingerprint {
		return errors.New("task-launch/new-node-execution authorization request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected, receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, false, errors.New("task-launch/new-node-execution authorization decision is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(value, expected, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected, receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest{}, false, errors.New("task-launch/new-node-execution authorization request is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(value, expected, receipt, decision) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest{}, false, errors.New("task-launch/new-node-execution authorization request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactMaxBytes {
		return errors.New("task-launch/new-node-execution authorization artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("task-launch/new-node-execution authorization artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("task-launch/new-node-execution authorization artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding(receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding {
	return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding{
		SchedulingReceiptID: receipt.SchedulingReceiptID, SchedulingReceiptFingerprint: receipt.ReceiptFingerprint,
		GraphRunID: receipt.GraphRunID, TerminalTaskID: receipt.TerminalTaskID, Route: receipt.Route,
		TransitionReceiptID: receipt.TransitionReceiptID, TransitionReceiptFingerprint: receipt.TransitionReceiptFingerprint,
		Transitions: cloneNodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence(receipt.Transitions), TransitionsFingerprint: receipt.TransitionsFingerprint,
		Candidates: cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(receipt.Candidates), CandidatesFingerprint: receipt.CandidatesFingerprint,
		SelectedTaskID: receipt.SelectedTaskID, SelectedReleasedDependencyPostimage: receipt.SelectedCandidate,
		SchedulingRecordPostimage: receipt.Postimage, SchedulingRecordPostimageFingerprint: receipt.PostimageFingerprint, SchedulingRecordPostimageVersion: receipt.PostimageVersion,
		SchedulingPolicyDecisionID: receipt.PolicyDecisionID, SchedulingPolicyDecisionFingerprint: receipt.PolicyDecisionFingerprint,
		SchedulingPolicyRequestID: receipt.PolicyRequestID, SchedulingPolicyRequestFingerprint: receipt.PolicyRequestFingerprint,
		SchedulingPolicyAuthenticationID: receipt.AuthenticationID, SchedulingPolicyAuthenticationDigest: receipt.AuthenticationDigest,
		SchedulingAuthorizationConsumed: receipt.AuthorizationConsumed, SchedulingTransitionCount: receipt.TransitionCount,
		SchedulingRecordWriteCount: receipt.RecordWriteCount, SchedulingEvidenceFixtureOwned: receipt.FixtureOwned,
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
