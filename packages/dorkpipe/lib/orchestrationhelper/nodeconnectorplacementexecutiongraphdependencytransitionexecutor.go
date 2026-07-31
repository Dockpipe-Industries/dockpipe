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
	NodeConnectorPlacementExecutionGraphDependencyRecordSchema                    = "dorkpipe.node-placement-execution-graph-dependency-record-fixture/v1"
	NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptSchema = "dorkpipe.node-placement-execution-graph-dependency-transition-executor-receipt/v1"

	nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName      = "node-placement-execution-graph-dependency-transition-executor-receipt.json"
	nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteReceiptAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorLocks              sync.Map
)

// NodeConnectorPlacementExecutionGraphDependencyRecord is fixture-owned local
// dependency state. A successor is bound to one exact policy request and route.
type NodeConnectorPlacementExecutionGraphDependencyRecord struct {
	Schema                       string `json:"schema"`
	GraphRunID                   string `json:"graph_run_id"`
	DependencyID                 string `json:"dependency_id"`
	DependencyRecordID           string `json:"dependency_record_id"`
	State                        string `json:"state"`
	Version                      uint64 `json:"version"`
	PreviousRecordFingerprint    string `json:"previous_record_fingerprint,omitempty"`
	TransitionRequestID          string `json:"transition_request_id,omitempty"`
	TransitionRequestFingerprint string `json:"transition_request_fingerprint,omitempty"`
	Route                        string `json:"route,omitempty"`
	RecordFingerprint            string `json:"record_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected struct {
	Policy                    NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected `json:"policy"`
	PolicyDecisionFingerprint string                                                                 `json:"policy_decision_fingerprint"`
	PolicyRequestFingerprint  string                                                                 `json:"policy_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphDependencyTransitionEvidenceAuthority
// describes only the route already performed. It grants no future action.
type NodeConnectorPlacementExecutionGraphDependencyTransitionEvidenceAuthority struct {
	DependencyReleasePerformed  bool `json:"dependency_release_performed"`
	FailurePropagationPerformed bool `json:"failure_propagation_performed"`
	NextTaskScheduling          bool `json:"next_task_scheduling"`
	NewExecution                bool `json:"new_execution"`
	Retry                       bool `json:"retry"`
	Repair                      bool `json:"repair"`
	Cancellation                bool `json:"cancellation"`
	Callback                    bool `json:"callback"`
	Validation                  bool `json:"validation"`
	Publication                 bool `json:"publication"`
	Network                     bool `json:"network"`
	Broker                      bool `json:"broker"`
	Provider                    bool `json:"provider"`
	ForgePipe                   bool `json:"forgepipe"`
	Checkout                    bool `json:"checkout"`
	Git                         bool `json:"git"`
	Commit                      bool `json:"commit"`
	Push                        bool `json:"push"`
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence struct {
	Target               NodeConnectorPlacementExecutionGraphDependencyTransitionTarget `json:"target"`
	Preimage             NodeConnectorPlacementExecutionGraphDependencyRecord           `json:"preimage"`
	PreimageFingerprint  string                                                         `json:"preimage_fingerprint"`
	PreimageVersion      uint64                                                         `json:"preimage_version"`
	Postimage            NodeConnectorPlacementExecutionGraphDependencyRecord           `json:"postimage"`
	PostimageFingerprint string                                                         `json:"postimage_fingerprint"`
	PostimageVersion     uint64                                                         `json:"postimage_version"`
}

// NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt is
// canonical durable evidence of one completed local route. It is not authority
// for scheduling or any adjacent lifecycle action.
type NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt struct {
	Schema                       string                                                                    `json:"schema"`
	TransitionReceiptID          string                                                                    `json:"transition_receipt_id"`
	PolicyBinding                NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding     `json:"policy_binding"`
	PolicyDecisionID             string                                                                    `json:"policy_decision_id"`
	PolicyDecisionFingerprint    string                                                                    `json:"policy_decision_fingerprint"`
	PolicyRequestID              string                                                                    `json:"policy_request_id"`
	PolicyRequestFingerprint     string                                                                    `json:"policy_request_fingerprint"`
	AuthenticationID             string                                                                    `json:"authentication_id"`
	AuthenticationDigest         string                                                                    `json:"authentication_digest"`
	Route                        string                                                                    `json:"route"`
	DependencyTargets            []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget          `json:"dependency_targets"`
	DependencyTargetsFingerprint string                                                                    `json:"dependency_targets_fingerprint"`
	Transitions                  []NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence  `json:"transitions"`
	TransitionCount              uint64                                                                    `json:"transition_count"`
	RecordWriteCount             uint64                                                                    `json:"record_write_count"`
	AuthorizationConsumed        bool                                                                      `json:"authorization_consumed"`
	FixtureOwned                 bool                                                                      `json:"fixture_owned"`
	Evidence                     NodeConnectorPlacementExecutionGraphDependencyTransitionEvidenceAuthority `json:"evidence"`
	ReceiptFingerprint           string                                                                    `json:"receipt_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInput struct {
	target    NodeConnectorPlacementExecutionGraphDependencyTransitionTarget
	path      string
	preimage  NodeConnectorPlacementExecutionGraphDependencyRecord
	postimage NodeConnectorPlacementExecutionGraphDependencyRecord
	isPost    bool
}

type nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs struct {
	expected      NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected
	decision      NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision
	request       NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest
	targets       []nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInput
	receipt       NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt
	receiptExists bool
}

type NodeConnectorPlacementExecutionGraphDependencyTransitionExecutor struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(root string, expected NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) (*NodeConnectorPlacementExecutionGraphDependencyTransitionExecutor, error) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs(root, expected)
	if err != nil {
		return nil, err
	}
	return &NodeConnectorPlacementExecutionGraphDependencyTransitionExecutor{root: root, expected: inputs.expected}, nil
}

func (executor *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutor) Execute() (NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	lockKey := filepath.Join(executor.root, "graph-stores")
	pathLock, _ := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorLocks.LoadOrStore(lockKey, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs(executor.root, executor.expected)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, err
	}
	if inputs.receiptExists {
		return cloneNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(inputs.receipt), nil
	}
	for _, target := range inputs.targets {
		if target.isPost {
			continue
		}
		if err := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic(target.path, target.postimage); err != nil {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("dependency-transition target replacement failed")
		}
	}
	receipt := deriveNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(inputs)
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(receipt, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, err
	}
	receiptPath := filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName)
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(receiptPath, "dependency-transition executor receipt"); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, err
	}
	if err := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteReceiptAtomic(receiptPath, receipt); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, errors.New("dependency-transition executor receipt could not be published")
	}
	return cloneNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(receipt), nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs(root string, expected NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) (nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs, error) {
	policy, lifecycleReceipt, err := normalizeNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected(root, expected.Policy)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition executor requires the complete immutable lifecycle predecessor chain")
	}
	expected.Policy = policy
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision(root, policy, lifecycleReceipt)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.PolicyDecisionFingerprint || !decision.IndependentlyAuthenticated || !decision.FixtureOwned || decision.ApprovalInferred {
		return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition executor requires the exact approved authenticated policy decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest(root, policy, lifecycleReceipt, decision, true)
	expectedAuthority, routeValid := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRouteAuthority(lifecycleReceipt.ProjectedTerminalPostState, request.Route)
	if err != nil || !requestExists || request.RequestFingerprint != expected.PolicyRequestFingerprint || request.DecisionFingerprint != decision.DecisionFingerprint || !routeValid || request.Authority != expectedAuthority || !request.OneTimeRequest || request.AuthorizationConsumed || request.TransitionInvoked || request.CallbacksInvoked || !request.FixtureOwned {
		return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition executor requires the exact approved unconsumed route request")
	}
	if !nodeExecutionEqual(request.Binding, decision.Binding) || request.Route != decision.Route || !nodeExecutionEqual(request.DependencyTargets, decision.DependencyTargets) || request.DependencyTargetsFingerprint != decision.DependencyTargetsFingerprint {
		return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition policy decision and request bindings conflict")
	}
	inputs := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{expected: expected, decision: decision, request: request}
	inputs.targets = make([]nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInput, len(request.DependencyTargets))
	allPost := true
	seenPaths := make(map[string]struct{}, len(request.DependencyTargets))
	for index, target := range request.DependencyTargets {
		preimage, postimage, err := deriveNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRecords(request, target)
		if err != nil {
			return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, err
		}
		path, err := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRecordPath(root, request.Binding.GraphStoreID, target.DependencyRecordID)
		if err != nil {
			return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, err
		}
		if _, exists := seenPaths[path]; exists {
			return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition target record identity is duplicated")
		}
		seenPaths[path] = struct{}{}
		current, err := loadNodeConnectorPlacementExecutionGraphDependencyRecord(root, path)
		if err != nil {
			return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, err
		}
		isPost := nodeExecutionEqual(current, postimage)
		if !isPost && !nodeExecutionEqual(current, preimage) {
			return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition target does not match its exact preimage or same-request postimage")
		}
		allPost = allPost && isPost
		inputs.targets[index] = nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInput{target: target, path: path, preimage: preimage, postimage: postimage, isPost: isPost}
	}
	receipt, receiptExists, err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(root, inputs)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, err
	}
	if receiptExists && !allPost {
		return nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs{}, errors.New("dependency-transition receipt is stale because one or more exact postimages are absent")
	}
	inputs.receipt = receipt
	inputs.receiptExists = receiptExists
	return inputs, nil
}

func deriveNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRecords(request NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest, target NodeConnectorPlacementExecutionGraphDependencyTransitionTarget) (NodeConnectorPlacementExecutionGraphDependencyRecord, NodeConnectorPlacementExecutionGraphDependencyRecord, error) {
	if request.Route != "dependency_release_transition" && request.Route != "failure_propagation_transition" || target.ExpectedPreimageVersion == ^uint64(0) {
		return NodeConnectorPlacementExecutionGraphDependencyRecord{}, NodeConnectorPlacementExecutionGraphDependencyRecord{}, errors.New("dependency-transition route or target version is invalid")
	}
	preimage := NodeConnectorPlacementExecutionGraphDependencyRecord{Schema: NodeConnectorPlacementExecutionGraphDependencyRecordSchema, GraphRunID: request.Binding.GraphRunID, DependencyID: target.DependencyID, DependencyRecordID: target.DependencyRecordID, State: "blocked", Version: target.ExpectedPreimageVersion}
	preimageFingerprint, err := nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(preimage)
	if err != nil || preimageFingerprint != target.ExpectedPreimageFingerprint {
		return NodeConnectorPlacementExecutionGraphDependencyRecord{}, NodeConnectorPlacementExecutionGraphDependencyRecord{}, errors.New("dependency-transition target preimage fingerprint is stale or conflicting")
	}
	preimage.RecordFingerprint = preimageFingerprint
	state := "dependency_released"
	if request.Route == "failure_propagation_transition" {
		state = "failure_propagated"
	}
	postimage := NodeConnectorPlacementExecutionGraphDependencyRecord{
		Schema: NodeConnectorPlacementExecutionGraphDependencyRecordSchema, GraphRunID: request.Binding.GraphRunID, DependencyID: target.DependencyID, DependencyRecordID: target.DependencyRecordID,
		State: state, Version: target.ExpectedPreimageVersion + 1, PreviousRecordFingerprint: preimageFingerprint, TransitionRequestID: request.RequestID, TransitionRequestFingerprint: request.RequestFingerprint, Route: request.Route,
	}
	postimage.RecordFingerprint, err = nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(postimage)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyRecord{}, NodeConnectorPlacementExecutionGraphDependencyRecord{}, err
	}
	return preimage, postimage, nil
}

func deriveNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(inputs nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs) NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt {
	transitions := make([]NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence, len(inputs.targets))
	for index, target := range inputs.targets {
		transitions[index] = NodeConnectorPlacementExecutionGraphDependencyTransitionRecordEvidence{Target: target.target, Preimage: target.preimage, PreimageFingerprint: target.preimage.RecordFingerprint, PreimageVersion: target.preimage.Version, Postimage: target.postimage, PostimageFingerprint: target.postimage.RecordFingerprint, PostimageVersion: target.postimage.Version}
	}
	evidence := NodeConnectorPlacementExecutionGraphDependencyTransitionEvidenceAuthority{}
	if inputs.request.Route == "dependency_release_transition" {
		evidence.DependencyReleasePerformed = true
	} else {
		evidence.FailurePropagationPerformed = true
	}
	receipt := NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{
		Schema: NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptSchema, TransitionReceiptID: inputs.request.RequestID + "-evidence",
		PolicyBinding: inputs.request.Binding, PolicyDecisionID: inputs.decision.DecisionID, PolicyDecisionFingerprint: inputs.decision.DecisionFingerprint,
		PolicyRequestID: inputs.request.RequestID, PolicyRequestFingerprint: inputs.request.RequestFingerprint, AuthenticationID: inputs.request.AuthenticationID, AuthenticationDigest: inputs.request.AuthenticationDigest,
		Route: inputs.request.Route, DependencyTargets: cloneNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(inputs.request.DependencyTargets), DependencyTargetsFingerprint: inputs.request.DependencyTargetsFingerprint,
		Transitions: transitions, TransitionCount: uint64(len(transitions)), RecordWriteCount: uint64(len(transitions)), AuthorizationConsumed: true, FixtureOwned: true, Evidence: evidence,
	}
	receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptFingerprint(receipt)
	return receipt
}

func validateNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(value NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, inputs nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs) error {
	expected := deriveNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(inputs)
	fingerprint, err := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptFingerprint(value)
	if err != nil || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TransitionReceiptID) || fingerprint != value.ReceiptFingerprint || !nodeExecutionEqual(value, expected) {
		return errors.New("dependency-transition executor receipt is invalid, conflicting, or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyRecord(root, path string) (NodeConnectorPlacementExecutionGraphDependencyRecord, error) {
	var value NodeConnectorPlacementExecutionGraphDependencyRecord
	if err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorCanonicalArtifact(root, path, &value, false); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyRecord{}, errors.New("dependency-transition target is missing, malformed, noncanonical, oversized, symlinked, or unsafe")
	}
	fingerprint, err := nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(value)
	validState := value.State == "blocked" || value.State == "dependency_released" || value.State == "failure_propagated"
	if err != nil || value.Schema != NodeConnectorPlacementExecutionGraphDependencyRecordSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.GraphRunID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DependencyID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.DependencyRecordID) || value.DependencyID == value.DependencyRecordID || !validState || value.Version == 0 || fingerprint != value.RecordFingerprint || value.State == "blocked" && (value.PreviousRecordFingerprint != "" || value.TransitionRequestID != "" || value.TransitionRequestFingerprint != "" || value.Route != "") || value.State != "blocked" && (!nodeExecutionFingerprint.MatchString(value.PreviousRecordFingerprint) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.TransitionRequestID) || !nodeExecutionFingerprint.MatchString(value.TransitionRequestFingerprint) || value.Route != "dependency_release_transition" && value.Route != "failure_propagation_transition") {
		return NodeConnectorPlacementExecutionGraphDependencyRecord{}, errors.New("dependency-transition target record is invalid or tampered")
	}
	return value, nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(root string, inputs nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorInputs) (NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName)
	var value NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt
	if err := loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorCanonicalArtifact(root, path, &value, true); err != nil {
		if os.IsNotExist(err) {
			return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, false, nil
		}
		return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, false, errors.New("dependency-transition executor receipt is malformed, noncanonical, oversized, symlinked, unsafe, or conflicting")
	}
	if err := validateNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(value, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt{}, false, err
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorCanonicalArtifact(root, path string, target any, allowMissing bool) error {
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
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorArtifactMaxBytes {
		return errors.New("dependency-transition executor artifact is unsafe or exceeds its encoded bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil || decodeNodeExecutionStrict(raw, target) != nil {
		return errors.New("dependency-transition executor artifact is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("dependency-transition executor artifact is noncanonical")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorPath(root, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return errors.New("dependency-transition root is unsafe")
	}
	rootInfo, err := os.Lstat(rootAbs)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return errors.New("dependency-transition root is missing, symlinked, or unsafe")
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return errors.New("dependency-transition path is unsafe")
	}
	relative, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || relative == "." || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return errors.New("dependency-transition path escapes its fixture root")
	}
	current := rootAbs
	parts := splitCleanNodeConnectorPlacementExecutionGraphDependencyTransitionPath(relative)
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("dependency-transition path has an unsafe parent")
		}
	}
	return nil
}

func splitCleanNodeConnectorPlacementExecutionGraphDependencyTransitionPath(path string) []string {
	clean := filepath.Clean(path)
	parts := make([]string, 0, 4)
	for clean != "." && clean != string(filepath.Separator) {
		dir, base := filepath.Split(clean)
		parts = append([]string{base}, parts...)
		clean = filepath.Clean(dir)
	}
	return parts
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRecordPath(root, storeID, recordID string) (string, error) {
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(storeID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(recordID) {
		return "", errors.New("dependency-transition store or record identity is invalid")
	}
	path, err := filepath.Abs(filepath.Join(root, "graph-stores", storeID, "dependencies", recordID+".json"))
	if err != nil {
		return "", errors.New("dependency-transition target path is invalid")
	}
	return filepath.Clean(path), nil
}

func nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(value NodeConnectorPlacementExecutionGraphDependencyRecord) (string, error) {
	value.RecordFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptFingerprint(value NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt(value NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt) NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
