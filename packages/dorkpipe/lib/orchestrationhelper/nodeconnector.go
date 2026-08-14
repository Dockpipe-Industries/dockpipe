package orchestrationhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NodeValidationInvocation is the complete authority exposed to a prepared,
// read-only local DockPipe validation boundary.
type NodeValidationInvocation struct {
	Workflow       NodeExecutionWorkflowReference
	SourceRevision string
}

// NodeValidationEventEvidence carries one unchanged canonical DockPipe event
// and only checksum-backed references to any bounded output.
type NodeValidationEventEvidence struct {
	Sequence         int64
	LocalRunID       string
	RecordedAt       string
	OutputReferences []NodeExecutionArtifactReference
	Event            json.RawMessage
}

// NodeValidationEvidence is the bounded result of one prepared local
// validation run. It grants no lifecycle authority beyond evidence delivery.
type NodeValidationEvidence struct {
	Workflow                 NodeExecutionWorkflowReference
	SourceRevision           string
	LocalRunID               string
	Events                   []NodeValidationEventEvidence
	TerminalResult           string
	Artifacts                []NodeExecutionArtifactReference
	CancellationID           string
	CancellationAcknowledged bool
	Cleanup                  NodeExecutionCleanupOutcome
	CompletedAt              string
}

type NodeValidationFunction func(NodeValidationInvocation) (NodeValidationEvidence, error)

type nodeValidationPreparedDelivery struct {
	events       []NodeExecutionEventEnvelope
	cancellation *NodeExecutionCancellation
	receipt      NodeExecutionReceipt
}

type nodeValidationCachedResult struct {
	delivery nodeValidationPreparedDelivery
	err      error
}

// NodeValidationConnector adapts exactly one typed workflow at one immutable
// source revision to the existing node-execution broker. The injected
// function owns the already-prepared local validation boundary; this adapter
// performs no process, shell, network, provider, Git, or workflow invocation.
type NodeValidationConnector struct {
	expectedWorkflow NodeExecutionWorkflowReference
	expectedRevision string
	validate         NodeValidationFunction

	mu      sync.Mutex
	results map[string]nodeValidationCachedResult
}

func NewNodeValidationConnector(workflow NodeExecutionWorkflowReference, sourceRevision string, validate NodeValidationFunction) (*NodeValidationConnector, error) {
	if workflow.Kind != "dockpipe.workflow" {
		return nil, errors.New("node validation connector workflow kind must be dockpipe.workflow")
	}
	if err := validateNodeExecutionName("workflow package", workflow.Package); err != nil {
		return nil, err
	}
	if err := validateNodeExecutionName("workflow name", workflow.Name); err != nil {
		return nil, err
	}
	if !nodeExecutionRevisionPattern.MatchString(sourceRevision) {
		return nil, errors.New("node validation connector source revision must be an exact 40-character commit")
	}
	if validate == nil {
		return nil, errors.New("node validation connector requires an injected local validation function")
	}
	return &NodeValidationConnector{
		expectedWorkflow: workflow,
		expectedRevision: sourceRevision,
		validate:         validate,
		results:          map[string]nodeValidationCachedResult{},
	}, nil
}

