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
	NodeConnectorPlacementExecutionGraphLifecycleRecordSchema               = "dorkpipe.node-placement-execution-graph-lifecycle-record-fixture/v1"
	NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptSchema = "dorkpipe.node-placement-execution-graph-lifecycle-executor-audit-receipt/v1"

	nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptName = "node-placement-execution-graph-lifecycle-executor-audit-receipt.json"
	nodeConnectorPlacementExecutionGraphLifecycleExecutorArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteReceiptAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphLifecycleExecutorLocks              sync.Map
)

// NodeConnectorPlacementExecutionGraphLifecycleRecord is a strict
// package-owned fixture record. The executor may replace exactly one existing
// bound record; it cannot create a missing record or mutate any adjacent state.
type NodeConnectorPlacementExecutionGraphLifecycleRecord struct {
	Schema                    string `json:"schema"`
	GraphStoreID              string `json:"graph_store_id"`
	GraphRecordID             string `json:"graph_record_id"`
	GraphRunID                string `json:"graph_run_id"`
	LifecycleState            string `json:"lifecycle_state"`
	Version                   uint64 `json:"version"`
	PreviousRecordFingerprint string `json:"previous_record_fingerprint,omitempty"`
	RecordFingerprint         string `json:"record_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditAuthority proves
// only the exact local record projection. Every adjacent lifecycle action
// remains independently unauthorized.
type NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditAuthority struct {
	LocalGraphRecordStateProjection bool `json:"local_graph_record_state_projection"`
	DependencyRelease               bool `json:"dependency_release"`
	NextTask                        bool `json:"next_task"`
	Retry                           bool `json:"retry"`
	Repair                          bool `json:"repair"`
	Cancellation                    bool `json:"cancellation"`
	Execution                       bool `json:"execution"`
	Broker                          bool `json:"broker"`
	ForgePipe                       bool `json:"forgepipe"`
	Provider                        bool `json:"provider"`
	Network                         bool `json:"network"`
	Validation                      bool `json:"validation"`
	Checkout                        bool `json:"checkout"`
	Git                             bool `json:"git"`
	Commit                          bool `json:"commit"`
	Push                            bool `json:"push"`
	Publication                     bool `json:"publication"`
	Lifecycle                       bool `json:"lifecycle"`
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                              `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                              `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt binds the
// exact immutable predecessor chain and the one-record compare-and-swap. It is
// evidence only and grants no downstream lifecycle authority.
type NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt struct {
	Schema                          string                                                                `json:"schema"`
	AuditReceiptID                  string                                                                `json:"audit_receipt_id"`
	GraphStoreID                    string                                                                `json:"graph_store_id"`
	GraphRecordID                   string                                                                `json:"graph_record_id"`
	GraphRunID                      string                                                                `json:"graph_run_id"`
	Preimage                        NodeConnectorPlacementExecutionGraphLifecycleRecord                   `json:"preimage"`
	PreimageFingerprint             string                                                                `json:"preimage_fingerprint"`
	PreimageVersion                 uint64                                                                `json:"preimage_version"`
	Postimage                       NodeConnectorPlacementExecutionGraphLifecycleRecord                   `json:"postimage"`
	PostimageFingerprint            string                                                                `json:"postimage_fingerprint"`
	PostimageVersion                uint64                                                                `json:"postimage_version"`
	PolicyDecisionID                string                                                                `json:"policy_decision_id"`
	PolicyDecisionFingerprint       string                                                                `json:"policy_decision_fingerprint"`
	PolicyRequestID                 string                                                                `json:"policy_request_id"`
	PolicyRequestFingerprint        string                                                                `json:"policy_request_fingerprint"`
	ProjectionDecisionID            string                                                                `json:"projection_decision_id"`
	ProjectionDecisionFingerprint   string                                                                `json:"projection_decision_fingerprint"`
	ProjectionRequestID             string                                                                `json:"projection_request_id"`
	ProjectionRequestFingerprint    string                                                                `json:"projection_request_fingerprint"`
	FinalizationDecisionID          string                                                                `json:"finalization_decision_id"`
	FinalizationDecisionFingerprint string                                                                `json:"finalization_decision_fingerprint"`
	FinalizationRequestID           string                                                                `json:"finalization_request_id"`
	FinalizationRequestFingerprint  string                                                                `json:"finalization_request_fingerprint"`
	TaskBindings                    []NodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBinding `json:"task_bindings"`
	TaskBindingsFingerprint         string                                                                `json:"task_bindings_fingerprint"`
	ProjectedTerminalPostState      string                                                                `json:"projected_terminal_post_state"`
	CompareAndSwapMatched           bool                                                                  `json:"compare_and_swap_matched"`
	RecordWriteCount                uint64                                                                `json:"record_write_count"`
	AuthorizationConsumed           bool                                                                  `json:"authorization_consumed"`
	FixtureOwned                    bool                                                                  `json:"fixture_owned"`
	Authority                       NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditAuthority   `json:"authority"`
	ReceiptFingerprint              string                                                                `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs struct {
	expected       NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected
	policyDecision NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision
	policyRequest  NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest
	preimage       NodeConnectorPlacementExecutionGraphLifecycleRecord
	postimage      NodeConnectorPlacementExecutionGraphLifecycleRecord
	receipt        NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt
	receiptExists  bool
	recordIsPost   bool
}

type NodeConnectorPlacementExecutionGraphLifecycleExecutor struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(root string, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected) (*NodeConnectorPlacementExecutionGraphLifecycleExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphLifecycleExecutor{root: root, expected: inputs.expected}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphLifecycleExecutor) Execute() (NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	recordPath := nodeConnectorPlacementExecutionGraphLifecycleExecutorRecordPath(executor.root, executor.expected.Policy.StorePrecondition)
	pathLock, _ := nodeConnectorPlacementExecutionGraphLifecycleExecutorLocks.LoadOrStore(recordPath, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(inputs.receipt), nil
	}
	if !inputs.recordIsPost {
		if err := nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic(recordPath, inputs.postimage); err != nil {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, errors.New("graph lifecycle record compare-and-swap replacement failed")
		}
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, err
	}
	receiptPath := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(receiptPath, "graph lifecycle executor audit receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, err
	}
	if err := nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteReceiptAtomic(receiptPath, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, errors.New("graph lifecycle executor audit receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphLifecycleExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected) (nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs, error) {
	policy, projectionDecision, projectionRequest, err := normalizeNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, errors.New("graph lifecycle executor requires the exact immutable projection and finalization chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision(root, policy, projectionDecision, projectionRequest)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, errors.New("graph lifecycle executor requires the exact approved policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest(root, policy, projectionDecision, projectionRequest, decision, true)
	if err != nil || !requestExists || request.AuthorizationConsumed || request.ExecutorInvoked || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint || request.Authority != (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority{LocalGraphStateProjectionExecutorAttempt: true}) || request.Requirements != nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees() {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, errors.New("graph lifecycle executor requires the exact approved unconsumed policy request")
	}
	preimage, postimage, err := deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorRecords(request)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, err
	}
	recordPath := nodeConnectorPlacementExecutionGraphLifecycleExecutorRecordPath(root, request.StorePrecondition)
	current, err := loadNodeConnectorPlacementExecutionGraphLifecycleRecord(recordPath)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, err
	}
	recordIsPost := nodeExecutionEqual(current, postimage)
	if !recordIsPost && !nodeExecutionEqual(current, preimage) {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, errors.New("graph lifecycle record does not match the exact expected preimage or recoverable postimage")
	}
	inputs := nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{expected: expected, policyDecision: decision, policyRequest: request, preimage: preimage, postimage: postimage, recordIsPost: recordIsPost}
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, err
	}
	if receiptExists && !recordIsPost {
		return nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs{}, errors.New("graph lifecycle executor audit receipt is stale because its exact postimage is absent")
	}
	inputs.receipt = receipt
	inputs.receiptExists = receiptExists
	return inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorRecords(request NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest) (NodeConnectorPlacementExecutionGraphLifecycleRecord, NodeConnectorPlacementExecutionGraphLifecycleRecord, error) {
	precondition := request.StorePrecondition
	if !validNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition(precondition) || precondition.ExpectedPreimageVersion == ^uint64(0) || (request.ProjectedTerminalPostState != "succeeded" && request.ProjectedTerminalPostState != "failed") {
		return NodeConnectorPlacementExecutionGraphLifecycleRecord{}, NodeConnectorPlacementExecutionGraphLifecycleRecord{}, errors.New("graph lifecycle executor store precondition or projected terminal state is invalid")
	}
	preimage := NodeConnectorPlacementExecutionGraphLifecycleRecord{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleRecordSchema, GraphStoreID: precondition.GraphStoreID, GraphRecordID: precondition.GraphRecordID,
		GraphRunID: request.GraphRunID, LifecycleState: "running", Version: precondition.ExpectedPreimageVersion,
	}
	preimageFingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleRecordFingerprint(preimage)
	if err != nil || preimageFingerprint != precondition.ExpectedPreimageFingerprint {
		return NodeConnectorPlacementExecutionGraphLifecycleRecord{}, NodeConnectorPlacementExecutionGraphLifecycleRecord{}, errors.New("graph lifecycle executor expected preimage fingerprint does not bind the exact fixture record")
	}
	preimage.RecordFingerprint = preimageFingerprint
	postimage := NodeConnectorPlacementExecutionGraphLifecycleRecord{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleRecordSchema, GraphStoreID: precondition.GraphStoreID, GraphRecordID: precondition.GraphRecordID,
		GraphRunID: request.GraphRunID, LifecycleState: request.ProjectedTerminalPostState, Version: precondition.ExpectedPreimageVersion + 1, PreviousRecordFingerprint: preimageFingerprint,
	}
	postimageFingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleRecordFingerprint(postimage)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleRecord{}, NodeConnectorPlacementExecutionGraphLifecycleRecord{}, err
	}
	postimage.RecordFingerprint = postimageFingerprint
	return preimage, postimage, nil
}

func deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(inputs nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs) NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt {
	request := inputs.policyRequest
	receipt := NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptSchema, AuditReceiptID: request.RequestID + "-audit",
		GraphStoreID: request.StorePrecondition.GraphStoreID, GraphRecordID: request.StorePrecondition.GraphRecordID, GraphRunID: request.GraphRunID,
		Preimage: inputs.preimage, PreimageFingerprint: inputs.preimage.RecordFingerprint, PreimageVersion: inputs.preimage.Version,
		Postimage: inputs.postimage, PostimageFingerprint: inputs.postimage.RecordFingerprint, PostimageVersion: inputs.postimage.Version,
		PolicyDecisionID: inputs.policyDecision.DecisionID, PolicyDecisionFingerprint: inputs.policyDecision.DecisionFingerprint, PolicyRequestID: request.RequestID, PolicyRequestFingerprint: request.RequestFingerprint,
		ProjectionDecisionID: request.ProjectionDecisionID, ProjectionDecisionFingerprint: request.ProjectionDecisionFingerprint, ProjectionRequestID: request.ProjectionRequestID, ProjectionRequestFingerprint: request.ProjectionRequestFingerprint,
		FinalizationDecisionID: request.FinalizationDecisionID, FinalizationDecisionFingerprint: request.FinalizationDecisionFingerprint, FinalizationRequestID: request.FinalizationRequestID, FinalizationRequestFingerprint: request.FinalizationRequestFingerprint,
		TaskBindings: cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(request.TaskBindings), TaskBindingsFingerprint: request.TaskBindingsFingerprint,
		ProjectedTerminalPostState: request.ProjectedTerminalPostState, CompareAndSwapMatched: true, RecordWriteCount: 1, AuthorizationConsumed: true, FixtureOwned: true,
		Authority: NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditAuthority{LocalGraphRecordStateProjection: true},
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(value NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, inputs nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.AuditReceiptID) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("graph lifecycle executor audit receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphLifecycleRecord(path string) (NodeConnectorPlacementExecutionGraphLifecycleRecord, error) {
	var value NodeConnectorPlacementExecutionGraphLifecycleRecord
	if err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorCanonicalArtifact(path, &value, false); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleRecord{}, errors.New("bound graph lifecycle record is missing, malformed, noncanonical, oversized, or unsafe")
	}
	fingerprint, err := nodeConnectorPlacementExecutionGraphLifecycleRecordFingerprint(value)
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphLifecycleRecordSchema || !validNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition(NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition{GraphStoreID: value.GraphStoreID, GraphRecordID: value.GraphRecordID, ExpectedPreimageFingerprint: value.RecordFingerprint, ExpectedPreimageVersion: value.Version}) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.GraphRunID) || (value.LifecycleState != "running" && value.LifecycleState != "succeeded" && value.LifecycleState != "failed") || fingerprint != value.RecordFingerprint || (value.LifecycleState == "running" && value.PreviousRecordFingerprint != "") || (value.LifecycleState != "running" && !nodeExecutionFingerprint.MatchString(value.PreviousRecordFingerprint)) {
		return NodeConnectorPlacementExecutionGraphLifecycleRecord{}, errors.New("bound graph lifecycle record is invalid or tampered")
	}
	return value, nil
}

func loadNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(root string, inputs nodeConnectorPlacementExecutionGraphLifecycleExecutorInputs) (NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptName)
	var value NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt
	if err := loadNodeConnectorPlacementExecutionGraphLifecycleExecutorCanonicalArtifact(path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, false, errors.New("graph lifecycle executor audit receipt is malformed, noncanonical, oversized, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphLifecycleExecutorCanonicalArtifact(path string, target any, allowMissing bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return err
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphLifecycleExecutorArtifactMaxBytes {
		return errors.New("graph lifecycle executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("graph lifecycle executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("graph lifecycle executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphLifecycleExecutorRecordPath(root string, precondition NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition) string {
	path := filepath.Join(root, "graph-stores", precondition.GraphStoreID, precondition.GraphRecordID+".json")
	absolute, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(path)
}

func nodeConnectorPlacementExecutionGraphLifecycleRecordFingerprint(value NodeConnectorPlacementExecutionGraphLifecycleRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptFingerprint(value NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt(value NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt) NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
