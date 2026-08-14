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
	NodeConnectorPlacementExecutionGraphNextTaskResultObservationSchema    = "dorkpipe.node-placement-execution-graph-next-task-result-observation-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskAcceptedResultSchema       = "dorkpipe.node-placement-execution-graph-next-task-accepted-result/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSchema = "dorkpipe.node-placement-execution-graph-next-task-result-reconciliation-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultObservationName                = "node-placement-execution-graph-next-task-result-observation.json"
	nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName                   = "node-placement-execution-graph-next-task-accepted-result.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptName      = "node-placement-execution-graph-next-task-result-reconciliation-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationArtifactMaxBytes = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationLocks sync.Map

type NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected struct {
	Executor                   NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected `json:"executor"`
	ExecutorReceiptFingerprint string                                                                      `json:"executor_receipt_fingerprint"`
	ObservationID              string                                                                      `json:"observation_id"`
	ReplayIdentity             string                                                                      `json:"replay_identity"`
	AuthenticationID           string                                                                      `json:"authentication_id"`
	AuthenticationDigest       string                                                                      `json:"authentication_digest"`
	AcceptedResultID           string                                                                      `json:"accepted_result_id"`
	ReconciliationReceiptID    string                                                                      `json:"reconciliation_receipt_id"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultAuthority is deliberately
// all-negative. Result evidence and reconciliation never grant adjacent lifecycle authority.
type NodeConnectorPlacementExecutionGraphNextTaskResultAuthority struct {
	TaskProcess         bool `json:"task_process"`
	TaskLaunch          bool `json:"task_launch"`
	NodeExecution       bool `json:"node_execution"`
	ExecutionReceipt    bool `json:"execution_receipt"`
	GraphCompletion     bool `json:"graph_completion"`
	GraphFailure        bool `json:"graph_failure"`
	GraphProgress       bool `json:"graph_progress"`
	DependencyRelease   bool `json:"dependency_release"`
	NextTaskScheduling  bool `json:"next_task_scheduling"`
	Placement           bool `json:"placement"`
	Dispatch            bool `json:"dispatch"`
	Connector           bool `json:"connector"`
	Lease               bool `json:"lease"`
	Broker              bool `json:"broker"`
	Provider            bool `json:"provider"`
	ForgePipe           bool `json:"forgepipe"`
	Callback            bool `json:"callback"`
	ExternalAction      bool `json:"external_action"`
	Network             bool `json:"network"`
	RemoteExecution     bool `json:"remote_execution"`
	Validation          bool `json:"validation"`
	CheckoutMutation    bool `json:"checkout_mutation"`
	Git                 bool `json:"git"`
	Retry               bool `json:"retry"`
	Repair              bool `json:"repair"`
	Cancellation        bool `json:"cancellation"`
	Publication         bool `json:"publication"`
	LifecycleTransition bool `json:"lifecycle_transition"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultObservation is the sole
// source of terminal result. It is immutable fixture input, not executor evidence.
type NodeConnectorPlacementExecutionGraphNextTaskResultObservation struct {
	Schema                              string                                                          `json:"schema"`
	ObservationID                       string                                                          `json:"observation_id"`
	ReplayIdentity                      string                                                          `json:"replay_identity"`
	AuthenticationID                    string                                                          `json:"authentication_id"`
	AuthenticationDigest                string                                                          `json:"authentication_digest"`
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
	ScheduledRecordFingerprint          string                                                          `json:"scheduled_record_fingerprint"`
	ScheduledRecordVersion              uint64                                                          `json:"scheduled_record_version"`
	PredecessorBindingFingerprint       string                                                          `json:"predecessor_binding_fingerprint"`
	TerminalResult                      string                                                          `json:"terminal_result"`
	Deterministic                       bool                                                            `json:"deterministic"`
	OneTimeObservation                  bool                                                            `json:"one_time_observation"`
	ObservationConsumed                 bool                                                            `json:"observation_consumed"`
	ResultInferred                      bool                                                            `json:"result_inferred"`
	InferenceSource                     string                                                          `json:"inference_source,omitempty"`
	FixtureOwned                        bool                                                            `json:"fixture_owned"`
	Authority                           NodeConnectorPlacementExecutionGraphNextTaskResultAuthority     `json:"authority"`
	ObservationFingerprint              string                                                          `json:"observation_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult struct {
	Schema                              string                                                          `json:"schema"`
	AcceptedResultID                    string                                                          `json:"accepted_result_id"`
	ObservationID                       string                                                          `json:"observation_id"`
	ReplayIdentity                      string                                                          `json:"replay_identity"`
	ObservationFingerprint              string                                                          `json:"observation_fingerprint"`
	AuthenticationID                    string                                                          `json:"authentication_id"`
	AuthenticationDigest                string                                                          `json:"authentication_digest"`
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
	ScheduledRecordFingerprint          string                                                          `json:"scheduled_record_fingerprint"`
	ScheduledRecordVersion              uint64                                                          `json:"scheduled_record_version"`
	PredecessorBindingFingerprint       string                                                          `json:"predecessor_binding_fingerprint"`
	TerminalResult                      string                                                          `json:"terminal_result"`
	ResultIngestionCount                uint64                                                          `json:"result_ingestion_count"`
	FixtureOwned                        bool                                                            `json:"fixture_owned"`
	Authority                           NodeConnectorPlacementExecutionGraphNextTaskResultAuthority     `json:"authority"`
	AcceptedResultFingerprint           string                                                          `json:"accepted_result_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
// proves only durable result ingestion and task-level outcome interpretation.
type NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt struct {
	Schema                            string                                                      `json:"schema"`
	ReconciliationReceiptID           string                                                      `json:"reconciliation_receipt_id"`
	AcceptedResultID                  string                                                      `json:"accepted_result_id"`
	AcceptedResultFingerprint         string                                                      `json:"accepted_result_fingerprint"`
	ObservationID                     string                                                      `json:"observation_id"`
	ReplayIdentity                    string                                                      `json:"replay_identity"`
	ObservationFingerprint            string                                                      `json:"observation_fingerprint"`
	ExecutorReceiptID                 string                                                      `json:"executor_receipt_id"`
	ExecutorReceiptFingerprint        string                                                      `json:"executor_receipt_fingerprint"`
	AttemptID                         string                                                      `json:"attempt_id"`
	AttemptRecordFingerprint          string                                                      `json:"attempt_record_fingerprint"`
	GraphRunID                        string                                                      `json:"graph_run_id"`
	TerminalTaskID                    string                                                      `json:"terminal_task_id"`
	SelectedTaskID                    string                                                      `json:"selected_task_id"`
	ScheduledRecordFingerprint        string                                                      `json:"scheduled_record_fingerprint"`
	ScheduledRecordVersion            uint64                                                      `json:"scheduled_record_version"`
	TerminalResult                    string                                                      `json:"terminal_result"`
	TaskOutcome                       string                                                      `json:"task_outcome"`
	ResultIngestionCount              uint64                                                      `json:"result_ingestion_count"`
	AcceptedResultWriteCount          uint64                                                      `json:"accepted_result_write_count"`
	ReconciliationWriteCount          uint64                                                      `json:"reconciliation_write_count"`
	ObservationConsumed               bool                                                        `json:"observation_consumed"`
	CompleteImmutableChainRevalidated bool                                                        `json:"complete_immutable_chain_revalidated"`
	TaskLevelResultOutcomeReconciled  bool                                                        `json:"task_level_result_outcome_reconciled"`
	GraphCompletionClaimed            bool                                                        `json:"graph_completion_claimed"`
	GraphFailurePropagated            bool                                                        `json:"graph_failure_propagated"`
	GraphProgressClaimed              bool                                                        `json:"graph_progress_claimed"`
	DependencyReleased                bool                                                        `json:"dependency_released"`
	NextTaskScheduled                 bool                                                        `json:"next_task_scheduled"`
	ExecutionInvoked                  bool                                                        `json:"execution_invoked"`
	CallbackInvoked                   bool                                                        `json:"callback_invoked"`
	ExternalActionInvoked             bool                                                        `json:"external_action_invoked"`
	FixtureOwned                      bool                                                        `json:"fixture_owned"`
	Authority                         NodeConnectorPlacementExecutionGraphNextTaskResultAuthority `json:"authority"`
	ReceiptFingerprint                string                                                      `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource struct {
	expected    NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected
	attempt     NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord
	executor    NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt
	observation NodeConnectorPlacementExecutionGraphNextTaskResultObservation
}

type nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs struct {
	source         nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource
	accepted       NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult
	acceptedExists bool
	receipt        NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
	receiptExists  bool
}

type NodeConnectorPlacementExecutionGraphNextTaskResultReconciler struct {
	root                string
	expected            NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected
	source              nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource
	writeAcceptedAtomic func(string, any) error
	writeReceiptAtomic  func(string, any) error
	mu                  sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultReconciler, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskResultReconciler{
		root: root, expected: inputs.source.expected, source: inputs.source,
		writeAcceptedAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic,
	}, nil
}

func (reconciler *NodeConnectorPlacementExecutionGraphNextTaskResultReconciler) Reconcile() (NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, error) {
	reconciler.mu.Lock()
	defer reconciler.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationLocks.LoadOrStore(reconciler.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(reconciler.root, reconciler.expected)
	if err != nil || !nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSourceEqual(inputs.source, reconciler.source) {
		return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, errors.New("next-task result reconciliation could not revalidate the exact immutable attempt, receipt, observation, and predecessor chain")
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(inputs.accepted), cloneNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(inputs.receipt), nil
	}
	if !inputs.acceptedExists {
		inputs.accepted = deriveNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(inputs.source)
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(inputs.accepted, inputs.source); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
		}
		path := filepath.Join(reconciler.root, nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "next-task accepted result"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
		}
		if err := reconciler.writeAcceptedAtomic(path, inputs.accepted); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, errors.New("next-task accepted result could not be published")
		}
		inputs.acceptedExists = true
	}
	receipt, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(inputs.source, inputs.accepted)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
	}
	path := filepath.Join(reconciler.root, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "next-task result reconciliation receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
	}
	if err := reconciler.writeReceiptAtomic(path, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, errors.New("next-task result reconciliation receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(inputs.accepted), cloneNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(receipt), nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSourceEqual(left, right nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource) bool {
	return nodeExecutionEqual(left.expected, right.expected) &&
		nodeExecutionEqual(left.attempt, right.attempt) &&
		nodeExecutionEqual(left.executor, right.executor) &&
		nodeExecutionEqual(left.observation, right.observation)
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) (nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs, error) {
	executorInputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(root, expected.Executor)
	if err != nil || !executorInputs.attemptExists || !executorInputs.receiptExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("next-task result reconciliation requires the exact attempt record, executor receipt, and complete immutable predecessor chain")
	}
	expected.Executor = executorInputs.expected
	if !validNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected(expected) || executorInputs.receipt.ReceiptFingerprint != expected.ExecutorReceiptFingerprint {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("next-task result reconciliation expected identities, authentication, or executor receipt binding is invalid")
	}
	wantExecutorEvidence := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorEvidence{LocalAttemptMaterialized: true}
	if executorInputs.receipt.AttemptCount != 1 || executorInputs.receipt.AttemptRecordWriteCount != 1 || !executorInputs.receipt.AuthorizationConsumed || !executorInputs.receipt.FixtureOwned || executorInputs.receipt.Evidence != wantExecutorEvidence || !executorInputs.attempt.AttemptMaterialized || !executorInputs.attempt.FixtureOwned || executorInputs.receipt.AttemptID != executorInputs.attempt.AttemptID || executorInputs.receipt.AttemptRecordFingerprint != executorInputs.attempt.RecordFingerprint || !nodeExecutionEqual(executorInputs.receipt.Binding, executorInputs.attempt.Binding) {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("next-task result reconciliation executor evidence is ambiguous, incomplete, or claims adjacent authority")
	}
	source := nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource{expected: expected, attempt: executorInputs.attempt, executor: executorInputs.receipt}
	observation, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultObservation(root, source)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, err
	}
	source.observation = observation
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{source: source}
	accepted, acceptedExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(root, source)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, err
	}
	inputs.accepted, inputs.acceptedExists = accepted, acceptedExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(root, source, accepted, acceptedExists)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, err
	}
	if receiptExists && !acceptedExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs{}, errors.New("next-task result reconciliation receipt is orphaned from its exact accepted result")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected(value NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) bool {
	ids := []string{value.ObservationID, value.ReplayIdentity, value.AuthenticationID, value.AcceptedResultID, value.ReconciliationReceiptID}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return nodeExecutionFingerprint.MatchString(value.ExecutorReceiptFingerprint) && nodeExecutionFingerprint.MatchString(value.AuthenticationDigest)
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultObservation(value NodeConnectorPlacementExecutionGraphNextTaskResultObservation, source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource) error {
	binding := source.executor.Binding
	bindingFingerprint, err := nodeExecutionFingerprintValue(binding)
	fingerprint, fingerprintErr := nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(value)
	terminalValid := value.TerminalResult == "succeeded" || value.TerminalResult == "failed"
	if err != nil || fingerprintErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultObservationSchema || value.ObservationID != source.expected.ObservationID || value.ReplayIdentity != source.expected.ReplayIdentity || value.AuthenticationID != source.expected.AuthenticationID || value.AuthenticationDigest != source.expected.AuthenticationDigest || value.ExecutorReceiptID != source.executor.ExecutorReceiptID || value.ExecutorReceiptFingerprint != source.executor.ReceiptFingerprint || value.AttemptID != source.attempt.AttemptID || value.AttemptRecordFingerprint != source.attempt.RecordFingerprint || value.AuthorizationDecisionID != binding.AuthorizationDecisionID || value.AuthorizationDecisionFingerprint != binding.AuthorizationDecisionFingerprint || value.AuthorizationRequestID != binding.AuthorizationRequestID || value.AuthorizationRequestFingerprint != binding.AuthorizationRequestFingerprint || value.SchedulingReceiptID != binding.SchedulingReceiptID || value.SchedulingReceiptFingerprint != binding.SchedulingReceiptFingerprint || value.GraphRunID != binding.GraphRunID || value.TerminalTaskID != binding.TerminalTaskID || value.SelectedTaskID != binding.SelectedTaskID || value.CandidatesFingerprint != binding.CandidatesFingerprint || !nodeExecutionEqual(value.SelectedReleasedDependencyPostimage, binding.SelectedReleasedDependencyPostimage) || !nodeExecutionEqual(value.ScheduledRecordPostimage, binding.ScheduledRecordPostimage) || value.ScheduledRecordFingerprint != binding.ScheduledRecordPostimageFingerprint || value.ScheduledRecordVersion != binding.ScheduledRecordPostimageVersion || value.PredecessorBindingFingerprint != bindingFingerprint || !terminalValid || !value.Deterministic || !value.OneTimeObservation || value.ObservationConsumed || value.ResultInferred || value.InferenceSource != "" || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) || fingerprint != value.ObservationFingerprint {
		return errors.New("next-task result observation is unauthenticated, inferred, consumed, replayed, ambiguous, mismatched, or escalates authority")
	}
	return nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource) NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult {
	o, b := source.observation, source.executor.Binding
	result := NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskAcceptedResultSchema, AcceptedResultID: source.expected.AcceptedResultID,
		ObservationID: o.ObservationID, ReplayIdentity: o.ReplayIdentity, ObservationFingerprint: o.ObservationFingerprint,
		AuthenticationID: o.AuthenticationID, AuthenticationDigest: o.AuthenticationDigest,
		ExecutorReceiptID: source.executor.ExecutorReceiptID, ExecutorReceiptFingerprint: source.executor.ReceiptFingerprint,
		AttemptID: source.attempt.AttemptID, AttemptRecordFingerprint: source.attempt.RecordFingerprint,
		AuthorizationDecisionID: b.AuthorizationDecisionID, AuthorizationDecisionFingerprint: b.AuthorizationDecisionFingerprint,
		AuthorizationRequestID: b.AuthorizationRequestID, AuthorizationRequestFingerprint: b.AuthorizationRequestFingerprint,
		SchedulingReceiptID: b.SchedulingReceiptID, SchedulingReceiptFingerprint: b.SchedulingReceiptFingerprint,
		GraphRunID: b.GraphRunID, TerminalTaskID: b.TerminalTaskID, SelectedTaskID: b.SelectedTaskID, CandidatesFingerprint: b.CandidatesFingerprint,
		SelectedReleasedDependencyPostimage: b.SelectedReleasedDependencyPostimage, ScheduledRecordPostimage: b.ScheduledRecordPostimage,
		ScheduledRecordFingerprint: b.ScheduledRecordPostimageFingerprint, ScheduledRecordVersion: b.ScheduledRecordPostimageVersion,
		PredecessorBindingFingerprint: o.PredecessorBindingFingerprint, TerminalResult: o.TerminalResult, ResultIngestionCount: 1, FixtureOwned: true,
	}
	result.AcceptedResultFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultFingerprint(result)
	return result
}

func validateNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(value NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(source)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultFingerprint(value)
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskAcceptedResultSchema || value.ResultIngestionCount != 1 || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) || fingerprint != value.AcceptedResultFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("next-task accepted result is invalid, conflicting, orphaned, or escalates authority")
	}
	return nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult) (NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, error) {
	outcome, err := nodeConnectorPlacementExecutionGraphNextTaskResultOutcome(accepted.TerminalResult)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
	}
	receipt := NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSchema, ReconciliationReceiptID: source.expected.ReconciliationReceiptID,
		AcceptedResultID: accepted.AcceptedResultID, AcceptedResultFingerprint: accepted.AcceptedResultFingerprint,
		ObservationID: accepted.ObservationID, ReplayIdentity: accepted.ReplayIdentity, ObservationFingerprint: accepted.ObservationFingerprint,
		ExecutorReceiptID: accepted.ExecutorReceiptID, ExecutorReceiptFingerprint: accepted.ExecutorReceiptFingerprint,
		AttemptID: accepted.AttemptID, AttemptRecordFingerprint: accepted.AttemptRecordFingerprint,
		GraphRunID: accepted.GraphRunID, TerminalTaskID: accepted.TerminalTaskID, SelectedTaskID: accepted.SelectedTaskID,
		ScheduledRecordFingerprint: accepted.ScheduledRecordFingerprint, ScheduledRecordVersion: accepted.ScheduledRecordVersion,
		TerminalResult: accepted.TerminalResult, TaskOutcome: outcome,
		ResultIngestionCount: 1, AcceptedResultWriteCount: 1, ReconciliationWriteCount: 1, ObservationConsumed: true,
		CompleteImmutableChainRevalidated: true, TaskLevelResultOutcomeReconciled: true, FixtureOwned: true,
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptFingerprint(receipt)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(receipt, source, accepted); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
	}
	return receipt, nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultOutcome(result string) (string, error) {
	switch result {
	case "succeeded":
		return "passed", nil
	case "failed":
		return "failed", nil
	default:
		return "", errors.New("next-task result reconciliation supports only explicit succeeded or failed terminal results")
	}
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult) error {
	expected, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptUnchecked(source, accepted)
	fingerprint, fingerprintErr := nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptFingerprint(value)
	if err != nil || fingerprintErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSchema || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) || !value.FixtureOwned || !value.ObservationConsumed || !value.CompleteImmutableChainRevalidated || !value.TaskLevelResultOutcomeReconciled || value.GraphCompletionClaimed || value.GraphFailurePropagated || value.GraphProgressClaimed || value.DependencyReleased || value.NextTaskScheduled || value.ExecutionInvoked || value.CallbackInvoked || value.ExternalActionInvoked || value.ResultIngestionCount != 1 || value.AcceptedResultWriteCount != 1 || value.ReconciliationWriteCount != 1 || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("next-task result reconciliation receipt is invalid, conflicting, orphaned, or escalates authority")
	}
	return nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptUnchecked(source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult) (NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, error) {
	outcome, err := nodeConnectorPlacementExecutionGraphNextTaskResultOutcome(accepted.TerminalResult)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, err
	}
	value := NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSchema, ReconciliationReceiptID: source.expected.ReconciliationReceiptID,
		AcceptedResultID: accepted.AcceptedResultID, AcceptedResultFingerprint: accepted.AcceptedResultFingerprint,
		ObservationID: accepted.ObservationID, ReplayIdentity: accepted.ReplayIdentity, ObservationFingerprint: accepted.ObservationFingerprint,
		ExecutorReceiptID: accepted.ExecutorReceiptID, ExecutorReceiptFingerprint: accepted.ExecutorReceiptFingerprint,
		AttemptID: accepted.AttemptID, AttemptRecordFingerprint: accepted.AttemptRecordFingerprint,
		GraphRunID: accepted.GraphRunID, TerminalTaskID: accepted.TerminalTaskID, SelectedTaskID: accepted.SelectedTaskID,
		ScheduledRecordFingerprint: accepted.ScheduledRecordFingerprint, ScheduledRecordVersion: accepted.ScheduledRecordVersion,
		TerminalResult: accepted.TerminalResult, TaskOutcome: outcome,
		ResultIngestionCount: 1, AcceptedResultWriteCount: 1, ReconciliationWriteCount: 1, ObservationConsumed: true,
		CompleteImmutableChainRevalidated: true, TaskLevelResultOutcomeReconciled: true, FixtureOwned: true,
	}
	value.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptFingerprint(value)
	return value, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultObservation(root string, source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource) (NodeConnectorPlacementExecutionGraphNextTaskResultObservation, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultObservationName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultObservation
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationCanonicalArtifact(root, path, &value, false); err != nil || validateNodeConnectorPlacementExecutionGraphNextTaskResultObservation(value, source) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultObservation{}, errors.New("next-task result observation is missing, malformed, noncanonical, oversized, symlinked, unsafe, unauthenticated, replayed, tampered, or conflicting")
	}
	return value, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(root string, source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource) (NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName)
	var value NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, false, errors.New("next-task accepted result is malformed, noncanonical, oversized, symlinked, unsafe, tampered, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(value, source); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(root string, source nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationSource, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, acceptedExists bool) (NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, false, errors.New("next-task result reconciliation receipt is malformed, noncanonical, oversized, symlinked, unsafe, tampered, or conflicting")
	}
	if !acceptedExists || validateNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(value, source, accepted) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt{}, false, errors.New("next-task result reconciliation receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationArtifactMaxBytes {
		return errors.New("next-task result reconciliation artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("next-task result reconciliation artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("next-task result reconciliation artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultObservation) (string, error) {
	value.ObservationFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult) (string, error) {
	value.AcceptedResultFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(value NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult) NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