// Execute validates the complete returned evidence before publishing any of
// it through the broker. Exact repeats reuse the prepared result and never
// invoke local validation twice.
func (connector *NodeValidationConnector) Execute(broker *NodeExecutionFakeBroker, connectionID string, request NodeExecutionRequest, lease NodeExecutionTaskLease, cancellation *NodeExecutionCancellation) (NodeExecutionReceipt, error) {
	if broker == nil {
		return NodeExecutionReceipt{}, errors.New("node validation connector requires a broker")
	}
	if err := connector.validateAcceptedRequest(request, lease); err != nil {
		return NodeExecutionReceipt{}, err
	}
	connectedMachine, err := broker.connectedMachine(connectionID)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	operation, accepted := broker.state.Operations[request.OperationID]
	if !accepted || connectedMachine != lease.MachineID || !nodeExecutionEqual(operation.Request, request) || !nodeExecutionEqual(operation.Lease, lease) {
		return NodeExecutionReceipt{}, errors.New("node validation connector requires the exact broker-accepted request and lease")
	}

	connector.mu.Lock()
	defer connector.mu.Unlock()
	cached, ok := connector.results[request.RequestFingerprint]
	if !ok {
		evidence, err := connector.validate(NodeValidationInvocation{Workflow: request.Workflow, SourceRevision: request.SourceRevision})
		if err != nil {
			cached.err = fmt.Errorf("local node validation failed: %w", err)
		} else {
			cached.delivery, cached.err = prepareNodeValidationDelivery(request, lease, cancellation, evidence)
		}
		connector.results[request.RequestFingerprint] = cached
	}
	if cached.err != nil {
		return NodeExecutionReceipt{}, cached.err
	}
	return deliverNodeValidationEvidence(broker, connectionID, cached.delivery)
}

func (connector *NodeValidationConnector) validateAcceptedRequest(request NodeExecutionRequest, lease NodeExecutionTaskLease) error {
	if err := validateNodeExecutionRequest(request); err != nil {
		return err
	}
	if err := validateNodeExecutionLease(lease); err != nil {
		return err
	}
	if !nodeExecutionEqual(request.Workflow, connector.expectedWorkflow) {
		return errors.New("execution request workflow does not match the prepared local validation boundary")
	}
	if request.SourceRevision != connector.expectedRevision {
		return errors.New("execution request source revision does not match the prepared local validation boundary")
	}
	if lease.OperationID != request.OperationID || lease.CapabilitySnapshotID != request.CapabilitySnapshotID {
		return errors.New("task lease is not bound to the accepted validation request")
	}
	return nil
}

