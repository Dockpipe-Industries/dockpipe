package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	NodeConnectorMultiTargetValidationInputSchema     = "dorkpipe.multi-target-validation-input/v1"
	NodeConnectorMultiTargetValidationAggregateSchema = "dorkpipe.multi-target-validation-aggregate/v1"

	nodeConnectorMultiTargetValidationProvenance     = "fixture_only_untrusted_receipt_aggregation"
	nodeConnectorMultiTargetValidationArtifactName   = "multi-target-validation-aggregate.json"
	nodeConnectorMultiTargetValidationTargetCount    = 3
	nodeConnectorMultiTargetValidationMaxInputBytes  = 256 << 10
	nodeConnectorMultiTargetValidationMaxOutputBytes = 64 << 10
	nodeConnectorMultiTargetValidationMaxEvents      = 1024
)

var nodeConnectorMultiTargetValidationWriteAtomic = writeJSONFileAtomic

type NodeConnectorMultiTargetValidationProfile struct {
	HostOS  string `json:"host_os"`
	Runtime string `json:"runtime"`
	GuestOS string `json:"guest_os"`
}

type NodeConnectorMultiTargetValidationExpectedTarget struct {
	TargetID             string                                    `json:"target_id"`
	Profile              NodeConnectorMultiTargetValidationProfile `json:"profile"`
	MachineID            string                                    `json:"machine_id"`
	MachineFingerprint   string                                    `json:"machine_fingerprint"`
	CapabilitySnapshotID string                                    `json:"capability_snapshot_id"`
	OperationID          string                                    `json:"operation_id"`
	RequestFingerprint   string                                    `json:"request_fingerprint"`
	LeaseID              string                                    `json:"lease_id"`
	LeaseFingerprint     string                                    `json:"lease_fingerprint"`
	Attempt              int                                       `json:"attempt"`
	EventsFingerprint    string                                    `json:"events_fingerprint"`
	ReceiptID            string                                    `json:"receipt_id"`
	ReceiptFingerprint   string                                    `json:"receipt_fingerprint"`
	LocalRunID           string                                    `json:"local_run_id"`
}

type NodeConnectorMultiTargetValidationExpected struct {
	AggregateID string                                             `json:"aggregate_id"`
	Targets     []NodeConnectorMultiTargetValidationExpectedTarget `json:"targets"`
}

// NodeConnectorMultiTargetValidationTargetEvidence contains only immutable
// node-execution evidence. Provider, connection, ingress, availability, quota,
// audit, retention, and managed-service claims are deliberately not inputs.
type NodeConnectorMultiTargetValidationTargetEvidence struct {
	TargetID        string                          `json:"target_id"`
	Machine         NodeExecutionMachineIdentity    `json:"machine"`
	Capability      NodeExecutionCapabilitySnapshot `json:"capability"`
	Request         NodeExecutionRequest            `json:"request"`
	Lease           NodeExecutionTaskLease          `json:"lease"`
	Events          []NodeExecutionEventEnvelope    `json:"events"`
	Cancellation    *NodeExecutionCancellation      `json:"cancellation,omitempty"`
	CancellationAck *NodeExecutionCancellationAck   `json:"cancellation_ack,omitempty"`
	Receipt         NodeExecutionReceipt            `json:"receipt"`
}

type NodeConnectorMultiTargetValidationInput struct {
	Schema      string                                             `json:"schema"`
	AggregateID string                                             `json:"aggregate_id"`
	Targets     []NodeConnectorMultiTargetValidationTargetEvidence `json:"targets"`
}

type NodeConnectorMultiTargetValidationReceiptBinding struct {
	TargetID                    string                                    `json:"target_id"`
	Profile                     NodeConnectorMultiTargetValidationProfile `json:"profile"`
	MachineID                   string                                    `json:"machine_id"`
	MachineFingerprint          string                                    `json:"machine_fingerprint"`
	CapabilitySnapshotID        string                                    `json:"capability_snapshot_id"`
	OperationID                 string                                    `json:"operation_id"`
	RequestFingerprint          string                                    `json:"request_fingerprint"`
	LeaseID                     string                                    `json:"lease_id"`
	LeaseFingerprint            string                                    `json:"lease_fingerprint"`
	Attempt                     int                                       `json:"attempt"`
	EventsFingerprint           string                                    `json:"events_fingerprint"`
	ReceiptID                   string                                    `json:"receipt_id"`
	ReceiptFingerprint          string                                    `json:"receipt_fingerprint"`
	LocalRunID                  string                                    `json:"local_run_id"`
	FinalCursor                 string                                    `json:"final_cursor"`
	ArtifactManifestFingerprint string                                    `json:"artifact_manifest_fingerprint"`
	TerminalResult              string                                    `json:"terminal_result"`
	CleanupStatus               string                                    `json:"cleanup_status"`
	CleanupEvidenceDigest       string                                    `json:"cleanup_evidence_digest,omitempty"`
	Outcome                     string                                    `json:"outcome"`
}

