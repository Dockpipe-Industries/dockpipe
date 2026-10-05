package orchestrationhelper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	NodeExecutionMachineIdentitySchema    = "dorkpipe.node-execution.machine-identity/v1"
	NodeExecutionCapabilitySnapshotSchema = "dorkpipe.node-execution.capability-snapshot/v1"
	NodeExecutionRequestSchema            = "dorkpipe.node-execution.execution-request/v1"
	NodeExecutionLeaseSchema              = "dorkpipe.node-execution.task-lease/v1"
	NodeExecutionEventSchema              = "dorkpipe.node-execution.event-envelope/v1"
	NodeExecutionCancellationSchema       = "dorkpipe.node-execution.cancellation/v1"
	NodeExecutionCancellationAckSchema    = "dorkpipe.node-execution.cancellation-ack/v1"
	NodeExecutionArtifactManifestSchema   = "dorkpipe.node-execution.artifact-manifest/v1"
	NodeExecutionReceiptSchema            = "dorkpipe.node-execution.execution-receipt/v1"
	nodeExecutionBrokerStateSchema        = "dorkpipe.node-execution.fake-broker-state/v1"

	nodeExecutionMaxInputs        = 32
	nodeExecutionMaxArtifacts     = 128
	nodeExecutionMaxInputBytes    = 4096
	nodeExecutionMaxArtifactBytes = int64(1 << 40)
)

var (
	nodeExecutionIDPattern       = regexp.MustCompile(`^[a-z][a-z0-9._:-]{7,127}$`)
	nodeExecutionNamePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	nodeExecutionRevisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	nodeExecutionFingerprint     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	nodeExecutionStateName       = regexp.MustCompile(`^state-([0-9]{12})\.json$`)
	nodeExecutionWriteAtomic     = writeJSONFileAtomic
)

type NodeExecutionMachineIdentity struct {
	Schema     string `json:"schema"`
	MachineID  string `json:"machine_id"`
	EnrolledAt string `json:"enrolled_at"`
}

type NodeExecutionObservedCapabilities struct {
	HostOS       string   `json:"host_os"`
	Runtime      string   `json:"runtime"`
	GuestOS      string   `json:"guest_os,omitempty"`
	GuestImageID string   `json:"guest_image_id,omitempty"`
	Toolchains   []string `json:"toolchains"`
}

type NodeExecutionApprovedCapabilities struct {
	PolicyClass          string   `json:"policy_class"`
	AllowedWorkflowKinds []string `json:"allowed_workflow_kinds"`
}

type NodeExecutionCapabilitySnapshot struct {
	Schema     string                            `json:"schema"`
	SnapshotID string                            `json:"snapshot_id"`
	MachineID  string                            `json:"machine_id"`
	Observed   NodeExecutionObservedCapabilities `json:"observed"`
	Approved   NodeExecutionApprovedCapabilities `json:"approved"`
	ObservedAt string                            `json:"observed_at"`
}

type NodeExecutionWorkflowReference struct {
	Kind    string `json:"kind"`
	Package string `json:"package"`
	Name    string `json:"name"`
}

type NodeExecutionInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type NodeExecutionArtifactReference struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Digest    string `json:"digest"`
	Bytes     int64  `json:"bytes"`
}

type NodeExecutionRequest struct {
	Schema               string                           `json:"schema"`
	OperationID          string                           `json:"operation_id"`
	GraphRunID           string                           `json:"graph_run_id"`
	RunID                string                           `json:"run_id"`
	TaskID               string                           `json:"task_id"`
	SourceRevision       string                           `json:"source_revision"`
	Workflow             NodeExecutionWorkflowReference   `json:"workflow"`
	CapabilitySnapshotID string                           `json:"capability_snapshot_id"`
	Inputs               []NodeExecutionInput             `json:"inputs"`
	Artifacts            []NodeExecutionArtifactReference `json:"artifacts"`
	RequestedAt          string                           `json:"requested_at"`
	RequestFingerprint   string                           `json:"request_fingerprint"`
}

type NodeExecutionTaskLease struct {
	Schema               string `json:"schema"`
	LeaseID              string `json:"lease_id"`
	MachineID            string `json:"machine_id"`
	CapabilitySnapshotID string `json:"capability_snapshot_id"`
	OperationID          string `json:"operation_id"`
	Attempt              int    `json:"attempt"`
	IssuedAt             string `json:"issued_at"`
	ExpiresAt            string `json:"expires_at"`
	CancellationID       string `json:"cancellation_id"`
}

type NodeExecutionEventEnvelope struct {
	Schema               string                           `json:"schema"`
	OperationID          string                           `json:"operation_id"`
	GraphRunID           string                           `json:"graph_run_id"`
	RunID                string                           `json:"run_id"`
	TaskID               string                           `json:"task_id"`
	MachineID            string                           `json:"machine_id"`
	CapabilitySnapshotID string                           `json:"capability_snapshot_id"`
	LeaseID              string                           `json:"lease_id"`
	Attempt              int                              `json:"attempt"`
	Sequence             int64                            `json:"sequence"`
	Cursor               string                           `json:"cursor"`
	RecordedAt           string                           `json:"recorded_at"`
	OutputReferences     []NodeExecutionArtifactReference `json:"output_references"`
	Event                json.RawMessage                  `json:"event"`
	EnvelopeFingerprint  string                           `json:"envelope_fingerprint"`
}

type NodeExecutionCancellation struct {
	Schema                  string `json:"schema"`
	CancellationID          string `json:"cancellation_id"`
	OperationID             string `json:"operation_id"`
	MachineID               string `json:"machine_id"`
	CapabilitySnapshotID    string `json:"capability_snapshot_id"`
	LeaseID                 string `json:"lease_id"`
	Attempt                 int    `json:"attempt"`
	RequestedAt             string `json:"requested_at"`
	CancellationFingerprint string `json:"cancellation_fingerprint"`
}

type NodeExecutionCancellationAck struct {
	Schema                  string `json:"schema"`
	CancellationID          string `json:"cancellation_id"`
	OperationID             string `json:"operation_id"`
	LeaseID                 string `json:"lease_id"`
	AcknowledgedAt          string `json:"acknowledged_at"`
	CancellationFingerprint string `json:"cancellation_fingerprint"`
	AckFingerprint          string `json:"ack_fingerprint"`
}

type NodeExecutionArtifactManifest struct {
	Schema              string                           `json:"schema"`
	Entries             []NodeExecutionArtifactReference `json:"entries"`
	ManifestFingerprint string                           `json:"manifest_fingerprint"`
}