func prepareNodeValidationDelivery(request NodeExecutionRequest, lease NodeExecutionTaskLease, cancellation *NodeExecutionCancellation, evidence NodeValidationEvidence) (nodeValidationPreparedDelivery, error) {
	if !nodeExecutionEqual(evidence.Workflow, request.Workflow) {
		return nodeValidationPreparedDelivery{}, errors.New("local validation evidence workflow does not match the accepted request")
	}
	if evidence.SourceRevision != request.SourceRevision {
		return nodeValidationPreparedDelivery{}, errors.New("local validation evidence source revision does not match the accepted request")
	}
	if err := validateNodeExecutionTypedID("local-run", evidence.LocalRunID); err != nil {
		return nodeValidationPreparedDelivery{}, err
	}
	if len(evidence.Events) == 0 || len(evidence.Events) > nodeExecutionMaxArtifacts {
		return nodeValidationPreparedDelivery{}, errors.New("local validation events must be non-empty and bounded")
	}

	delivery := nodeValidationPreparedDelivery{events: make([]NodeExecutionEventEnvelope, 0, len(evidence.Events))}
	var previousRecordedAt time.Time
	leaseExpiresAt, _ := parseNodeExecutionTime(lease.ExpiresAt)
	totalReferences := len(evidence.Artifacts)
	var totalBytes int64
	for index, returned := range evidence.Events {
		if returned.Sequence != int64(index+1) {
			return nodeValidationPreparedDelivery{}, errors.New("local validation event ordering is invalid")
		}
		if returned.LocalRunID != evidence.LocalRunID {
			return nodeValidationPreparedDelivery{}, errors.New("local validation events contain inconsistent local run identities")
		}
		recordedAt, err := parseNodeExecutionTime(returned.RecordedAt)
		if err != nil {
			return nodeValidationPreparedDelivery{}, err
		}
		if !previousRecordedAt.IsZero() && !recordedAt.After(previousRecordedAt) {
			return nodeValidationPreparedDelivery{}, errors.New("local validation event timestamps are not strictly ordered")
		}
		if !recordedAt.Before(leaseExpiresAt) {
			return nodeValidationPreparedDelivery{}, errors.New("local validation event is outside the active lease")
		}
		previousRecordedAt = recordedAt
		if err := validateCanonicalDockPipeEvent(returned.Event); err != nil {
			return nodeValidationPreparedDelivery{}, err
		}
		if err := validateNodeExecutionArtifactReferences(returned.OutputReferences); err != nil {
			return nodeValidationPreparedDelivery{}, err
		}
		totalReferences += len(returned.OutputReferences)
		for _, reference := range returned.OutputReferences {
			if totalBytes > nodeExecutionMaxArtifactBytes-reference.Bytes {
				return nodeValidationPreparedDelivery{}, errors.New("local validation output exceeds the bounded transfer limit")
			}
			totalBytes += reference.Bytes
		}
		event, err := FinalizeNodeExecutionEvent(NodeExecutionEventEnvelope{
			OperationID: request.OperationID, GraphRunID: request.GraphRunID, RunID: request.RunID, TaskID: request.TaskID,
			MachineID: lease.MachineID, CapabilitySnapshotID: lease.CapabilitySnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt,
			Sequence: returned.Sequence, RecordedAt: nodeExecutionTime(recordedAt), OutputReferences: append([]NodeExecutionArtifactReference{}, returned.OutputReferences...), Event: append(json.RawMessage(nil), returned.Event...),
		})
		if err != nil {
			return nodeValidationPreparedDelivery{}, err
		}
		delivery.events = append(delivery.events, event)
	}
	if totalReferences > nodeExecutionMaxArtifacts {
		return nodeValidationPreparedDelivery{}, errors.New("local validation output and artifacts exceed the bounded reference limit")
	}
	if err := validateNodeExecutionArtifactReferences(evidence.Artifacts); err != nil {
		return nodeValidationPreparedDelivery{}, err
	}
	for _, reference := range evidence.Artifacts {
		if totalBytes > nodeExecutionMaxArtifactBytes-reference.Bytes {
			return nodeValidationPreparedDelivery{}, errors.New("local validation output exceeds the bounded transfer limit")
		}
		totalBytes += reference.Bytes
	}

	terminalStatus, err := nodeValidationEventStatus(evidence.Events[len(evidence.Events)-1].Event)
	if err != nil {
		return nodeValidationPreparedDelivery{}, err
	}
	if (evidence.TerminalResult == "succeeded" && terminalStatus != "done") ||
		((evidence.TerminalResult == "failed" || evidence.TerminalResult == "degraded" || evidence.TerminalResult == "cancelled") && terminalStatus != "fail") {
		return nodeValidationPreparedDelivery{}, errors.New("local validation terminal result conflicts with its final DockPipe event")
	}

	if cancellation == nil {
		if evidence.CancellationAcknowledged || evidence.CancellationID != "" {
			return nodeValidationPreparedDelivery{}, errors.New("local validation evidence claims a cancellation that was not requested")
		}
	} else {
		if err := validateNodeExecutionCancellation(*cancellation); err != nil {
			return nodeValidationPreparedDelivery{}, err
		}
		if cancellation.OperationID != request.OperationID || cancellation.MachineID != lease.MachineID || cancellation.CapabilitySnapshotID != lease.CapabilitySnapshotID || cancellation.LeaseID != lease.LeaseID || cancellation.Attempt != lease.Attempt || cancellation.CancellationID != lease.CancellationID {
			return nodeValidationPreparedDelivery{}, errors.New("stale or differently bound local validation cancellation is rejected")
		}
		requestedAt, _ := parseNodeExecutionTime(cancellation.RequestedAt)
		if !requestedAt.Before(leaseExpiresAt) {
			return nodeValidationPreparedDelivery{}, errors.New("local validation cancellation is outside the active lease")
		}
		if !evidence.CancellationAcknowledged || evidence.CancellationID != cancellation.CancellationID {
			return nodeValidationPreparedDelivery{}, errors.New("local validation evidence is missing the accepted cancellation acknowledgement")
		}
		value := *cancellation
		delivery.cancellation = &value
	}

	manifest, err := NewNodeExecutionArtifactManifest(evidence.Artifacts)
	if err != nil {
		return nodeValidationPreparedDelivery{}, err
	}
	completedAt, err := parseNodeExecutionTime(evidence.CompletedAt)
	if err != nil {
		return nodeValidationPreparedDelivery{}, err
	}
	if !completedAt.Before(leaseExpiresAt) || !completedAt.After(previousRecordedAt) {
		return nodeValidationPreparedDelivery{}, errors.New("local validation completion is not after its events within the active lease")
	}
	receipt, err := FinalizeNodeExecutionReceipt(NodeExecutionReceipt{
		ReceiptID: nodeExecutionReceiptID(request.OperationID), OperationID: request.OperationID,
		MachineID: lease.MachineID, CapabilitySnapshotID: lease.CapabilitySnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt,
		RequestFingerprint: request.RequestFingerprint, LocalRunID: evidence.LocalRunID,
		FinalCursor: nodeExecutionCursor(int64(len(delivery.events))), Result: evidence.TerminalResult, Artifacts: manifest,
		CancellationID: evidence.CancellationID, CancellationAcknowledged: evidence.CancellationAcknowledged,
		Cleanup: evidence.Cleanup, CompletedAt: evidence.CompletedAt,
	})
	if err != nil {
		return nodeValidationPreparedDelivery{}, err
	}
	delivery.receipt = receipt
	return delivery, nil
}