// NodeConnectorMultiTargetValidationAuthority is intentionally all-negative.
// An aggregate is evidence only and grants no execution or lifecycle authority.
type NodeConnectorMultiTargetValidationAuthority struct {
	Execution   bool `json:"execution"`
	Validation  bool `json:"validation"`
	Scheduling  bool `json:"scheduling"`
	Network     bool `json:"network"`
	Repair      bool `json:"repair"`
	Mutation    bool `json:"mutation"`
	Git         bool `json:"git"`
	Apply       bool `json:"apply"`
	Checkpoint  bool `json:"checkpoint"`
	Commit      bool `json:"commit"`
	Push        bool `json:"push"`
	Publication bool `json:"publication"`
	NextTask    bool `json:"next_task"`
	Completion  bool `json:"completion"`
}

type NodeConnectorMultiTargetValidationAggregate struct {
	Schema               string                                             `json:"schema"`
	AggregateID          string                                             `json:"aggregate_id"`
	AggregateFingerprint string                                             `json:"aggregate_fingerprint"`
	Targets              []NodeConnectorMultiTargetValidationExpectedTarget `json:"targets"`
	ReceiptBindings      []NodeConnectorMultiTargetValidationReceiptBinding `json:"receipt_bindings"`
	Status               string                                             `json:"status"`
	FailedTargetIDs      []string                                           `json:"failed_target_ids"`
	Provenance           string                                             `json:"provenance"`
	RepairDispatched     bool                                               `json:"repair_dispatched"`
	Authority            NodeConnectorMultiTargetValidationAuthority        `json:"authority"`
}

type NodeConnectorMultiTargetValidation struct {
	root      string
	expected  NodeConnectorMultiTargetValidationExpected
	aggregate *NodeConnectorMultiTargetValidationAggregate
	mu        sync.Mutex
}

func OpenNodeConnectorMultiTargetValidation(root string, expected NodeConnectorMultiTargetValidationExpected) (*NodeConnectorMultiTargetValidation, error) {
	normalized, err := normalizeNodeConnectorMultiTargetValidationExpected(expected)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("multi-target validation root must be an existing regular directory")
	}
	validation := &NodeConnectorMultiTargetValidation{root: root, expected: normalized}
	path := filepath.Join(root, nodeConnectorMultiTargetValidationArtifactName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return validation, nil
	}
	if err != nil {
		return nil, errors.New("multi-target validation aggregate cannot be read")
	}
	if len(raw) == 0 || len(raw) > nodeConnectorMultiTargetValidationMaxOutputBytes {
		return nil, errors.New("multi-target validation aggregate exceeds its encoded bound")
	}
	var aggregate NodeConnectorMultiTargetValidationAggregate
	if err := decodeNodeConnectorMultiTargetValidationArtifact(raw, &aggregate); err != nil {
		return nil, err
	}
	if err := validateNodeConnectorMultiTargetValidationAggregate(aggregate); err != nil {
		return nil, err
	}
	if !nodeExecutionEqual(aggregate.Targets, normalized.Targets) || aggregate.AggregateID != normalized.AggregateID {
		return nil, errors.New("multi-target validation expected bindings conflict with the durable aggregate")
	}
	validation.aggregate = &aggregate
	return validation, nil
}

