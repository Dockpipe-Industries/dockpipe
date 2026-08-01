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
	NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordSchema = "dorkpipe.node-placement-execution-graph-next-task-launch-execution-executor-attempt-record/v1"
	NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptSchema       = "dorkpipe.node-placement-execution-graph-next-task-launch-execution-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName = "node-placement-execution-graph-next-task-launch-execution-executor-attempt-record.json"
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName       = "node-placement-execution-graph-next-task-launch-execution-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactMaxBytes  = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorLocks sync.Map

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected struct {
	Policy                           NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected `json:"policy"`
	AuthorizationDecisionFingerprint string                                                                                 `json:"authorization_decision_fingerprint"`
	AuthorizationRequestFingerprint  string                                                                                 `json:"authorization_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding
// preserves the exact authorization and scheduling evidence for one local
// attempt record. It grants no authority beyond recording that attempt.
type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding struct {
	AuthorizationRequestID               string                                                                   `json:"authorization_request_id"`
	AuthorizationRequestFingerprint      string                                                                   `json:"authorization_request_fingerprint"`
	AuthorizationDecisionID              string                                                                   `json:"authorization_decision_id"`
	AuthorizationDecisionFingerprint     string                                                                   `json:"authorization_decision_fingerprint"`
	AuthorizationAuthenticationID        string                                                                   `json:"authorization_authentication_id"`
	AuthorizationAuthenticationDigest    string                                                                   `json:"authorization_authentication_digest"`
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
	ScheduledRecordPostimage             NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord             `json:"scheduled_record_postimage"`
	ScheduledRecordPostimageFingerprint  string                                                                   `json:"scheduled_record_postimage_fingerprint"`
	ScheduledRecordPostimageVersion      uint64                                                                   `json:"scheduled_record_postimage_version"`
	SchedulingPolicyDecisionID           string                                                                   `json:"scheduling_policy_decision_id"`
	SchedulingPolicyDecisionFingerprint  string                                                                   `json:"scheduling_policy_decision_fingerprint"`
	SchedulingPolicyRequestID            string                                                                   `json:"scheduling_policy_request_id"`
	SchedulingPolicyRequestFingerprint   string                                                                   `json:"scheduling_policy_request_fingerprint"`
	SchedulingPolicyAuthenticationID     string                                                                   `json:"scheduling_policy_authentication_id"`
	SchedulingPolicyAuthenticationDigest string                                                                   `json:"scheduling_policy_authentication_digest"`
}

// NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord
// is one deterministic fixture-owned local launch/new-node-execution attempt.
// It is not a task process, node execution, result, or graph transition.
type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord struct {
	Schema              string                                                                     `json:"schema"`
	AttemptID           string                                                                     `json:"attempt_id"`
	Binding             NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding `json:"binding"`
	AttemptMaterialized bool                                                                       `json:"attempt_materialized"`
	FixtureOwned        bool                                                                       `json:"fixture_owned"`
	RecordFingerprint   string                                                                     `json:"record_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorEvidence
// proves only that one local attempt record was materialized. It grants and
// implies no process, execution, result, completion, or adjacent authority.
type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorEvidence struct {
	LocalAttemptMaterialized bool `json:"local_attempt_materialized"`
	TaskProcess              bool `json:"task_process"`
	TaskLaunch               bool `json:"task_launch"`
	NodeExecution            bool `json:"node_execution"`
	NodeExecutionReceipt     bool `json:"node_execution_receipt"`
	SuccessfulTaskOutcome    bool `json:"successful_task_outcome"`
	GraphProgress            bool `json:"graph_progress"`
	Placement                bool `json:"placement"`
	Dispatch                 bool `json:"dispatch"`
	Connector                bool `json:"connector"`
	Broker                   bool `json:"broker"`
	Provider                 bool `json:"provider"`
	ForgePipe                bool `json:"forgepipe"`
	Callback                 bool `json:"callback"`
	ExternalAction           bool `json:"external_action"`
	Network                  bool `json:"network"`
	RemoteExecution          bool `json:"remote_execution"`
	Validation               bool `json:"validation"`
	CheckoutMutation         bool `json:"checkout_mutation"`
	Git                      bool `json:"git"`
	Retry                    bool `json:"retry"`
	Repair                   bool `json:"repair"`
	Cancellation             bool `json:"cancellation"`
	Publication              bool `json:"publication"`
	LifecycleTransition      bool `json:"lifecycle_transition"`
}

// NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt
// is durable consumption evidence for the immutable authorization request.
// The request itself remains unchanged.
type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt struct {
	Schema                   string                                                                      `json:"schema"`
	ExecutorReceiptID        string                                                                      `json:"executor_receipt_id"`
	Binding                  NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding  `json:"binding"`
	AttemptID                string                                                                      `json:"attempt_id"`
	AttemptRecordFingerprint string                                                                      `json:"attempt_record_fingerprint"`
	AttemptCount             uint64                                                                      `json:"attempt_count"`
	AttemptRecordWriteCount  uint64                                                                      `json:"attempt_record_write_count"`
	AuthorizationConsumed    bool                                                                        `json:"authorization_consumed"`
	FixtureOwned             bool                                                                        `json:"fixture_owned"`
	Evidence                 NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorEvidence `json:"evidence"`
	ReceiptFingerprint       string                                                                      `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs struct {
	expected      NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected
	scheduling    NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt
	decision      NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
	request       NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
	attempt       NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord
	attemptExists bool
	receipt       NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt
	receiptExists bool
}

type NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor struct {
	root               string
	expected           NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected
	writeAttemptAtomic func(string, any) error
	writeReceiptAtomic func(string, any) error
	mu                 sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) (*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor{
		root: root, expected: inputs.expected,
		writeAttemptAtomic: writeJSONFileAtomic,
		writeReceiptAtomic: writeJSONFileAtomic,
	}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorLocks.LoadOrStore(executor.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.attemptExists {
		inputs.attempt = deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(inputs)
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(inputs.attempt, inputs); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, err
		}
		path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "task-launch/new-node-execution executor attempt record"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, err
		}
		if err := executor.writeAttemptAtomic(path, inputs.attempt); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, errors.New("task-launch/new-node-execution executor attempt record could not be published")
		}
		inputs.attemptExists = true
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, err
	}
	receiptPath := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(receiptPath, "task-launch/new-node-execution executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, err
	}
	if err := executor.writeReceiptAtomic(receiptPath, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, errors.New("task-launch/new-node-execution executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) (nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs, error) {
	policy, scheduling, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, errors.New("task-launch/new-node-execution executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(root, policy, scheduling)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.AuthorizationDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || decision.ApprovalInferred || decision.InferenceSource != "" || decision.AuthenticationID != policy.DecisionAuthenticationID || decision.AuthenticationDigest != policy.DecisionAuthenticationDigest {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, errors.New("task-launch/new-node-execution executor requires the exact approved independently authenticated authorization decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(root, policy, scheduling, decision, true)
	narrowAuthority := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{TaskLaunchNewNodeExecutionExecutorAttempt: true}
	if err != nil || !requestExists || request.RequestFingerprint != expected.AuthorizationRequestFingerprint || request.RequestID != policy.AuthorizationRequestID || request.DecisionID != decision.DecisionID || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest || !nodeExecutionEqual(request.Binding, decision.Binding) || !request.OneTimeRequest || request.AuthorizationConsumed || request.TaskLaunchInvoked || request.NodeExecutionInvoked || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned || request.Authority != narrowAuthority {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, errors.New("task-launch/new-node-execution executor requires the exact approved unconsumed narrow authorization request")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorSchedulingEvidence(request, scheduling); err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, err
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{expected: expected, scheduling: scheduling, decision: decision, request: request}
	attempt, attemptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, err
	}
	inputs.attempt, inputs.attemptExists = attempt, attemptExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, err
	}
	if receiptExists && !attemptExists {
		return nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{}, errors.New("task-launch/new-node-execution executor receipt is orphaned from its exact attempt record")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorSchedulingEvidence(request NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest, scheduling NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) error {
	binding := request.Binding
	selectedCount := 0
	for _, candidate := range binding.Candidates {
		if candidate.TaskID == binding.SelectedTaskID {
			selectedCount++
		}
	}
	expectedSchedulingEvidence := NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority{LocalSchedulingTransitionPerformed: true}
	if binding.SchedulingReceiptID != scheduling.SchedulingReceiptID || binding.SchedulingReceiptFingerprint != scheduling.ReceiptFingerprint || binding.GraphRunID != scheduling.GraphRunID || binding.TerminalTaskID != scheduling.TerminalTaskID || binding.Route != "dependency_release_transition" || binding.Route != scheduling.Route || binding.TransitionReceiptID != scheduling.TransitionReceiptID || binding.TransitionReceiptFingerprint != scheduling.TransitionReceiptFingerprint || !nodeExecutionEqual(binding.Transitions, scheduling.Transitions) || binding.TransitionsFingerprint != scheduling.TransitionsFingerprint || len(binding.Candidates) == 0 || selectedCount != 1 || !nodeExecutionEqual(binding.Candidates, scheduling.Candidates) || binding.CandidatesFingerprint != scheduling.CandidatesFingerprint || binding.SelectedTaskID != scheduling.SelectedTaskID || !nodeExecutionEqual(binding.SelectedReleasedDependencyPostimage, scheduling.SelectedCandidate) || !nodeExecutionEqual(binding.SchedulingRecordPostimage, scheduling.Postimage) || binding.SchedulingRecordPostimageFingerprint != scheduling.PostimageFingerprint || binding.SchedulingRecordPostimageVersion != scheduling.PostimageVersion || binding.SchedulingPolicyDecisionID != scheduling.PolicyDecisionID || binding.SchedulingPolicyDecisionFingerprint != scheduling.PolicyDecisionFingerprint || binding.SchedulingPolicyRequestID != scheduling.PolicyRequestID || binding.SchedulingPolicyRequestFingerprint != scheduling.PolicyRequestFingerprint || binding.SchedulingPolicyAuthenticationID != scheduling.AuthenticationID || binding.SchedulingPolicyAuthenticationDigest != scheduling.AuthenticationDigest || !binding.SchedulingAuthorizationConsumed || binding.SchedulingTransitionCount != 1 || binding.SchedulingRecordWriteCount != 1 || !binding.SchedulingEvidenceFixtureOwned || scheduling.SchedulingTransition != "dependency_released_to_scheduled" || !scheduling.AuthorizationConsumed || !scheduling.FixtureOwned || scheduling.TransitionCount != 1 || scheduling.RecordWriteCount != 1 || scheduling.Postimage.State != "scheduled" || scheduling.Postimage.TaskID != binding.SelectedTaskID || scheduling.Postimage.RecordFingerprint != binding.SchedulingRecordPostimageFingerprint || scheduling.Postimage.Version != binding.SchedulingRecordPostimageVersion || scheduling.Evidence != expectedSchedulingEvidence {
		return errors.New("task-launch/new-node-execution executor scheduling evidence is missing, stale, ambiguous, changed, or escalates authority")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding(inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding {
	request := inputs.request
	binding := request.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding{
		AuthorizationRequestID: request.RequestID, AuthorizationRequestFingerprint: request.RequestFingerprint,
		AuthorizationDecisionID: inputs.decision.DecisionID, AuthorizationDecisionFingerprint: inputs.decision.DecisionFingerprint,
		AuthorizationAuthenticationID: request.AuthenticationID, AuthorizationAuthenticationDigest: request.AuthenticationDigest,
		SchedulingReceiptID: binding.SchedulingReceiptID, SchedulingReceiptFingerprint: binding.SchedulingReceiptFingerprint,
		GraphRunID: binding.GraphRunID, TerminalTaskID: binding.TerminalTaskID, Route: binding.Route,
		TransitionReceiptID: binding.TransitionReceiptID, TransitionReceiptFingerprint: binding.TransitionReceiptFingerprint,
		Transitions: cloneNodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence(binding.Transitions), TransitionsFingerprint: binding.TransitionsFingerprint,
		Candidates: cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(binding.Candidates), CandidatesFingerprint: binding.CandidatesFingerprint,
		SelectedTaskID: binding.SelectedTaskID, SelectedReleasedDependencyPostimage: binding.SelectedReleasedDependencyPostimage,
		ScheduledRecordPostimage: binding.SchedulingRecordPostimage, ScheduledRecordPostimageFingerprint: binding.SchedulingRecordPostimageFingerprint, ScheduledRecordPostimageVersion: binding.SchedulingRecordPostimageVersion,
		SchedulingPolicyDecisionID: binding.SchedulingPolicyDecisionID, SchedulingPolicyDecisionFingerprint: binding.SchedulingPolicyDecisionFingerprint,
		SchedulingPolicyRequestID: binding.SchedulingPolicyRequestID, SchedulingPolicyRequestFingerprint: binding.SchedulingPolicyRequestFingerprint,
		SchedulingPolicyAuthenticationID: binding.SchedulingPolicyAuthenticationID, SchedulingPolicyAuthenticationDigest: binding.SchedulingPolicyAuthenticationDigest,
	}
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord {
	attempt := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord{
		Schema:    NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordSchema,
		AttemptID: inputs.request.RequestID + "-attempt", Binding: nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding(inputs),
		AttemptMaterialized: true, FixtureOwned: true,
	}
	attempt.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptFingerprint(attempt)
	return attempt
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt {
	receipt := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{
		Schema:            NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptSchema,
		ExecutorReceiptID: inputs.request.RequestID + "-executor-receipt", Binding: nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding(inputs),
		AttemptID: inputs.attempt.AttemptID, AttemptRecordFingerprint: inputs.attempt.RecordFingerprint,
		AttemptCount: 1, AttemptRecordWriteCount: 1, AuthorizationConsumed: true, FixtureOwned: true,
		Evidence: NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorEvidence{LocalAttemptMaterialized: true},
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord, inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.AttemptID) || fingerprint != value.RecordFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("task-launch/new-node-execution executor attempt record is invalid, conflicting, or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ExecutorReceiptID) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("task-launch/new-node-execution executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName)
	var value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord{}, false, errors.New("task-launch/new-node-execution executor attempt record is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, false, errors.New("task-launch/new-node-execution executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !inputs.attemptExists || validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(value, inputs) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt{}, false, errors.New("task-launch/new-node-execution executor receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactMaxBytes {
		return errors.New("task-launch/new-node-execution executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("task-launch/new-node-execution executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("task-launch/new-node-execution executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
