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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-policy-request/v1"

	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName  = "node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionMax  = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationArtifactMax  = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyLocks sync.Map

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority
// grants only one future route-compatible local acknowledgement-reconciliation attempt.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority struct {
	ContinuationHandoffAcknowledgementReconciliationAttempt           bool `json:"continuation_handoff_acknowledgement_reconciliation_attempt"`
	SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt bool `json:"successful_terminal_graph_result_acknowledgement_reconciliation_attempt"`
	FailedTerminalGraphResultAcknowledgementReconciliationAttempt     bool `json:"failed_terminal_graph_result_acknowledgement_reconciliation_attempt"`
	AcknowledgementReconciliation                                     bool `json:"acknowledgement_reconciliation"`
	LifecycleAdvancement                                              bool `json:"lifecycle_advancement"`
	GraphMutation                                                     bool `json:"graph_mutation"`
	DependencyWork                                                    bool `json:"dependency_work"`
	DependencyRelease                                                 bool `json:"dependency_release"`
	FailurePropagation                                                bool `json:"failure_propagation"`
	CandidateDiscovery                                                bool `json:"candidate_discovery"`
	CandidateSelection                                                bool `json:"candidate_selection"`
	Scheduling                                                        bool `json:"scheduling"`
	Execution                                                         bool `json:"execution"`
	NodeExecution                                                     bool `json:"node_execution"`
	QueueProcessing                                                   bool `json:"queue_processing"`
	Retry                                                             bool `json:"retry"`
	Repair                                                            bool `json:"repair"`
	Cancellation                                                      bool `json:"cancellation"`
	Callback                                                          bool `json:"callback"`
	Publication                                                       bool `json:"publication"`
	Provider                                                          bool `json:"provider"`
	Connector                                                         bool `json:"connector"`
	Broker                                                            bool `json:"broker"`
	ForgePipe                                                         bool `json:"forgepipe"`
	Process                                                           bool `json:"process"`
	Network                                                           bool `json:"network"`
	RemoteExecution                                                   bool `json:"remote_execution"`
	Validation                                                        bool `json:"validation"`
	CheckoutMutation                                                  bool `json:"checkout_mutation"`
	Git                                                               bool `json:"git"`
	Checkpoint                                                        bool `json:"checkpoint"`
	Commit                                                            bool `json:"commit"`
	Push                                                              bool `json:"push"`
	ExternalAction                                                    bool `json:"external_action"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding
// binds the exact acknowledgement, delivery receipt, consumer contract, and immutable predecessor chain.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding struct {
	AcknowledgementID                  string                                                                                      `json:"acknowledgement_id"`
	AcknowledgementFingerprint         string                                                                                      `json:"acknowledgement_fingerprint"`
	OperationKey                       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey    `json:"operation_key"`
	DeliveryExecutorReceiptID          string                                                                                      `json:"delivery_executor_receipt_id"`
	DeliveryExecutorReceiptFingerprint string                                                                                      `json:"delivery_executor_receipt_fingerprint"`
	Route                              string                                                                                      `json:"route"`
	PostState                          string                                                                                      `json:"post_state"`
	RouteSpecificEffect                string                                                                                      `json:"route_specific_effect"`
	OutputType                         string                                                                                      `json:"output_type"`
	DeliveryType                       string                                                                                      `json:"delivery_type"`
	ConsumerID                         string                                                                                      `json:"consumer_id"`
	ConsumerContractFingerprint        string                                                                                      `json:"consumer_contract_fingerprint"`
	TerminalResult                     string                                                                                      `json:"terminal_result"`
	TaskOutcome                        string                                                                                      `json:"task_outcome"`
	DeliveryExecutorBinding            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorBinding `json:"delivery_executor_binding"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected struct {
	Executor                           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected `json:"executor"`
	AcknowledgementFingerprint         string                                                                                       `json:"acknowledgement_fingerprint"`
	DeliveryExecutorReceiptFingerprint string                                                                                       `json:"delivery_executor_receipt_fingerprint"`
	DecisionAuthenticationID           string                                                                                       `json:"decision_authentication_id"`
	DecisionAuthenticationDigest       string                                                                                       `json:"decision_authentication_digest"`
	ReconciliationRequestID            string                                                                                       `json:"reconciliation_request_id"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixture struct {
	Schema                      string                                                                                                                   `json:"schema"`
	DecisionID                  string                                                                                                                   `json:"decision_id"`
	ReplayIdentity              string                                                                                                                   `json:"replay_identity"`
	AuthenticationID            string                                                                                                                   `json:"authentication_id"`
	AuthenticationDigest        string                                                                                                                   `json:"authentication_digest"`
	Decision                    string                                                                                                                   `json:"decision"`
	Route                       string                                                                                                                   `json:"route,omitempty"`
	PostState                   string                                                                                                                   `json:"post_state,omitempty"`
	RouteSpecificEffect         string                                                                                                                   `json:"route_specific_effect,omitempty"`
	OutputType                  string                                                                                                                   `json:"output_type,omitempty"`
	DeliveryType                string                                                                                                                   `json:"delivery_type,omitempty"`
	TerminalResult              string                                                                                                                   `json:"terminal_result,omitempty"`
	TaskOutcome                 string                                                                                                                   `json:"task_outcome,omitempty"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding   `json:"binding"`
	ReconciliationRequestID     string                                                                                                                   `json:"reconciliation_request_id,omitempty"`
	ConsumerID                  string                                                                                                                   `json:"consumer_id,omitempty"`
	ConsumerContractFingerprint string                                                                                                                   `json:"consumer_contract_fingerprint,omitempty"`
	Deterministic               bool                                                                                                                     `json:"deterministic"`
	OneTimeDecision             bool                                                                                                                     `json:"one_time_decision"`
	DecisionConsumed            bool                                                                                                                     `json:"decision_consumed"`
	ApprovalInferred            bool                                                                                                                     `json:"approval_inferred"`
	RouteInferred               bool                                                                                                                     `json:"route_inferred"`
	AcknowledgementInferred     bool                                                                                                                     `json:"acknowledgement_inferred"`
	ConsumerInferred            bool                                                                                                                     `json:"consumer_inferred"`
	ReconciliationInferred      bool                                                                                                                     `json:"reconciliation_inferred"`
	AuthorityInferred           bool                                                                                                                     `json:"authority_inferred"`
	InferenceSource             string                                                                                                                   `json:"inference_source,omitempty"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority `json:"authority"`
	Provenance                  string                                                                                                                   `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision struct {
	Schema                      string                                                                                                                   `json:"schema"`
	DecisionID                  string                                                                                                                   `json:"decision_id"`
	ReplayIdentity              string                                                                                                                   `json:"replay_identity"`
	AuthenticationID            string                                                                                                                   `json:"authentication_id"`
	AuthenticationDigest        string                                                                                                                   `json:"authentication_digest"`
	Decision                    string                                                                                                                   `json:"decision"`
	Route                       string                                                                                                                   `json:"route,omitempty"`
	PostState                   string                                                                                                                   `json:"post_state,omitempty"`
	RouteSpecificEffect         string                                                                                                                   `json:"route_specific_effect,omitempty"`
	OutputType                  string                                                                                                                   `json:"output_type,omitempty"`
	DeliveryType                string                                                                                                                   `json:"delivery_type,omitempty"`
	TerminalResult              string                                                                                                                   `json:"terminal_result,omitempty"`
	TaskOutcome                 string                                                                                                                   `json:"task_outcome,omitempty"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding   `json:"binding"`
	ReconciliationRequestID     string                                                                                                                   `json:"reconciliation_request_id,omitempty"`
	ConsumerID                  string                                                                                                                   `json:"consumer_id,omitempty"`
	ConsumerContractFingerprint string                                                                                                                   `json:"consumer_contract_fingerprint,omitempty"`
	Deterministic               bool                                                                                                                     `json:"deterministic"`
	OneTimeDecision             bool                                                                                                                     `json:"one_time_decision"`
	DecisionConsumed            bool                                                                                                                     `json:"decision_consumed"`
	ApprovalInferred            bool                                                                                                                     `json:"approval_inferred"`
	RouteInferred               bool                                                                                                                     `json:"route_inferred"`
	AcknowledgementInferred     bool                                                                                                                     `json:"acknowledgement_inferred"`
	ConsumerInferred            bool                                                                                                                     `json:"consumer_inferred"`
	ReconciliationInferred      bool                                                                                                                     `json:"reconciliation_inferred"`
	AuthorityInferred           bool                                                                                                                     `json:"authority_inferred"`
	InferenceSource             string                                                                                                                   `json:"inference_source,omitempty"`
	IndependentlyAuthenticated  bool                                                                                                                     `json:"independently_authenticated"`
	FixtureOwned                bool                                                                                                                     `json:"fixture_owned"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority `json:"authority"`
	DecisionFingerprint         string                                                                                                                   `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest struct {
	Schema                      string                                                                                                                   `json:"schema"`
	RequestID                   string                                                                                                                   `json:"request_id"`
	DecisionID                  string                                                                                                                   `json:"decision_id"`
	DecisionReplayIdentity      string                                                                                                                   `json:"decision_replay_identity"`
	DecisionFingerprint         string                                                                                                                   `json:"decision_fingerprint"`
	AuthenticationID            string                                                                                                                   `json:"authentication_id"`
	AuthenticationDigest        string                                                                                                                   `json:"authentication_digest"`
	AcknowledgementID           string                                                                                                                   `json:"acknowledgement_id"`
	AcknowledgementFingerprint  string                                                                                                                   `json:"acknowledgement_fingerprint"`
	OperationKey                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey                                 `json:"operation_key"`
	Route                       string                                                                                                                   `json:"route"`
	PostState                   string                                                                                                                   `json:"post_state"`
	RouteSpecificEffect         string                                                                                                                   `json:"route_specific_effect"`
	OutputType                  string                                                                                                                   `json:"output_type"`
	DeliveryType                string                                                                                                                   `json:"delivery_type"`
	TerminalResult              string                                                                                                                   `json:"terminal_result"`
	TaskOutcome                 string                                                                                                                   `json:"task_outcome"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding   `json:"binding"`
	ConsumerID                  string                                                                                                                   `json:"consumer_id"`
	ConsumerContractFingerprint string                                                                                                                   `json:"consumer_contract_fingerprint"`
	OneTimeRequest              bool                                                                                                                     `json:"one_time_request"`
	AuthorizationConsumed       bool                                                                                                                     `json:"authorization_consumed"`
	AcknowledgementReconciled   bool                                                                                                                     `json:"acknowledgement_reconciled"`
	LifecycleAdvanced           bool                                                                                                                     `json:"lifecycle_advanced"`
	GraphMutated                bool                                                                                                                     `json:"graph_mutated"`
	DependencyWorkPerformed     bool                                                                                                                     `json:"dependency_work_performed"`
	DependencyReleasePerformed  bool                                                                                                                     `json:"dependency_release_performed"`
	FailurePropagationPerformed bool                                                                                                                     `json:"failure_propagation_performed"`
	CandidateDiscoveryPerformed bool                                                                                                                     `json:"candidate_discovery_performed"`
	CandidateSelectionPerformed bool                                                                                                                     `json:"candidate_selection_performed"`
	SchedulingPerformed         bool                                                                                                                     `json:"scheduling_performed"`
	ExecutionPerformed          bool                                                                                                                     `json:"execution_performed"`
	NodeExecutionPerformed      bool                                                                                                                     `json:"node_execution_performed"`
	QueueProcessingPerformed    bool                                                                                                                     `json:"queue_processing_performed"`
	RetryPerformed              bool                                                                                                                     `json:"retry_performed"`
	RepairPerformed             bool                                                                                                                     `json:"repair_performed"`
	CancellationPerformed       bool                                                                                                                     `json:"cancellation_performed"`
	CallbackInvoked             bool                                                                                                                     `json:"callback_invoked"`
	PublicationPerformed        bool                                                                                                                     `json:"publication_performed"`
	ProviderInvoked             bool                                                                                                                     `json:"provider_invoked"`
	ConnectorInvoked            bool                                                                                                                     `json:"connector_invoked"`
	BrokerInvoked               bool                                                                                                                     `json:"broker_invoked"`
	ForgePipeInvoked            bool                                                                                                                     `json:"forgepipe_invoked"`
	ProcessLaunched             bool                                                                                                                     `json:"process_launched"`
	NetworkUsed                 bool                                                                                                                     `json:"network_used"`
	RemoteExecutionPerformed    bool                                                                                                                     `json:"remote_execution_performed"`
	ValidationPerformed         bool                                                                                                                     `json:"validation_performed"`
	CheckoutMutated             bool                                                                                                                     `json:"checkout_mutated"`
	GitActionPerformed          bool                                                                                                                     `json:"git_action_performed"`
	CheckpointPerformed         bool                                                                                                                     `json:"checkpoint_performed"`
	CommitPerformed             bool                                                                                                                     `json:"commit_performed"`
	PushPerformed               bool                                                                                                                     `json:"push_performed"`
	ExternalActionPerformed     bool                                                                                                                     `json:"external_action_performed"`
	FixtureOwned                bool                                                                                                                     `json:"fixture_owned"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority `json:"authority"`
	RequestFingerprint          string                                                                                                                   `json:"request_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumer struct {
	id, contract string
}

func (value nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumer) ConsumerID() string {
	return value.id
}
func (value nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumer) ConsumerContractFingerprint() string {
	return value.contract
}
func (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumer) LookupAcknowledgement(NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, bool, error) {
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, errors.New("acknowledgement reconciliation policy cannot invoke a consumer")
}
func (nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumer) Deliver(NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, error) {
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, errors.New("acknowledgement reconciliation policy cannot deliver output")
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies struct {
	root            string
	expected        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected
	acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	receipt         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	decision        *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision
	request         *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest
	writeDecision   func(string, any) error
	writeRequest    func(string, any) error
	mu              sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies, error) {
	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies{root: root, expected: normalized, acknowledgement: inputs.acknowledgement, receipt: inputs.receipt, writeDecision: writeJSONFileAtomic, writeRequest: writeJSONFileAtomic}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(root, normalized, inputs)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(root, normalized, inputs, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("acknowledgement reconciliation policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionMax {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(policies.expected, policies.acknowledgement, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected(policies.root, policies.expected)
	if err != nil || !nodeExecutionEqual(normalized, policies.expected) || !nodeExecutionEqual(inputs.acknowledgement, policies.acknowledgement) || !nodeExecutionEqual(inputs.receipt, policies.receipt) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation policy could not revalidate the complete immutable predecessor chain")
	}
	durableDecision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(policies.root, policies.expected, inputs)
	if err != nil || policies.decision != nil && !decisionExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation decision is missing or conflicting")
	}
	durableRequest, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(policies.root, policies.expected, inputs, durableDecision, decisionExists)
	if err != nil || requestExists && !decisionExists || policies.request != nil && !requestExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation request is missing, orphaned, or conflicting")
	}
	if decisionExists {
		policies.decision = &durableDecision
	}
	if requestExists {
		policies.request = &durableRequest
	}
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "acknowledgement reconciliation decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, err
		}
		if err := policies.writeDecision(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("rejected acknowledgement reconciliation decision conflicts with a request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "acknowledgement reconciliation request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, err
	}
	if err := policies.writeRequest(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs, error) {
	consumer := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumer{id: value.Executor.Policy.ConsumerID, contract: value.Executor.Policy.ConsumerContractFingerprint}
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs(root, value.Executor, consumer)
	if err != nil || !inputs.acknowledgementExists || !inputs.receiptExists {
		return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("acknowledgement reconciliation policy requires the exact acknowledgement and delivery-executor receipt")
	}
	value.Executor = inputs.expected
	if value.AcknowledgementFingerprint != inputs.acknowledgement.AcknowledgementFingerprint || value.DeliveryExecutorReceiptFingerprint != inputs.receipt.ReceiptFingerprint || inputs.receipt.AcknowledgementID != inputs.acknowledgement.AcknowledgementID || inputs.receipt.AcknowledgementFingerprint != inputs.acknowledgement.AcknowledgementFingerprint || inputs.receipt.OperationKey != inputs.acknowledgement.OperationKey || !nodeExecutionEqual(inputs.receipt.Binding, inputs.acknowledgement.Binding) || !inputs.acknowledgement.Accepted || inputs.acknowledgement.AcceptedLocalConsumerDeliveryCount != 1 || !inputs.acknowledgement.FixtureOwned || inputs.acknowledgement.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) || inputs.receipt.LogicalDeliveryAttemptCount != 1 || inputs.receipt.ConsumerInvocationCount != 1 || inputs.receipt.AcceptedAcknowledgementCount != 1 || inputs.receipt.AcknowledgementArtifactWriteCount != 1 || inputs.receipt.ExecutorReceiptWriteCount != 1 || !inputs.receipt.AuthorizationConsumed || !inputs.receipt.CompleteImmutablePredecessorChainRevalidated || !inputs.receipt.NoDuplicateDelivery || inputs.receipt.ConsumerReinvoked || !inputs.receipt.FixtureOwned || inputs.receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) {
		return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("acknowledgement reconciliation policy delivery evidence is missing, stale, conflicting, or escalates authority")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReconciliationRequestID) {
		return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("acknowledgement reconciliation policy requires exact fixture authentication and request identities")
	}
	reconciliation := inputs.outputInputs.expected.Policy.Executor.Policy.Reconciliation
	priorIDs := []string{inputs.request.AuthenticationID, inputs.outputInputs.request.AuthenticationID, inputs.outputInputs.transition.Binding.PolicyAuthenticationID, reconciliation.AuthenticationID, reconciliation.Executor.Policy.DecisionAuthenticationID, reconciliation.Executor.Policy.Executor.Policy.DecisionAuthenticationID, reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.DecisionAuthenticationID}
	priorDigests := []string{inputs.request.AuthenticationDigest, inputs.outputInputs.request.AuthenticationDigest, inputs.outputInputs.transition.Binding.PolicyAuthenticationDigest, reconciliation.AuthenticationDigest, reconciliation.Executor.Policy.DecisionAuthenticationDigest, reconciliation.Executor.Policy.Executor.Policy.DecisionAuthenticationDigest, reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.DecisionAuthenticationDigest}
	for _, prior := range priorIDs {
		if value.DecisionAuthenticationID == prior {
			return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("acknowledgement reconciliation authentication reuses a prior identity")
		}
	}
	for _, prior := range priorDigests {
		if value.DecisionAuthenticationDigest == prior {
			return value, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{}, errors.New("acknowledgement reconciliation authentication reuses a prior digest")
		}
	}
	return value, inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected, acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest, error) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(acknowledgement, receipt)
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(fixture.Binding, binding) || !fixture.Deterministic || !fixture.OneTimeDecision || fixture.DecisionConsumed || fixture.ApprovalInferred || fixture.RouteInferred || fixture.AcknowledgementInferred || fixture.ConsumerInferred || fixture.ReconciliationInferred || fixture.AuthorityInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_post_delivery_acknowledgement_reconciliation_policy_decision" || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyIdentityCollides(fixture.DecisionID, fixture.ReplayIdentity, binding) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation fixture identity, authentication, binding, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("acknowledgement reconciliation decision is invalid")
	}
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding)
	if fixture.Decision == "rejected" {
		if fixture.Route != "" || fixture.PostState != "" || fixture.RouteSpecificEffect != "" || fixture.OutputType != "" || fixture.DeliveryType != "" || fixture.TerminalResult != "" || fixture.TaskOutcome != "" || fixture.ReconciliationRequestID != "" || fixture.ConsumerID != "" || fixture.ConsumerContractFingerprint != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("rejected acknowledgement reconciliation decision cannot name a route, output, delivery, consumer, request, or authority")
		}
	} else if !compatible || fixture.Route != binding.Route || fixture.PostState != binding.PostState || fixture.RouteSpecificEffect != binding.RouteSpecificEffect || fixture.OutputType != binding.OutputType || fixture.DeliveryType != binding.DeliveryType || fixture.TerminalResult != binding.TerminalResult || fixture.TaskOutcome != binding.TaskOutcome || fixture.ReconciliationRequestID != expected.ReconciliationRequestID || fixture.ConsumerID != binding.ConsumerID || fixture.ConsumerContractFingerprint != binding.ConsumerContractFingerprint || fixture.Authority != authority {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, errors.New("approved acknowledgement reconciliation decision requires the exact compatible route, consumer, and narrow authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Route: fixture.Route, PostState: fixture.PostState, RouteSpecificEffect: fixture.RouteSpecificEffect, OutputType: fixture.OutputType, DeliveryType: fixture.DeliveryType, TerminalResult: fixture.TerminalResult, TaskOutcome: fixture.TaskOutcome, Binding: binding, ReconciliationRequestID: fixture.ReconciliationRequestID, ConsumerID: fixture.ConsumerID, ConsumerContractFingerprint: fixture.ConsumerContractFingerprint, Deterministic: true, OneTimeDecision: true, IndependentlyAuthenticated: true, FixtureOwned: true}
	decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFingerprint(decision)
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(decision, expected, acknowledgement, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestSchema, RequestID: decision.ReconciliationRequestID, DecisionID: decision.DecisionID, DecisionReplayIdentity: decision.ReplayIdentity, DecisionFingerprint: decision.DecisionFingerprint, AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest, AcknowledgementID: acknowledgement.AcknowledgementID, AcknowledgementFingerprint: acknowledgement.AcknowledgementFingerprint, OperationKey: acknowledgement.OperationKey, Route: decision.Route, PostState: decision.PostState, RouteSpecificEffect: decision.RouteSpecificEffect, OutputType: decision.OutputType, DeliveryType: decision.DeliveryType, TerminalResult: decision.TerminalResult, TaskOutcome: decision.TaskOutcome, Binding: binding, ConsumerID: decision.ConsumerID, ConsumerContractFingerprint: decision.ConsumerContractFingerprint, OneTimeRequest: true, FixtureOwned: true, Authority: authority}
	request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestFingerprint(*request)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(decision, expected, acknowledgement, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(*request, expected, acknowledgement, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected, acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(acknowledgement, receipt)
	_, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFingerprint(value)
	rejected := value.Decision == "rejected" && value.Route == "" && value.PostState == "" && value.RouteSpecificEffect == "" && value.OutputType == "" && value.DeliveryType == "" && value.TerminalResult == "" && value.TaskOutcome == "" && value.ReconciliationRequestID == "" && value.ConsumerID == "" && value.ConsumerContractFingerprint == ""
	approved := value.Decision == "approved" && compatible && value.Route == binding.Route && value.PostState == binding.PostState && value.RouteSpecificEffect == binding.RouteSpecificEffect && value.OutputType == binding.OutputType && value.DeliveryType == binding.DeliveryType && value.TerminalResult == binding.TerminalResult && value.TaskOutcome == binding.TaskOutcome && value.ReconciliationRequestID == expected.ReconciliationRequestID && value.ConsumerID == binding.ConsumerID && value.ConsumerContractFingerprint == binding.ConsumerContractFingerprint
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.DecisionID == value.ReplayIdentity || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || !rejected && !approved || !nodeExecutionEqual(value.Binding, binding) || !value.Deterministic || !value.OneTimeDecision || value.DecisionConsumed || value.ApprovalInferred || value.RouteInferred || value.AcknowledgementInferred || value.ConsumerInferred || value.ReconciliationInferred || value.AuthorityInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}) || fingerprint != value.DecisionFingerprint || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyIdentityCollides(value.DecisionID, value.ReplayIdentity, binding) {
		return errors.New("acknowledgement reconciliation decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected, acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(acknowledgement, receipt)
	authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestFingerprint(value)
	if err != nil || decision.Decision != "approved" || !compatible || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestSchema || value.RequestID != expected.ReconciliationRequestID || value.DecisionID != decision.DecisionID || value.DecisionReplayIdentity != decision.ReplayIdentity || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || value.AcknowledgementID != acknowledgement.AcknowledgementID || value.AcknowledgementFingerprint != acknowledgement.AcknowledgementFingerprint || value.OperationKey != acknowledgement.OperationKey || value.Route != decision.Route || value.PostState != decision.PostState || value.RouteSpecificEffect != decision.RouteSpecificEffect || value.OutputType != decision.OutputType || value.DeliveryType != decision.DeliveryType || value.TerminalResult != decision.TerminalResult || value.TaskOutcome != decision.TaskOutcome || !nodeExecutionEqual(value.Binding, binding) || value.ConsumerID != decision.ConsumerID || value.ConsumerContractFingerprint != decision.ConsumerContractFingerprint || !value.OneTimeRequest || value.AuthorizationConsumed || value.AcknowledgementReconciled || value.LifecycleAdvanced || value.GraphMutated || value.DependencyWorkPerformed || value.DependencyReleasePerformed || value.FailurePropagationPerformed || value.CandidateDiscoveryPerformed || value.CandidateSelectionPerformed || value.SchedulingPerformed || value.ExecutionPerformed || value.NodeExecutionPerformed || value.QueueProcessingPerformed || value.RetryPerformed || value.RepairPerformed || value.CancellationPerformed || value.CallbackInvoked || value.PublicationPerformed || value.ProviderInvoked || value.ConnectorInvoked || value.BrokerInvoked || value.ForgePipeInvoked || value.ProcessLaunched || value.NetworkUsed || value.RemoteExecutionPerformed || value.ValidationPerformed || value.CheckoutMutated || value.GitActionPerformed || value.CheckpointPerformed || value.CommitPerformed || value.PushPerformed || value.ExternalActionPerformed || !value.FixtureOwned || value.Authority != authority || fingerprint != value.RequestFingerprint {
		return errors.New("acknowledgement reconciliation request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("acknowledgement reconciliation decision is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(value, expected, inputs.acknowledgement, inputs.receipt); err != nil {
		return value, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return value, false, nil
		}
		return value, false, errors.New("acknowledgement reconciliation request is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(value, expected, inputs.acknowledgement, inputs.receipt, decision) != nil {
		return value, false, errors.New("acknowledgement reconciliation request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationArtifactMax {
		return errors.New("acknowledgement reconciliation artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("acknowledgement reconciliation artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("acknowledgement reconciliation artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding {
	b := acknowledgement.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding{AcknowledgementID: acknowledgement.AcknowledgementID, AcknowledgementFingerprint: acknowledgement.AcknowledgementFingerprint, OperationKey: acknowledgement.OperationKey, DeliveryExecutorReceiptID: receipt.ExecutorReceiptID, DeliveryExecutorReceiptFingerprint: receipt.ReceiptFingerprint, Route: b.Route, PostState: b.PostState, RouteSpecificEffect: b.RouteSpecificEffect, OutputType: b.OutputType, DeliveryType: b.DeliveryType, ConsumerID: b.ConsumerID, ConsumerContractFingerprint: b.ConsumerContractFingerprint, TerminalResult: b.TerminalResult, TaskOutcome: b.TaskOutcome, DeliveryExecutorBinding: b}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority, bool) {
	switch {
	case binding.Route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute && binding.PostState == "continued" && binding.RouteSpecificEffect == "passed_selected_task_continued_local_graph" && binding.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput && binding.DeliveryType == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery && binding.TaskOutcome == "passed" && binding.TerminalResult == "succeeded":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{ContinuationHandoffAcknowledgementReconciliationAttempt: true}, true
	case binding.Route == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute && binding.PostState == "succeeded" && binding.RouteSpecificEffect == "passed_result_finalized_local_graph_successfully" && binding.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization && binding.DeliveryType == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery && binding.TaskOutcome == "passed" && binding.TerminalResult == "succeeded":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt: true}, true
	case binding.Route == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute && binding.PostState == "failed" && binding.RouteSpecificEffect == "failed_result_finalized_local_graph_with_failure_propagation" && binding.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization && binding.DeliveryType == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery && binding.TaskOutcome == "failed" && binding.TerminalResult == "failed":
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{FailedTerminalGraphResultAcknowledgementReconciliationAttempt: true}, true
	default:
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}, false
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyIdentityCollides(decisionID, replayIdentity string, binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding) bool {
	b := binding.DeliveryExecutorBinding
	for _, value := range []string{binding.AcknowledgementID, binding.DeliveryExecutorReceiptID, binding.ConsumerID, b.DeliveryPolicyDecisionID, b.DeliveryPolicyRequestID, b.OutputRecordID, b.OutputExecutorReceiptID, b.OutputPolicyDecisionID, b.OutputPolicyRequestID, b.TransitionExecutorReceiptID, b.TransitionRecordID, b.GraphRunID, b.TerminalTaskID, b.SelectedTaskID, b.AcceptedResultID, b.ReconciliationReceiptID} {
		if decisionID == value || replayIdentity == value {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}
func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}
func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