func (validation *NodeConnectorMultiTargetValidation) Accept(raw []byte) (NodeConnectorMultiTargetValidationAggregate, error) {
	validation.mu.Lock()
	defer validation.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorMultiTargetValidationMaxInputBytes {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation input exceeds its encoded bound")
	}
	var input NodeConnectorMultiTargetValidationInput
	if err := decodeNodeExecutionCanonical(raw, &input); err != nil {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation input is not strict canonical JSON")
	}
	aggregate, err := deriveNodeConnectorMultiTargetValidationAggregate(validation.expected, input)
	if err != nil {
		return NodeConnectorMultiTargetValidationAggregate{}, err
	}
	if validation.aggregate != nil {
		if nodeExecutionEqual(*validation.aggregate, aggregate) {
			return cloneNodeConnectorMultiTargetValidationAggregate(*validation.aggregate), nil
		}
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("changed or conflicting multi-target validation replay is rejected")
	}
	path := filepath.Join(validation.root, nodeConnectorMultiTargetValidationArtifactName)
	if _, err := os.Lstat(path); err == nil {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation aggregate already exists")
	} else if !os.IsNotExist(err) {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation aggregate path cannot be inspected")
	}
	if err := nodeConnectorMultiTargetValidationWriteAtomic(path, aggregate); err != nil {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation aggregate could not be published")
	}
	validation.aggregate = &aggregate
	return cloneNodeConnectorMultiTargetValidationAggregate(aggregate), nil
}

func (validation *NodeConnectorMultiTargetValidation) Aggregate() (NodeConnectorMultiTargetValidationAggregate, bool) {
	validation.mu.Lock()
	defer validation.mu.Unlock()
	if validation.aggregate == nil {
		return NodeConnectorMultiTargetValidationAggregate{}, false
	}
	return cloneNodeConnectorMultiTargetValidationAggregate(*validation.aggregate), true
}

func deriveNodeConnectorMultiTargetValidationAggregate(expected NodeConnectorMultiTargetValidationExpected, input NodeConnectorMultiTargetValidationInput) (NodeConnectorMultiTargetValidationAggregate, error) {
	if input.Schema != NodeConnectorMultiTargetValidationInputSchema || input.AggregateID != expected.AggregateID || len(input.Targets) != nodeConnectorMultiTargetValidationTargetCount {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation input schema, aggregate, or target count is invalid")
	}
	evidenceByTarget := map[string]NodeConnectorMultiTargetValidationTargetEvidence{}
	for _, evidence := range input.Targets {
		if evidenceByTarget[evidence.TargetID].TargetID != "" {
			return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation input contains a duplicate target")
		}
		evidenceByTarget[evidence.TargetID] = evidence
	}
	aggregate := NodeConnectorMultiTargetValidationAggregate{
		Schema: NodeConnectorMultiTargetValidationAggregateSchema, AggregateID: expected.AggregateID,
		Targets:         append([]NodeConnectorMultiTargetValidationExpectedTarget{}, expected.Targets...),
		ReceiptBindings: []NodeConnectorMultiTargetValidationReceiptBinding{}, FailedTargetIDs: []string{},
		Provenance: nodeConnectorMultiTargetValidationProvenance,
	}
	for _, target := range expected.Targets {
		evidence, ok := evidenceByTarget[target.TargetID]
		if !ok {
			return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation input is missing an expected target")
		}
		binding, err := validateNodeConnectorMultiTargetValidationEvidence(target, evidence)
		if err != nil {
			return NodeConnectorMultiTargetValidationAggregate{}, fmt.Errorf("multi-target validation target %q failed closed: %w", target.TargetID, err)
		}
		aggregate.ReceiptBindings = append(aggregate.ReceiptBindings, binding)
		if binding.Outcome != "passed" {
			aggregate.FailedTargetIDs = append(aggregate.FailedTargetIDs, target.TargetID)
		}
		delete(evidenceByTarget, target.TargetID)
	}
	if len(evidenceByTarget) != 0 {
		return NodeConnectorMultiTargetValidationAggregate{}, errors.New("multi-target validation input contains an extra or unknown target")
	}
	if len(aggregate.FailedTargetIDs) == 0 {
		aggregate.Status = "passed"
	} else {
		aggregate.Status = "failed"
	}
	aggregate.AggregateFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(aggregate)
	if err != nil {
		return NodeConnectorMultiTargetValidationAggregate{}, err
	}
	aggregate.AggregateFingerprint = fingerprint
	if err := validateNodeConnectorMultiTargetValidationAggregate(aggregate); err != nil {
		return NodeConnectorMultiTargetValidationAggregate{}, err
	}
	return aggregate, nil
}

