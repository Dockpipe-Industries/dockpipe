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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordSchema          = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-record/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName          = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-record.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorMaxBytes    = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorLocks sync.Map

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                                                                                  `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                                                                                  `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding
// preserves the exact policy, acknowledgement, delivery receipt, consumer, route, and complete immutable predecessor chain.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding struct {
	ReconciliationPolicyDecisionID          string                                                                                                                 `json:"reconciliation_policy_decision_id"`
	ReconciliationPolicyDecisionFingerprint string                                                                                                                 `json:"reconciliation_policy_decision_fingerprint"`
	ReconciliationPolicyRequestID           string                                                                                                                 `json:"reconciliation_policy_request_id"`
	ReconciliationPolicyRequestFingerprint  string                                                                                                                 `json:"reconciliation_policy_request_fingerprint"`
	DecisionReplayIdentity                  string                                                                                                                 `json:"decision_replay_identity"`
	DecisionAuthenticationID                string                                                                                                                 `json:"decision_authentication_id"`
	DecisionAuthenticationDigest            string                                                                                                                 `json:"decision_authentication_digest"`
	AcknowledgementID                       string                                                                                                                 `json:"acknowledgement_id"`
	AcknowledgementFingerprint              string                                                                                                                 `json:"acknowledgement_fingerprint"`
	OperationKey                            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey                               `json:"operation_key"`
	DeliveryExecutorReceiptID               string                                                                                                                 `json:"delivery_executor_receipt_id"`
	DeliveryExecutorReceiptFingerprint      string                                                                                                                 `json:"delivery_executor_receipt_fingerprint"`
	Route                                   string                                                                                                                 `json:"route"`
	PostState                               string                                                                                                                 `json:"post_state"`
	RouteSpecificEffect                     string                                                                                                                 `json:"route_specific_effect"`
	OutputType                              string                                                                                                                 `json:"output_type"`
	DeliveryType                            string                                                                                                                 `json:"delivery_type"`
	ConsumerID                              string                                                                                                                 `json:"consumer_id"`
	ConsumerContractFingerprint             string                                                                                                                 `json:"consumer_contract_fingerprint"`
	GraphRunID                              string                                                                                                                 `json:"graph_run_id"`
	TerminalTaskID                          string                                                                                                                 `json:"terminal_task_id"`
	SelectedTaskID                          string                                                                                                                 `json:"selected_task_id"`
	CandidatesFingerprint                   string                                                                                                                 `json:"candidates_fingerprint"`
	AcceptedResultID                        string                                                                                                                 `json:"accepted_result_id"`
	AcceptedResultFingerprint               string                                                                                                                 `json:"accepted_result_fingerprint"`
	PriorReconciliationReceiptID            string                                                                                                                 `json:"prior_reconciliation_receipt_id"`
	PriorReconciliationReceiptFingerprint   string                                                                                                                 `json:"prior_reconciliation_receipt_fingerprint"`
	TransitionExecutorReceiptID             string                                                                                                                 `json:"transition_executor_receipt_id"`
	TransitionExecutorReceiptFingerprint    string                                                                                                                 `json:"transition_executor_receipt_fingerprint"`
	TransitionRecordID                      string                                                                                                                 `json:"transition_record_id"`
	TransitionRecordFingerprint             string                                                                                                                 `json:"transition_record_fingerprint"`
	OutputPolicyDecisionID                  string                                                                                                                 `json:"output_policy_decision_id"`
	OutputPolicyDecisionFingerprint         string                                                                                                                 `json:"output_policy_decision_fingerprint"`
	OutputPolicyRequestID                   string                                                                                                                 `json:"output_policy_request_id"`
	OutputPolicyRequestFingerprint          string                                                                                                                 `json:"output_policy_request_fingerprint"`
	OutputRecordID                          string                                                                                                                 `json:"output_record_id"`
	OutputRecordFingerprint                 string                                                                                                                 `json:"output_record_fingerprint"`
	OutputRecordVersion                     uint64                                                                                                                 `json:"output_record_version"`
	OutputExecutorReceiptID                 string                                                                                                                 `json:"output_executor_receipt_id"`
	OutputExecutorReceiptFingerprint        string                                                                                                                 `json:"output_executor_receipt_fingerprint"`
	DeliveryPolicyDecisionID                string                                                                                                                 `json:"delivery_policy_decision_id"`
	DeliveryPolicyDecisionFingerprint       string                                                                                                                 `json:"delivery_policy_decision_fingerprint"`
	DeliveryPolicyRequestID                 string                                                                                                                 `json:"delivery_policy_request_id"`
	DeliveryPolicyRequestFingerprint        string                                                                                                                 `json:"delivery_policy_request_fingerprint"`
	TerminalResult                          string                                                                                                                 `json:"terminal_result"`
	TaskOutcome                             string                                                                                                                 `json:"task_outcome"`
	PolicyBinding                           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding `json:"policy_binding"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority
// is deliberately all false: reconciliation evidence grants no adjacent action.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority struct {
	LifecycleAdvancement bool `json:"lifecycle_advancement"`
	GraphMutation        bool `json:"graph_mutation"`
	DependencyWork       bool `json:"dependency_work"`
	DependencyRelease    bool `json:"dependency_release"`
	FailurePropagation   bool `json:"failure_propagation"`
	CandidateDiscovery   bool `json:"candidate_discovery"`
	CandidateSelection   bool `json:"candidate_selection"`
	Scheduling           bool `json:"scheduling"`
	Execution            bool `json:"execution"`
	NodeExecution        bool `json:"node_execution"`
	ResultCollection     bool `json:"result_collection"`
	Delivery             bool `json:"delivery"`
	Redelivery           bool `json:"redelivery"`
	ConsumerInvocation   bool `json:"consumer_invocation"`
	ConsumerReinvocation bool `json:"consumer_reinvocation"`
	Retry                bool `json:"retry"`
	Repair               bool `json:"repair"`
	Cancellation         bool `json:"cancellation"`
	QueueProcessing      bool `json:"queue_processing"`
	Callback             bool `json:"callback"`
	Publication          bool `json:"publication"`
	Provider             bool `json:"provider"`
	Connector            bool `json:"connector"`
	Broker               bool `json:"broker"`
	ForgePipe            bool `json:"forgepipe"`
	Process              bool `json:"process"`
	Network              bool `json:"network"`
	RemoteExecution      bool `json:"remote_execution"`
	Validation           bool `json:"validation"`
	CheckoutMutation     bool `json:"checkout_mutation"`
	Git                  bool `json:"git"`
	Checkpoint           bool `json:"checkpoint"`
	Commit               bool `json:"commit"`
	Push                 bool `json:"push"`
	ExternalAction       bool `json:"external_action"`
	DownstreamAuthority  bool `json:"downstream_authority"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
