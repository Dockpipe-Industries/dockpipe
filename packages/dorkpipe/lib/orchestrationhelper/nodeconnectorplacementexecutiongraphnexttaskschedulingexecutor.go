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
	NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordSchema          = "dorkpipe.node-placement-execution-graph-next-task-scheduling-record-fixture/v1"
	NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptSchema = "dorkpipe.node-placement-execution-graph-next-task-scheduling-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptName      = "node-placement-execution-graph-next-task-scheduling-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteReceiptAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorLocks              sync.Map
)

// NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord is fixture-owned
// local scheduling state. It is separate from the immutable dependency postimage.
type NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord struct {
	Schema                                 string `json:"schema"`
	GraphRunID                             string `json:"graph_run_id"`
	TaskID                                 string `json:"task_id"`
	DependencyRecordID                     string `json:"dependency_record_id"`
	ReleasedDependencyPostimageFingerprint string `json:"released_dependency_postimage_fingerprint"`
	ReleasedDependencyPostimageVersion     uint64 `json:"released_dependency_postimage_version"`
	State                                  string `json:"state"`
	Version                                uint64 `json:"version"`
	PreviousRecordFingerprint              string `json:"previous_record_fingerprint,omitempty"`
	SchedulingRequestID                    string `json:"scheduling_request_id,omitempty"`
	SchedulingRequestFingerprint           string `json:"scheduling_request_fingerprint,omitempty"`
	RecordFingerprint                      string `json:"record_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                               `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                               `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority
// describes only the local transition already performed. It grants no future action.
type NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority struct {
	LocalSchedulingTransitionPerformed bool `json:"local_scheduling_transition_performed"`
	TaskLaunch                         bool `json:"task_launch"`
	NodeExecution                      bool `json:"node_execution"`
	Retry                              bool `json:"retry"`
	Repair                             bool `json:"repair"`
	Cancellation                       bool `json:"cancellation"`
	Publication                        bool `json:"publication"`
	Callback                           bool `json:"callback"`
	Validation                         bool `json:"validation"`
	Network                            bool `json:"network"`
	Broker                             bool `json:"broker"`
	Provider                           bool `json:"provider"`
	ForgePipe                          bool `json:"forgepipe"`
	RemoteExecution                    bool `json:"remote_execution"`
	Checkout                           bool `json:"checkout"`
	Git                                bool `json:"git"`
	Commit                             bool `json:"commit"`
	Push                               bool `json:"push"`
}

// NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt is
// durable proof of one local scheduling transition, never launch authority.
type NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt struct {
	Schema                       string                                                                   `json:"schema"`
	SchedulingReceiptID          string                                                                   `json:"scheduling_receipt_id"`
	GraphRunID                   string                                                                   `json:"graph_run_id"`
	TerminalTaskID               string                                                                   `json:"terminal_task_id"`
	PolicyDecisionID             string                                                                   `json:"policy_decision_id"`
	PolicyDecisionFingerprint    string                                                                   `json:"policy_decision_fingerprint"`
	PolicyRequestID              string                                                                   `json:"policy_request_id"`
	PolicyRequestFingerprint     string                                                                   `json:"policy_request_fingerprint"`
	AuthenticationID             string                                                                   `json:"authentication_id"`
	AuthenticationDigest         string                                                                   `json:"authentication_digest"`
	TransitionReceiptID          string                                                                   `json:"transition_receipt_id"`
	TransitionReceiptFingerprint string                                                                   `json:"transition_receipt_fingerprint"`
	Route                        string                                                                   `json:"route"`
	Transitions                  []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence `json:"transitions"`
	TransitionsFingerprint       string                                                                   `json:"transitions_fingerprint"`
	Candidates                   []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate        `json:"candidates"`
	CandidatesFingerprint        string                                                                   `json:"candidates_fingerprint"`
	SelectedTaskID               string                                                                   `json:"selected_task_id"`
	SelectedCandidate            NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate          `json:"selected_candidate"`
	Preimage                     NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord             `json:"preimage"`
	PreimageFingerprint          string                                                                   `json:"preimage_fingerprint"`
	PreimageVersion              uint64                                                                   `json:"preimage_version"`
	Postimage                    NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord             `json:"postimage"`
	PostimageFingerprint         string                                                                   `json:"postimage_fingerprint"`
	PostimageVersion             uint64                                                                   `json:"postimage_version"`
	SchedulingTransition         string                                                                   `json:"scheduling_transition"`
	TransitionCount              uint64                                                                   `json:"transition_count"`
	RecordWriteCount             uint64                                                                   `json:"record_write_count"`
	AuthorizationConsumed        bool                                                                     `json:"authorization_consumed"`
	FixtureOwned                 bool                                                                     `json:"fixture_owned"`
	Evidence                     NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority  `json:"evidence"`
	ReceiptFingerprint           string                                                                   `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs struct {
	expected      NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected
	decision      NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision
	request       NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest
	selected      NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate
	recordPath    string
	preimage      NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord
	postimage     NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord
	isPost        bool
	receipt       NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt
	receiptExists bool
}

type NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(root string, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected) (*NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor{root: root, expected: inputs.expected}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor) Execute() (NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	lockKey := filepath.Join(executor.root, "graph-stores")
	pathLock, _ := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorLocks.LoadOrStore(lockKey, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(inputs.receipt), nil
	}
	if !inputs.isPost {
		if err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic(inputs.recordPath, inputs.postimage); err != nil {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, errors.New("next-task scheduling record replacement failed")
		}
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, err
	}
	receiptPath := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(receiptPath, "next-task scheduling executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, err
	}
	if err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteReceiptAtomic(receiptPath, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, errors.New("next-task scheduling executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected) (nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs, error) {
	policy, transitionReceipt, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling executor requires the complete immutable predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(root, policy, transitionReceipt)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || decision.ApprovalInferred {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling executor requires the exact approved authenticated policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(root, policy, transitionReceipt, decision, true)
	expectedAuthority := NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{NextTaskSchedulingExecutorAttempt: true}
	if err != nil || !requestExists || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint || request.Binding.Route != "dependency_release_transition" || request.Authority != expectedAuthority || !request.OneTimeRequest || request.AuthorizationConsumed || request.SchedulingInvoked || request.TaskLaunched || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling executor requires the exact approved unconsumed scheduling request")
	}
	if !nodeExecutionEqual(request.Binding, decision.Binding) || !nodeExecutionEqual(request.Candidates, decision.Candidates) || request.CandidatesFingerprint != decision.CandidatesFingerprint || request.SelectedTaskID != decision.SelectedTaskID {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling policy decision and request bindings conflict")
	}
	selected, count := NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate{}, 0
	for _, candidate := range request.Candidates {
		if candidate.TaskID == request.SelectedTaskID {
			selected, count = candidate, count+1
		}
	}
	if count != 1 {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling selection is missing, duplicated, or outside the exact candidate set")
	}
	preimage, postimage, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecords(request, selected)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, err
	}
	path, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecordPath(root, request.Binding.PolicyBinding.GraphStoreID, selected.DependencyRecordID)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, err
	}
	current, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord(root, path)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, err
	}
	isPost := nodeExecutionEqual(current, postimage)
	if !isPost && !nodeExecutionEqual(current, preimage) {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling record does not match its exact preimage or same-request postimage")
	}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{expected: expected, decision: decision, request: request, selected: selected, recordPath: path, preimage: preimage, postimage: postimage, isPost: isPost}
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, err
	}
	if receiptExists && !isPost {
		return nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs{}, errors.New("next-task scheduling receipt is stale because its exact postimage is absent")
	}
	inputs.receipt, inputs.receiptExists = receipt, receiptExists
	return inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecords(request NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest, candidate NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord, NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord, error) {
	if request.Binding.Route != "dependency_release_transition" || request.SelectedTaskID != candidate.TaskID || candidate.ReleasedPostimageVersion == ^uint64(0) || !nodeExecutionFingerprint.MatchString(candidate.ReleasedPostimageFingerprint) {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{}, NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{}, errors.New("next-task scheduling route or selected released candidate is invalid")
	}
	preimage := NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordSchema, GraphRunID: request.Binding.GraphRunID, TaskID: candidate.TaskID,
		DependencyRecordID: candidate.DependencyRecordID, ReleasedDependencyPostimageFingerprint: candidate.ReleasedPostimageFingerprint,
		ReleasedDependencyPostimageVersion: candidate.ReleasedPostimageVersion, State: "dependency_released", Version: candidate.ReleasedPostimageVersion,
	}
	var err error
	preimage.RecordFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordFingerprint(preimage)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{}, NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{}, err
	}
	postimage := preimage
	postimage.State = "scheduled"
	postimage.Version++
	postimage.PreviousRecordFingerprint = preimage.RecordFingerprint
	postimage.SchedulingRequestID = request.RequestID
	postimage.SchedulingRequestFingerprint = request.RequestFingerprint
	postimage.RecordFingerprint = ""
	postimage.RecordFingerprint, err = nodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordFingerprint(postimage)
	return preimage, postimage, err
}

func deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs) NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt {
	receipt := NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptSchema, SchedulingReceiptID: inputs.request.RequestID + "-evidence",
		GraphRunID: inputs.request.Binding.GraphRunID, TerminalTaskID: inputs.request.Binding.TerminalTaskID,
		PolicyDecisionID: inputs.decision.DecisionID, PolicyDecisionFingerprint: inputs.decision.DecisionFingerprint,
		PolicyRequestID: inputs.request.RequestID, PolicyRequestFingerprint: inputs.request.RequestFingerprint,
		AuthenticationID: inputs.request.AuthenticationID, AuthenticationDigest: inputs.request.AuthenticationDigest,
		TransitionReceiptID: inputs.request.Binding.TransitionReceiptID, TransitionReceiptFingerprint: inputs.request.Binding.TransitionReceiptFingerprint,
		Route: inputs.request.Binding.Route, Transitions: cloneNodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence(inputs.request.Binding.Transitions), TransitionsFingerprint: inputs.request.Binding.TransitionsFingerprint,
		Candidates: cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(inputs.request.Candidates), CandidatesFingerprint: inputs.request.CandidatesFingerprint,
		SelectedTaskID: inputs.request.SelectedTaskID, SelectedCandidate: inputs.selected,
		Preimage: inputs.preimage, PreimageFingerprint: inputs.preimage.RecordFingerprint, PreimageVersion: inputs.preimage.Version,
		Postimage: inputs.postimage, PostimageFingerprint: inputs.postimage.RecordFingerprint, PostimageVersion: inputs.postimage.Version,
		SchedulingTransition: "dependency_released_to_scheduled", TransitionCount: 1, RecordWriteCount: 1, AuthorizationConsumed: true, FixtureOwned: true,
		Evidence: NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority{LocalSchedulingTransitionPerformed: true},
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.SchedulingReceiptID) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("next-task scheduling executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord(root, path string) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord, error) {
	var value NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorCanonicalArtifact(root, path, &value, false); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{}, errors.New("next-task scheduling record is missing, malformed, noncanonical, oversized, symlinked, or unsafe")
	}
	fingerprint, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordFingerprint(value)
	validState := value.State == "dependency_released" || value.State == "scheduled"
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.GraphRunID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TaskID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DependencyRecordID) || value.TaskID == value.DependencyRecordID || !nodeExecutionFingerprint.MatchString(value.ReleasedDependencyPostimageFingerprint) || value.ReleasedDependencyPostimageVersion == 0 || !validState || value.Version == 0 || fingerprint != value.RecordFingerprint || value.State == "dependency_released" && (value.Version != value.ReleasedDependencyPostimageVersion || value.PreviousRecordFingerprint != "" || value.SchedulingRequestID != "" || value.SchedulingRequestFingerprint != "") || value.State == "scheduled" && (value.Version != value.ReleasedDependencyPostimageVersion+1 || !nodeExecutionFingerprint.MatchString(value.PreviousRecordFingerprint) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.SchedulingRequestID) || !nodeExecutionFingerprint.MatchString(value.SchedulingRequestFingerprint)) {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord{}, errors.New("next-task scheduling record is invalid or tampered")
	}
	return value, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorInputs) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, false, errors.New("next-task scheduling executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifactMaxBytes {
		return errors.New("next-task scheduling executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("next-task scheduling executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("next-task scheduling executor artifact is noncanonical")
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecordPath(root, storeID, recordID string) (string, error) {
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(storeID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(recordID) {
		return "", errors.New("next-task scheduling store or record identity is invalid")
	}
	path, err := filepath.Abs(filepath.Join(root, "graph-stores", storeID, "scheduling", recordID+".json"))
	if err != nil {
		return "", errors.New("next-task scheduling record path is invalid")
	}
	return filepath.Clean(path), nil
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt(value NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