func validateNodeConnectorMultiTargetValidationEvidence(expected NodeConnectorMultiTargetValidationExpectedTarget, evidence NodeConnectorMultiTargetValidationTargetEvidence) (NodeConnectorMultiTargetValidationReceiptBinding, error) {
	if evidence.TargetID != expected.TargetID || len(evidence.Events) > nodeConnectorMultiTargetValidationMaxEvents {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, errors.New("target identity or event count is invalid")
	}
	if err := validateNodeExecutionMachine(evidence.Machine); err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	if err := validateNodeExecutionCapability(evidence.Capability); err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	if err := validateNodeExecutionRequest(evidence.Request); err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	if err := validateNodeExecutionLease(evidence.Lease); err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	operation := nodeExecutionOperationState{
		Request: evidence.Request, Lease: evidence.Lease, Events: append([]NodeExecutionEventEnvelope{}, evidence.Events...),
		Cancellation: evidence.Cancellation, CancellationAck: evidence.CancellationAck, Receipt: &evidence.Receipt, ExecutionCount: 1,
	}
	state := nodeExecutionBrokerState{
		Schema: nodeExecutionBrokerStateSchema, Generation: 1, Machine: evidence.Machine,
		Capabilities: []NodeExecutionCapabilitySnapshot{evidence.Capability},
		Operations:   map[string]nodeExecutionOperationState{evidence.Request.OperationID: operation},
	}
	if err := finalizeNodeExecutionState(&state); err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	machineFingerprint, err := nodeExecutionFingerprintValue(evidence.Machine)
	if err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	leaseFingerprint, err := nodeExecutionFingerprintValue(evidence.Lease)
	if err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	eventsFingerprint, err := nodeExecutionFingerprintValue(evidence.Events)
	if err != nil {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, err
	}
	actualGuest := evidence.Capability.Observed.GuestOS
	if actualGuest == "" {
		actualGuest = "none"
	}
	if evidence.Machine.MachineID != expected.MachineID || machineFingerprint != expected.MachineFingerprint || evidence.Capability.MachineID != expected.MachineID ||
		evidence.Capability.SnapshotID != expected.CapabilitySnapshotID || evidence.Request.CapabilitySnapshotID != expected.CapabilitySnapshotID ||
		evidence.Request.OperationID != expected.OperationID || evidence.Request.RequestFingerprint != expected.RequestFingerprint ||
		evidence.Lease.MachineID != expected.MachineID || evidence.Lease.CapabilitySnapshotID != expected.CapabilitySnapshotID || evidence.Lease.OperationID != expected.OperationID ||
		evidence.Lease.LeaseID != expected.LeaseID || leaseFingerprint != expected.LeaseFingerprint || evidence.Lease.Attempt != expected.Attempt || eventsFingerprint != expected.EventsFingerprint ||
		evidence.Receipt.ReceiptID != expected.ReceiptID || evidence.Receipt.ReceiptFingerprint != expected.ReceiptFingerprint || evidence.Receipt.LocalRunID != expected.LocalRunID ||
		evidence.Capability.Observed.HostOS != expected.Profile.HostOS || evidence.Capability.Observed.Runtime != expected.Profile.Runtime || actualGuest != expected.Profile.GuestOS {
		return NodeConnectorMultiTargetValidationReceiptBinding{}, errors.New("immutable target identity or profile binding is substituted")
	}
	outcome := "failed"
	if evidence.Receipt.Result == "succeeded" && evidence.Receipt.Cleanup.Status == "not_required" {
		outcome = "passed"
	}
	return NodeConnectorMultiTargetValidationReceiptBinding{
		TargetID: expected.TargetID, Profile: expected.Profile, MachineID: evidence.Receipt.MachineID, MachineFingerprint: machineFingerprint,
		CapabilitySnapshotID: evidence.Receipt.CapabilitySnapshotID, OperationID: evidence.Receipt.OperationID,
		RequestFingerprint: evidence.Receipt.RequestFingerprint, LeaseID: evidence.Receipt.LeaseID, LeaseFingerprint: leaseFingerprint, Attempt: evidence.Receipt.Attempt,
		EventsFingerprint: eventsFingerprint,
		ReceiptID:         evidence.Receipt.ReceiptID, ReceiptFingerprint: evidence.Receipt.ReceiptFingerprint,
		LocalRunID: evidence.Receipt.LocalRunID, FinalCursor: evidence.Receipt.FinalCursor,
		ArtifactManifestFingerprint: evidence.Receipt.Artifacts.ManifestFingerprint,
		TerminalResult:              evidence.Receipt.Result, CleanupStatus: evidence.Receipt.Cleanup.Status,
		CleanupEvidenceDigest: evidence.Receipt.Cleanup.EvidenceDigest, Outcome: outcome,
	}, nil
}