func nodeValidationEventStatus(raw json.RawMessage) (string, error) {
	var event map[string]any
	if err := decodeNodeExecutionStrict(raw, &event); err != nil {
		return "", err
	}
	return stringValue(event["status"]), nil
}

func deliverNodeValidationEvidence(broker *NodeExecutionFakeBroker, connectionID string, delivery nodeValidationPreparedDelivery) (NodeExecutionReceipt, error) {
	resume, err := broker.Resume(connectionID, delivery.receipt.OperationID)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	if resume.Receipt != nil {
		if !nodeExecutionEqual(*resume.Receipt, delivery.receipt) {
			return NodeExecutionReceipt{}, errors.New("terminal broker receipt conflicts with local validation evidence")
		}
		return *resume.Receipt, nil
	}
	operation := broker.state.Operations[delivery.receipt.OperationID]
	if len(operation.Events) > len(delivery.events) {
		return NodeExecutionReceipt{}, errors.New("broker event cursor exceeds local validation evidence")
	}
	for index := range operation.Events {
		if !nodeExecutionEqual(operation.Events[index], delivery.events[index]) {
			return NodeExecutionReceipt{}, errors.New("broker event history conflicts with local validation evidence")
		}
	}
	for index := len(operation.Events); index < len(delivery.events); index++ {
		event := delivery.events[index]
		if err := broker.AcceptEvent(connectionID, nodeValidationJSON(event), mustNodeValidationTime(event.RecordedAt)); err != nil {
			return NodeExecutionReceipt{}, err
		}
	}
	if delivery.cancellation != nil {
		if _, err := broker.RequestCancellation(connectionID, nodeValidationJSON(*delivery.cancellation), mustNodeValidationTime(delivery.cancellation.RequestedAt)); err != nil {
			return NodeExecutionReceipt{}, err
		}
	}
	return broker.AcceptReceipt(connectionID, nodeValidationJSON(delivery.receipt), mustNodeValidationTime(delivery.receipt.CompletedAt))
}

func nodeValidationJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func mustNodeValidationTime(value string) (resultTime time.Time) {
	resultTime, _ = parseNodeExecutionTime(value)
	return resultTime
}
