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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-transition-record/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptSchema  = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName = "node-placement-execution-graph-next-task-result-continuation-transition-record.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName  = "node-placement-execution-graph-next-task-result-continuation-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorMaxBytes     = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorLocks sync.Map

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                                       `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                                       `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding
// preserves the exact approved post-reconciliation route and immutable result,
// launch, scheduling, and graph evidence needed for the one local transition.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding struct {
	PolicyDecisionID                       string                                                          `json:"policy_decision_id"`
	PolicyDecisionFingerprint              string                                                          `json:"policy_decision_fingerprint"`
	PolicyRequestID                        string                                                          `json:"policy_request_id"`
	PolicyRequestFingerprint               string                                                          `json:"policy_request_fingerprint"`
	PolicyAuthenticationID                 string                                                          `json:"policy_authentication_id"`
	PolicyAuthenticationDigest             string                                                          `json:"policy_authentication_digest"`
	ReconciliationReceiptID                string                                                          `json:"reconciliation_receipt_id"`
	ReconciliationReceiptFingerprint       string                                                          `json:"reconciliation_receipt_fingerprint"`
	AcceptedResultID                       string                                                          `json:"accepted_result_id"`
	AcceptedResultFingerprint              string                                                          `json:"accepted_result_fingerprint"`
	ObservationID                          string                                                          `json:"observation_id"`
	ObservationFingerprint                 string                                                          `json:"observation_fingerprint"`
	AttemptID                              string                                                          `json:"attempt_id"`
	AttemptRecordFingerprint               string                                                          `json:"attempt_record_fingerprint"`
	ExecutorReceiptID                      string                                                          `json:"executor_receipt_id"`
	ExecutorReceiptFingerprint             string                                                          `json:"executor_receipt_fingerprint"`
	LaunchAuthorizationDecisionID          string                                                          `json:"launch_authorization_decision_id"`
	LaunchAuthorizationDecisionFingerprint string                                                          `json:"launch_authorization_decision_fingerprint"`
	LaunchAuthorizationRequestID           string                                                          `json:"launch_authorization_request_id"`
	LaunchAuthorizationRequestFingerprint  string                                                          `json:"launch_authorization_request_fingerprint"`
	SchedulingReceiptID                    string                                                          `json:"scheduling_receipt_id"`
	SchedulingReceiptFingerprint           string                                                          `json:"scheduling_receipt_fingerprint"`
	SchedulingPolicyDecisionID             string                                                          `json:"scheduling_policy_decision_id"`
	SchedulingPolicyDecisionFingerprint    string                                                          `json:"scheduling_policy_decision_fingerprint"`
	SchedulingPolicyRequestID              string                                                          `json:"scheduling_policy_request_id"`
	SchedulingPolicyRequestFingerprint     string                                                          `json:"scheduling_policy_request_fingerprint"`
	GraphRunID                             string                                                          `json:"graph_run_id"`
	TerminalTaskID                         string                                                          `json:"terminal_task_id"`
	SelectedTaskID                         string                                                          `json:"selected_task_id"`
	CandidatesFingerprint                  string                                                          `json:"candidates_fingerprint"`
	SelectedReleasedDependencyPostimage    NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate `json:"selected_released_dependency_postimage"`
	ScheduledRecordID                      string                                                          `json:"scheduled_record_id"`
	ScheduledRecordFingerprint             string                                                          `json:"scheduled_record_fingerprint"`
	ScheduledRecordVersion                 uint64                                                          `json:"scheduled_record_version"`
	TerminalResult                         string                                                          `json:"terminal_result"`
	TaskOutcome                            string                                                          `json:"task_outcome"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
// is the smallest fixture-owned local record for the exact route. It is an
// absent-to-exact postimage and never rewrites predecessor lifecycle state.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord struct {
	Schema             string                                                                        `json:"schema"`
	TransitionRecordID string                                                                        `json:"transition_record_id"`
	Binding            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding `json:"binding"`
	Route              string                                                                        `json:"route"`
	Effect             string                                                                        `json:"effect"`
	PostState          string                                                                        `json:"post_state"`
	Version            uint64                                                                        `json:"version"`
	FixtureOwned       bool                                                                          `json:"fixture_owned"`
	RecordFingerprint  string                                                                        `json:"record_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence
// describes only the local route transition already performed. Every adjacent
// capability remains false and no callback or external collaborator exists.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence struct {
	LocalRouteTransitionPerformed bool `json:"local_route_transition_performed"`
	DependencyRelease             bool `json:"dependency_release"`
	NextTaskScheduling            bool `json:"next_task_scheduling"`
	TaskLaunch                    bool `json:"task_launch"`
	NodeExecution                 bool `json:"node_execution"`
	ResultCollection              bool `json:"result_collection"`
	ResultReconciliation          bool `json:"result_reconciliation"`
	Placement                     bool `json:"placement"`
	Dispatch                      bool `json:"dispatch"`
	Connector                     bool `json:"connector"`
	Broker                        bool `json:"broker"`
	Provider                      bool `json:"provider"`
	ForgePipe                     bool `json:"forgepipe"`
	Retry                         bool `json:"retry"`
	Repair                        bool `json:"repair"`
	Cancellation                  bool `json:"cancellation"`
	Callback                      bool `json:"callback"`
	GeneralQueueProcessing        bool `json:"general_queue_processing"`
	ExternalAction                bool `json:"external_action"`
	Network                       bool `json:"network"`
	RemoteExecution               bool `json:"remote_execution"`
	Validation                    bool `json:"validation"`
	CheckoutMutation              bool `json:"checkout_mutation"`
	Git                           bool `json:"git"`
	Checkpoint                    bool `json:"checkpoint"`
	Commit                        bool `json:"commit"`
	Push                          bool `json:"push"`
	Publication                   bool `json:"publication"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
// is separate durable consumption evidence for the immutable policy request.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt struct {
	Schema                      string                                                                         `json:"schema"`
	ExecutorReceiptID           string                                                                         `json:"executor_receipt_id"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding  `json:"binding"`
	TransitionRecordID          string                                                                         `json:"transition_record_id"`
	TransitionRecordFingerprint string                                                                         `json:"transition_record_fingerprint"`
	TransitionRecordVersion     uint64                                                                         `json:"transition_record_version"`
	ExactPostState              string                                                                         `json:"exact_post_state"`
	Route                       string                                                                         `json:"route"`
	RouteSpecificEffect         string                                                                         `json:"route_specific_effect"`
	TransitionCount             uint64                                                                         `json:"transition_count"`
	RecordWriteCount            uint64                                                                         `json:"record_write_count"`
	AuthorizationConsumed       bool                                                                           `json:"authorization_consumed"`
	FixtureOwned                bool                                                                           `json:"fixture_owned"`
	Evidence                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence `json:"evidence"`
	ReceiptFingerprint          string                                                                         `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs struct {
	expected         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected
	source           nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource
	accepted         NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult
	reconciliation   NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
	decision         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision
	request          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest
	transition       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
	transitionExists bool
	receipt          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
	receiptExists    bool
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor struct {
	root                  string
	expected              NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected
	writeTransitionAtomic func(string, any) error
	writeReceiptAtomic    func(string, any) error
	mu                    sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor{
		root: root, expected: inputs.expected,
		writeTransitionAtomic: writeJSONFileAtomic,
		writeReceiptAtomic:    writeJSONFileAtomic,
	}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorLocks.LoadOrStore(executor.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.transitionExists {
		inputs.transition = deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(inputs)
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(inputs.transition, inputs); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, err
		}
		path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation graph transition record"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, err
		}
		if err := executor.writeTransitionAtomic(path, inputs.transition); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, errors.New("post-reconciliation graph transition record could not be published")
		}
		inputs.transitionExists = true
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, err
	}
	path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation graph transition executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, err
	}
	if err := executor.writeReceiptAtomic(path, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, errors.New("post-reconciliation graph transition executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected) (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs, error) {
	policy, reconciliationInputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-reconciliation graph transition executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(root, policy, reconciliationInputs)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || decision.ApprovalInferred || decision.RouteInferred || decision.InferenceSource != "" || decision.AuthenticationID != policy.DecisionAuthenticationID || decision.AuthenticationDigest != policy.DecisionAuthenticationDigest {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-reconciliation graph transition executor requires the exact approved independently authenticated policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(root, policy, reconciliationInputs, decision, true)
	narrowAuthority, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(reconciliationInputs.receipt.TaskOutcome, request.Route)
	if err != nil || !requestExists || !routeValid || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.RequestID != policy.ContinuationRequestID || request.DecisionID != decision.DecisionID || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest || request.Route != decision.Route || !nodeExecutionEqual(request.Binding, decision.Binding) || !request.OneTimeRequest || request.AuthorizationConsumed || request.GraphContinuationInvoked || request.GraphFinalizationInvoked || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned || request.Authority != narrowAuthority {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-reconciliation graph transition executor requires the exact approved unconsumed route request")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBindings(request, reconciliationInputs); err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, err
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{
		expected: expected, source: reconciliationInputs.source, accepted: reconciliationInputs.accepted,
		reconciliation: reconciliationInputs.receipt, decision: decision, request: request,
	}
	transition, transitionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, err
	}
	inputs.transition, inputs.transitionExists = transition, transitionExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, err
	}
	if receiptExists && !transitionExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs{}, errors.New("post-reconciliation graph transition receipt is orphaned from its exact transition record")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBindings(request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest, inputs nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs) error {
	accepted, receipt, executor := inputs.accepted, inputs.receipt, inputs.source.executor
	binding := request.Binding
	scheduled := executor.Binding.ScheduledRecordPostimage
	if binding.ReconciliationReceiptID != receipt.ReconciliationReceiptID || binding.ReconciliationReceiptFingerprint != receipt.ReceiptFingerprint || binding.AcceptedResultID != accepted.AcceptedResultID || binding.AcceptedResultFingerprint != accepted.AcceptedResultFingerprint || binding.ObservationID != accepted.ObservationID || binding.ObservationFingerprint != accepted.ObservationFingerprint || binding.ExecutorReceiptID != executor.ExecutorReceiptID || binding.ExecutorReceiptFingerprint != executor.ReceiptFingerprint || binding.AttemptID != executor.AttemptID || binding.AttemptRecordFingerprint != executor.AttemptRecordFingerprint || binding.AuthorizationDecisionID != executor.Binding.AuthorizationDecisionID || binding.AuthorizationDecisionFingerprint != executor.Binding.AuthorizationDecisionFingerprint || binding.AuthorizationRequestID != executor.Binding.AuthorizationRequestID || binding.AuthorizationRequestFingerprint != executor.Binding.AuthorizationRequestFingerprint || binding.SchedulingReceiptID != executor.Binding.SchedulingReceiptID || binding.SchedulingReceiptFingerprint != executor.Binding.SchedulingReceiptFingerprint || binding.GraphRunID != executor.Binding.GraphRunID || binding.TerminalTaskID != executor.Binding.TerminalTaskID || binding.SelectedTaskID != executor.Binding.SelectedTaskID || binding.CandidatesFingerprint != executor.Binding.CandidatesFingerprint || !nodeExecutionEqual(binding.SelectedReleasedDependencyPostimage, executor.Binding.SelectedReleasedDependencyPostimage) || !nodeExecutionEqual(binding.ScheduledRecordPostimage, scheduled) || binding.ScheduledRecordID != scheduled.TaskID || binding.ScheduledRecordFingerprint != scheduled.RecordFingerprint || binding.ScheduledRecordVersion != scheduled.Version || scheduled.State != "scheduled" || scheduled.TaskID != binding.SelectedTaskID || binding.TerminalResult != receipt.TerminalResult || binding.TaskOutcome != receipt.TaskOutcome {
		return errors.New("post-reconciliation graph transition executor predecessor or persisted scheduled-record binding is missing, stale, changed, or ambiguous")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding {
	policyBinding, executorBinding := inputs.request.Binding, inputs.source.executor.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding{
		PolicyDecisionID: inputs.decision.DecisionID, PolicyDecisionFingerprint: inputs.decision.DecisionFingerprint,
		PolicyRequestID: inputs.request.RequestID, PolicyRequestFingerprint: inputs.request.RequestFingerprint,
		PolicyAuthenticationID: inputs.request.AuthenticationID, PolicyAuthenticationDigest: inputs.request.AuthenticationDigest,
		ReconciliationReceiptID: policyBinding.ReconciliationReceiptID, ReconciliationReceiptFingerprint: policyBinding.ReconciliationReceiptFingerprint,
		AcceptedResultID: policyBinding.AcceptedResultID, AcceptedResultFingerprint: policyBinding.AcceptedResultFingerprint,
		ObservationID: policyBinding.ObservationID, ObservationFingerprint: policyBinding.ObservationFingerprint,
		AttemptID: policyBinding.AttemptID, AttemptRecordFingerprint: policyBinding.AttemptRecordFingerprint,
		ExecutorReceiptID: policyBinding.ExecutorReceiptID, ExecutorReceiptFingerprint: policyBinding.ExecutorReceiptFingerprint,
		LaunchAuthorizationDecisionID: policyBinding.AuthorizationDecisionID, LaunchAuthorizationDecisionFingerprint: policyBinding.AuthorizationDecisionFingerprint,
		LaunchAuthorizationRequestID: policyBinding.AuthorizationRequestID, LaunchAuthorizationRequestFingerprint: policyBinding.AuthorizationRequestFingerprint,
		SchedulingReceiptID: policyBinding.SchedulingReceiptID, SchedulingReceiptFingerprint: policyBinding.SchedulingReceiptFingerprint,
		SchedulingPolicyDecisionID: executorBinding.SchedulingPolicyDecisionID, SchedulingPolicyDecisionFingerprint: executorBinding.SchedulingPolicyDecisionFingerprint,
		SchedulingPolicyRequestID: executorBinding.SchedulingPolicyRequestID, SchedulingPolicyRequestFingerprint: executorBinding.SchedulingPolicyRequestFingerprint,
		GraphRunID: policyBinding.GraphRunID, TerminalTaskID: policyBinding.TerminalTaskID, SelectedTaskID: policyBinding.SelectedTaskID,
		CandidatesFingerprint: policyBinding.CandidatesFingerprint, SelectedReleasedDependencyPostimage: policyBinding.SelectedReleasedDependencyPostimage,
		ScheduledRecordID: policyBinding.ScheduledRecordID, ScheduledRecordFingerprint: policyBinding.ScheduledRecordFingerprint,
		ScheduledRecordVersion: policyBinding.ScheduledRecordVersion, TerminalResult: policyBinding.TerminalResult, TaskOutcome: policyBinding.TaskOutcome,
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationRouteEffect(outcome, route string) (string, string, bool) {
	switch {
	case outcome == "passed" && route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute:
		return "passed_selected_task_continued_local_graph", "continued", true
	case outcome == "passed" && route == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute:
		return "passed_result_finalized_local_graph_successfully", "succeeded", true
	case outcome == "failed" && route == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:
		return "failed_result_finalized_local_graph_with_failure_propagation", "failed", true
	default:
		return "", "", false
	}
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord {
	effect, postState, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationRouteEffect(inputs.reconciliation.TaskOutcome, inputs.request.Route)
	record := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord{
		Schema:             NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordSchema,
		TransitionRecordID: inputs.request.RequestID + "-transition", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding(inputs),
		Route: inputs.request.Route, Effect: effect, PostState: postState, Version: 1, FixtureOwned: true,
	}
	record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordFingerprint(record)
	return record
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt {
	receipt := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{
		Schema:            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptSchema,
		ExecutorReceiptID: inputs.request.RequestID + "-executor-receipt", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding(inputs),
		TransitionRecordID: inputs.transition.TransitionRecordID, TransitionRecordFingerprint: inputs.transition.RecordFingerprint,
		TransitionRecordVersion: inputs.transition.Version, ExactPostState: inputs.transition.PostState,
		Route: inputs.transition.Route, RouteSpecificEffect: inputs.transition.Effect,
		TransitionCount: 1, RecordWriteCount: 1, AuthorizationConsumed: true, FixtureOwned: true,
		Evidence: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence{LocalRouteTransitionPerformed: true},
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordFingerprint(value)
	_, _, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationRouteEffect(inputs.reconciliation.TaskOutcome, inputs.request.Route)
	if err != nil || !routeValid || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TransitionRecordID) || value.Version != 1 || !value.FixtureOwned || fingerprint != value.RecordFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("post-reconciliation graph transition record is invalid, conflicting, or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ExecutorReceiptID) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("post-reconciliation graph transition executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord{}, false, errors.New("post-reconciliation graph transition record is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, false, errors.New("post-reconciliation graph transition executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !inputs.transitionExists || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(value, inputs) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt{}, false, errors.New("post-reconciliation graph transition executor receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorMaxBytes {
		return errors.New("post-reconciliation graph transition executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("post-reconciliation graph transition executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("post-reconciliation graph transition executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