func normalizeNodeConnectorMultiTargetValidationExpected(value NodeConnectorMultiTargetValidationExpected) (NodeConnectorMultiTargetValidationExpected, error) {
	value.Targets = append([]NodeConnectorMultiTargetValidationExpectedTarget{}, value.Targets...)
	sort.Slice(value.Targets, func(i, j int) bool { return value.Targets[i].TargetID < value.Targets[j].TargetID })
	if err := validateNodeExecutionTypedID("aggregate", value.AggregateID); err != nil || len(value.Targets) != nodeConnectorMultiTargetValidationTargetCount {
		return NodeConnectorMultiTargetValidationExpected{}, errors.New("multi-target validation expected aggregate or target count is invalid")
	}
	wantedProfiles := map[NodeConnectorMultiTargetValidationProfile]bool{
		{HostOS: "linux", Runtime: "host", GuestOS: "none"}:    false,
		{HostOS: "windows", Runtime: "host", GuestOS: "none"}:  false,
		{HostOS: "linux", Runtime: "qemu", GuestOS: "windows"}: false,
	}
	identities := map[string]string{value.AggregateID: "aggregate"}
	last := ""
	for _, target := range value.Targets {
		wanted, known := wantedProfiles[target.Profile]
		if target.TargetID <= last || !known || wanted {
			return NodeConnectorMultiTargetValidationExpected{}, errors.New("multi-target validation targets or profiles are duplicate, unknown, or ambiguous")
		}
		wantedProfiles[target.Profile] = true
		last = target.TargetID
		for kind, identity := range map[string]string{
			"target": target.TargetID, "machine": target.MachineID, "operation": target.OperationID,
			"lease": target.LeaseID, "receipt": target.ReceiptID, "local-run": target.LocalRunID,
		} {
			if err := validateNodeExecutionTypedID(kind, identity); err != nil {
				return NodeConnectorMultiTargetValidationExpected{}, errors.New("multi-target validation expected identity is invalid")
			}
			if prior := identities[identity]; prior != "" {
				return NodeConnectorMultiTargetValidationExpected{}, fmt.Errorf("multi-target validation %s identity collides with %s identity", kind, prior)
			}
			identities[identity] = kind
		}
		for kind, identity := range map[string]string{
			"machine-evidence": target.MachineFingerprint, "capability": target.CapabilitySnapshotID, "request": target.RequestFingerprint,
			"lease-evidence": target.LeaseFingerprint, "events": target.EventsFingerprint, "receipt-evidence": target.ReceiptFingerprint,
		} {
			if !nodeExecutionFingerprint.MatchString(identity) {
				return NodeConnectorMultiTargetValidationExpected{}, errors.New("multi-target validation expected fingerprint is invalid")
			}
			if prior := identities[identity]; prior != "" {
				return NodeConnectorMultiTargetValidationExpected{}, fmt.Errorf("multi-target validation %s identity collides with %s identity", kind, prior)
			}
			identities[identity] = kind
		}
		if target.Attempt < 1 {
			return NodeConnectorMultiTargetValidationExpected{}, errors.New("multi-target validation expected attempt is invalid")
		}
	}
	for _, seen := range wantedProfiles {
		if !seen {
			return NodeConnectorMultiTargetValidationExpected{}, errors.New("multi-target validation requires each exact target profile")
		}
	}
	return value, nil
}

