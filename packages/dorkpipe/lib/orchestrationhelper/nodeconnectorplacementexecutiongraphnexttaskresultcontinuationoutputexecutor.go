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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordSchema          = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-record/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName          = "node-placement-execution-graph-next-task-result-continuation-output-record.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName = "node-placement-execution-graph-next-task-result-continuation-output-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorMaxBytes    = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorLocks sync.Map

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                                             `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                                             `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding
// preserves the exact output policy, completed route transition, and immutable
// result, launch, scheduling, and graph evidence consumed by the local output.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding struct {
	OutputPolicyDecisionID               string                                                                        `json:"output_policy_decision_id"`
	OutputPolicyDecisionFingerprint      string                                                                        `json:"output_policy_decision_fingerprint"`
	OutputPolicyRequestID                string                                                                        `json:"output_policy_request_id"`
	OutputPolicyRequestFingerprint       string                                                                        `json:"output_policy_request_fingerprint"`
	OutputPolicyAuthenticationID         string                                                                        `json:"output_policy_authentication_id"`
	OutputPolicyAuthenticationDigest     string                                                                        `json:"output_policy_authentication_digest"`
	TransitionExecutorReceiptID          string                                                                        `json:"transition_executor_receipt_id"`
	TransitionExecutorReceiptFingerprint string                                                                        `json:"transition_executor_receipt_fingerprint"`
	TransitionRecordID                   string                                                                        `json:"transition_record_id"`
	TransitionRecordFingerprint          string                                                                        `json:"transition_record_fingerprint"`
	TransitionRecordVersion              uint64                                                                        `json:"transition_record_version"`
	Route                                string                                                                        `json:"route"`
	PostState                            string                                                                        `json:"post_state"`
	RouteSpecificEffect                  string                                                                        `json:"route_specific_effect"`
	OutputType                           string                                                                        `json:"output_type"`
	GraphRunID                           string                                                                        `json:"graph_run_id"`
	TerminalTaskID                       string                                                                        `json:"terminal_task_id"`
	SelectedTaskID                       string                                                                        `json:"selected_task_id"`
	CandidatesFingerprint                string                                                                        `json:"candidates_fingerprint"`
	AcceptedResultID                     string                                                                        `json:"accepted_result_id"`
	AcceptedResultFingerprint            string                                                                        `json:"accepted_result_fingerprint"`
	ReconciliationReceiptID              string                                                                        `json:"reconciliation_receipt_id"`
	ReconciliationReceiptFingerprint     string                                                                        `json:"reconciliation_receipt_fingerprint"`
	TerminalResult                       string                                                                        `json:"terminal_result"`
	TaskOutcome                          string                                                                        `json:"task_outcome"`
	PriorPolicyAuthenticationID          string                                                                        `json:"prior_policy_authentication_id"`
	PriorPolicyAuthenticationDigest      string                                                                        `json:"prior_policy_authentication_digest"`
	ExecutorBinding                      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding `json:"executor_binding"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord is
// the complete fixture-owned local continuation handoff or terminal graph result.
// Presence alone grants no downstream or lifecycle authority.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord struct {
	Schema            string                                                                              `json:"schema"`
	OutputRecordID    string                                                                              `json:"output_record_id"`
	Binding           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding `json:"binding"`
	Version           uint64                                                                              `json:"version"`
	FixtureOwned      bool                                                                                `json:"fixture_owned"`
	RecordFingerprint string                                                                              `json:"record_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence
// records exactly one completed local materialization. Every future capability
// remains false and no callback, receiver, process, or external collaborator exists.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence struct {
	ContinuationHandoffMaterialized           bool `json:"continuation_handoff_materialized"`
	SuccessfulTerminalGraphResultMaterialized bool `json:"successful_terminal_graph_result_materialized"`
	FailedTerminalGraphResultMaterialized     bool `json:"failed_terminal_graph_result_materialized"`
	ReceiverInvoked                           bool `json:"receiver_invoked"`
	TerminalResultPublished                   bool `json:"terminal_result_published"`
	TerminalResultDelivered                   bool `json:"terminal_result_delivered"`
	LifecycleActionTriggered                  bool `json:"lifecycle_action_triggered"`
	GraphMutation                             bool `json:"graph_mutation"`
	DependencyRelease                         bool `json:"dependency_release"`
	FailurePropagation                        bool `json:"failure_propagation"`
	CandidateDiscovery                        bool `json:"candidate_discovery"`
	CandidateSelection                        bool `json:"candidate_selection"`
	NextTaskScheduling                        bool `json:"next_task_scheduling"`
	TaskLaunch                                bool `json:"task_launch"`
	NodeExecution                             bool `json:"node_execution"`
	ResultCollection                          bool `json:"result_collection"`
	Retry                                     bool `json:"retry"`
	Repair                                    bool `json:"repair"`
	Cancellation                              bool `json:"cancellation"`
	GeneralQueueProcessing                    bool `json:"general_queue_processing"`
	Callback                                  bool `json:"callback"`
	Connector                                 bool `json:"connector"`
	Broker                                    bool `json:"broker"`
	Provider                                  bool `json:"provider"`
	ForgePipe                                 bool `json:"forgepipe"`
	Process                                   bool `json:"process"`
	Network                                   bool `json:"network"`
	RemoteExecution                           bool `json:"remote_execution"`
	Validation                                bool `json:"validation"`
	CheckoutMutation                          bool `json:"checkout_mutation"`
	Git                                       bool `json:"git"`
	Checkpoint                                bool `json:"checkpoint"`
	Commit                                    bool `json:"commit"`
	Push                                      bool `json:"push"`
	Publication                               bool `json:"publication"`
	ExternalAction                            bool `json:"external_action"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
// is separate durable consumption evidence for the immutable output request.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt struct {
	Schema                  string                                                                               `json:"schema"`
	ExecutorReceiptID       string                                                                               `json:"executor_receipt_id"`
	Binding                 NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding  `json:"binding"`
	OutputRecordID          string                                                                               `json:"output_record_id"`
	OutputRecordFingerprint string                                                                               `json:"output_record_fingerprint"`
	OutputRecordVersion     uint64                                                                               `json:"output_record_version"`
	Route                   string                                                                               `json:"route"`
	ExactPostState          string                                                                               `json:"exact_post_state"`
	RouteSpecificEffect     string                                                                               `json:"route_specific_effect"`
	OutputType              string                                                                               `json:"output_type"`
	OutputActionCount       uint64                                                                               `json:"output_action_count"`
	OutputRecordWriteCount  uint64                                                                               `json:"output_record_write_count"`
	AuthorizationConsumed   bool                                                                                 `json:"authorization_consumed"`
	FixtureOwned            bool                                                                                 `json:"fixture_owned"`
	Evidence                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence `json:"evidence"`
	ReceiptFingerprint      string                                                                               `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs struct {
	expected          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected
	transition        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
	transitionReceipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
	decision          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision
	request           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest
	output            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	outputExists      bool
	receipt           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	receiptExists     bool
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor struct {
	root               string
	expected           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected
	writeOutputAtomic  func(string, any) error
	writeReceiptAtomic func(string, any) error
	mu                 sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor{
		root: root, expected: inputs.expected, writeOutputAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic,
	}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorLocks.LoadOrStore(executor.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.outputExists {
		inputs.output = deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(inputs)
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(inputs.output, inputs); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, err
		}
		path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-transition graph output record"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, err
		}
		if err := executor.writeOutputAtomic(path, inputs.output); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, errors.New("post-transition graph output record could not be published")
		}
		inputs.outputExists = true
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, err
	}
	path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-transition graph output executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, err
	}
	if err := executor.writeReceiptAtomic(path, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, errors.New("post-transition graph output executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs, error) {
	policy, transitionInputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("post-transition graph output executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(root, policy, transitionInputs)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || !decision.Deterministic || !decision.OneTimeDecision || decision.DecisionConsumed || decision.ApprovalInferred || decision.RouteInferred || decision.OutputTypeInferred || decision.AuthorityInferred || decision.InferenceSource != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{}) {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("post-transition graph output executor requires the exact approved independently authenticated policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(root, policy, transitionInputs, decision, true)
	outputType, authority, routeValid := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(transitionInputs.transition.Route, transitionInputs.transition.PostState, transitionInputs.transition.Effect)
	if err != nil || !requestExists || !routeValid || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.RequestID != policy.OutputRequestID || request.DecisionID != decision.DecisionID || request.DecisionReplayIdentity != decision.ReplayIdentity || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest || request.Route != decision.Route || request.OutputType != decision.OutputType || request.OutputType != outputType || !nodeExecutionEqual(request.Binding, decision.Binding) || !request.OneTimeRequest || request.AuthorizationConsumed || request.ContinuationHandoffInvoked || request.TerminalGraphResultMaterializationInvoked || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned || request.Authority != authority {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("post-transition graph output executor requires the exact approved unconsumed route-compatible output request")
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{
		expected: expected, transition: transitionInputs.transition, transitionReceipt: transitionInputs.receipt, decision: decision, request: request,
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBindings(inputs); err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, err
	}
	output, outputExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, err
	}
	inputs.output, inputs.outputExists = output, outputExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, err
	}
	if receiptExists && !outputExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("post-transition graph output receipt is orphaned from its exact output record")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBindings(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) error {
	policyBinding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(inputs.transition, inputs.transitionReceipt)
	if !nodeExecutionEqual(inputs.request.Binding, policyBinding) || !nodeExecutionEqual(inputs.decision.Binding, policyBinding) || inputs.request.DecisionID != inputs.decision.DecisionID || inputs.request.DecisionReplayIdentity != inputs.decision.ReplayIdentity || inputs.request.DecisionFingerprint != inputs.decision.DecisionFingerprint || inputs.request.AuthenticationID != inputs.decision.AuthenticationID || inputs.request.AuthenticationDigest != inputs.decision.AuthenticationDigest || inputs.transitionReceipt.TransitionRecordID != inputs.transition.TransitionRecordID || inputs.transitionReceipt.TransitionRecordFingerprint != inputs.transition.RecordFingerprint || inputs.transitionReceipt.TransitionRecordVersion != inputs.transition.Version || inputs.transitionReceipt.Route != inputs.transition.Route || inputs.transitionReceipt.ExactPostState != inputs.transition.PostState || inputs.transitionReceipt.RouteSpecificEffect != inputs.transition.Effect || !nodeExecutionEqual(inputs.transitionReceipt.Binding, inputs.transition.Binding) || inputs.transitionReceipt.TransitionCount != 1 || inputs.transitionReceipt.RecordWriteCount != 1 || !inputs.transitionReceipt.AuthorizationConsumed || !inputs.transitionReceipt.FixtureOwned || inputs.transitionReceipt.Evidence != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence{LocalRouteTransitionPerformed: true}) || !inputs.transition.FixtureOwned || inputs.transition.Version != 1 {
		return errors.New("post-transition graph output executor predecessor, transition, policy, or authentication binding is missing, stale, changed, or ambiguous")
	}
	if _, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidenceFor(inputs.request, inputs.transition); !ok {
		return errors.New("post-transition graph output executor route, output, state, effect, outcome, or authority is incompatible")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding {
	policyBinding := inputs.request.Binding
	executorBinding := policyBinding.ExecutorBinding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding{
		OutputPolicyDecisionID: inputs.decision.DecisionID, OutputPolicyDecisionFingerprint: inputs.decision.DecisionFingerprint,
		OutputPolicyRequestID: inputs.request.RequestID, OutputPolicyRequestFingerprint: inputs.request.RequestFingerprint,
		OutputPolicyAuthenticationID: inputs.request.AuthenticationID, OutputPolicyAuthenticationDigest: inputs.request.AuthenticationDigest,
		TransitionExecutorReceiptID: policyBinding.TransitionExecutorReceiptID, TransitionExecutorReceiptFingerprint: policyBinding.TransitionExecutorReceiptFingerprint,
		TransitionRecordID: policyBinding.TransitionRecordID, TransitionRecordFingerprint: policyBinding.TransitionRecordFingerprint, TransitionRecordVersion: policyBinding.TransitionRecordVersion,
		Route: policyBinding.Route, PostState: policyBinding.PostState, RouteSpecificEffect: policyBinding.RouteSpecificEffect, OutputType: inputs.request.OutputType,
		GraphRunID: executorBinding.GraphRunID, TerminalTaskID: executorBinding.TerminalTaskID, SelectedTaskID: executorBinding.SelectedTaskID, CandidatesFingerprint: executorBinding.CandidatesFingerprint,
		AcceptedResultID: executorBinding.AcceptedResultID, AcceptedResultFingerprint: executorBinding.AcceptedResultFingerprint,
		ReconciliationReceiptID: executorBinding.ReconciliationReceiptID, ReconciliationReceiptFingerprint: executorBinding.ReconciliationReceiptFingerprint,
		TerminalResult: executorBinding.TerminalResult, TaskOutcome: executorBinding.TaskOutcome,
		PriorPolicyAuthenticationID: executorBinding.PolicyAuthenticationID, PriorPolicyAuthenticationDigest: executorBinding.PolicyAuthenticationDigest,
		ExecutorBinding: executorBinding,
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidenceFor(request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest, transition NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence, bool) {
	outputType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(transition.Route, transition.PostState, transition.Effect)
	if !compatible || request.Route != transition.Route || request.OutputType != outputType || request.Authority != authority {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{}, false
	}
	outcome := transition.Binding.TaskOutcome
	terminalResult := transition.Binding.TerminalResult
	switch request.Route {
	case NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute:
		if outcome != "passed" || terminalResult != "succeeded" {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{}, false
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{ContinuationHandoffMaterialized: true}, true
	case NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute:
		if outcome != "passed" || terminalResult != "succeeded" {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{}, false
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{SuccessfulTerminalGraphResultMaterialized: true}, true
	case NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:
		if outcome != "failed" || terminalResult != "failed" {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{}, false
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{FailedTerminalGraphResultMaterialized: true}, true
	default:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{}, false
	}
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord {
	record := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord{
		Schema:         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordSchema,
		OutputRecordID: inputs.request.RequestID + "-output-record", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding(inputs),
		Version: 1, FixtureOwned: true,
	}
	record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(record)
	return record
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt {
	evidence, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidenceFor(inputs.request, inputs.transition)
	receipt := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{
		Schema:            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptSchema,
		ExecutorReceiptID: inputs.request.RequestID + "-output-executor-receipt", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding(inputs),
		OutputRecordID: inputs.output.OutputRecordID, OutputRecordFingerprint: inputs.output.RecordFingerprint, OutputRecordVersion: inputs.output.Version,
		Route: inputs.request.Route, ExactPostState: inputs.transition.PostState, RouteSpecificEffect: inputs.transition.Effect, OutputType: inputs.request.OutputType,
		OutputActionCount: 1, OutputRecordWriteCount: 1, AuthorizationConsumed: true, FixtureOwned: true, Evidence: evidence,
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.OutputRecordID) || value.Version != 1 || !value.FixtureOwned || fingerprint != value.RecordFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("post-transition graph output record is invalid, conflicting, or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ExecutorReceiptID) || value.OutputActionCount != 1 || value.OutputRecordWriteCount != 1 || !value.AuthorizationConsumed || !value.FixtureOwned || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("post-transition graph output executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord{}, false, errors.New("post-transition graph output record is malformed, noncanonical, oversized, symlinked, unsafe, partial, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, false, errors.New("post-transition graph output executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, partial, or conflicting")
	}
	if !inputs.outputExists || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(value, inputs) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt{}, false, errors.New("post-transition graph output executor receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorMaxBytes {
		return errors.New("post-transition graph output executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("post-transition graph output executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("post-transition graph output executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
