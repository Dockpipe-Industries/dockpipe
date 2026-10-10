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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-executor-attempt-record/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptSchema       = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-executor-attempt-record.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptName       = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorArtifactMaxBytes  = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorLocks sync.Map

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                                                                                      `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                                                                                      `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding
// preserves the exact policy, reconciliation, acknowledgement, delivery, consumer,
// route, outcome, and complete immutable predecessor-chain evidence.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding struct {
	PolicyDecisionID                  string                                                                                                                     `json:"policy_decision_id"`
	PolicyDecisionFingerprint         string                                                                                                                     `json:"policy_decision_fingerprint"`
	PolicyRequestID                   string                                                                                                                     `json:"policy_request_id"`
	PolicyRequestFingerprint          string                                                                                                                     `json:"policy_request_fingerprint"`
	PolicyReplayIdentity              string                                                                                                                     `json:"policy_replay_identity"`
	PolicyAuthenticationID            string                                                                                                                     `json:"policy_authentication_id"`
	PolicyAuthenticationDigest        string                                                                                                                     `json:"policy_authentication_digest"`
	ReconciliationRecordID            string                                                                                                                     `json:"reconciliation_record_id"`
	ReconciliationRecordFingerprint   string                                                                                                                     `json:"reconciliation_record_fingerprint"`
	ReconciliationRecordVersion       uint64                                                                                                                     `json:"reconciliation_record_version"`
	ReconciliationExecutorReceiptID   string                                                                                                                     `json:"reconciliation_executor_receipt_id"`
	ReconciliationExecutorFingerprint string                                                                                                                     `json:"reconciliation_executor_receipt_fingerprint"`
	AcknowledgementID                 string                                                                                                                     `json:"acknowledgement_id"`
	AcknowledgementFingerprint        string                                                                                                                     `json:"acknowledgement_fingerprint"`
	OperationKey                      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey                                   `json:"operation_key"`
	DeliveryExecutorReceiptID         string                                                                                                                     `json:"delivery_executor_receipt_id"`
	DeliveryExecutorFingerprint       string                                                                                                                     `json:"delivery_executor_receipt_fingerprint"`
	Route                             string                                                                                                                     `json:"route"`
	PostState                         string                                                                                                                     `json:"post_state"`
	RouteSpecificEffect               string                                                                                                                     `json:"route_specific_effect"`
	OutputType                        string                                                                                                                     `json:"output_type"`
	DeliveryType                      string                                                                                                                     `json:"delivery_type"`
	ConsumerID                        string                                                                                                                     `json:"consumer_id"`
	ConsumerContractFingerprint       string                                                                                                                     `json:"consumer_contract_fingerprint"`
	TerminalResult                    string                                                                                                                     `json:"terminal_result"`
	TaskOutcome                       string                                                                                                                     `json:"task_outcome"`
	PredecessorBinding                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding `json:"predecessor_binding"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence
// distinguishes the single local evidence write from every prohibited adjacent operation.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence struct {
	LocalPostReconciliationAttemptRecorded bool `json:"local_post_reconciliation_attempt_recorded"`
	LifecycleAdvanced                      bool `json:"lifecycle_advanced"`
	GraphMutated                           bool `json:"graph_mutated"`
	DependencyWorkPerformed                bool `json:"dependency_work_performed"`
	CandidateSelected                      bool `json:"candidate_selected"`
	SchedulingPerformed                    bool `json:"scheduling_performed"`
	TaskLaunched                           bool `json:"task_launched"`
	NodeExecuted                           bool `json:"node_executed"`
	ResultCollected                        bool `json:"result_collected"`
	OutputMaterialized                     bool `json:"output_materialized"`
	DeliveryPerformed                      bool `json:"delivery_performed"`
	ConsumerInvoked                        bool `json:"consumer_invoked"`
	RetryPerformed                         bool `json:"retry_performed"`
	RepairPerformed                        bool `json:"repair_performed"`
	CancellationPerformed                  bool `json:"cancellation_performed"`
	QueueProcessed                         bool `json:"queue_processed"`
	CallbackInvoked                        bool `json:"callback_invoked"`
	PublicationPerformed                   bool `json:"publication_performed"`
	ProviderUsed                           bool `json:"provider_used"`
	ConnectorUsed                          bool `json:"connector_used"`
	BrokerUsed                             bool `json:"broker_used"`
	ForgePipeUsed                          bool `json:"forgepipe_used"`
	ProcessLaunched                        bool `json:"process_launched"`
	NetworkUsed                            bool `json:"network_used"`
	RemoteExecutionPerformed               bool `json:"remote_execution_performed"`
	ValidationPerformed                    bool `json:"validation_performed"`
	CheckoutMutated                        bool `json:"checkout_mutated"`
	GitActionPerformed                     bool `json:"git_action_performed"`
	CheckpointPerformed                    bool `json:"checkpoint_performed"`
	CommitPerformed                        bool `json:"commit_performed"`
	PushPerformed                          bool `json:"push_performed"`
	ExternalActionPerformed                bool `json:"external_action_performed"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord
// records one opaque, route-compatible local attempt. It performs no adjacent action.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord struct {
	Schema            string                                                                                                                        `json:"schema"`
	AttemptID         string                                                                                                                        `json:"attempt_id"`
	Version           uint64                                                                                                                        `json:"version"`
	Binding           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding  `json:"binding"`
	AttemptType       string                                                                                                                        `json:"attempt_type"`
	Evidence          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence `json:"evidence"`
	FixtureOwned      bool                                                                                                                          `json:"fixture_owned"`
	Authority         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority  `json:"authority"`
	RecordFingerprint string                                                                                                                        `json:"record_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt
// proves consumption of one exact request without mutating that request or granting future authority.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt struct {
	Schema                                       string                                                                                                                        `json:"schema"`
	ExecutorReceiptID                            string                                                                                                                        `json:"executor_receipt_id"`
	Binding                                      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding  `json:"binding"`
	AttemptID                                    string                                                                                                                        `json:"attempt_id"`
	AttemptRecordFingerprint                     string                                                                                                                        `json:"attempt_record_fingerprint"`
	AttemptType                                  string                                                                                                                        `json:"attempt_type"`
	LogicalLocalPostReconciliationAttemptCount   uint64                                                                                                                        `json:"logical_local_post_reconciliation_attempt_count"`
	AttemptRecordWriteCount                      uint64                                                                                                                        `json:"attempt_record_write_count"`
	ExecutorReceiptWriteCount                    uint64                                                                                                                        `json:"executor_receipt_write_count"`
	AuthorizationConsumed                        bool                                                                                                                          `json:"authorization_consumed"`
	ConsumedAuthority                            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority  `json:"consumed_authority"`
	CompleteImmutablePredecessorChainRevalidated bool                                                                                                                          `json:"complete_immutable_predecessor_chain_revalidated"`
	NoDuplicateAttempt                           bool                                                                                                                          `json:"no_duplicate_attempt"`
	RequestUnchanged                             bool                                                                                                                          `json:"request_unchanged"`
	FixtureOwned                                 bool                                                                                                                          `json:"fixture_owned"`
	Evidence                                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence `json:"evidence"`
	Authority                                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority  `json:"authority"`
	ReceiptFingerprint                           string                                                                                                                        `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs struct {
	expected      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected
	record        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
	reconReceipt  NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	decision      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision
	request       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest
	attemptType   string
	consumed      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority
	attempt       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord
	attemptExists bool
	receipt       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt
	receiptExists bool
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor struct {
	root               string
	expected           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected
	writeAttemptAtomic func(string, any) error
	writeReceiptAtomic func(string, any) error
	mu                 sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor{root: root, expected: inputs.expected, writeAttemptAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorLocks.LoadOrStore(executor.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.attemptExists {
		inputs.attempt = deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(inputs)
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(inputs.attempt, inputs); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, err
		}
		path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation executor attempt record"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, err
		}
		if err := executor.writeAttemptAtomic(path, inputs.attempt); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, errors.New("post-reconciliation executor attempt record could not be published")
		}
		inputs.attemptExists = true
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, err
	}
	path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, err
	}
	if err := executor.writeReceiptAtomic(path, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}, errors.New("post-reconciliation executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected) (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs, error) {
	policy, reconciliationInputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, errors.New("post-reconciliation executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(root, policy, reconciliationInputs)
	if err != nil || !decisionExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, errors.New("post-reconciliation executor requires the exact approved independently authenticated policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(root, policy, reconciliationInputs, decision, true)
	if err != nil || !requestExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, errors.New("post-reconciliation executor requires the exact approved unconsumed fixture-owned request")
	}
	attemptType, consumed, err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorPolicyEvidence(expected, reconciliationInputs.record, reconciliationInputs.receipt, decision, request)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, err
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{expected: expected, record: reconciliationInputs.record, reconReceipt: reconciliationInputs.receipt, decision: decision, request: request, attemptType: attemptType, consumed: consumed}
	attempt, attemptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, err
	}
	inputs.attempt, inputs.attemptExists = attempt, attemptExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, err
	}
	if receiptExists && !attemptExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{}, errors.New("post-reconciliation executor receipt is orphaned from its exact attempt record")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorPolicyEvidence(expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) (string, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority, error) {
	if decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.Deterministic || !decision.OneTimeDecision || decision.DecisionConsumed || decision.ApprovalInferred || decision.RouteInferred || decision.ConsumerInferred || decision.ReconciliationInferred || decision.FutureAuthorityInferred || decision.InferenceSource != "" || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) {
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, errors.New("post-reconciliation executor requires the exact approved independently authenticated policy decision")
	}
	if request.RequestFingerprint != expected.PolicyRequestFingerprint || !request.OneTimeRequest || request.AuthorizationConsumed || request.LifecycleAdvanced || request.GraphMutated || request.DependencyWorkPerformed || request.SchedulingPerformed || request.ExecutionPerformed || request.DeliveryPerformed || request.ConsumerInvoked || request.CallbackInvoked || request.PublicationPerformed || request.NetworkUsed || request.GitActionPerformed || request.ExternalActionPerformed || !request.FixtureOwned {
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, errors.New("post-reconciliation executor requires the exact approved unconsumed fixture-owned request")
	}
	attemptType, consumed, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptAuthority(request)
	if !ok {
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, errors.New("post-reconciliation executor request authority is empty, mixed, inferred, or route-incompatible")
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs{expected: expected, record: record, reconReceipt: receipt, decision: decision, request: request, attemptType: attemptType, consumed: consumed}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBindings(inputs); err != nil {
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, err
	}
	return attemptType, consumed, nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptAuthority(request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) (string, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority, bool) {
	expected, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(request.Binding)
	if !compatible || request.Authority != expected {
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, false
	}
	switch expected {
	case NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{ContinuationHandoffPostReconciliationAttempt: true}:
		return "continuation_handoff_post_reconciliation_attempt", expected, true
	case NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{SuccessfulTerminalGraphResultPostReconciliationAttempt: true}:
		return "successful_terminal_graph_result_post_reconciliation_attempt", expected, true
	case NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{FailedTerminalGraphResultPostReconciliationAttempt: true}:
		return "failed_terminal_graph_result_post_reconciliation_attempt", expected, true
	default:
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, false
	}
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBindings(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(inputs.record, inputs.reconReceipt)
	r := inputs.request
	if !nodeExecutionEqual(r.Binding, binding) || r.DecisionID != inputs.decision.DecisionID || r.DecisionReplayIdentity != inputs.decision.ReplayIdentity || r.DecisionFingerprint != inputs.decision.DecisionFingerprint || r.AuthenticationID != inputs.decision.AuthenticationID || r.AuthenticationDigest != inputs.decision.AuthenticationDigest || r.ReconciliationRecordID != inputs.record.ReconciliationRecordID || r.ReconciliationRecordFingerprint != inputs.record.RecordFingerprint || r.ReconciliationExecutorReceiptID != inputs.reconReceipt.ExecutorReceiptID || r.ReconciliationExecutorFingerprint != inputs.reconReceipt.ReceiptFingerprint || r.Route != binding.Route || r.PostState != binding.PostState || r.RouteSpecificEffect != binding.RouteSpecificEffect || r.OutputType != binding.OutputType || r.DeliveryType != binding.DeliveryType || r.ConsumerID != binding.ConsumerID || r.ConsumerContractFingerprint != binding.ConsumerContractFingerprint || r.TerminalResult != binding.TerminalResult || r.TaskOutcome != binding.TaskOutcome || !binding.AcknowledgementAccepted || binding.AcceptedLocalConsumerDeliveryCount != 1 || binding.OperationKey == (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey{}) || inputs.record.Version != 1 || inputs.reconReceipt.ReconciliationRecordVersion != 1 {
		return errors.New("post-reconciliation executor evidence is missing, stale, ambiguous, changed, or escalates authority")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding {
	b := inputs.request.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding{PolicyDecisionID: inputs.decision.DecisionID, PolicyDecisionFingerprint: inputs.decision.DecisionFingerprint, PolicyRequestID: inputs.request.RequestID, PolicyRequestFingerprint: inputs.request.RequestFingerprint, PolicyReplayIdentity: inputs.decision.ReplayIdentity, PolicyAuthenticationID: inputs.decision.AuthenticationID, PolicyAuthenticationDigest: inputs.decision.AuthenticationDigest, ReconciliationRecordID: inputs.record.ReconciliationRecordID, ReconciliationRecordFingerprint: inputs.record.RecordFingerprint, ReconciliationRecordVersion: inputs.record.Version, ReconciliationExecutorReceiptID: inputs.reconReceipt.ExecutorReceiptID, ReconciliationExecutorFingerprint: inputs.reconReceipt.ReceiptFingerprint, AcknowledgementID: b.AcknowledgementID, AcknowledgementFingerprint: b.AcknowledgementFingerprint, OperationKey: b.OperationKey, DeliveryExecutorReceiptID: b.DeliveryExecutorReceiptID, DeliveryExecutorFingerprint: b.DeliveryExecutorReceiptFingerprint, Route: b.Route, PostState: b.PostState, RouteSpecificEffect: b.RouteSpecificEffect, OutputType: b.OutputType, DeliveryType: b.DeliveryType, ConsumerID: b.ConsumerID, ConsumerContractFingerprint: b.ConsumerContractFingerprint, TerminalResult: b.TerminalResult, TaskOutcome: b.TaskOutcome, PredecessorBinding: b}
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord {
	attempt := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordSchema, AttemptID: inputs.request.RequestID + "-attempt", Version: 1, Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding(inputs), AttemptType: inputs.attemptType, Evidence: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence{LocalPostReconciliationAttemptRecorded: true}, FixtureOwned: true}
	attempt.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptFingerprint(attempt)
	return attempt
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt {
	receipt := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptSchema, ExecutorReceiptID: inputs.request.RequestID + "-executor-receipt", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding(inputs), AttemptID: inputs.attempt.AttemptID, AttemptRecordFingerprint: inputs.attempt.RecordFingerprint, AttemptType: inputs.attemptType, LogicalLocalPostReconciliationAttemptCount: 1, AttemptRecordWriteCount: 1, ExecutorReceiptWriteCount: 1, AuthorizationConsumed: true, ConsumedAuthority: inputs.consumed, CompleteImmutablePredecessorChainRevalidated: true, NoDuplicateAttempt: true, RequestUnchanged: true, FixtureOwned: true, Evidence: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence{LocalPostReconciliationAttemptRecorded: true}}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.AttemptID) || value.Version != 1 || fingerprint != value.RecordFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("post-reconciliation executor attempt record is invalid, conflicting, or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ExecutorReceiptID) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("post-reconciliation executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("post-reconciliation executor attempt record is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(value, inputs); err != nil {
		return value, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("post-reconciliation executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !inputs.attemptExists || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(value, inputs) != nil {
		return value, false, errors.New("post-reconciliation executor receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorArtifactMaxBytes {
		return errors.New("post-reconciliation executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("post-reconciliation executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("post-reconciliation executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
