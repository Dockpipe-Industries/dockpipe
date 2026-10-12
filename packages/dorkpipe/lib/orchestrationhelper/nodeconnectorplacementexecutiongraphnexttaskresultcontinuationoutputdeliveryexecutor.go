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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorMaxBytes    = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorLocks sync.Map

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey
// is the exact request/replay pair used for durable consumer idempotency.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey struct {
	RequestID      string `json:"request_id"`
	ReplayIdentity string `json:"replay_identity"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumer is the
// smallest local delivery seam. Implementations expose one immutable identity and contract,
// durably look up prior acceptance, and accept only the exact request/output pair supplied here.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumer interface {
	ConsumerID() string
	ConsumerContractFingerprint() string
	LookupAcknowledgement(NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, bool, error)
	Deliver(NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, error)
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                                                     `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                                                     `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding
// retains the exact delivery policy, durable output, and complete immutable predecessor binding.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding struct {
	DeliveryPolicyDecisionID          string                                                                                    `json:"delivery_policy_decision_id"`
	DeliveryPolicyDecisionFingerprint string                                                                                    `json:"delivery_policy_decision_fingerprint"`
	DeliveryPolicyRequestID           string                                                                                    `json:"delivery_policy_request_id"`
	DeliveryPolicyRequestFingerprint  string                                                                                    `json:"delivery_policy_request_fingerprint"`
	DeliveryAuthenticationID          string                                                                                    `json:"delivery_authentication_id"`
	DeliveryAuthenticationDigest      string                                                                                    `json:"delivery_authentication_digest"`
	OutputRecordID                    string                                                                                    `json:"output_record_id"`
	OutputRecordFingerprint           string                                                                                    `json:"output_record_fingerprint"`
	OutputRecordVersion               uint64                                                                                    `json:"output_record_version"`
	OutputExecutorReceiptID           string                                                                                    `json:"output_executor_receipt_id"`
	OutputExecutorReceiptFingerprint  string                                                                                    `json:"output_executor_receipt_fingerprint"`
	OutputPolicyDecisionID            string                                                                                    `json:"output_policy_decision_id"`
	OutputPolicyDecisionFingerprint   string                                                                                    `json:"output_policy_decision_fingerprint"`
	OutputPolicyRequestID             string                                                                                    `json:"output_policy_request_id"`
	OutputPolicyRequestFingerprint    string                                                                                    `json:"output_policy_request_fingerprint"`
	TransitionExecutorReceiptID       string                                                                                    `json:"transition_executor_receipt_id"`
	TransitionExecutorFingerprint     string                                                                                    `json:"transition_executor_receipt_fingerprint"`
	TransitionRecordID                string                                                                                    `json:"transition_record_id"`
	TransitionRecordFingerprint       string                                                                                    `json:"transition_record_fingerprint"`
	Route                             string                                                                                    `json:"route"`
	PostState                         string                                                                                    `json:"post_state"`
	RouteSpecificEffect               string                                                                                    `json:"route_specific_effect"`
	OutputType                        string                                                                                    `json:"output_type"`
	DeliveryType                      string                                                                                    `json:"delivery_type"`
	ConsumerID                        string                                                                                    `json:"consumer_id"`
	ConsumerContractFingerprint       string                                                                                    `json:"consumer_contract_fingerprint"`
	GraphRunID                        string                                                                                    `json:"graph_run_id"`
	TerminalTaskID                    string                                                                                    `json:"terminal_task_id"`
	SelectedTaskID                    string                                                                                    `json:"selected_task_id"`
	CandidatesFingerprint             string                                                                                    `json:"candidates_fingerprint"`
	AcceptedResultID                  string                                                                                    `json:"accepted_result_id"`
	AcceptedResultFingerprint         string                                                                                    `json:"accepted_result_fingerprint"`
	ReconciliationReceiptID           string                                                                                    `json:"reconciliation_receipt_id"`
	ReconciliationReceiptFingerprint  string                                                                                    `json:"reconciliation_receipt_fingerprint"`
	TerminalResult                    string                                                                                    `json:"terminal_result"`
	TaskOutcome                       string                                                                                    `json:"task_outcome"`
	DeliveryPolicyBinding             NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding `json:"delivery_policy_binding"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority
// records that an acknowledgement grants no adjacent or future authority.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority struct {
	LifecycleAdvancement bool `json:"lifecycle_advancement"`
	GraphMutation        bool `json:"graph_mutation"`
	DependencyRelease    bool `json:"dependency_release"`
	FailurePropagation   bool `json:"failure_propagation"`
	CandidateDiscovery   bool `json:"candidate_discovery"`
	CandidateSelection   bool `json:"candidate_selection"`
	Scheduling           bool `json:"scheduling"`
	Execution            bool `json:"execution"`
	Retry                bool `json:"retry"`
	Repair               bool `json:"repair"`
	Cancellation         bool `json:"cancellation"`
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
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
// is fixture-owned evidence for exactly one accepted local consumer delivery.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement struct {
	Schema                             string                                                                                      `json:"schema"`
	AcknowledgementID                  string                                                                                      `json:"acknowledgement_id"`
	OperationKey                       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey    `json:"operation_key"`
	Binding                            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding `json:"binding"`
	Accepted                           bool                                                                                        `json:"accepted"`
	AcceptedLocalConsumerDeliveryCount uint64                                                                                      `json:"accepted_local_consumer_delivery_count"`
	FixtureOwned                       bool                                                                                        `json:"fixture_owned"`
	Authority                          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority       `json:"authority"`
	AcknowledgementFingerprint         string                                                                                      `json:"acknowledgement_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
// is separate durable request-consumption evidence bound to the acknowledgement.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt struct {
	Schema                                       string                                                                                      `json:"schema"`
	ExecutorReceiptID                            string                                                                                      `json:"executor_receipt_id"`
	OperationKey                                 NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey    `json:"operation_key"`
	Binding                                      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding `json:"binding"`
	AcknowledgementID                            string                                                                                      `json:"acknowledgement_id"`
	AcknowledgementFingerprint                   string                                                                                      `json:"acknowledgement_fingerprint"`
	LogicalDeliveryAttemptCount                  uint64                                                                                      `json:"logical_delivery_attempt_count"`
	ConsumerInvocationCount                      uint64                                                                                      `json:"consumer_invocation_count"`
	AcceptedAcknowledgementCount                 uint64                                                                                      `json:"accepted_acknowledgement_count"`
	AcknowledgementArtifactWriteCount            uint64                                                                                      `json:"acknowledgement_artifact_write_count"`
	ExecutorReceiptWriteCount                    uint64                                                                                      `json:"executor_receipt_write_count"`
	AuthorizationConsumed                        bool                                                                                        `json:"authorization_consumed"`
	CompleteImmutablePredecessorChainRevalidated bool                                                                                        `json:"complete_immutable_predecessor_chain_revalidated"`
	NoDuplicateDelivery                          bool                                                                                        `json:"no_duplicate_delivery"`
	ConsumerReinvoked                            bool                                                                                        `json:"consumer_reinvoked"`
	FixtureOwned                                 bool                                                                                        `json:"fixture_owned"`
	Authority                                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority       `json:"authority"`
	ReceiptFingerprint                           string                                                                                      `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs struct {
	expected              NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected
	outputInputs          nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs
	output                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	outputReceipt         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	decision              NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision
	request               NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	acknowledgement       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	acknowledgementExists bool
	receipt               NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	receiptExists         bool
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor struct {
	root                       string
	expected                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected
	consumer                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumer
	writeAcknowledgementAtomic func(string, any) error
	writeReceiptAtomic         func(string, any) error
	mu                         sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected, consumer NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumer) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs(root, expected, consumer)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor{
		root: root, expected: inputs.expected, consumer: consumer, writeAcknowledgementAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic,
	}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorLocks.LoadOrStore(executor.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs(executor.root, executor.expected, executor.consumer)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs.acknowledgement), cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.acknowledgementExists {
		operationKey := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(inputs.request)
		acknowledgement, accepted, err := executor.consumer.LookupAcknowledgement(operationKey)
		if err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, errors.New("downstream graph output consumer acknowledgement lookup failed")
		}
		if !accepted {
			acknowledgement, err = executor.consumer.Deliver(operationKey, cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(inputs.request), cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(inputs.output))
			if err != nil {
				return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, errors.New("downstream graph output consumer rejected delivery")
			}
		}
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(acknowledgement, inputs); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, err
		}
		path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "downstream graph output delivery acknowledgement"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, err
		}
		if err := executor.writeAcknowledgementAtomic(path, acknowledgement); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, errors.New("downstream graph output delivery acknowledgement could not be published")
		}
		inputs.acknowledgement, inputs.acknowledgementExists = acknowledgement, true
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, err
	}
	path := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "downstream graph output delivery executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, err
	}
	if err := executor.writeReceiptAtomic(path, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, errors.New("downstream graph output delivery executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs.acknowledgement), cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected, consumer NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumer) (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs, error) {
	if consumer == nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("downstream graph output delivery executor requires one exact local consumer")
	}
	policy, outputInputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("downstream graph output delivery executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(root, policy, outputInputs)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || !decision.Deterministic || !decision.OneTimeDecision || decision.DecisionConsumed || decision.ApprovalInferred || decision.RouteInferred || decision.OutputInferred || decision.DeliveryTypeInferred || decision.ConsumerInferred || decision.AuthorityInferred || decision.InferenceSource != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{}) {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("downstream graph output delivery executor requires the exact approved independently authenticated delivery decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(root, policy, outputInputs, decision, true)
	deliveryType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(outputInputs.output, outputInputs.receipt)
	if err != nil || !requestExists || !compatible || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.RequestID != policy.DeliveryRequestID || request.DecisionID != decision.DecisionID || request.DecisionReplayIdentity != decision.ReplayIdentity || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest || request.Route != decision.Route || request.OutputType != decision.OutputType || request.DeliveryType != deliveryType || request.DeliveryType != decision.DeliveryType || request.ConsumerID != decision.ConsumerID || request.ConsumerContractFingerprint != decision.ConsumerContractFingerprint || !request.OneTimeRequest || request.AuthorizationConsumed || request.DeliveryPerformed || request.ConsumerInvoked || request.AcknowledgementReceived || request.CallbackInvoked || request.LifecycleActionTriggered || request.PublicationPerformed || request.ExternalActionPerformed || !request.FixtureOwned || request.Authority != authority {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("downstream graph output delivery executor requires the exact approved unconsumed route-compatible delivery request")
	}
	if consumer.ConsumerID() != request.ConsumerID || consumer.ConsumerContractFingerprint() != request.ConsumerContractFingerprint || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(consumer.ConsumerID()) || !nodeExecutionFingerprint.MatchString(consumer.ConsumerContractFingerprint()) {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("downstream graph output delivery consumer identity or contract is missing, ambiguous, or conflicting")
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{expected: expected, outputInputs: outputInputs, output: outputInputs.output, outputReceipt: outputInputs.receipt, decision: decision, request: request}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBindings(inputs); err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, err
	}
	acknowledgement, acknowledgementExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, err
	}
	inputs.acknowledgement, inputs.acknowledgementExists = acknowledgement, acknowledgementExists
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, err
	}
	if receiptExists && !acknowledgementExists {
		return nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("downstream graph output delivery receipt is orphaned from its exact acknowledgement")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBindings(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(inputs.output, inputs.outputReceipt)
	deliveryType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(inputs.output, inputs.outputReceipt)
	if !compatible || !nodeExecutionEqual(inputs.request.Binding, binding) || !nodeExecutionEqual(inputs.decision.Binding, binding) || inputs.request.DecisionID != inputs.decision.DecisionID || inputs.request.DecisionReplayIdentity != inputs.decision.ReplayIdentity || inputs.request.DecisionFingerprint != inputs.decision.DecisionFingerprint || inputs.request.AuthenticationID != inputs.decision.AuthenticationID || inputs.request.AuthenticationDigest != inputs.decision.AuthenticationDigest || inputs.request.Route != binding.Route || inputs.request.OutputType != binding.OutputType || inputs.request.DeliveryType != deliveryType || inputs.request.ConsumerID != inputs.expected.Policy.ConsumerID || inputs.request.ConsumerContractFingerprint != inputs.expected.Policy.ConsumerContractFingerprint || inputs.request.Authority != authority || inputs.outputReceipt.OutputRecordID != inputs.output.OutputRecordID || inputs.outputReceipt.OutputRecordFingerprint != inputs.output.RecordFingerprint || inputs.outputReceipt.OutputRecordVersion != inputs.output.Version || !nodeExecutionEqual(inputs.outputReceipt.Binding, inputs.output.Binding) || inputs.outputReceipt.OutputActionCount != 1 || inputs.outputReceipt.OutputRecordWriteCount != 1 || !inputs.outputReceipt.AuthorizationConsumed || !inputs.outputReceipt.FixtureOwned || !inputs.output.FixtureOwned || inputs.output.Version != 1 {
		return errors.New("downstream graph output delivery executor predecessor, output, policy, authentication, route, or consumer binding is missing, stale, changed, or ambiguous")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey {
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey{RequestID: request.RequestID, ReplayIdentity: request.DecisionReplayIdentity}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding {
	b := inputs.request.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding{
		DeliveryPolicyDecisionID: inputs.decision.DecisionID, DeliveryPolicyDecisionFingerprint: inputs.decision.DecisionFingerprint,
		DeliveryPolicyRequestID: inputs.request.RequestID, DeliveryPolicyRequestFingerprint: inputs.request.RequestFingerprint,
		DeliveryAuthenticationID: inputs.request.AuthenticationID, DeliveryAuthenticationDigest: inputs.request.AuthenticationDigest,
		OutputRecordID: inputs.output.OutputRecordID, OutputRecordFingerprint: inputs.output.RecordFingerprint, OutputRecordVersion: inputs.output.Version,
		OutputExecutorReceiptID: b.OutputExecutorReceiptID, OutputExecutorReceiptFingerprint: b.OutputExecutorReceiptFingerprint,
		OutputPolicyDecisionID: b.OutputPolicyDecisionID, OutputPolicyDecisionFingerprint: b.OutputPolicyDecisionFingerprint,
		OutputPolicyRequestID: b.OutputPolicyRequestID, OutputPolicyRequestFingerprint: b.OutputPolicyRequestFingerprint,
		TransitionExecutorReceiptID: b.TransitionExecutorReceiptID, TransitionExecutorFingerprint: b.TransitionExecutorReceiptFingerprint,
		TransitionRecordID: b.TransitionRecordID, TransitionRecordFingerprint: b.TransitionRecordFingerprint,
		Route: inputs.request.Route, PostState: b.PostState, RouteSpecificEffect: b.RouteSpecificEffect, OutputType: inputs.request.OutputType, DeliveryType: inputs.request.DeliveryType,
		ConsumerID: inputs.request.ConsumerID, ConsumerContractFingerprint: inputs.request.ConsumerContractFingerprint,
		GraphRunID: b.GraphRunID, TerminalTaskID: b.TerminalTaskID, SelectedTaskID: b.SelectedTaskID, CandidatesFingerprint: b.CandidatesFingerprint,
		AcceptedResultID: b.AcceptedResultID, AcceptedResultFingerprint: b.AcceptedResultFingerprint,
		ReconciliationReceiptID: b.ReconciliationReceiptID, ReconciliationReceiptFingerprint: b.ReconciliationReceiptFingerprint,
		TerminalResult: b.TerminalResult, TaskOutcome: b.TaskOutcome, DeliveryPolicyBinding: b,
	}
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement {
	acknowledgement := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{
		Schema:            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementSchema,
		AcknowledgementID: inputs.request.RequestID + "-acknowledgement", OperationKey: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(inputs.request),
		Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding(inputs), Accepted: true, AcceptedLocalConsumerDeliveryCount: 1, FixtureOwned: true,
	}
	acknowledgement.AcknowledgementFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementFingerprint(acknowledgement)
	return acknowledgement
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt {
	receipt := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{
		Schema:            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptSchema,
		ExecutorReceiptID: inputs.request.RequestID + "-delivery-executor-receipt", OperationKey: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(inputs.request),
		Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding(inputs), AcknowledgementID: inputs.acknowledgement.AcknowledgementID, AcknowledgementFingerprint: inputs.acknowledgement.AcknowledgementFingerprint,
		LogicalDeliveryAttemptCount: 1, ConsumerInvocationCount: 1, AcceptedAcknowledgementCount: 1, AcknowledgementArtifactWriteCount: 1, ExecutorReceiptWriteCount: 1,
		AuthorizationConsumed: true, CompleteImmutablePredecessorChainRevalidated: true, NoDuplicateDelivery: true, FixtureOwned: true,
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.AcknowledgementID) || value.OperationKey != nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(inputs.request) || !value.Accepted || value.AcceptedLocalConsumerDeliveryCount != 1 || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) || fingerprint != value.AcknowledgementFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("downstream graph output delivery acknowledgement is invalid, conflicting, or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ExecutorReceiptID) || value.OperationKey != nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(inputs.request) || value.LogicalDeliveryAttemptCount != 1 || value.ConsumerInvocationCount != 1 || value.AcceptedAcknowledgementCount != 1 || value.AcknowledgementArtifactWriteCount != 1 || value.ExecutorReceiptWriteCount != 1 || !value.AuthorizationConsumed || !value.CompleteImmutablePredecessorChainRevalidated || !value.NoDuplicateDelivery || value.ConsumerReinvoked || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("downstream graph output delivery executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, errors.New("downstream graph output delivery acknowledgement is malformed, noncanonical, oversized, symlinked, unsafe, partial, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, false, errors.New("downstream graph output delivery executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, partial, or conflicting")
	}
	if !inputs.acknowledgementExists || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(value, inputs) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt{}, false, errors.New("downstream graph output delivery executor receipt is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorMaxBytes {
		return errors.New("downstream graph output delivery executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("downstream graph output delivery executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("downstream graph output delivery executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement) (string, error) {
	value.AcknowledgementFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
