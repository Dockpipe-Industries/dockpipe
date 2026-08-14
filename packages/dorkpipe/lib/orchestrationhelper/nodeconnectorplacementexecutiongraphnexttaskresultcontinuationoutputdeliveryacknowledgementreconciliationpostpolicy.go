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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-policy-request/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestName  = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionMax  = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyArtifactMax  = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyLocks sync.Map

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority
// grants exactly one opaque future route-compatible local executor attempt.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority struct {
	ContinuationHandoffPostReconciliationAttempt           bool `json:"continuation_handoff_post_reconciliation_attempt"`
	SuccessfulTerminalGraphResultPostReconciliationAttempt bool `json:"successful_terminal_graph_result_post_reconciliation_attempt"`
	FailedTerminalGraphResultPostReconciliationAttempt     bool `json:"failed_terminal_graph_result_post_reconciliation_attempt"`
	AcknowledgementReconciliation                          bool `json:"acknowledgement_reconciliation"`
	LifecycleAdvancement                                   bool `json:"lifecycle_advancement"`
	GraphMutation                                          bool `json:"graph_mutation"`
	DependencyWork                                         bool `json:"dependency_work"`
	DependencyRelease                                      bool `json:"dependency_release"`
	FailurePropagation                                     bool `json:"failure_propagation"`
	CandidateSelection                                     bool `json:"candidate_selection"`
	Scheduling                                             bool `json:"scheduling"`
	Execution                                              bool `json:"execution"`
	ResultCollection                                       bool `json:"result_collection"`
	Delivery                                               bool `json:"delivery"`
	ConsumerInvocation                                     bool `json:"consumer_invocation"`
	Retry                                                  bool `json:"retry"`
	Repair                                                 bool `json:"repair"`
	Cancellation                                           bool `json:"cancellation"`
	QueueProcessing                                        bool `json:"queue_processing"`
	Callback                                               bool `json:"callback"`
	Publication                                            bool `json:"publication"`
	Provider                                               bool `json:"provider"`
	Connector                                              bool `json:"connector"`
	Broker                                                 bool `json:"broker"`
	ForgePipe                                              bool `json:"forgepipe"`
	Process                                                bool `json:"process"`
	Network                                                bool `json:"network"`
	RemoteExecution                                        bool `json:"remote_execution"`
	Validation                                             bool `json:"validation"`
	CheckoutMutation                                       bool `json:"checkout_mutation"`
	Git                                                    bool `json:"git"`
	Checkpoint                                             bool `json:"checkpoint"`
	Commit                                                 bool `json:"commit"`
	Push                                                   bool `json:"push"`
	ExternalAction                                         bool `json:"external_action"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding
// binds the exact reconciliation evidence and its complete immutable predecessor chain.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding struct {
	ReconciliationRecordID             string                                                                                                                   `json:"reconciliation_record_id"`
	ReconciliationRecordFingerprint    string                                                                                                                   `json:"reconciliation_record_fingerprint"`
	ReconciliationRecordVersion        uint64                                                                                                                   `json:"reconciliation_record_version"`
	ReconciliationExecutorReceiptID    string                                                                                                                   `json:"reconciliation_executor_receipt_id"`
	ReconciliationExecutorFingerprint  string                                                                                                                   `json:"reconciliation_executor_receipt_fingerprint"`
	PriorPolicyDecisionID              string                                                                                                                   `json:"prior_policy_decision_id"`
	PriorPolicyDecisionFingerprint     string                                                                                                                   `json:"prior_policy_decision_fingerprint"`
	PriorPolicyRequestID               string                                                                                                                   `json:"prior_policy_request_id"`
	PriorPolicyRequestFingerprint      string                                                                                                                   `json:"prior_policy_request_fingerprint"`
	PriorPolicyReplayIdentity          string                                                                                                                   `json:"prior_policy_replay_identity"`
	PriorPolicyAuthenticationID        string                                                                                                                   `json:"prior_policy_authentication_id"`
	PriorPolicyAuthenticationDigest    string                                                                                                                   `json:"prior_policy_authentication_digest"`
	AcknowledgementID                  string                                                                                                                   `json:"acknowledgement_id"`
	AcknowledgementFingerprint         string                                                                                                                   `json:"acknowledgement_fingerprint"`
	OperationKey                       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey                                 `json:"operation_key"`
	AcknowledgementAccepted            bool                                                                                                                     `json:"acknowledgement_accepted"`
	AcceptedLocalConsumerDeliveryCount uint64                                                                                                                   `json:"accepted_local_consumer_delivery_count"`
	DeliveryExecutorReceiptID          string                                                                                                                   `json:"delivery_executor_receipt_id"`
	DeliveryExecutorReceiptFingerprint string                                                                                                                   `json:"delivery_executor_receipt_fingerprint"`
	Route                              string                                                                                                                   `json:"route"`
	PostState                          string                                                                                                                   `json:"post_state"`
	RouteSpecificEffect                string                                                                                                                   `json:"route_specific_effect"`
	OutputType                         string                                                                                                                   `json:"output_type"`
	DeliveryType                       string                                                                                                                   `json:"delivery_type"`
	ConsumerID                         string                                                                                                                   `json:"consumer_id"`
	ConsumerContractFingerprint        string                                                                                                                   `json:"consumer_contract_fingerprint"`
	TerminalResult                     string                                                                                                                   `json:"terminal_result"`
	TaskOutcome                        string                                                                                                                   `json:"task_outcome"`
	ExecutorBinding                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorBinding `json:"executor_binding"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected struct {
	Executor                         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected `json:"executor"`
	ReconciliationRecordFingerprint  string                                                                                                                    `json:"reconciliation_record_fingerprint"`
	ReconciliationReceiptFingerprint string                                                                                                                    `json:"reconciliation_receipt_fingerprint"`
	DecisionAuthenticationID         string                                                                                                                    `json:"decision_authentication_id"`
	DecisionAuthenticationDigest     string                                                                                                                    `json:"decision_authentication_digest"`
	PostReconciliationRequestID      string                                                                                                                    `json:"post_reconciliation_request_id"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture struct {
	Schema                      string                                                                                                                       `json:"schema"`
	DecisionID                  string                                                                                                                       `json:"decision_id"`
	ReplayIdentity              string                                                                                                                       `json:"replay_identity"`
	AuthenticationID            string                                                                                                                       `json:"authentication_id"`
	AuthenticationDigest        string                                                                                                                       `json:"authentication_digest"`
	Decision                    string                                                                                                                       `json:"decision"`
	Route                       string                                                                                                                       `json:"route,omitempty"`
	PostState                   string                                                                                                                       `json:"post_state,omitempty"`
	RouteSpecificEffect         string                                                                                                                       `json:"route_specific_effect,omitempty"`
	OutputType                  string                                                                                                                       `json:"output_type,omitempty"`
	DeliveryType                string                                                                                                                       `json:"delivery_type,omitempty"`
	TerminalResult              string                                                                                                                       `json:"terminal_result,omitempty"`
	TaskOutcome                 string                                                                                                                       `json:"task_outcome,omitempty"`
	ConsumerID                  string                                                                                                                       `json:"consumer_id,omitempty"`
	ConsumerContractFingerprint string                                                                                                                       `json:"consumer_contract_fingerprint,omitempty"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding   `json:"binding"`
	PostReconciliationRequestID string                                                                                                                       `json:"post_reconciliation_request_id,omitempty"`
	Deterministic               bool                                                                                                                         `json:"deterministic"`
	OneTimeDecision             bool                                                                                                                         `json:"one_time_decision"`
	DecisionConsumed            bool                                                                                                                         `json:"decision_consumed"`
	ApprovalInferred            bool                                                                                                                         `json:"approval_inferred"`
	RouteInferred               bool                                                                                                                         `json:"route_inferred"`
	ConsumerInferred            bool                                                                                                                         `json:"consumer_inferred"`
	ReconciliationInferred      bool                                                                                                                         `json:"reconciliation_inferred"`
	FutureAuthorityInferred     bool                                                                                                                         `json:"future_authority_inferred"`
	InferenceSource             string                                                                                                                       `json:"inference_source,omitempty"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority `json:"authority"`
	Provenance                  string                                                                                                                       `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision struct {
	Schema                      string                                                                                                                       `json:"schema"`
	DecisionID                  string                                                                                                                       `json:"decision_id"`
	ReplayIdentity              string                                                                                                                       `json:"replay_identity"`
	AuthenticationID            string                                                                                                                       `json:"authentication_id"`
	AuthenticationDigest        string                                                                                                                       `json:"authentication_digest"`
	Decision                    string                                                                                                                       `json:"decision"`
	Route                       string                                                                                                                       `json:"route,omitempty"`
	PostState                   string                                                                                                                       `json:"post_state,omitempty"`
	RouteSpecificEffect         string                                                                                                                       `json:"route_specific_effect,omitempty"`
	OutputType                  string                                                                                                                       `json:"output_type,omitempty"`
	DeliveryType                string                                                                                                                       `json:"delivery_type,omitempty"`
	TerminalResult              string                                                                                                                       `json:"terminal_result,omitempty"`
	TaskOutcome                 string                                                                                                                       `json:"task_outcome,omitempty"`
	ConsumerID                  string                                                                                                                       `json:"consumer_id,omitempty"`
	ConsumerContractFingerprint string                                                                                                                       `json:"consumer_contract_fingerprint,omitempty"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding   `json:"binding"`
	PostReconciliationRequestID string                                                                                                                       `json:"post_reconciliation_request_id,omitempty"`
	Deterministic               bool                                                                                                                         `json:"deterministic"`
	OneTimeDecision             bool                                                                                                                         `json:"one_time_decision"`
	DecisionConsumed            bool                                                                                                                         `json:"decision_consumed"`
	ApprovalInferred            bool                                                                                                                         `json:"approval_inferred"`
	RouteInferred               bool                                                                                                                         `json:"route_inferred"`
	ConsumerInferred            bool                                                                                                                         `json:"consumer_inferred"`
	ReconciliationInferred      bool                                                                                                                         `json:"reconciliation_inferred"`
	FutureAuthorityInferred     bool                                                                                                                         `json:"future_authority_inferred"`
	InferenceSource             string                                                                                                                       `json:"inference_source,omitempty"`
	IndependentlyAuthenticated  bool                                                                                                                         `json:"independently_authenticated"`
	FixtureOwned                bool                                                                                                                         `json:"fixture_owned"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority `json:"authority"`
	DecisionFingerprint         string                                                                                                                       `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest struct {
	Schema                            string                                                                                                                       `json:"schema"`
	RequestID                         string                                                                                                                       `json:"request_id"`
	DecisionID                        string                                                                                                                       `json:"decision_id"`
	DecisionReplayIdentity            string                                                                                                                       `json:"decision_replay_identity"`
	DecisionFingerprint               string                                                                                                                       `json:"decision_fingerprint"`
	AuthenticationID                  string                                                                                                                       `json:"authentication_id"`
	AuthenticationDigest              string                                                                                                                       `json:"authentication_digest"`
	ReconciliationRecordID            string                                                                                                                       `json:"reconciliation_record_id"`
	ReconciliationRecordFingerprint   string                                                                                                                       `json:"reconciliation_record_fingerprint"`
	ReconciliationExecutorReceiptID   string                                                                                                                       `json:"reconciliation_executor_receipt_id"`
	ReconciliationExecutorFingerprint string                                                                                                                       `json:"reconciliation_executor_receipt_fingerprint"`
	Route                             string                                                                                                                       `json:"route"`
	PostState                         string                                                                                                                       `json:"post_state"`
	RouteSpecificEffect               string                                                                                                                       `json:"route_specific_effect"`
	OutputType                        string                                                                                                                       `json:"output_type"`
	DeliveryType                      string                                                                                                                       `json:"delivery_type"`
	TerminalResult                    string                                                                                                                       `json:"terminal_result"`
	TaskOutcome                       string                                                                                                                       `json:"task_outcome"`
	ConsumerID                        string                                                                                                                       `json:"consumer_id"`
	ConsumerContractFingerprint       string                                                                                                                       `json:"consumer_contract_fingerprint"`
	Binding                           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding   `json:"binding"`
	OneTimeRequest                    bool                                                                                                                         `json:"one_time_request"`
	AuthorizationConsumed             bool                                                                                                                         `json:"authorization_consumed"`
	LifecycleAdvanced                 bool                                                                                                                         `json:"lifecycle_advanced"`
	GraphMutated                      bool                                                                                                                         `json:"graph_mutated"`
	DependencyWorkPerformed           bool                                                                                                                         `json:"dependency_work_performed"`
	SchedulingPerformed               bool                                                                                                                         `json:"scheduling_performed"`
	ExecutionPerformed                bool                                                                                                                         `json:"execution_performed"`
	DeliveryPerformed                 bool                                                                                                                         `json:"delivery_performed"`
	ConsumerInvoked                   bool                                                                                                                         `json:"consumer_invoked"`
	CallbackInvoked                   bool                                                                                                                         `json:"callback_invoked"`
	PublicationPerformed              bool                                                                                                                         `json:"publication_performed"`
	NetworkUsed                       bool                                                                                                                         `json:"network_used"`
	GitActionPerformed                bool                                                                                                                         `json:"git_action_performed"`
	ExternalActionPerformed           bool                                                                                                                         `json:"external_action_performed"`
	FixtureOwned                      bool                                                                                                                         `json:"fixture_owned"`
	Authority                         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority `json:"authority"`
	RequestFingerprint                string                                                                                                                       `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies struct {
	root          string
	expected      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected
	record        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
	receipt       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	decision      *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision
	request       *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest
	writeDecision func(string, any) error
	writeRequest  func(string, any) error
	mu            sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies, error) {
	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies{root: root, expected: normalized, record: inputs.record, receipt: inputs.receipt, writeDecision: writeJSONFileAtomic, writeRequest: writeJSONFileAtomic}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(root, normalized, inputs)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(root, normalized, inputs, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("post-reconciliation policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionMax {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(policies.expected, policies.record, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected(policies.root, policies.expected)
	if err != nil || !nodeExecutionEqual(normalized, policies.expected) || !nodeExecutionEqual(inputs.record, policies.record) || !nodeExecutionEqual(inputs.receipt, policies.receipt) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation policy could not revalidate the complete immutable predecessor chain")
	}
	durableDecision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(policies.root, policies.expected, inputs)
	if err != nil || policies.decision != nil && !decisionExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation decision is missing or conflicting")
	}
	durableRequest, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(policies.root, policies.expected, inputs, durableDecision, decisionExists)
	if err != nil || requestExists && !decisionExists || policies.request != nil && !requestExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation request is missing, orphaned, or conflicting")
	}
	if decisionExists {
		policies.decision = &durableDecision
	}
	if requestExists {
		policies.request = &durableRequest
	}
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, err
		}
		if err := policies.writeDecision(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("rejected post-reconciliation decision conflicts with a request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "post-reconciliation request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, err
	}
	if err := policies.writeRequest(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(root, value.Executor)
	if err != nil || !inputs.recordExists || !inputs.receiptExists {
		return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("post-reconciliation policy requires the exact reconciliation record and executor receipt")
	}
	value.Executor = inputs.expected
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(inputs.record, inputs.receipt)
	_, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(binding)
	if !compatible || value.ReconciliationRecordFingerprint != inputs.record.RecordFingerprint || value.ReconciliationReceiptFingerprint != inputs.receipt.ReceiptFingerprint || inputs.receipt.ReconciliationRecordID != inputs.record.ReconciliationRecordID || inputs.receipt.ReconciliationRecordFingerprint != inputs.record.RecordFingerprint || inputs.receipt.ReconciliationRecordVersion != inputs.record.Version || !nodeExecutionEqual(inputs.receipt.Binding, inputs.record.Binding) || inputs.record.Version != 1 || inputs.record.AcknowledgementReconciliationCount != 1 || !inputs.record.FixtureOwned || inputs.record.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority{}) || inputs.receipt.LogicalReconciliationAttemptCount != 1 || inputs.receipt.ReconciliationRecordWriteCount != 1 || inputs.receipt.ExecutorReceiptWriteCount != 1 || !inputs.receipt.AuthorizationConsumed || !inputs.receipt.CompleteImmutablePredecessorChainRevalidated || !inputs.receipt.NoConsumerReinvocation || !inputs.receipt.NoDuplicateReconciliation || !inputs.receipt.FixtureOwned || inputs.receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority{}) {
		return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("post-reconciliation evidence is missing, stale, conflicting, or escalates authority")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.PostReconciliationRequestID) {
		return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("post-reconciliation policy requires exact fixture authentication and intended request identity")
	}
	b := inputs.record.Binding
	db := b.PolicyBinding.DeliveryExecutorBinding
	priorIDs := []string{b.DecisionAuthenticationID, db.DeliveryAuthenticationID, db.DeliveryPolicyBinding.OutputPolicyAuthenticationID, db.DeliveryPolicyBinding.OutputExecutorBinding.PriorPolicyAuthenticationID, db.DeliveryPolicyBinding.OutputExecutorBinding.ExecutorBinding.PolicyAuthenticationID}
	priorDigests := []string{b.DecisionAuthenticationDigest, db.DeliveryAuthenticationDigest, db.DeliveryPolicyBinding.OutputPolicyAuthenticationDigest, db.DeliveryPolicyBinding.OutputExecutorBinding.PriorPolicyAuthenticationDigest, db.DeliveryPolicyBinding.OutputExecutorBinding.ExecutorBinding.PolicyAuthenticationDigest}
	for _, prior := range priorIDs {
		if value.DecisionAuthenticationID == prior {
			return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("post-reconciliation authentication reuses a prior identity")
		}
	}
	for _, prior := range priorDigests {
		if value.DecisionAuthenticationDigest == prior {
			return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs{}, errors.New("post-reconciliation authentication reuses a prior digest")
		}
	}
	return value, inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest, error) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(record, receipt)
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(fixture.Binding, binding) || !fixture.Deterministic || !fixture.OneTimeDecision || fixture.DecisionConsumed || fixture.ApprovalInferred || fixture.RouteInferred || fixture.ConsumerInferred || fixture.ReconciliationInferred || fixture.FutureAuthorityInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_post_reconciliation_policy_decision" || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyIdentityCollides(fixture.DecisionID, fixture.ReplayIdentity, binding) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation fixture identity, authentication, binding, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("post-reconciliation decision is invalid")
	}
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(binding)
	if fixture.Decision == "rejected" {
		if fixture.Route != "" || fixture.PostState != "" || fixture.RouteSpecificEffect != "" || fixture.OutputType != "" || fixture.DeliveryType != "" || fixture.TerminalResult != "" || fixture.TaskOutcome != "" || fixture.ConsumerID != "" || fixture.ConsumerContractFingerprint != "" || fixture.PostReconciliationRequestID != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("rejected post-reconciliation decision cannot name a route, output, delivery, consumer, request, or future authority")
		}
	} else if !compatible || fixture.Route != binding.Route || fixture.PostState != binding.PostState || fixture.RouteSpecificEffect != binding.RouteSpecificEffect || fixture.OutputType != binding.OutputType || fixture.DeliveryType != binding.DeliveryType || fixture.TerminalResult != binding.TerminalResult || fixture.TaskOutcome != binding.TaskOutcome || fixture.ConsumerID != binding.ConsumerID || fixture.ConsumerContractFingerprint != binding.ConsumerContractFingerprint || fixture.PostReconciliationRequestID != expected.PostReconciliationRequestID || fixture.Authority != authority {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, errors.New("approved post-reconciliation decision requires the exact compatible route, consumer, request, and narrow authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Route: fixture.Route, PostState: fixture.PostState, RouteSpecificEffect: fixture.RouteSpecificEffect, OutputType: fixture.OutputType, DeliveryType: fixture.DeliveryType, TerminalResult: fixture.TerminalResult, TaskOutcome: fixture.TaskOutcome, ConsumerID: fixture.ConsumerID, ConsumerContractFingerprint: fixture.ConsumerContractFingerprint, Binding: binding, PostReconciliationRequestID: fixture.PostReconciliationRequestID, Deterministic: true, OneTimeDecision: true, IndependentlyAuthenticated: true, FixtureOwned: true}
	decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFingerprint(decision)
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(decision, expected, record, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestSchema, RequestID: decision.PostReconciliationRequestID, DecisionID: decision.DecisionID, DecisionReplayIdentity: decision.ReplayIdentity, DecisionFingerprint: decision.DecisionFingerprint, AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest, ReconciliationRecordID: binding.ReconciliationRecordID, ReconciliationRecordFingerprint: binding.ReconciliationRecordFingerprint, ReconciliationExecutorReceiptID: binding.ReconciliationExecutorReceiptID, ReconciliationExecutorFingerprint: binding.ReconciliationExecutorFingerprint, Route: decision.Route, PostState: decision.PostState, RouteSpecificEffect: decision.RouteSpecificEffect, OutputType: decision.OutputType, DeliveryType: decision.DeliveryType, TerminalResult: decision.TerminalResult, TaskOutcome: decision.TaskOutcome, ConsumerID: decision.ConsumerID, ConsumerContractFingerprint: decision.ConsumerContractFingerprint, Binding: binding, OneTimeRequest: true, FixtureOwned: true, Authority: authority}
	request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestFingerprint(*request)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(decision, expected, record, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(*request, expected, record, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(record, receipt)
	_, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(binding)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFingerprint(value)
	rejected := value.Decision == "rejected" && value.Route == "" && value.PostState == "" && value.RouteSpecificEffect == "" && value.OutputType == "" && value.DeliveryType == "" && value.TerminalResult == "" && value.TaskOutcome == "" && value.ConsumerID == "" && value.ConsumerContractFingerprint == "" && value.PostReconciliationRequestID == ""
	approved := value.Decision == "approved" && compatible && value.Route == binding.Route && value.PostState == binding.PostState && value.RouteSpecificEffect == binding.RouteSpecificEffect && value.OutputType == binding.OutputType && value.DeliveryType == binding.DeliveryType && value.TerminalResult == binding.TerminalResult && value.TaskOutcome == binding.TaskOutcome && value.ConsumerID == binding.ConsumerID && value.ConsumerContractFingerprint == binding.ConsumerContractFingerprint && value.PostReconciliationRequestID == expected.PostReconciliationRequestID
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.DecisionID == value.ReplayIdentity || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || !rejected && !approved || !nodeExecutionEqual(value.Binding, binding) || !value.Deterministic || !value.OneTimeDecision || value.DecisionConsumed || value.ApprovalInferred || value.RouteInferred || value.ConsumerInferred || value.ReconciliationInferred || value.FutureAuthorityInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) || fingerprint != value.DecisionFingerprint || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyIdentityCollides(value.DecisionID, value.ReplayIdentity, binding) {
		return errors.New("post-reconciliation decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(record, receipt)
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(binding)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestFingerprint(value)
	if err != nil || decision.Decision != "approved" || !compatible || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestSchema || value.RequestID != expected.PostReconciliationRequestID || value.DecisionID != decision.DecisionID || value.DecisionReplayIdentity != decision.ReplayIdentity || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || value.ReconciliationRecordID != record.ReconciliationRecordID || value.ReconciliationRecordFingerprint != record.RecordFingerprint || value.ReconciliationExecutorReceiptID != receipt.ExecutorReceiptID || value.ReconciliationExecutorFingerprint != receipt.ReceiptFingerprint || value.Route != decision.Route || value.PostState != decision.PostState || value.RouteSpecificEffect != decision.RouteSpecificEffect || value.OutputType != decision.OutputType || value.DeliveryType != decision.DeliveryType || value.TerminalResult != decision.TerminalResult || value.TaskOutcome != decision.TaskOutcome || value.ConsumerID != decision.ConsumerID || value.ConsumerContractFingerprint != decision.ConsumerContractFingerprint || !nodeExecutionEqual(value.Binding, binding) || !value.OneTimeRequest || value.AuthorizationConsumed || value.LifecycleAdvanced || value.GraphMutated || value.DependencyWorkPerformed || value.SchedulingPerformed || value.ExecutionPerformed || value.DeliveryPerformed || value.ConsumerInvoked || value.CallbackInvoked || value.PublicationPerformed || value.NetworkUsed || value.GitActionPerformed || value.ExternalActionPerformed || !value.FixtureOwned || value.Authority != authority || fingerprint != value.RequestFingerprint {
		return errors.New("post-reconciliation request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("post-reconciliation decision is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(value, expected, inputs.record, inputs.receipt); err != nil {
		return value, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("post-reconciliation request is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(value, expected, inputs.record, inputs.receipt, decision) != nil {
		return value, false, errors.New("post-reconciliation request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyArtifactMax {
		return errors.New("post-reconciliation artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("post-reconciliation artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("post-reconciliation artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding {
	b := record.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding{ReconciliationRecordID: record.ReconciliationRecordID, ReconciliationRecordFingerprint: record.RecordFingerprint, ReconciliationRecordVersion: record.Version, ReconciliationExecutorReceiptID: receipt.ExecutorReceiptID, ReconciliationExecutorFingerprint: receipt.ReceiptFingerprint, PriorPolicyDecisionID: b.ReconciliationPolicyDecisionID, PriorPolicyDecisionFingerprint: b.ReconciliationPolicyDecisionFingerprint, PriorPolicyRequestID: b.ReconciliationPolicyRequestID, PriorPolicyRequestFingerprint: b.ReconciliationPolicyRequestFingerprint, PriorPolicyReplayIdentity: b.DecisionReplayIdentity, PriorPolicyAuthenticationID: b.DecisionAuthenticationID, PriorPolicyAuthenticationDigest: b.DecisionAuthenticationDigest, AcknowledgementID: b.AcknowledgementID, AcknowledgementFingerprint: b.AcknowledgementFingerprint, OperationKey: b.OperationKey, AcknowledgementAccepted: true, AcceptedLocalConsumerDeliveryCount: 1, DeliveryExecutorReceiptID: b.DeliveryExecutorReceiptID, DeliveryExecutorReceiptFingerprint: b.DeliveryExecutorReceiptFingerprint, Route: b.Route, PostState: b.PostState, RouteSpecificEffect: b.RouteSpecificEffect, OutputType: b.OutputType, DeliveryType: b.DeliveryType, ConsumerID: b.ConsumerID, ConsumerContractFingerprint: b.ConsumerContractFingerprint, TerminalResult: b.TerminalResult, TaskOutcome: b.TaskOutcome, ExecutorBinding: b}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority, bool) {
	if !binding.AcknowledgementAccepted || binding.AcceptedLocalConsumerDeliveryCount != 1 || binding.ReconciliationRecordVersion != 1 {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, false
	}
	switch {
	case binding.Route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute && binding.PostState == "continued" && binding.RouteSpecificEffect == "passed_selected_task_continued_local_graph" && binding.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput && binding.DeliveryType == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery && binding.TaskOutcome == "passed" && binding.TerminalResult == "succeeded":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{ContinuationHandoffPostReconciliationAttempt: true}, true
	case binding.Route == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute && binding.PostState == "succeeded" && binding.RouteSpecificEffect == "passed_result_finalized_local_graph_successfully" && binding.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization && binding.DeliveryType == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery && binding.TaskOutcome == "passed" && binding.TerminalResult == "succeeded":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{SuccessfulTerminalGraphResultPostReconciliationAttempt: true}, true
	case binding.Route == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute && binding.PostState == "failed" && binding.RouteSpecificEffect == "failed_result_finalized_local_graph_with_failure_propagation" && binding.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization && binding.DeliveryType == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery && binding.TaskOutcome == "failed" && binding.TerminalResult == "failed":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{FailedTerminalGraphResultPostReconciliationAttempt: true}, true
	default:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}, false
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyIdentityCollides(decisionID, replayIdentity string, binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding) bool {
	b := binding.ExecutorBinding
	for _, value := range []string{binding.ReconciliationRecordID, binding.ReconciliationExecutorReceiptID, binding.PriorPolicyDecisionID, binding.PriorPolicyRequestID, binding.PriorPolicyReplayIdentity, binding.AcknowledgementID, binding.DeliveryExecutorReceiptID, binding.ConsumerID, b.OutputPolicyDecisionID, b.OutputPolicyRequestID, b.TransitionExecutorReceiptID, b.TransitionRecordID, b.GraphRunID, b.TerminalTaskID, b.SelectedTaskID, b.AcceptedResultID, b.PriorReconciliationReceiptID} {
		if decisionID == value || replayIdentity == value {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