func validateNodeConnectorMultiTargetValidationAggregate(value NodeConnectorMultiTargetValidationAggregate) error {
	expected, err := normalizeNodeConnectorMultiTargetValidationExpected(NodeConnectorMultiTargetValidationExpected{AggregateID: value.AggregateID, Targets: value.Targets})
	if err != nil || !nodeExecutionEqual(value.Targets, expected.Targets) {
		return errors.New("multi-target validation aggregate declaration is invalid or unsorted")
	}
	if value.Schema != NodeConnectorMultiTargetValidationAggregateSchema || value.Provenance != nodeConnectorMultiTargetValidationProvenance ||
		value.RepairDispatched || value.Authority != (NodeConnectorMultiTargetValidationAuthority{}) ||
		len(value.ReceiptBindings) != nodeConnectorMultiTargetValidationTargetCount {
		return errors.New("multi-target validation aggregate contract or authority is invalid")
	}
	failed := []string{}
	for index, binding := range value.ReceiptBindings {
		target := value.Targets[index]
		expectedOutcome := "failed"
		if binding.TerminalResult == "succeeded" && binding.CleanupStatus == "not_required" {
			expectedOutcome = "passed"
		}
		if binding.TargetID != target.TargetID || binding.Profile != target.Profile || binding.MachineID != target.MachineID || binding.MachineFingerprint != target.MachineFingerprint ||
			binding.CapabilitySnapshotID != target.CapabilitySnapshotID || binding.OperationID != target.OperationID ||
			binding.RequestFingerprint != target.RequestFingerprint || binding.LeaseID != target.LeaseID || binding.LeaseFingerprint != target.LeaseFingerprint || binding.Attempt != target.Attempt ||
			binding.EventsFingerprint != target.EventsFingerprint || binding.ReceiptID != target.ReceiptID || binding.ReceiptFingerprint != target.ReceiptFingerprint || binding.LocalRunID != target.LocalRunID ||
			!nodeExecutionFingerprint.MatchString(binding.ReceiptFingerprint) || !nodeExecutionFingerprint.MatchString(binding.ArtifactManifestFingerprint) ||
			(binding.TerminalResult != "succeeded" && binding.TerminalResult != "failed" && binding.TerminalResult != "cancelled" && binding.TerminalResult != "degraded") ||
			(binding.CleanupStatus != "not_required" && binding.CleanupStatus != "succeeded" && binding.CleanupStatus != "failed") ||
			(binding.CleanupEvidenceDigest != "" && !nodeExecutionFingerprint.MatchString(binding.CleanupEvidenceDigest)) || binding.Outcome != expectedOutcome {
			return errors.New("multi-target validation receipt binding or outcome is invalid")
		}
		if binding.Outcome == "failed" {
			failed = append(failed, binding.TargetID)
		}
	}
	status := "passed"
	if len(failed) != 0 {
		status = "failed"
	}
	if value.Status != status || !nodeExecutionEqual(value.FailedTargetIDs, failed) {
		return errors.New("multi-target validation aggregate status is not derived from all targets")
	}
	expectedFingerprint := value.AggregateFingerprint
	value.AggregateFingerprint = ""
	actualFingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil || expectedFingerprint != actualFingerprint {
		return errors.New("multi-target validation aggregate fingerprint is invalid")
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorMultiTargetValidationMaxOutputBytes {
		return errors.New("multi-target validation aggregate exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorMultiTargetValidationArtifact(raw []byte, target *NodeConnectorMultiTargetValidationAggregate) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return errors.New("multi-target validation aggregate is malformed")
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("multi-target validation aggregate is not canonical")
	}
	return nil
}

func cloneNodeConnectorMultiTargetValidationAggregate(value NodeConnectorMultiTargetValidationAggregate) NodeConnectorMultiTargetValidationAggregate {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorMultiTargetValidationAggregate
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorMultiTargetValidationInput(value NodeConnectorMultiTargetValidationInput) ([]byte, error) {
	if value.Schema == "" {
		value.Schema = NodeConnectorMultiTargetValidationInputSchema
	}
	if len(value.Targets) != nodeConnectorMultiTargetValidationTargetCount {
		return nil, errors.New("multi-target validation input requires exactly three targets")
	}
	return json.Marshal(value)
}

func nodeConnectorMultiTargetValidationContainsForbiddenSurface(value any) bool {
	raw, _ := json.Marshal(value)
	lower := strings.ToLower(string(raw))
	for _, fragment := range []string{"availability", "connection", "provider", "ingress", "quota", "audit", "retention", "managed_service"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