// is evidence only. It records one exact acknowledgement reconciliation without advancing the graph.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord struct {
	Schema                             string                                                                                                                   `json:"schema"`
	ReconciliationRecordID             string                                                                                                                   `json:"reconciliation_record_id"`
	Binding                            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding `json:"binding"`
	Version                            uint64                                                                                                                   `json:"version"`
	AcknowledgementReconciliationCount uint64                                                                                                                   `json:"acknowledgement_reconciliation_count"`
	FixtureOwned                       bool                                                                                                                     `json:"fixture_owned"`
	Authority                          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority       `json:"authority"`
	RecordFingerprint                  string                                                                                                                   `json:"record_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
// is the sole durable representation that the immutable policy request was consumed.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt struct {
	Schema                                       string                                                                                                                   `json:"schema"`
	ExecutorReceiptID                            string                                                                                                                   `json:"executor_receipt_id"`
	Binding                                      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding `json:"binding"`
	ReconciliationRecordID                       string                                                                                                                   `json:"reconciliation_record_id"`
	ReconciliationRecordFingerprint              string                                                                                                                   `json:"reconciliation_record_fingerprint"`
	ReconciliationRecordVersion                  uint64                                                                                                                   `json:"reconciliation_record_version"`
	Route                                        string                                                                                                                   `json:"route"`
	ExactPostState                               string                                                                                                                   `json:"exact_post_state"`
	RouteSpecificEffect                          string                                                                                                                   `json:"route_specific_effect"`
	OutputType                                   string                                                                                                                   `json:"output_type"`
	DeliveryType                                 string                                                                                                                   `json:"delivery_type"`
	ConsumerID                                   string                                                                                                                   `json:"consumer_id"`
	ConsumerContractFingerprint                  string                                                                                                                   `json:"consumer_contract_fingerprint"`
	LogicalReconciliationAttemptCount            uint64                                                                                                                   `json:"logical_reconciliation_attempt_count"`
	ReconciliationRecordWriteCount               uint64                                                                                                                   `json:"reconciliation_record_write_count"`
	ExecutorReceiptWriteCount                    uint64                                                                                                                   `json:"executor_receipt_write_count"`
	AuthorizationConsumed                        bool                                                                                                                     `json:"authorization_consumed"`
	ConsumedAuthority                            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority `json:"consumed_authority"`
	CompleteImmutablePredecessorChainRevalidated bool                                                                                                                     `json:"complete_immutable_predecessor_chain_revalidated"`
	NoConsumerReinvocation                       bool                                                                                                                     `json:"no_consumer_reinvocation"`
	NoDuplicateReconciliation                    bool                                                                                                                     `json:"no_duplicate_reconciliation"`
	FixtureOwned                                 bool                                                                                                                     `json:"fixture_owned"`
	Authority                                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority       `json:"authority"`
	ReceiptFingerprint                           string                                                                                                                   `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs struct {
	expected        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected
	deliveryInputs  nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs
	acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	deliveryReceipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	decision        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision
	request         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest
	record          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
	recordExists    bool
	receipt         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	receiptExists   bool
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor struct {
	root               string
	expected           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected
	writeRecordAtomic  func(string, any) error
	writeReceiptAtomic func(string, any) error
	mu                 sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor{root: root, expected: inputs.expected, writeRecordAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorLocks.LoadOrStore(executor.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.recordExists {
		inputs.record = deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(inputs)
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(inputs.record, inputs); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, err
		}
		path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "acknowledgement reconciliation record"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, err
		}
		if err := executor.writeRecordAtomic(path, inputs.record); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, errors.New("acknowledgement reconciliation record could not be published")
		}
		inputs.recordExists = true
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, err
	}
	path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "acknowledgement reconciliation executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, err
	}
	if err := executor.writeReceiptAtomic(path, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{}, errors.New("acknowledgement reconciliation executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected) (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs, error) {
	policy, deliveryInputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("acknowledgement reconciliation executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(root, policy, deliveryInputs)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || !decision.Deterministic || !decision.OneTimeDecision || decision.DecisionConsumed || decision.ApprovalInferred || decision.RouteInferred || decision.AcknowledgementInferred || decision.ConsumerInferred || decision.ReconciliationInferred || decision.AuthorityInferred || decision.InferenceSource != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}) {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("acknowledgement reconciliation executor requires the exact approved independently authenticated policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(root, policy, deliveryInputs, decision, true)
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(request.Binding)
	if err != nil || !requestExists || !compatible || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.RequestID != policy.ReconciliationRequestID || request.DecisionID != decision.DecisionID || request.DecisionReplayIdentity != decision.ReplayIdentity || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest || !request.OneTimeRequest || request.AuthorizationConsumed || request.AcknowledgementReconciled || request.LifecycleAdvanced || request.GraphMutated || request.DependencyWorkPerformed || request.DependencyReleasePerformed || request.FailurePropagationPerformed || request.CandidateDiscoveryPerformed || request.CandidateSelectionPerformed || request.SchedulingPerformed || request.ExecutionPerformed || request.NodeExecutionPerformed || request.QueueProcessingPerformed || request.RetryPerformed || request.RepairPerformed || request.CancellationPerformed || request.CallbackInvoked || request.PublicationPerformed || request.ProviderInvoked || request.ConnectorInvoked || request.BrokerInvoked || request.ForgePipeInvoked || request.ProcessLaunched || request.NetworkUsed || request.RemoteExecutionPerformed || request.ValidationPerformed || request.CheckoutMutated || request.GitActionPerformed || request.CheckpointPerformed || request.CommitPerformed || request.PushPerformed || request.ExternalActionPerformed || !request.FixtureOwned || request.Authority != authority {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("acknowledgement reconciliation executor requires the exact approved unconsumed route-compatible request")
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{expected: expected, deliveryInputs: deliveryInputs, acknowledgement: deliveryInputs.acknowledgement, deliveryReceipt: deliveryInputs.receipt, decision: decision, request: request}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBindings(inputs); err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, err
	}
	record, recordExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, err
	}
	inputs.record, inputs.recordExists = record, recordExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, err
	}
	if receiptExists && !recordExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("acknowledgement reconciliation executor receipt is orphaned from its exact record")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBindings(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(inputs.acknowledgement, inputs.deliveryReceipt)
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding)
	if !compatible || !nodeExecutionEqual(inputs.request.Binding, binding) || !nodeExecutionEqual(inputs.decision.Binding, binding) || inputs.request.DecisionID != inputs.decision.DecisionID || inputs.request.DecisionReplayIdentity != inputs.decision.ReplayIdentity || inputs.request.DecisionFingerprint != inputs.decision.DecisionFingerprint || inputs.request.AuthenticationID != inputs.decision.AuthenticationID || inputs.request.AuthenticationDigest != inputs.decision.AuthenticationDigest || inputs.request.AcknowledgementID != inputs.acknowledgement.AcknowledgementID || inputs.request.AcknowledgementFingerprint != inputs.acknowledgement.AcknowledgementFingerprint || inputs.request.OperationKey != inputs.acknowledgement.OperationKey || inputs.request.Route != binding.Route || inputs.request.PostState != binding.PostState || inputs.request.RouteSpecificEffect != binding.RouteSpecificEffect || inputs.request.OutputType != binding.OutputType || inputs.request.DeliveryType != binding.DeliveryType || inputs.request.TerminalResult != binding.TerminalResult || inputs.request.TaskOutcome != binding.TaskOutcome || inputs.request.ConsumerID != binding.ConsumerID || inputs.request.ConsumerContractFingerprint != binding.ConsumerContractFingerprint || inputs.request.Authority != authority || inputs.deliveryReceipt.AcknowledgementID != inputs.acknowledgement.AcknowledgementID || inputs.deliveryReceipt.AcknowledgementFingerprint != inputs.acknowledgement.AcknowledgementFingerprint || inputs.deliveryReceipt.OperationKey != inputs.acknowledgement.OperationKey || !nodeExecutionEqual(inputs.deliveryReceipt.Binding, inputs.acknowledgement.Binding) {
		return errors.New("acknowledgement reconciliation executor policy, acknowledgement, receipt, route, consumer, or predecessor binding is missing, stale, changed, or ambiguous")
	}
	if _, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumedAuthorityFor(inputs.request, inputs.acknowledgement, inputs.deliveryReceipt); !ok {
		return errors.New("acknowledgement reconciliation executor route, state, effect, output, delivery, outcome, or authority is incompatible")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumedAuthorityFor(request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest, acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority, bool) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(acknowledgement, receipt)
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding)
	if !compatible || !nodeExecutionEqual(request.Binding, binding) || request.Route != binding.Route || request.PostState != binding.PostState || request.RouteSpecificEffect != binding.RouteSpecificEffect || request.OutputType != binding.OutputType || request.DeliveryType != binding.DeliveryType || request.TerminalResult != binding.TerminalResult || request.TaskOutcome != binding.TaskOutcome || request.ConsumerID != binding.ConsumerID || request.ConsumerContractFingerprint != binding.ConsumerContractFingerprint || request.Authority != authority || !acknowledgement.Accepted || acknowledgement.AcceptedLocalConsumerDeliveryCount != 1 || !acknowledgement.FixtureOwned || acknowledgement.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) || receipt.LogicalDeliveryAttemptCount != 1 || receipt.ConsumerInvocationCount != 1 || receipt.AcceptedAcknowledgementCount != 1 || receipt.AcknowledgementArtifactWriteCount != 1 || receipt.ExecutorReceiptWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.CompleteImmutablePredecessorChainRevalidated || !receipt.NoDuplicateDelivery || receipt.ConsumerReinvoked || !receipt.FixtureOwned || receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}, false
	}
	return authority, true
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding {
	b := inputs.request.Binding
	db := b.DeliveryExecutorBinding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding{
		ReconciliationPolicyDecisionID: inputs.decision.DecisionID, ReconciliationPolicyDecisionFingerprint: inputs.decision.DecisionFingerprint,
		ReconciliationPolicyRequestID: inputs.request.RequestID, ReconciliationPolicyRequestFingerprint: inputs.request.RequestFingerprint,
		DecisionReplayIdentity: inputs.request.DecisionReplayIdentity, DecisionAuthenticationID: inputs.request.AuthenticationID, DecisionAuthenticationDigest: inputs.request.AuthenticationDigest,
		AcknowledgementID: inputs.acknowledgement.AcknowledgementID, AcknowledgementFingerprint: inputs.acknowledgement.AcknowledgementFingerprint, OperationKey: inputs.acknowledgement.OperationKey,
		DeliveryExecutorReceiptID: inputs.deliveryReceipt.ExecutorReceiptID, DeliveryExecutorReceiptFingerprint: inputs.deliveryReceipt.ReceiptFingerprint,
		Route: b.Route, PostState: b.PostState, RouteSpecificEffect: b.RouteSpecificEffect, OutputType: b.OutputType, DeliveryType: b.DeliveryType,
		ConsumerID: b.ConsumerID, ConsumerContractFingerprint: b.ConsumerContractFingerprint,
		GraphRunID: db.GraphRunID, TerminalTaskID: db.TerminalTaskID, SelectedTaskID: db.SelectedTaskID, CandidatesFingerprint: db.CandidatesFingerprint,
		AcceptedResultID: db.AcceptedResultID, AcceptedResultFingerprint: db.AcceptedResultFingerprint,
		PriorReconciliationReceiptID: db.ReconciliationReceiptID, PriorReconciliationReceiptFingerprint: db.ReconciliationReceiptFingerprint,
		TransitionExecutorReceiptID: db.TransitionExecutorReceiptID, TransitionExecutorReceiptFingerprint: db.TransitionExecutorFingerprint,
		TransitionRecordID: db.TransitionRecordID, TransitionRecordFingerprint: db.TransitionRecordFingerprint,
		OutputPolicyDecisionID: db.OutputPolicyDecisionID, OutputPolicyDecisionFingerprint: db.OutputPolicyDecisionFingerprint,
		OutputPolicyRequestID: db.OutputPolicyRequestID, OutputPolicyRequestFingerprint: db.OutputPolicyRequestFingerprint,
		OutputRecordID: db.OutputRecordID, OutputRecordFingerprint: db.OutputRecordFingerprint, OutputRecordVersion: db.OutputRecordVersion,
		OutputExecutorReceiptID: db.OutputExecutorReceiptID, OutputExecutorReceiptFingerprint: db.OutputExecutorReceiptFingerprint,
		DeliveryPolicyDecisionID: db.DeliveryPolicyDecisionID, DeliveryPolicyDecisionFingerprint: db.DeliveryPolicyDecisionFingerprint,
		DeliveryPolicyRequestID: db.DeliveryPolicyRequestID, DeliveryPolicyRequestFingerprint: db.DeliveryPolicyRequestFingerprint,
		TerminalResult: b.TerminalResult, TaskOutcome: b.TaskOutcome, PolicyBinding: b,
	}
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord {
	record := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordSchema, ReconciliationRecordID: inputs.request.RequestID + "-reconciliation-record", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding(inputs), Version: 1, AcknowledgementReconciliationCount: 1, FixtureOwned: true}
	record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordFingerprint(record)
	return record
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt {
	consumed, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumedAuthorityFor(inputs.request, inputs.acknowledgement, inputs.deliveryReceipt)
	receipt := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptSchema, ExecutorReceiptID: inputs.request.RequestID + "-reconciliation-executor-receipt", Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding(inputs), ReconciliationRecordID: inputs.record.ReconciliationRecordID, ReconciliationRecordFingerprint: inputs.record.RecordFingerprint, ReconciliationRecordVersion: inputs.record.Version, Route: inputs.request.Route, ExactPostState: inputs.request.PostState, RouteSpecificEffect: inputs.request.RouteSpecificEffect, OutputType: inputs.request.OutputType, DeliveryType: inputs.request.DeliveryType, ConsumerID: inputs.request.ConsumerID, ConsumerContractFingerprint: inputs.request.ConsumerContractFingerprint, LogicalReconciliationAttemptCount: 1, ReconciliationRecordWriteCount: 1, ExecutorReceiptWriteCount: 1, AuthorizationConsumed: true, ConsumedAuthority: consumed, CompleteImmutablePredecessorChainRevalidated: true, NoConsumerReinvocation: true, NoDuplicateReconciliation: true, FixtureOwned: true}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReconciliationRecordID) || value.Version != 1 || value.AcknowledgementReconciliationCount != 1 || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority{}) || fingerprint != value.RecordFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("acknowledgement reconciliation record is invalid, conflicting, or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ExecutorReceiptID) || value.LogicalReconciliationAttemptCount != 1 || value.ReconciliationRecordWriteCount != 1 || value.ExecutorReceiptWriteCount != 1 || !value.AuthorizationConsumed || !value.CompleteImmutablePredecessorChainRevalidated || !value.NoConsumerReinvocation || !value.NoDuplicateReconciliation || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority{}) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("acknowledgement reconciliation executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("acknowledgement reconciliation record is malformed, noncanonical, oversized, symlinked, unsafe, partial, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(value, inputs); err != nil {
		return value, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("acknowledgement reconciliation executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, partial, or conflicting")
	}
	if !inputs.recordExists || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(value, inputs) != nil {
		return value, false, errors.New("acknowledgement reconciliation executor receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorMaxBytes {
		return errors.New("acknowledgement reconciliation executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("acknowledgement reconciliation executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("acknowledgement reconciliation executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