type NodeExecutionCleanupOutcome struct {
	Status         string `json:"status"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

type NodeExecutionReceipt struct {
	Schema                   string                        `json:"schema"`
	ReceiptID                string                        `json:"receipt_id"`
	OperationID              string                        `json:"operation_id"`
	MachineID                string                        `json:"machine_id"`
	CapabilitySnapshotID     string                        `json:"capability_snapshot_id"`
	LeaseID                  string                        `json:"lease_id"`
	Attempt                  int                           `json:"attempt"`
	RequestFingerprint       string                        `json:"request_fingerprint"`
	LocalRunID               string                        `json:"local_run_id,omitempty"`
	FinalCursor              string                        `json:"final_cursor"`
	Result                   string                        `json:"result"`
	Artifacts                NodeExecutionArtifactManifest `json:"artifacts"`
	CancellationID           string                        `json:"cancellation_id,omitempty"`
	CancellationAcknowledged bool                          `json:"cancellation_acknowledged"`
	Cleanup                  NodeExecutionCleanupOutcome   `json:"cleanup"`
	CompletedAt              string                        `json:"completed_at"`
	ReceiptFingerprint       string                        `json:"receipt_fingerprint"`
}

type nodeExecutionOperationState struct {
	Request         NodeExecutionRequest          `json:"request"`
	Lease           NodeExecutionTaskLease        `json:"lease"`
	Events          []NodeExecutionEventEnvelope  `json:"events"`
	Cancellation    *NodeExecutionCancellation    `json:"cancellation,omitempty"`
	CancellationAck *NodeExecutionCancellationAck `json:"cancellation_ack,omitempty"`
	Receipt         *NodeExecutionReceipt         `json:"receipt,omitempty"`
	ExecutionCount  int                           `json:"execution_count"`
}

type nodeExecutionBrokerState struct {
	Schema                   string                                 `json:"schema"`
	Generation               int64                                  `json:"generation"`
	PreviousStateFingerprint string                                 `json:"previous_state_fingerprint,omitempty"`
	Machine                  NodeExecutionMachineIdentity           `json:"machine"`
	Capabilities             []NodeExecutionCapabilitySnapshot      `json:"capabilities"`
	Operations               map[string]nodeExecutionOperationState `json:"operations"`
	StateFingerprint         string                                 `json:"state_fingerprint"`
}

type NodeExecutionResume struct {
	Lease   NodeExecutionTaskLease
	Cursor  string
	Receipt *NodeExecutionReceipt
}

type NodeExecutionFakeExecutor func(NodeExecutionRequest, NodeExecutionTaskLease)

type NodeExecutionFakeBroker struct {
	root        string
	state       nodeExecutionBrokerState
	connections map[string]string
	executor    NodeExecutionFakeExecutor
}

func NewNodeExecutionCapabilitySnapshot(machineID string, observed NodeExecutionObservedCapabilities, approved NodeExecutionApprovedCapabilities, observedAt time.Time) (NodeExecutionCapabilitySnapshot, error) {
	snapshot := NodeExecutionCapabilitySnapshot{
		Schema: NodeExecutionCapabilitySnapshotSchema, MachineID: machineID,
		Observed: observed, Approved: approved, ObservedAt: nodeExecutionTime(observedAt),
	}
	fingerprint, err := nodeExecutionCapabilityFingerprint(snapshot)
	if err != nil {
		return NodeExecutionCapabilitySnapshot{}, err
	}
	snapshot.SnapshotID = fingerprint
	if err := validateNodeExecutionCapability(snapshot); err != nil {
		return NodeExecutionCapabilitySnapshot{}, err
	}
	return snapshot, nil
}

func FinalizeNodeExecutionRequest(request NodeExecutionRequest) (NodeExecutionRequest, error) {
	request.Schema = NodeExecutionRequestSchema
	request.RequestFingerprint = ""
	fingerprint, err := nodeExecutionRequestFingerprint(request)
	if err != nil {
		return NodeExecutionRequest{}, err
	}
	request.RequestFingerprint = fingerprint
	if err := validateNodeExecutionRequest(request); err != nil {
		return NodeExecutionRequest{}, err
	}
	return request, nil
}

func FinalizeNodeExecutionEvent(event NodeExecutionEventEnvelope) (NodeExecutionEventEnvelope, error) {
	event.Schema = NodeExecutionEventSchema
	event.Cursor = nodeExecutionCursor(event.Sequence)
	event.EnvelopeFingerprint = ""
	fingerprint, err := nodeExecutionEventFingerprint(event)
	if err != nil {
		return NodeExecutionEventEnvelope{}, err
	}
	event.EnvelopeFingerprint = fingerprint
	if err := validateNodeExecutionEvent(event); err != nil {
		return NodeExecutionEventEnvelope{}, err
	}
	return event, nil
}

func FinalizeNodeExecutionCancellation(cancellation NodeExecutionCancellation) (NodeExecutionCancellation, error) {
	cancellation.Schema = NodeExecutionCancellationSchema
	cancellation.CancellationFingerprint = ""
	fingerprint, err := nodeExecutionCancellationFingerprint(cancellation)
	if err != nil {
		return NodeExecutionCancellation{}, err
	}
	cancellation.CancellationFingerprint = fingerprint
	if err := validateNodeExecutionCancellation(cancellation); err != nil {
		return NodeExecutionCancellation{}, err
	}
	return cancellation, nil
}

func NewNodeExecutionArtifactManifest(entries []NodeExecutionArtifactReference) (NodeExecutionArtifactManifest, error) {
	manifest := NodeExecutionArtifactManifest{Schema: NodeExecutionArtifactManifestSchema, Entries: append([]NodeExecutionArtifactReference{}, entries...)}
	fingerprint, err := nodeExecutionManifestFingerprint(manifest)
	if err != nil {
		return NodeExecutionArtifactManifest{}, err
	}
	manifest.ManifestFingerprint = fingerprint
	if err := validateNodeExecutionManifest(manifest); err != nil {
		return NodeExecutionArtifactManifest{}, err
	}
	return manifest, nil
}

func FinalizeNodeExecutionReceipt(receipt NodeExecutionReceipt) (NodeExecutionReceipt, error) {
	receipt.Schema = NodeExecutionReceiptSchema
	receipt.ReceiptFingerprint = ""
	fingerprint, err := nodeExecutionReceiptFingerprint(receipt)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	receipt.ReceiptFingerprint = fingerprint
	if err := validateNodeExecutionReceiptShape(receipt); err != nil {
		return NodeExecutionReceipt{}, err
	}
	return receipt, nil
}

func NewNodeExecutionFakeBroker(root string, machine NodeExecutionMachineIdentity, capabilities []NodeExecutionCapabilitySnapshot, executor NodeExecutionFakeExecutor) (*NodeExecutionFakeBroker, error) {
	if err := validateNodeExecutionMachine(machine); err != nil {
		return nil, err
	}
	if len(capabilities) == 0 {
		return nil, errors.New("node execution broker requires at least one capability snapshot")
	}
	capabilities = append([]NodeExecutionCapabilitySnapshot{}, capabilities...)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].SnapshotID < capabilities[j].SnapshotID })
	for _, capability := range capabilities {
		if err := validateNodeExecutionCapability(capability); err != nil {
			return nil, err
		}
		if capability.MachineID != machine.MachineID {
			return nil, errors.New("capability snapshot is bound to a different machine")
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	states, err := loadNodeExecutionStates(root)
	if err != nil {
		return nil, err
	}
	broker := &NodeExecutionFakeBroker{root: root, connections: map[string]string{}, executor: executor}
	if len(states) == 0 {
		state := nodeExecutionBrokerState{
			Schema: nodeExecutionBrokerStateSchema, Generation: 1, Machine: machine,
			Capabilities: capabilities, Operations: map[string]nodeExecutionOperationState{},
		}
		if err := finalizeNodeExecutionState(&state); err != nil {
			return nil, err
		}
		if err := nodeExecutionWriteAtomic(filepath.Join(root, nodeExecutionStateFileName(state.Generation)), state); err != nil {
			return nil, err
		}
		broker.state = state
		return broker, nil
	}
	state := states[len(states)-1]
	if !nodeExecutionEqual(state.Machine, machine) || !nodeExecutionEqual(state.Capabilities, capabilities) {
		return nil, errors.New("preconfigured machine or capability snapshots conflict with durable broker state")
	}
	broker.state = state
	return broker, nil
}

func (broker *NodeExecutionFakeBroker) Connect(machineID, connectionID string) error {
	if err := validateNodeExecutionTypedID("connection", connectionID); err != nil {
		return err
	}
	if machineID != broker.state.Machine.MachineID {
		return errors.New("connection machine does not match enrolled machine")
	}
	if machineID == connectionID {
		return errors.New("connection identity cannot substitute for machine identity")
	}
	if existing, ok := broker.connections[connectionID]; ok && existing != machineID {
		return errors.New("connection identity is already bound to another machine")
	}
	broker.connections[connectionID] = machineID
	return nil
}

func (broker *NodeExecutionFakeBroker) Disconnect(connectionID string) {
	delete(broker.connections, connectionID)
}

func (broker *NodeExecutionFakeBroker) RegisterCapabilitySnapshot(snapshot NodeExecutionCapabilitySnapshot) error {
	if err := validateNodeExecutionCapability(snapshot); err != nil {
		return err
	}
	if snapshot.MachineID != broker.state.Machine.MachineID {
		return errors.New("capability snapshot is bound to a different machine")
	}
	for _, existing := range broker.state.Capabilities {
		if existing.SnapshotID != snapshot.SnapshotID {
			continue
		}
		if nodeExecutionEqual(existing, snapshot) {
			return nil
		}
		return errors.New("capability snapshot identity conflicts with durable evidence")
	}
	next := cloneNodeExecutionState(broker.state)
	next.Capabilities = append(next.Capabilities, snapshot)
	sort.Slice(next.Capabilities, func(i, j int) bool { return next.Capabilities[i].SnapshotID < next.Capabilities[j].SnapshotID })
	return broker.persist(next)
}

func (broker *NodeExecutionFakeBroker) Dispatch(connectionID string, raw []byte, issuedAt time.Time, leaseDuration time.Duration) (NodeExecutionTaskLease, error) {
	machineID, err := broker.connectedMachine(connectionID)
	if err != nil {
		return NodeExecutionTaskLease{}, err
	}
	var request NodeExecutionRequest
	if err := decodeNodeExecutionCanonical(raw, &request); err != nil {
		return NodeExecutionTaskLease{}, fmt.Errorf("execution request is invalid: %w", err)
	}
	if err := validateNodeExecutionRequest(request); err != nil {
		return NodeExecutionTaskLease{}, err
	}
	capability, ok := broker.capability(request.CapabilitySnapshotID)
	if !ok || capability.MachineID != machineID {
		return NodeExecutionTaskLease{}, errors.New("execution request capability snapshot is not registered for the connected machine")
	}
	if existing, ok := broker.state.Operations[request.OperationID]; ok {
		if existing.Request.RequestFingerprint != request.RequestFingerprint || !nodeExecutionEqual(existing.Request, request) {
			return NodeExecutionTaskLease{}, errors.New("operation identity conflicts with a changed execution request")
		}
		return existing.Lease, nil
	}
	if leaseDuration <= 0 {
		return NodeExecutionTaskLease{}, errors.New("task lease duration must be positive")
	}
	issuedAt = issuedAt.UTC()
	requestedAt, _ := parseNodeExecutionTime(request.RequestedAt)
	if requestedAt.After(issuedAt) {
		return NodeExecutionTaskLease{}, errors.New("execution request cannot be issued before it was requested")
	}
	lease := newNodeExecutionLease(request, machineID, issuedAt, issuedAt.Add(leaseDuration))
	if err := validateNodeExecutionLease(lease); err != nil {
		return NodeExecutionTaskLease{}, err
	}
	next := cloneNodeExecutionState(broker.state)
	next.Operations[request.OperationID] = nodeExecutionOperationState{
		Request: request, Lease: lease, Events: []NodeExecutionEventEnvelope{}, ExecutionCount: 1,
	}
	if err := broker.persist(next); err != nil {
		return NodeExecutionTaskLease{}, err
	}
	if broker.executor != nil {
		broker.executor(request, lease)
	}
	return lease, nil
}

func (broker *NodeExecutionFakeBroker) Resume(connectionID, operationID string) (NodeExecutionResume, error) {
	if _, err := broker.connectedMachine(connectionID); err != nil {
		return NodeExecutionResume{}, err
	}
	operation, ok := broker.state.Operations[operationID]
	if !ok {
		return NodeExecutionResume{}, errors.New("operation is not accepted by the broker")
	}
	resume := NodeExecutionResume{Lease: operation.Lease, Cursor: nodeExecutionCursor(int64(len(operation.Events)))}
	if operation.Receipt != nil {
		receipt := *operation.Receipt
		resume.Receipt = &receipt
	}
	return resume, nil
}

func (broker *NodeExecutionFakeBroker) AcceptEvent(connectionID string, raw []byte, at time.Time) error {
	machineID, err := broker.connectedMachine(connectionID)
	if err != nil {
		return err
	}
	var event NodeExecutionEventEnvelope
	if err := decodeNodeExecutionCanonical(raw, &event); err != nil {
		return fmt.Errorf("event envelope is invalid: %w", err)
	}
	if err := validateNodeExecutionEvent(event); err != nil {
		return err
	}
	operation, ok := broker.state.Operations[event.OperationID]
	if !ok {
		return errors.New("event operation is not accepted by the broker")
	}
	if operation.Receipt != nil {
		return errors.New("post-terminal events are rejected")
	}
	if err := validateNodeExecutionActiveLease(operation, machineID, event.MachineID, event.CapabilitySnapshotID, event.LeaseID, event.OperationID, event.Attempt, at); err != nil {
		return err
	}
	if event.GraphRunID != operation.Request.GraphRunID || event.RunID != operation.Request.RunID || event.TaskID != operation.Request.TaskID {
		return errors.New("event correlation does not match the execution request")
	}
	expected := int64(len(operation.Events) + 1)
	if event.Sequence < expected {
		existing := operation.Events[event.Sequence-1]
		if existing.EnvelopeFingerprint == event.EnvelopeFingerprint && nodeExecutionEqual(existing, event) {
			return nil
		}
		return errors.New("changed duplicate or regressed event is rejected")
	}
	if event.Sequence != expected {
		return errors.New("event sequence gap is rejected")
	}
	next := cloneNodeExecutionState(broker.state)
	op := next.Operations[event.OperationID]
	op.Events = append(op.Events, event)
	next.Operations[event.OperationID] = op
	return broker.persist(next)
}

func (broker *NodeExecutionFakeBroker) RequestCancellation(connectionID string, raw []byte, at time.Time) (NodeExecutionCancellationAck, error) {
	machineID, err := broker.connectedMachine(connectionID)
	if err != nil {
		return NodeExecutionCancellationAck{}, err
	}
	var cancellation NodeExecutionCancellation
	if err := decodeNodeExecutionCanonical(raw, &cancellation); err != nil {
		return NodeExecutionCancellationAck{}, fmt.Errorf("cancellation is invalid: %w", err)
	}
	if err := validateNodeExecutionCancellation(cancellation); err != nil {
		return NodeExecutionCancellationAck{}, err
	}
	operation, ok := broker.state.Operations[cancellation.OperationID]
	if !ok {
		return NodeExecutionCancellationAck{}, errors.New("cancellation operation is not accepted by the broker")
	}
	if operation.Receipt != nil {
		return NodeExecutionCancellationAck{}, errors.New("terminal operation cannot be cancelled")
	}
	if err := validateNodeExecutionActiveLease(operation, machineID, cancellation.MachineID, cancellation.CapabilitySnapshotID, cancellation.LeaseID, cancellation.OperationID, cancellation.Attempt, at); err != nil {
		return NodeExecutionCancellationAck{}, err
	}
	if cancellation.CancellationID != operation.Lease.CancellationID {
		return NodeExecutionCancellationAck{}, errors.New("cancellation identity does not match the active lease binding")
	}
	if operation.Cancellation != nil {
		if operation.Cancellation.CancellationFingerprint != cancellation.CancellationFingerprint || !nodeExecutionEqual(*operation.Cancellation, cancellation) {
			return NodeExecutionCancellationAck{}, errors.New("changed cancellation replay is rejected")
		}
		return *operation.CancellationAck, nil
	}
	ack := NodeExecutionCancellationAck{
		Schema: NodeExecutionCancellationAckSchema, CancellationID: cancellation.CancellationID,
		OperationID: cancellation.OperationID, LeaseID: cancellation.LeaseID,
		AcknowledgedAt: nodeExecutionTime(at), CancellationFingerprint: cancellation.CancellationFingerprint,
	}
	ack.AckFingerprint, err = nodeExecutionCancellationAckFingerprint(ack)
	if err != nil {
		return NodeExecutionCancellationAck{}, err
	}
	next := cloneNodeExecutionState(broker.state)
	op := next.Operations[cancellation.OperationID]
	op.Cancellation = &cancellation
	op.CancellationAck = &ack
	next.Operations[cancellation.OperationID] = op
	if err := broker.persist(next); err != nil {
		return NodeExecutionCancellationAck{}, err
	}
	return ack, nil
}

func (broker *NodeExecutionFakeBroker) AcceptReceipt(connectionID string, raw []byte, at time.Time) (NodeExecutionReceipt, error) {
	machineID, err := broker.connectedMachine(connectionID)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	var receipt NodeExecutionReceipt
	if err := decodeNodeExecutionCanonical(raw, &receipt); err != nil {
		return NodeExecutionReceipt{}, fmt.Errorf("execution receipt is invalid: %w", err)
	}
	if err := validateNodeExecutionReceiptShape(receipt); err != nil {
		return NodeExecutionReceipt{}, err
	}
	operation, ok := broker.state.Operations[receipt.OperationID]
	if !ok {
		return NodeExecutionReceipt{}, errors.New("receipt operation is not accepted by the broker")
	}
	if operation.Receipt != nil {
		if operation.Receipt.ReceiptFingerprint == receipt.ReceiptFingerprint && nodeExecutionEqual(*operation.Receipt, receipt) {
			return *operation.Receipt, nil
		}
		return NodeExecutionReceipt{}, errors.New("changed or conflicting terminal receipt replay is rejected")
	}
	if err := validateNodeExecutionActiveLease(operation, machineID, receipt.MachineID, receipt.CapabilitySnapshotID, receipt.LeaseID, receipt.OperationID, receipt.Attempt, at); err != nil {
		return NodeExecutionReceipt{}, err
	}
	if err := validateNodeExecutionReceiptBinding(receipt, operation); err != nil {
		return NodeExecutionReceipt{}, err
	}
	next := cloneNodeExecutionState(broker.state)
	op := next.Operations[receipt.OperationID]
	op.Receipt = &receipt
	next.Operations[receipt.OperationID] = op
	if err := broker.persist(next); err != nil {
		return NodeExecutionReceipt{}, err
	}
	return receipt, nil
}

func (broker *NodeExecutionFakeBroker) connectedMachine(connectionID string) (string, error) {
	machineID, ok := broker.connections[connectionID]
	if !ok {
		return "", errors.New("connector is not connected")
	}
	return machineID, nil
}

func (broker *NodeExecutionFakeBroker) capability(snapshotID string) (NodeExecutionCapabilitySnapshot, bool) {
	for _, capability := range broker.state.Capabilities {
		if capability.SnapshotID == snapshotID {
			return capability, true
		}
	}
	return NodeExecutionCapabilitySnapshot{}, false
}

func (broker *NodeExecutionFakeBroker) persist(next nodeExecutionBrokerState) error {
	next.Generation = broker.state.Generation + 1
	next.PreviousStateFingerprint = broker.state.StateFingerprint
	next.StateFingerprint = ""
	if err := finalizeNodeExecutionState(&next); err != nil {
		return err
	}
	path := filepath.Join(broker.root, nodeExecutionStateFileName(next.Generation))
	if _, err := os.Lstat(path); err == nil {
		return errors.New("next node execution state artifact already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := nodeExecutionWriteAtomic(path, next); err != nil {
		return err
	}
	broker.state = next
	return nil
}

func loadNodeExecutionStates(root string) ([]nodeExecutionBrokerState, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "state-") {
			if !nodeExecutionStateName.MatchString(entry.Name()) {
				return nil, fmt.Errorf("malformed node execution state artifact name %q", entry.Name())
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	states := make([]nodeExecutionBrokerState, 0, len(names))
	previous := ""
	for index, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		var state nodeExecutionBrokerState
		if err := decodeNodeExecutionStrict(raw, &state); err != nil {
			return nil, fmt.Errorf("node execution state %s is malformed: %w", name, err)
		}
		if state.Generation != int64(index+1) || name != nodeExecutionStateFileName(state.Generation) {
			return nil, fmt.Errorf("node execution state generation is not contiguous at %s", name)
		}
		if state.PreviousStateFingerprint != previous {
			return nil, fmt.Errorf("node execution state chain is broken at %s", name)
		}
		if err := validateNodeExecutionState(state); err != nil {
			return nil, fmt.Errorf("node execution state %s failed revalidation: %w", name, err)
		}
		previous = state.StateFingerprint
		states = append(states, state)
	}
	return states, nil
}

func finalizeNodeExecutionState(state *nodeExecutionBrokerState) error {
	state.StateFingerprint = ""
	fingerprint, err := nodeExecutionStateFingerprint(*state)
	if err != nil {
		return err
	}
	state.StateFingerprint = fingerprint
	return validateNodeExecutionState(*state)
}

func validateNodeExecutionState(state nodeExecutionBrokerState) error {
	if state.Schema != nodeExecutionBrokerStateSchema || state.Generation < 1 {
		return errors.New("broker state schema or generation is invalid")
	}
	if state.Generation == 1 && state.PreviousStateFingerprint != "" {
		return errors.New("initial broker state cannot have a previous fingerprint")
	}
	if state.Generation > 1 && !nodeExecutionFingerprint.MatchString(state.PreviousStateFingerprint) {
		return errors.New("broker state previous fingerprint is invalid")
	}
	if err := validateNodeExecutionMachine(state.Machine); err != nil {
		return err
	}
	if len(state.Capabilities) == 0 || !sort.SliceIsSorted(state.Capabilities, func(i, j int) bool { return state.Capabilities[i].SnapshotID < state.Capabilities[j].SnapshotID }) {
		return errors.New("broker capability snapshots must be a non-empty sorted immutable list")
	}
	seenCapabilities := map[string]bool{}
	for _, capability := range state.Capabilities {
		if err := validateNodeExecutionCapability(capability); err != nil {
			return err
		}
		if capability.MachineID != state.Machine.MachineID || seenCapabilities[capability.SnapshotID] {
			return errors.New("broker capability snapshot identity is duplicated or differently bound")
		}
		seenCapabilities[capability.SnapshotID] = true
	}
	if state.Operations == nil {
		return errors.New("broker operations map is missing")
	}
	for operationID, operation := range state.Operations {
		if operation.Request.OperationID != operationID || operation.ExecutionCount != 1 {
			return errors.New("broker operation identity or execution count is invalid")
		}
		if err := validateNodeExecutionRequest(operation.Request); err != nil {
			return err
		}
		if !seenCapabilities[operation.Request.CapabilitySnapshotID] {
			return errors.New("broker operation references an unknown capability snapshot")
		}
		if err := validateNodeExecutionLease(operation.Lease); err != nil {
			return err
		}
		if operation.Lease.MachineID != state.Machine.MachineID || operation.Lease.OperationID != operationID || operation.Lease.CapabilitySnapshotID != operation.Request.CapabilitySnapshotID {
			return errors.New("broker lease is differently bound from its operation")
		}
		leaseExpiry, _ := parseNodeExecutionTime(operation.Lease.ExpiresAt)
		for index, event := range operation.Events {
			if err := validateNodeExecutionEvent(event); err != nil {
				return err
			}
			recordedAt, _ := parseNodeExecutionTime(event.RecordedAt)
			if event.Sequence != int64(index+1) || event.OperationID != operationID || event.LeaseID != operation.Lease.LeaseID || event.MachineID != operation.Lease.MachineID || event.CapabilitySnapshotID != operation.Lease.CapabilitySnapshotID || event.Attempt != operation.Lease.Attempt || event.GraphRunID != operation.Request.GraphRunID || event.RunID != operation.Request.RunID || event.TaskID != operation.Request.TaskID || !recordedAt.Before(leaseExpiry) {
				return errors.New("durable event ordering or identity binding is invalid")
			}
		}
		if operation.Cancellation != nil {
			if operation.CancellationAck == nil {
				return errors.New("durable cancellation is missing its acknowledgement")
			}
			if err := validateNodeExecutionCancellation(*operation.Cancellation); err != nil {
				return err
			}
			if err := validateNodeExecutionCancellationAck(*operation.CancellationAck); err != nil {
				return err
			}
			requestedAt, _ := parseNodeExecutionTime(operation.Cancellation.RequestedAt)
			if operation.Cancellation.CancellationID != operation.Lease.CancellationID || operation.Cancellation.OperationID != operationID || operation.Cancellation.LeaseID != operation.Lease.LeaseID || operation.CancellationAck.CancellationFingerprint != operation.Cancellation.CancellationFingerprint || !requestedAt.Before(leaseExpiry) {
				return errors.New("durable cancellation binding is invalid")
			}
		} else if operation.CancellationAck != nil {
			return errors.New("cancellation acknowledgement has no request")
		}
		if operation.Receipt != nil {
			if err := validateNodeExecutionReceiptShape(*operation.Receipt); err != nil {
				return err
			}
			if err := validateNodeExecutionReceiptBinding(*operation.Receipt, operation); err != nil {
				return err
			}
		}
	}
	expected, err := nodeExecutionStateFingerprint(state)
	if err != nil {
		return err
	}
	if state.StateFingerprint != expected {
		return errors.New("broker state fingerprint does not match durable content")
	}
	return nil
}

func validateNodeExecutionMachine(machine NodeExecutionMachineIdentity) error {
	if machine.Schema != NodeExecutionMachineIdentitySchema {
		return errors.New("machine identity schema is invalid")
	}
	if err := validateNodeExecutionTypedID("machine", machine.MachineID); err != nil {
		return err
	}
	_, err := parseNodeExecutionTime(machine.EnrolledAt)
	return err
}

func validateNodeExecutionCapability(snapshot NodeExecutionCapabilitySnapshot) error {
	if snapshot.Schema != NodeExecutionCapabilitySnapshotSchema {
		return errors.New("capability snapshot schema is invalid")
	}
	if err := validateNodeExecutionTypedID("machine", snapshot.MachineID); err != nil {
		return err
	}
	if _, err := parseNodeExecutionTime(snapshot.ObservedAt); err != nil {
		return err
	}
	if err := validateNodeExecutionName("host_os", snapshot.Observed.HostOS); err != nil {
		return err
	}
	if err := validateNodeExecutionName("runtime", snapshot.Observed.Runtime); err != nil {
		return err
	}
	for _, optional := range []struct{ field, value string }{{"guest_os", snapshot.Observed.GuestOS}, {"guest_image_id", snapshot.Observed.GuestImageID}} {
		if optional.value != "" {
			if err := validateNodeExecutionName(optional.field, optional.value); err != nil {
				return err
			}
		}
	}
	if err := validateNodeExecutionSortedNames("toolchains", snapshot.Observed.Toolchains, false); err != nil {
		return err
	}
	if err := validateNodeExecutionName("policy_class", snapshot.Approved.PolicyClass); err != nil {
		return err
	}
	if err := validateNodeExecutionSortedNames("allowed_workflow_kinds", snapshot.Approved.AllowedWorkflowKinds, false); err != nil {
		return err
	}
	expected, err := nodeExecutionCapabilityFingerprint(snapshot)
	if err != nil {
		return err
	}
	if snapshot.SnapshotID != expected {
		return errors.New("capability snapshot fingerprint does not match immutable facts")
	}
	return nil
}

func validateNodeExecutionRequest(request NodeExecutionRequest) error {
	if request.Schema != NodeExecutionRequestSchema {
		return errors.New("execution request schema is invalid")
	}
	for kind, value := range map[string]string{"operation": request.OperationID, "graph": request.GraphRunID, "run": request.RunID, "task": request.TaskID} {
		if err := validateNodeExecutionTypedID(kind, value); err != nil {
			return err
		}
	}
	if !nodeExecutionRevisionPattern.MatchString(request.SourceRevision) {
		return errors.New("execution request source revision must be an exact 40-character commit")
	}
	if request.Workflow.Kind != "dockpipe.workflow" {
		return errors.New("execution request workflow kind must be dockpipe.workflow")
	}
	if err := validateNodeExecutionName("workflow package", request.Workflow.Package); err != nil {
		return err
	}
	if err := validateNodeExecutionName("workflow name", request.Workflow.Name); err != nil {
		return err
	}
	if !nodeExecutionFingerprint.MatchString(request.CapabilitySnapshotID) {
		return errors.New("execution request capability snapshot identity is invalid")
	}
	if len(request.Inputs) > nodeExecutionMaxInputs || len(request.Artifacts) > nodeExecutionMaxArtifacts {
		return errors.New("execution request exceeds bounded input or artifact limits")
	}
	last := ""
	for _, input := range request.Inputs {
		if err := validateNodeExecutionName("input name", input.Name); err != nil {
			return err
		}
		if input.Name <= last {
			return errors.New("execution request inputs must be unique and sorted")
		}
		last = input.Name
		if len(input.Value) > nodeExecutionMaxInputBytes || containsNodeExecutionSecret(input.Name) || containsNodeExecutionSecret(input.Value) || strings.Contains(input.Value, "://") {
			return errors.New("execution request input is oversized or credential-like")
		}
	}
	if err := validateNodeExecutionArtifactReferences(request.Artifacts); err != nil {
		return err
	}
	if _, err := parseNodeExecutionTime(request.RequestedAt); err != nil {
		return err
	}
	expected, err := nodeExecutionRequestFingerprint(request)
	if err != nil {
		return err
	}
	if request.RequestFingerprint != expected {
		return errors.New("execution request fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeExecutionLease(lease NodeExecutionTaskLease) error {
	if lease.Schema != NodeExecutionLeaseSchema {
		return errors.New("task lease schema is invalid")
	}
	for kind, value := range map[string]string{"lease": lease.LeaseID, "machine": lease.MachineID, "operation": lease.OperationID, "cancellation": lease.CancellationID} {
		if err := validateNodeExecutionTypedID(kind, value); err != nil {
			return err
		}
	}
	if !nodeExecutionFingerprint.MatchString(lease.CapabilitySnapshotID) || lease.Attempt < 1 {
		return errors.New("task lease capability identity or attempt is invalid")
	}
	issued, err := parseNodeExecutionTime(lease.IssuedAt)
	if err != nil {
		return err
	}
	expires, err := parseNodeExecutionTime(lease.ExpiresAt)
	if err != nil {
		return err
	}
	if !expires.After(issued) {
		return errors.New("task lease expiry must be after issue time")
	}
	return nil
}

func validateNodeExecutionEvent(event NodeExecutionEventEnvelope) error {
	if event.Schema != NodeExecutionEventSchema || event.Sequence < 1 || event.Cursor != nodeExecutionCursor(event.Sequence) {
		return errors.New("event envelope schema, sequence, or cursor is invalid")
	}
	for kind, value := range map[string]string{"operation": event.OperationID, "graph": event.GraphRunID, "run": event.RunID, "task": event.TaskID, "machine": event.MachineID, "lease": event.LeaseID} {
		if err := validateNodeExecutionTypedID(kind, value); err != nil {
			return err
		}
	}
	if !nodeExecutionFingerprint.MatchString(event.CapabilitySnapshotID) || event.Attempt < 1 {
		return errors.New("event capability identity or attempt is invalid")
	}
	if _, err := parseNodeExecutionTime(event.RecordedAt); err != nil {
		return err
	}
	if err := validateNodeExecutionArtifactReferences(event.OutputReferences); err != nil {
		return err
	}
	if err := validateCanonicalDockPipeEvent(event.Event); err != nil {
		return err
	}
	expected, err := nodeExecutionEventFingerprint(event)
	if err != nil {
		return err
	}
	if event.EnvelopeFingerprint != expected {
		return errors.New("event envelope fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeExecutionCancellation(cancellation NodeExecutionCancellation) error {
	if cancellation.Schema != NodeExecutionCancellationSchema {
		return errors.New("cancellation schema is invalid")
	}
	for kind, value := range map[string]string{"cancellation": cancellation.CancellationID, "operation": cancellation.OperationID, "machine": cancellation.MachineID, "lease": cancellation.LeaseID} {
		if err := validateNodeExecutionTypedID(kind, value); err != nil {
			return err
		}
	}
	if !nodeExecutionFingerprint.MatchString(cancellation.CapabilitySnapshotID) || cancellation.Attempt < 1 {
		return errors.New("cancellation capability identity or attempt is invalid")
	}
	if _, err := parseNodeExecutionTime(cancellation.RequestedAt); err != nil {
		return err
	}
	expected, err := nodeExecutionCancellationFingerprint(cancellation)
	if err != nil {
		return err
	}
	if cancellation.CancellationFingerprint != expected {
		return errors.New("cancellation fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeExecutionCancellationAck(ack NodeExecutionCancellationAck) error {
	if ack.Schema != NodeExecutionCancellationAckSchema {
		return errors.New("cancellation acknowledgement schema is invalid")
	}
	for kind, value := range map[string]string{"cancellation": ack.CancellationID, "operation": ack.OperationID, "lease": ack.LeaseID} {
		if err := validateNodeExecutionTypedID(kind, value); err != nil {
			return err
		}
	}
	if _, err := parseNodeExecutionTime(ack.AcknowledgedAt); err != nil {
		return err
	}
	if !nodeExecutionFingerprint.MatchString(ack.CancellationFingerprint) {
		return errors.New("cancellation acknowledgement request fingerprint is invalid")
	}
	expected, err := nodeExecutionCancellationAckFingerprint(ack)
	if err != nil {
		return err
	}
	if ack.AckFingerprint != expected {
		return errors.New("cancellation acknowledgement fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeExecutionManifest(manifest NodeExecutionArtifactManifest) error {
	if manifest.Schema != NodeExecutionArtifactManifestSchema || len(manifest.Entries) > nodeExecutionMaxArtifacts {
		return errors.New("artifact manifest schema or entry count is invalid")
	}
	if err := validateNodeExecutionArtifactReferences(manifest.Entries); err != nil {
		return err
	}
	expected, err := nodeExecutionManifestFingerprint(manifest)
	if err != nil {
		return err
	}
	if manifest.ManifestFingerprint != expected {
		return errors.New("artifact manifest fingerprint does not match entries")
	}
	return nil
}

func validateNodeExecutionReceiptShape(receipt NodeExecutionReceipt) error {
	if receipt.Schema != NodeExecutionReceiptSchema {
		return errors.New("execution receipt schema is invalid")
	}
	for kind, value := range map[string]string{"receipt": receipt.ReceiptID, "operation": receipt.OperationID, "machine": receipt.MachineID, "lease": receipt.LeaseID} {
		if err := validateNodeExecutionTypedID(kind, value); err != nil {
			return err
		}
	}
	if receipt.LocalRunID != "" {
		if err := validateNodeExecutionTypedID("local-run", receipt.LocalRunID); err != nil {
			return err
		}
	}
	if !nodeExecutionFingerprint.MatchString(receipt.CapabilitySnapshotID) || !nodeExecutionFingerprint.MatchString(receipt.RequestFingerprint) || receipt.Attempt < 1 {
		return errors.New("execution receipt fingerprint binding or attempt is invalid")
	}
	if !regexp.MustCompile(`^cursor:[0-9]{20}$`).MatchString(receipt.FinalCursor) {
		return errors.New("execution receipt final cursor is invalid")
	}
	if receipt.Result != "succeeded" && receipt.Result != "failed" && receipt.Result != "cancelled" && receipt.Result != "degraded" {
		return errors.New("execution receipt result is invalid")
	}
	if err := validateNodeExecutionManifest(receipt.Artifacts); err != nil {
		return err
	}
	if receipt.CancellationAcknowledged {
		if err := validateNodeExecutionTypedID("cancellation", receipt.CancellationID); err != nil {
			return err
		}
		if receipt.Cleanup.Status != "succeeded" && receipt.Cleanup.Status != "failed" {
			return errors.New("acknowledged cancellation requires explicit cleanup outcome")
		}
		if !nodeExecutionFingerprint.MatchString(receipt.Cleanup.EvidenceDigest) {
			return errors.New("cancellation cleanup success or failure requires evidence")
		}
		if receipt.Cleanup.Status == "failed" && receipt.Result != "failed" && receipt.Result != "degraded" {
			return errors.New("cleanup failure must remain a failed or degraded terminal outcome")
		}
		if receipt.Cleanup.Status == "succeeded" && receipt.Result != "cancelled" {
			return errors.New("successful cancellation cleanup must produce cancelled result")
		}
	} else if receipt.CancellationID != "" || receipt.Cleanup.Status != "not_required" || receipt.Cleanup.EvidenceDigest != "" || receipt.Result == "cancelled" {
		return errors.New("non-cancelled receipt cannot claim cancellation or cleanup evidence")
	}
	if _, err := parseNodeExecutionTime(receipt.CompletedAt); err != nil {
		return err
	}
	expected, err := nodeExecutionReceiptFingerprint(receipt)
	if err != nil {
		return err
	}
	if receipt.ReceiptFingerprint != expected {
		return errors.New("execution receipt fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeExecutionReceiptBinding(receipt NodeExecutionReceipt, operation nodeExecutionOperationState) error {
	lease := operation.Lease
	if receipt.ReceiptID != nodeExecutionReceiptID(receipt.OperationID) || receipt.MachineID != lease.MachineID || receipt.CapabilitySnapshotID != lease.CapabilitySnapshotID || receipt.LeaseID != lease.LeaseID || receipt.Attempt != lease.Attempt || receipt.RequestFingerprint != operation.Request.RequestFingerprint || receipt.FinalCursor != nodeExecutionCursor(int64(len(operation.Events))) {
		return errors.New("execution receipt identity or immutable binding does not match the accepted operation")
	}
	completedAt, _ := parseNodeExecutionTime(receipt.CompletedAt)
	expiresAt, _ := parseNodeExecutionTime(lease.ExpiresAt)
	if !completedAt.Before(expiresAt) {
		return errors.New("execution receipt completion is outside the active lease")
	}
	if operation.CancellationAck != nil {
		if !receipt.CancellationAcknowledged || receipt.CancellationID != operation.CancellationAck.CancellationID {
			return errors.New("execution receipt does not bind the acknowledged cancellation")
		}
	} else if receipt.CancellationAcknowledged {
		return errors.New("execution receipt claims an unacknowledged cancellation")
	}
	return nil
}

func validateNodeExecutionActiveLease(operation nodeExecutionOperationState, connectedMachine, claimedMachine, capabilityID, leaseID, operationID string, attempt int, at time.Time) error {
	lease := operation.Lease
	if connectedMachine != lease.MachineID || claimedMachine != lease.MachineID {
		return errors.New("wrong-machine lease claim is rejected")
	}
	if capabilityID != lease.CapabilitySnapshotID {
		return errors.New("wrong-capability lease claim is rejected")
	}
	if operationID != lease.OperationID {
		return errors.New("wrong-operation lease claim is rejected")
	}
	if leaseID != lease.LeaseID {
		return errors.New("stale, replaced, or wrong lease claim is rejected")
	}
	if attempt != lease.Attempt {
		return errors.New("wrong-attempt lease claim is rejected")
	}
	expires, _ := parseNodeExecutionTime(lease.ExpiresAt)
	if !at.UTC().Before(expires) {
		return errors.New("expired lease claim is rejected")
	}
	return nil
}

func validateCanonicalDockPipeEvent(raw json.RawMessage) error {
	var event map[string]any
	if err := decodeNodeExecutionStrict(raw, &event); err != nil {
		return fmt.Errorf("canonical DockPipe event is invalid: %w", err)
	}
	canonical, err := json.Marshal(event)
	if err != nil {
		return err
	}
	compacted := &bytes.Buffer{}
	if err := json.Compact(compacted, raw); err != nil || !bytes.Equal(compacted.Bytes(), canonical) {
		return errors.New("canonical DockPipe event uses a non-canonical key order or value encoding")
	}
	if stringValue(event["schema"]) != "dockpipe.operation_event.v1" || stringValue(event["type"]) == "" || stringValue(event["unit"]) == "" || stringValue(event["status"]) == "" {
		return errors.New("canonical DockPipe event is missing required fields")
	}
	if _, err := parseNodeExecutionTime(stringValue(event["ts"])); err != nil {
		return err
	}
	for key := range event {
		lower := strings.ToLower(key)
		if lower == "stdout" || lower == "stderr" || lower == "command" || lower == "shell" || containsNodeExecutionSecret(lower) {
			return errors.New("DockPipe event embeds forbidden output, command, or credential material")
		}
	}
	return nil
}

func validateNodeExecutionArtifactReferences(entries []NodeExecutionArtifactReference) error {
	last := ""
	for _, entry := range entries {
		if err := validateNodeExecutionName("artifact name", entry.Name); err != nil {
			return err
		}
		if entry.Name <= last {
			return errors.New("artifact references must be unique and sorted")
		}
		last = entry.Name
		if entry.MediaType == "" || len(entry.MediaType) > 128 || strings.Contains(entry.MediaType, "://") || !nodeExecutionFingerprint.MatchString(entry.Digest) || entry.Bytes < 0 || entry.Bytes > nodeExecutionMaxArtifactBytes {
			return errors.New("artifact reference is malformed or unbounded")
		}
		if strings.Contains(entry.Name, "/") || strings.Contains(entry.Name, `\`) || strings.Contains(entry.Name, "..") || strings.Contains(entry.Name, "://") || containsNodeExecutionSecret(entry.Name) {
			return errors.New("remote paths, URLs, and credential-like artifact names are not transferable artifacts")
		}
	}
	return nil
}

func validateNodeExecutionTypedID(kind, value string) error {
	prefix := kind + "-"
	if !nodeExecutionIDPattern.MatchString(value) || !strings.HasPrefix(value, prefix) || containsNodeExecutionSecret(value) || strings.Contains(value, "://") {
		return fmt.Errorf("%s identity is invalid", kind)
	}
	return nil
}

func validateNodeExecutionName(field, value string) error {
	if !nodeExecutionNamePattern.MatchString(value) || containsNodeExecutionSecret(value) || strings.Contains(value, "://") {
		return fmt.Errorf("%s is invalid", field)
	}
	return nil
}

func validateNodeExecutionSortedNames(field string, values []string, allowEmpty bool) error {
	if !allowEmpty && len(values) == 0 {
		return fmt.Errorf("%s must be non-empty", field)
	}
	last := ""
	for _, value := range values {
		if err := validateNodeExecutionName(field, value); err != nil {
			return err
		}
		if value <= last {
			return fmt.Errorf("%s must be unique and sorted", field)
		}
		last = value
	}
	return nil
}

func containsNodeExecutionSecret(value string) bool {
	lower := strings.ToLower(value)
	for _, fragment := range []string{"secret", "token", "password", "credential", "apikey", "api_key", "privatekey", "private_key"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func newNodeExecutionLease(request NodeExecutionRequest, machineID string, issuedAt, expiresAt time.Time) NodeExecutionTaskLease {
	seed := request.OperationID + "\n" + machineID + "\n" + request.CapabilitySnapshotID + "\n1\n" + nodeExecutionTime(issuedAt)
	leaseID := "lease-" + nodeExecutionShortHash(seed)
	return NodeExecutionTaskLease{
		Schema: NodeExecutionLeaseSchema, LeaseID: leaseID, MachineID: machineID,
		CapabilitySnapshotID: request.CapabilitySnapshotID, OperationID: request.OperationID, Attempt: 1,
		IssuedAt: nodeExecutionTime(issuedAt), ExpiresAt: nodeExecutionTime(expiresAt),
		CancellationID: "cancellation-" + nodeExecutionShortHash(request.OperationID+"\n"+leaseID),
	}
}

func nodeExecutionReceiptID(operationID string) string {
	return "receipt-" + nodeExecutionShortHash(operationID)
}

func nodeExecutionShortHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:12])
}

func nodeExecutionCursor(sequence int64) string {
	return fmt.Sprintf("cursor:%020d", sequence)
}

func nodeExecutionTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseNodeExecutionTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || parsed.Location() != time.UTC || parsed.Format(time.RFC3339Nano) != value {
		return time.Time{}, fmt.Errorf("timestamp %q is not canonical UTC RFC3339", value)
	}
	return parsed, nil
}

func decodeNodeExecutionCanonical(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.Marshal(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonical) {
		return errors.New("JSON must use the exact canonical encoding")
	}
	return nil
}

func decodeNodeExecutionStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("JSON contains multiple values")
		}
		return err
	}
	return nil
}

func nodeExecutionCapabilityFingerprint(value NodeExecutionCapabilitySnapshot) (string, error) {
	value.SnapshotID = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionRequestFingerprint(value NodeExecutionRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionEventFingerprint(value NodeExecutionEventEnvelope) (string, error) {
	value.EnvelopeFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionCancellationFingerprint(value NodeExecutionCancellation) (string, error) {
	value.CancellationFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionCancellationAckFingerprint(value NodeExecutionCancellationAck) (string, error) {
	value.AckFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionManifestFingerprint(value NodeExecutionArtifactManifest) (string, error) {
	value.ManifestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionReceiptFingerprint(value NodeExecutionReceipt) (string, error) {
	value.ReceiptFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionStateFingerprint(value nodeExecutionBrokerState) (string, error) {
	value.StateFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeExecutionFingerprintValue(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

func nodeExecutionStateFileName(generation int64) string {
	return fmt.Sprintf("state-%012d.json", generation)
}

func cloneNodeExecutionState(state nodeExecutionBrokerState) nodeExecutionBrokerState {
	raw, _ := json.Marshal(state)
	var cloned nodeExecutionBrokerState
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func nodeExecutionEqual(left, right any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}
