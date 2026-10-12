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
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-policy-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionSchema        = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-policy-decision/v1"
	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestSchema         = "dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-policy-request/v1"

	NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery            = "continuation_handoff_delivery"
	NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery         = "successful_terminal_graph_result_delivery"
	NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery             = "failed_terminal_graph_result_delivery"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName = "node-placement-execution-graph-next-task-result-continuation-output-delivery-policy-decision.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName  = "node-placement-execution-graph-next-task-result-continuation-output-delivery-policy-request.json"
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionMax  = 4 << 20
	nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryArtifactMax  = 8 << 20
)

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyLocks sync.Map

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority
// grants only one future route-compatible local consumer attempt.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority struct {
	ContinuationHandoffDeliveryAttempt           bool `json:"continuation_handoff_delivery_attempt"`
	SuccessfulTerminalGraphResultDeliveryAttempt bool `json:"successful_terminal_graph_result_delivery_attempt"`
	FailedTerminalGraphResultDeliveryAttempt     bool `json:"failed_terminal_graph_result_delivery_attempt"`
	Delivery                                     bool `json:"delivery"`
	Consumption                                  bool `json:"consumption"`
	Acknowledgement                              bool `json:"acknowledgement"`
	ReceiverInvocation                           bool `json:"receiver_invocation"`
	LifecycleAdvancement                         bool `json:"lifecycle_advancement"`
	GraphMutation                                bool `json:"graph_mutation"`
	DependencyRelease                            bool `json:"dependency_release"`
	FailurePropagation                           bool `json:"failure_propagation"`
	Scheduling                                   bool `json:"scheduling"`
	Execution                                    bool `json:"execution"`
	Retry                                        bool `json:"retry"`
	Repair                                       bool `json:"repair"`
	Cancellation                                 bool `json:"cancellation"`
	Callback                                     bool `json:"callback"`
	Publication                                  bool `json:"publication"`
	Provider                                     bool `json:"provider"`
	Connector                                    bool `json:"connector"`
	Broker                                       bool `json:"broker"`
	ForgePipe                                    bool `json:"forgepipe"`
	Network                                      bool `json:"network"`
	RemoteExecution                              bool `json:"remote_execution"`
	Validation                                   bool `json:"validation"`
	CheckoutMutation                             bool `json:"checkout_mutation"`
	Git                                          bool `json:"git"`
	Checkpoint                                   bool `json:"checkpoint"`
	Commit                                       bool `json:"commit"`
	Push                                         bool `json:"push"`
	ExternalAction                               bool `json:"external_action"`
}

// NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding
// binds the exact durable output, its executor receipt, and the complete immutable predecessor binding.
type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding struct {
	OutputRecordID                       string                                                                              `json:"output_record_id"`
	OutputRecordFingerprint              string                                                                              `json:"output_record_fingerprint"`
	OutputRecordVersion                  uint64                                                                              `json:"output_record_version"`
	OutputExecutorReceiptID              string                                                                              `json:"output_executor_receipt_id"`
	OutputExecutorReceiptFingerprint     string                                                                              `json:"output_executor_receipt_fingerprint"`
	OutputPolicyDecisionID               string                                                                              `json:"output_policy_decision_id"`
	OutputPolicyDecisionFingerprint      string                                                                              `json:"output_policy_decision_fingerprint"`
	OutputPolicyRequestID                string                                                                              `json:"output_policy_request_id"`
	OutputPolicyRequestFingerprint       string                                                                              `json:"output_policy_request_fingerprint"`
	OutputPolicyAuthenticationID         string                                                                              `json:"output_policy_authentication_id"`
	OutputPolicyAuthenticationDigest     string                                                                              `json:"output_policy_authentication_digest"`
	TransitionExecutorReceiptID          string                                                                              `json:"transition_executor_receipt_id"`
	TransitionExecutorReceiptFingerprint string                                                                              `json:"transition_executor_receipt_fingerprint"`
	TransitionRecordID                   string                                                                              `json:"transition_record_id"`
	TransitionRecordFingerprint          string                                                                              `json:"transition_record_fingerprint"`
	Route                                string                                                                              `json:"route"`
	PostState                            string                                                                              `json:"post_state"`
	RouteSpecificEffect                  string                                                                              `json:"route_specific_effect"`
	OutputType                           string                                                                              `json:"output_type"`
	GraphRunID                           string                                                                              `json:"graph_run_id"`
	TerminalTaskID                       string                                                                              `json:"terminal_task_id"`
	SelectedTaskID                       string                                                                              `json:"selected_task_id"`
	CandidatesFingerprint                string                                                                              `json:"candidates_fingerprint"`
	AcceptedResultID                     string                                                                              `json:"accepted_result_id"`
	AcceptedResultFingerprint            string                                                                              `json:"accepted_result_fingerprint"`
	ReconciliationReceiptID              string                                                                              `json:"reconciliation_receipt_id"`
	ReconciliationReceiptFingerprint     string                                                                              `json:"reconciliation_receipt_fingerprint"`
	TerminalResult                       string                                                                              `json:"terminal_result"`
	TaskOutcome                          string                                                                              `json:"task_outcome"`
	OutputExecutorBinding                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBinding `json:"output_executor_binding"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected struct {
	Executor                         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected `json:"executor"`
	OutputRecordFingerprint          string                                                                               `json:"output_record_fingerprint"`
	OutputExecutorReceiptFingerprint string                                                                               `json:"output_executor_receipt_fingerprint"`
	DecisionAuthenticationID         string                                                                               `json:"decision_authentication_id"`
	DecisionAuthenticationDigest     string                                                                               `json:"decision_authentication_digest"`
	DeliveryRequestID                string                                                                               `json:"delivery_request_id"`
	ConsumerID                       string                                                                               `json:"consumer_id"`
	ConsumerContractFingerprint      string                                                                               `json:"consumer_contract_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture struct {
	Schema                      string                                                                                      `json:"schema"`
	DecisionID                  string                                                                                      `json:"decision_id"`
	ReplayIdentity              string                                                                                      `json:"replay_identity"`
	AuthenticationID            string                                                                                      `json:"authentication_id"`
	AuthenticationDigest        string                                                                                      `json:"authentication_digest"`
	Decision                    string                                                                                      `json:"decision"`
	Route                       string                                                                                      `json:"route,omitempty"`
	OutputType                  string                                                                                      `json:"output_type,omitempty"`
	DeliveryType                string                                                                                      `json:"delivery_type,omitempty"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding   `json:"binding"`
	DeliveryRequestID           string                                                                                      `json:"delivery_request_id,omitempty"`
	ConsumerID                  string                                                                                      `json:"consumer_id,omitempty"`
	ConsumerContractFingerprint string                                                                                      `json:"consumer_contract_fingerprint,omitempty"`
	Deterministic               bool                                                                                        `json:"deterministic"`
	OneTimeDecision             bool                                                                                        `json:"one_time_decision"`
	DecisionConsumed            bool                                                                                        `json:"decision_consumed"`
	ApprovalInferred            bool                                                                                        `json:"approval_inferred"`
	RouteInferred               bool                                                                                        `json:"route_inferred"`
	OutputInferred              bool                                                                                        `json:"output_inferred"`
	DeliveryTypeInferred        bool                                                                                        `json:"delivery_type_inferred"`
	ConsumerInferred            bool                                                                                        `json:"consumer_inferred"`
	AuthorityInferred           bool                                                                                        `json:"authority_inferred"`
	InferenceSource             string                                                                                      `json:"inference_source,omitempty"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority `json:"authority"`
	Provenance                  string                                                                                      `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision struct {
	Schema                      string                                                                                      `json:"schema"`
	DecisionID                  string                                                                                      `json:"decision_id"`
	ReplayIdentity              string                                                                                      `json:"replay_identity"`
	AuthenticationID            string                                                                                      `json:"authentication_id"`
	AuthenticationDigest        string                                                                                      `json:"authentication_digest"`
	Decision                    string                                                                                      `json:"decision"`
	Route                       string                                                                                      `json:"route,omitempty"`
	OutputType                  string                                                                                      `json:"output_type,omitempty"`
	DeliveryType                string                                                                                      `json:"delivery_type,omitempty"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding   `json:"binding"`
	DeliveryRequestID           string                                                                                      `json:"delivery_request_id,omitempty"`
	ConsumerID                  string                                                                                      `json:"consumer_id,omitempty"`
	ConsumerContractFingerprint string                                                                                      `json:"consumer_contract_fingerprint,omitempty"`
	Deterministic               bool                                                                                        `json:"deterministic"`
	OneTimeDecision             bool                                                                                        `json:"one_time_decision"`
	DecisionConsumed            bool                                                                                        `json:"decision_consumed"`
	ApprovalInferred            bool                                                                                        `json:"approval_inferred"`
	RouteInferred               bool                                                                                        `json:"route_inferred"`
	OutputInferred              bool                                                                                        `json:"output_inferred"`
	DeliveryTypeInferred        bool                                                                                        `json:"delivery_type_inferred"`
	ConsumerInferred            bool                                                                                        `json:"consumer_inferred"`
	AuthorityInferred           bool                                                                                        `json:"authority_inferred"`
	InferenceSource             string                                                                                      `json:"inference_source,omitempty"`
	IndependentlyAuthenticated  bool                                                                                        `json:"independently_authenticated"`
	FixtureOwned                bool                                                                                        `json:"fixture_owned"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority `json:"authority"`
	DecisionFingerprint         string                                                                                      `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest struct {
	Schema                      string                                                                                      `json:"schema"`
	RequestID                   string                                                                                      `json:"request_id"`
	DecisionID                  string                                                                                      `json:"decision_id"`
	DecisionReplayIdentity      string                                                                                      `json:"decision_replay_identity"`
	DecisionFingerprint         string                                                                                      `json:"decision_fingerprint"`
	AuthenticationID            string                                                                                      `json:"authentication_id"`
	AuthenticationDigest        string                                                                                      `json:"authentication_digest"`
	Route                       string                                                                                      `json:"route"`
	OutputType                  string                                                                                      `json:"output_type"`
	DeliveryType                string                                                                                      `json:"delivery_type"`
	Binding                     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding   `json:"binding"`
	ConsumerID                  string                                                                                      `json:"consumer_id"`
	ConsumerContractFingerprint string                                                                                      `json:"consumer_contract_fingerprint"`
	OneTimeRequest              bool                                                                                        `json:"one_time_request"`
	AuthorizationConsumed       bool                                                                                        `json:"authorization_consumed"`
	DeliveryPerformed           bool                                                                                        `json:"delivery_performed"`
	ConsumerInvoked             bool                                                                                        `json:"consumer_invoked"`
	AcknowledgementReceived     bool                                                                                        `json:"acknowledgement_received"`
	CallbackInvoked             bool                                                                                        `json:"callback_invoked"`
	LifecycleActionTriggered    bool                                                                                        `json:"lifecycle_action_triggered"`
	PublicationPerformed        bool                                                                                        `json:"publication_performed"`
	ExternalActionPerformed     bool                                                                                        `json:"external_action_performed"`
	FixtureOwned                bool                                                                                        `json:"fixture_owned"`
	Authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority `json:"authority"`
	RequestFingerprint          string                                                                                      `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies struct {
	root          string
	expected      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected
	output        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	receipt       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	decision      *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision
	request       *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	writeDecision func(string, any) error
	writeRequest  func(string, any) error
	mu            sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) (*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies, error) {
	normalized, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected(root, expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies{root: root, expected: normalized, output: inputs.output, receipt: inputs.receipt, writeDecision: writeJSONFileAtomic, writeRequest: writeJSONFileAtomic}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(root, normalized, inputs)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(root, normalized, inputs, decision, decisionExists)
	if err != nil || requestExists && !decisionExists {
		return nil, errors.New("graph output delivery policy artifacts are orphaned or conflicting")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (policies *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, error) {
	policies.mu.Lock()
	defer policies.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionMax {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery policy decision fixture is empty or oversized")
	}
	var fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery policy decision fixture is malformed or noncanonical")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(policies.expected, policies.output, policies.receipt, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyLocks.LoadOrStore(policies.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	_, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected(policies.root, policies.expected)
	if err != nil || !nodeExecutionEqual(inputs.output, policies.output) || !nodeExecutionEqual(inputs.receipt, policies.receipt) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery policy could not revalidate the complete immutable predecessor chain")
	}
	durableDecision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(policies.root, policies.expected, inputs)
	if err != nil || policies.decision != nil && !decisionExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery decision is missing or conflicting")
	}
	durableRequest, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(policies.root, policies.expected, inputs, durableDecision, decisionExists)
	if err != nil || requestExists && !decisionExists || policies.request != nil && !requestExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery request is missing, orphaned, or conflicting")
	}
	if decisionExists {
		policies.decision = &durableDecision
	}
	if requestExists {
		policies.request = &durableRequest
	}
	if policies.decision != nil {
		if !nodeExecutionEqual(*policies.decision, decision) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery decision conflicts with accepted evidence")
		}
	} else {
		path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName)
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "graph output delivery policy decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
		}
		if err := policies.writeDecision(path, decision); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery policy decision could not be published")
		}
		policies.decision = &decision
	}
	if request == nil {
		if policies.request != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("rejected graph output delivery decision conflicts with a request")
		}
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(decision), nil, nil
	}
	if policies.request != nil {
		if !nodeExecutionEqual(*policies.request, *request) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery request conflicts with accepted evidence")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(*policies.request)
		return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(decision), &cloned, nil
	}
	path := filepath.Join(policies.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(path, "graph output delivery policy request"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
	}
	if err := policies.writeRequest(path, *request); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery policy request could not be published")
	}
	policies.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected(root string, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(root, value.Executor)
	if err != nil || !inputs.outputExists || !inputs.receiptExists {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("graph output delivery policy requires the exact durable output record and executor receipt")
	}
	value.Executor = inputs.expected
	_, _, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(inputs.output, inputs.receipt)
	if !compatible || value.OutputRecordFingerprint != inputs.output.RecordFingerprint || value.OutputExecutorReceiptFingerprint != inputs.receipt.ReceiptFingerprint || inputs.output.Version != 1 || !inputs.output.FixtureOwned || inputs.receipt.OutputRecordID != inputs.output.OutputRecordID || inputs.receipt.OutputRecordFingerprint != inputs.output.RecordFingerprint || inputs.receipt.OutputRecordVersion != inputs.output.Version || !nodeExecutionEqual(inputs.receipt.Binding, inputs.output.Binding) || inputs.receipt.OutputActionCount != 1 || inputs.receipt.OutputRecordWriteCount != 1 || !inputs.receipt.AuthorizationConsumed || !inputs.receipt.FixtureOwned {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("graph output delivery policy output evidence is missing, stale, conflicting, or escalates authority")
	}
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionAuthenticationID) || !nodeExecutionFingerprint.MatchString(value.DecisionAuthenticationDigest) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DeliveryRequestID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ConsumerID) || !nodeExecutionFingerprint.MatchString(value.ConsumerContractFingerprint) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("graph output delivery policy requires exact fixture authentication, request, consumer, and contract identities")
	}
	reconciliation := inputs.expected.Policy.Executor.Policy.Reconciliation
	priorIDs := []string{
		inputs.request.AuthenticationID,
		inputs.transition.Binding.PolicyAuthenticationID,
		reconciliation.AuthenticationID,
		reconciliation.Executor.Policy.DecisionAuthenticationID,
		reconciliation.Executor.Policy.Executor.Policy.DecisionAuthenticationID,
		reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.DecisionAuthenticationID,
	}
	priorDigests := []string{
		inputs.request.AuthenticationDigest,
		inputs.transition.Binding.PolicyAuthenticationDigest,
		reconciliation.AuthenticationDigest,
		reconciliation.Executor.Policy.DecisionAuthenticationDigest,
		reconciliation.Executor.Policy.Executor.Policy.DecisionAuthenticationDigest,
		reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.DecisionAuthenticationDigest,
	}
	for _, prior := range priorIDs {
		if value.DecisionAuthenticationID == prior {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("graph output delivery policy authentication reuses a prior identity")
		}
	}
	for _, prior := range priorDigests {
		if value.DecisionAuthenticationDigest == prior {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected{}, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs{}, errors.New("graph output delivery policy authentication reuses a prior digest")
		}
	}
	return value, inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected, output NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt, fixture NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, error) {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(output, receipt)
	if fixture.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.AuthenticationID != expected.DecisionAuthenticationID || fixture.AuthenticationDigest != expected.DecisionAuthenticationDigest || !nodeExecutionEqual(fixture.Binding, binding) || !fixture.Deterministic || !fixture.OneTimeDecision || fixture.DecisionConsumed || fixture.ApprovalInferred || fixture.RouteInferred || fixture.OutputInferred || fixture.DeliveryTypeInferred || fixture.ConsumerInferred || fixture.AuthorityInferred || fixture.InferenceSource != "" || fixture.Provenance != "fixture_only_graph_output_delivery_policy_decision" || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyIdentityCollides(fixture.DecisionID, fixture.ReplayIdentity, binding, expected.ConsumerID) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery fixture identity, authentication, binding, or independent authority is invalid")
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("graph output delivery decision is invalid")
	}
	deliveryType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(output, receipt)
	if fixture.Decision == "rejected" {
		if fixture.Route != "" || fixture.OutputType != "" || fixture.DeliveryType != "" || fixture.DeliveryRequestID != "" || fixture.ConsumerID != "" || fixture.ConsumerContractFingerprint != "" || fixture.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{}) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("rejected graph output delivery decision cannot name a route, output, delivery, request, consumer, or authority")
		}
	} else if !compatible || fixture.Route != output.Binding.Route || fixture.OutputType != output.Binding.OutputType || fixture.DeliveryType != deliveryType || fixture.DeliveryRequestID != expected.DeliveryRequestID || fixture.ConsumerID != expected.ConsumerID || fixture.ConsumerContractFingerprint != expected.ConsumerContractFingerprint || fixture.Authority != authority {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, errors.New("approved graph output delivery decision requires the exact compatible route, consumer, and narrow authority")
	}
	decision := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, AuthenticationID: fixture.AuthenticationID, AuthenticationDigest: fixture.AuthenticationDigest, Decision: fixture.Decision, Route: fixture.Route, OutputType: fixture.OutputType, DeliveryType: fixture.DeliveryType, Binding: binding, DeliveryRequestID: fixture.DeliveryRequestID, ConsumerID: fixture.ConsumerID, ConsumerContractFingerprint: fixture.ConsumerContractFingerprint, Deterministic: true, OneTimeDecision: true, IndependentlyAuthenticated: true, FixtureOwned: true}
	var err error
	decision.DecisionFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
	}
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(decision, expected, output, receipt)
	}
	request := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestSchema, RequestID: decision.DeliveryRequestID, DecisionID: decision.DecisionID, DecisionReplayIdentity: decision.ReplayIdentity, DecisionFingerprint: decision.DecisionFingerprint, AuthenticationID: decision.AuthenticationID, AuthenticationDigest: decision.AuthenticationDigest, Route: decision.Route, OutputType: decision.OutputType, DeliveryType: decision.DeliveryType, Binding: binding, ConsumerID: decision.ConsumerID, ConsumerContractFingerprint: decision.ConsumerContractFingerprint, OneTimeRequest: true, FixtureOwned: true, Authority: authority}
	request.RequestFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(decision, expected, output, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(*request, expected, output, receipt, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected, output NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(output, receipt)
	deliveryType, _, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(output, receipt)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFingerprint(value)
	rejected := value.Decision == "rejected" && value.Route == "" && value.OutputType == "" && value.DeliveryType == "" && value.DeliveryRequestID == "" && value.ConsumerID == "" && value.ConsumerContractFingerprint == ""
	approved := value.Decision == "approved" && compatible && value.Route == output.Binding.Route && value.OutputType == output.Binding.OutputType && value.DeliveryType == deliveryType && value.DeliveryRequestID == expected.DeliveryRequestID && value.ConsumerID == expected.ConsumerID && value.ConsumerContractFingerprint == expected.ConsumerContractFingerprint
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.ReplayIdentity) || value.DecisionID == value.ReplayIdentity || value.AuthenticationID != expected.DecisionAuthenticationID || value.AuthenticationDigest != expected.DecisionAuthenticationDigest || !rejected && !approved || !nodeExecutionEqual(value.Binding, binding) || !value.Deterministic || !value.OneTimeDecision || value.DecisionConsumed || value.ApprovalInferred || value.RouteInferred || value.OutputInferred || value.DeliveryTypeInferred || value.ConsumerInferred || value.AuthorityInferred || value.InferenceSource != "" || !value.IndependentlyAuthenticated || !value.FixtureOwned || value.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{}) || fingerprint != value.DecisionFingerprint || nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyIdentityCollides(value.DecisionID, value.ReplayIdentity, binding, expected.ConsumerID) {
		return errors.New("graph output delivery decision is invalid or escalates authority")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected, output NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision) error {
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(output, receipt)
	deliveryType, authority, compatible := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(output, receipt)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestFingerprint(value)
	if err != nil || decision.Decision != "approved" || !compatible || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestSchema || value.RequestID != expected.DeliveryRequestID || value.DecisionID != decision.DecisionID || value.DecisionReplayIdentity != decision.ReplayIdentity || value.DecisionFingerprint != decision.DecisionFingerprint || value.AuthenticationID != decision.AuthenticationID || value.AuthenticationDigest != decision.AuthenticationDigest || value.Route != output.Binding.Route || value.Route != decision.Route || value.OutputType != output.Binding.OutputType || value.OutputType != decision.OutputType || value.DeliveryType != deliveryType || value.DeliveryType != decision.DeliveryType || !nodeExecutionEqual(value.Binding, binding) || value.ConsumerID != expected.ConsumerID || value.ConsumerID != decision.ConsumerID || value.ConsumerContractFingerprint != expected.ConsumerContractFingerprint || value.ConsumerContractFingerprint != decision.ConsumerContractFingerprint || !value.OneTimeRequest || value.AuthorizationConsumed || value.DeliveryPerformed || value.ConsumerInvoked || value.AcknowledgementReceived || value.CallbackInvoked || value.LifecycleActionTriggered || value.PublicationPerformed || value.ExternalActionPerformed || !value.FixtureOwned || value.Authority != authority || fingerprint != value.RequestFingerprint {
		return errors.New("graph output delivery request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, false, errors.New("graph output delivery decision is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(value, expected, inputs.output, inputs.receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(root string, expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected, inputs nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)
	var value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest{}, false, errors.New("graph output delivery request is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if !decisionExists || decision.Decision != "approved" || validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(value, expected, inputs.output, inputs.receipt, decision) != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest{}, false, errors.New("graph output delivery request is orphaned, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryArtifactMax {
		return errors.New("graph output delivery artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("graph output delivery artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("graph output delivery artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(output NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding {
	b := output.Binding
	return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding{OutputRecordID: output.OutputRecordID, OutputRecordFingerprint: output.RecordFingerprint, OutputRecordVersion: output.Version, OutputExecutorReceiptID: receipt.ExecutorReceiptID, OutputExecutorReceiptFingerprint: receipt.ReceiptFingerprint, OutputPolicyDecisionID: b.OutputPolicyDecisionID, OutputPolicyDecisionFingerprint: b.OutputPolicyDecisionFingerprint, OutputPolicyRequestID: b.OutputPolicyRequestID, OutputPolicyRequestFingerprint: b.OutputPolicyRequestFingerprint, OutputPolicyAuthenticationID: b.OutputPolicyAuthenticationID, OutputPolicyAuthenticationDigest: b.OutputPolicyAuthenticationDigest, TransitionExecutorReceiptID: b.TransitionExecutorReceiptID, TransitionExecutorReceiptFingerprint: b.TransitionExecutorReceiptFingerprint, TransitionRecordID: b.TransitionRecordID, TransitionRecordFingerprint: b.TransitionRecordFingerprint, Route: b.Route, PostState: b.PostState, RouteSpecificEffect: b.RouteSpecificEffect, OutputType: b.OutputType, GraphRunID: b.GraphRunID, TerminalTaskID: b.TerminalTaskID, SelectedTaskID: b.SelectedTaskID, CandidatesFingerprint: b.CandidatesFingerprint, AcceptedResultID: b.AcceptedResultID, AcceptedResultFingerprint: b.AcceptedResultFingerprint, ReconciliationReceiptID: b.ReconciliationReceiptID, ReconciliationReceiptFingerprint: b.ReconciliationReceiptFingerprint, TerminalResult: b.TerminalResult, TaskOutcome: b.TaskOutcome, OutputExecutorBinding: b}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(output NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) (string, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority, bool) {
	if output.Binding.Route != receipt.Route || output.Binding.PostState != receipt.ExactPostState || output.Binding.RouteSpecificEffect != receipt.RouteSpecificEffect || output.Binding.OutputType != receipt.OutputType || output.Binding.TerminalResult != output.Binding.ExecutorBinding.TerminalResult || output.Binding.TaskOutcome != output.Binding.ExecutorBinding.TaskOutcome {
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{}, false
	}
	switch {
	case receipt.Route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute && receipt.ExactPostState == "continued" && receipt.RouteSpecificEffect == "passed_selected_task_continued_local_graph" && receipt.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput && output.Binding.TaskOutcome == "passed" && output.Binding.TerminalResult == "succeeded" && receipt.Evidence == (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{ContinuationHandoffMaterialized: true}):
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{ContinuationHandoffDeliveryAttempt: true}, true
	case receipt.Route == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute && receipt.ExactPostState == "succeeded" && receipt.RouteSpecificEffect == "passed_result_finalized_local_graph_successfully" && receipt.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization && output.Binding.TaskOutcome == "passed" && output.Binding.TerminalResult == "succeeded" && receipt.Evidence == (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{SuccessfulTerminalGraphResultMaterialized: true}):
		return NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{SuccessfulTerminalGraphResultDeliveryAttempt: true}, true
	case receipt.Route == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute && receipt.ExactPostState == "failed" && receipt.RouteSpecificEffect == "failed_result_finalized_local_graph_with_failure_propagation" && receipt.OutputType == NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization && output.Binding.TaskOutcome == "failed" && output.Binding.TerminalResult == "failed" && receipt.Evidence == (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{FailedTerminalGraphResultMaterialized: true}):
		return NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{FailedTerminalGraphResultDeliveryAttempt: true}, true
	default:
		return "", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{}, false
	}
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyIdentityCollides(decisionID, replayIdentity string, binding NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding, consumerID string) bool {
	b := binding.OutputExecutorBinding
	for _, value := range []string{binding.OutputRecordID, binding.OutputExecutorReceiptID, binding.OutputPolicyDecisionID, binding.OutputPolicyRequestID, binding.TransitionExecutorReceiptID, binding.TransitionRecordID, binding.AcceptedResultID, binding.ReconciliationReceiptID, b.ExecutorBinding.PolicyDecisionID, b.ExecutorBinding.PolicyRequestID, b.ExecutorBinding.ObservationID, b.ExecutorBinding.AttemptID, b.ExecutorBinding.ExecutorReceiptID, b.ExecutorBinding.LaunchAuthorizationDecisionID, b.ExecutorBinding.LaunchAuthorizationRequestID, b.ExecutorBinding.SchedulingReceiptID, b.ExecutorBinding.SchedulingPolicyDecisionID, b.ExecutorBinding.SchedulingPolicyRequestID, b.GraphRunID, b.TerminalTaskID, b.SelectedTaskID, b.ExecutorBinding.ScheduledRecordID, consumerID} {
		if decisionID == value || replayIdentity == value {
			return true
		}
	}
	return false
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
