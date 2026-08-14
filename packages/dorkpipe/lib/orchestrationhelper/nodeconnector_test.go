package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNodeConnectorExecutesValidationOnceAndBindsEvidence(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	evidence := nodeConnectorTestEvidence(t, fixture)
	calls := 0
	var invocation NodeValidationInvocation
	connector, err := NewNodeValidationConnector(fixture.request.Workflow, fixture.request.SourceRevision, func(got NodeValidationInvocation) (NodeValidationEvidence, error) {
		calls++
		invocation = got
		return evidence, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	connection := "connection-connector-001"
	var broker *NodeExecutionFakeBroker
	var delivered NodeExecutionReceipt
	var deliveryErr error
	broker, err = NewNodeExecutionFakeBroker(root, fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, func(request NodeExecutionRequest, lease NodeExecutionTaskLease) {
		delivered, deliveryErr = connector.Execute(broker, connection, request, lease, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.Connect(fixture.machine.MachineID, connection); err != nil {
		t.Fatal(err)
	}
	lease, err := broker.Dispatch(connection, fixture.requestRaw, fixture.now, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if deliveryErr != nil {
		t.Fatalf("connector delivery: %v", deliveryErr)
	}
	if calls != 1 || !nodeExecutionEqual(invocation.Workflow, fixture.request.Workflow) || invocation.SourceRevision != fixture.request.SourceRevision {
		t.Fatalf("validator did not receive the exact typed request once: calls=%d invocation=%#v", calls, invocation)
	}
	if delivered.LocalRunID != evidence.LocalRunID || delivered.Result != evidence.TerminalResult || delivered.Artifacts.ManifestFingerprint == "" || delivered.LeaseID != lease.LeaseID || delivered.RequestFingerprint != fixture.request.RequestFingerprint {
		t.Fatalf("receipt did not bind validation evidence: %#v", delivered)
	}
	operation := broker.state.Operations[fixture.request.OperationID]
	if len(operation.Events) != len(evidence.Events) || operation.Receipt == nil {
		t.Fatalf("connector did not publish the complete broker result: %#v", operation)
	}
	for index := range evidence.Events {
		if !bytes.Equal(operation.Events[index].Event, evidence.Events[index].Event) {
			t.Fatalf("canonical event %d was changed in the outer envelope", index+1)
		}
	}

	terminalArtifacts := nodeConnectorStateBytes(t, root)
	if _, err := broker.Dispatch(connection, fixture.requestRaw, fixture.now, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := connector.Execute(broker, connection, fixture.request, lease, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !nodeConnectorStateBytesEqual(terminalArtifacts, nodeConnectorStateBytes(t, root)) {
		t.Fatal("duplicate dispatch or terminal delivery reran validation or published new state")
	}

	broker.Disconnect(connection)
	reopenedCalls := 0
	reopened, err := NewNodeExecutionFakeBroker(root, fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, func(NodeExecutionRequest, NodeExecutionTaskLease) { reopenedCalls++ })
	if err != nil {
		t.Fatal(err)
	}
	reconnected := "connection-connector-reopened-001"
	if err := reopened.Connect(fixture.machine.MachineID, reconnected); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		resume, err := reopened.Resume(reconnected, fixture.request.OperationID)
		if err != nil || resume.Receipt == nil || resume.Receipt.ReceiptFingerprint != delivered.ReceiptFingerprint {
			t.Fatalf("restart resume lost the terminal receipt: resume=%#v err=%v", resume, err)
		}
	}
	if _, err := reopened.Dispatch(reconnected, fixture.requestRaw, fixture.now, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	if reopenedCalls != 0 || calls != 1 || !nodeConnectorStateBytesEqual(terminalArtifacts, nodeConnectorStateBytes(t, root)) {
		t.Fatal("reconnect, reopen, resume, or replay executed validation twice")
	}
}

func TestNodeConnectorRejectsInvalidEvidenceWithoutPartialState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeValidationEvidence)
	}{
		{name: "workflow kind mismatch", mutate: func(value *NodeValidationEvidence) { value.Workflow.Kind = "other.workflow" }},
		{name: "workflow package mismatch", mutate: func(value *NodeValidationEvidence) { value.Workflow.Package = "other" }},
		{name: "workflow name mismatch", mutate: func(value *NodeValidationEvidence) { value.Workflow.Name = "validate.other" }},
		{name: "source revision mismatch", mutate: func(value *NodeValidationEvidence) { value.SourceRevision = strings.Repeat("b", 40) }},
		{name: "malformed canonical event", mutate: func(value *NodeValidationEvidence) {
			value.Events[0].Event = json.RawMessage(`{"type":"operation_result","schema":"dockpipe.operation_event.v1","ts":"2026-07-25T12:01:00Z","unit":"validation.readonly","status":"start"}`)
		}},
		{name: "event ordering gap", mutate: func(value *NodeValidationEvidence) { value.Events[1].Sequence = 3 }},
		{name: "path bearing output", mutate: func(value *NodeValidationEvidence) { value.Events[0].OutputReferences[0].Name = "C:/temp/stdout.txt" }},
		{name: "invalid artifact digest", mutate: func(value *NodeValidationEvidence) { value.Artifacts[0].Digest = "sha256:invalid" }},
		{name: "inconsistent local run", mutate: func(value *NodeValidationEvidence) { value.Events[1].LocalRunID = "local-run-other-001" }},
		{name: "conflicting terminal result", mutate: func(value *NodeValidationEvidence) { value.TerminalResult = "failed" }},
		{name: "unrequested cleanup", mutate: func(value *NodeValidationEvidence) {
			value.Cleanup = NodeExecutionCleanupOutcome{Status: "failed", EvidenceDigest: nodeExecutionTestDigest("residue")}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newNodeExecutionTestFixture(t)
			broker, connection, lease := nodeConnectorAcceptedBroker(t, fixture)
			before := nodeConnectorStateBytes(t, broker.root)
			evidence := nodeConnectorTestEvidence(t, fixture)
			test.mutate(&evidence)
			calls := 0
			connector, err := NewNodeValidationConnector(fixture.request.Workflow, fixture.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
				calls++
				return evidence, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := connector.Execute(broker, connection, fixture.request, lease, nil); err == nil {
				t.Fatal("invalid local validation evidence was accepted")
			}
			if calls != 1 || !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, broker.root)) {
				t.Fatal("rejected evidence published partial broker state or reran validation")
			}
		})
	}

	fixture := newNodeExecutionTestFixture(t)
	broker, connection, lease := nodeConnectorAcceptedBroker(t, fixture)
	calls := 0
	connector, err := NewNodeValidationConnector(NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "validate.other"}, fixture.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		calls++
		return nodeConnectorTestEvidence(t, fixture), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := nodeConnectorStateBytes(t, broker.root)
	if _, err := connector.Execute(broker, connection, fixture.request, lease, nil); err == nil {
		t.Fatal("request workflow mismatch was accepted")
	}
	if calls != 0 || !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, broker.root)) {
		t.Fatal("request mismatch invoked validation or changed accepted artifacts")
	}

	calls = 0
	connector, err = NewNodeValidationConnector(fixture.request.Workflow, strings.Repeat("b", 40), func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		calls++
		return nodeConnectorTestEvidence(t, fixture), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connector.Execute(broker, connection, fixture.request, lease, nil); err == nil {
		t.Fatal("request source revision mismatch was accepted")
	}
	if calls != 0 || !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, broker.root)) {
		t.Fatal("source revision mismatch invoked validation or changed accepted artifacts")
	}
}

func TestNodeConnectorKeepsCancellationAndCleanupSeparate(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	broker, connection, lease := nodeConnectorAcceptedBroker(t, fixture)
	cancellation := nodeExecutionTestCancellation(t, fixture, lease)
	evidence := nodeConnectorTestEvidence(t, fixture)
	evidence.Events[len(evidence.Events)-1].Event = nodeConnectorOperationEvent(t, fixture.now.Add(2*time.Minute), "fail")
	evidence.TerminalResult = "cancelled"
	evidence.CancellationID = cancellation.CancellationID
	evidence.CancellationAcknowledged = true
	evidence.Cleanup = NodeExecutionCleanupOutcome{Status: "succeeded", EvidenceDigest: nodeExecutionTestDigest("cleanup-complete")}
	calls := 0
	connector, err := NewNodeValidationConnector(fixture.request.Workflow, fixture.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		calls++
		return evidence, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := connector.Execute(broker, connection, fixture.request, lease, &cancellation)
	if err != nil {
		t.Fatal(err)
	}
	operation := broker.state.Operations[fixture.request.OperationID]
	if operation.CancellationAck == nil || receipt.Result != "cancelled" || !receipt.CancellationAcknowledged || receipt.Cleanup.Status != "succeeded" || receipt.Cleanup.EvidenceDigest == "" {
		t.Fatalf("cancellation acknowledgement and cleanup were not separately bound: %#v", receipt)
	}
	if _, err := connector.Execute(broker, connection, fixture.request, lease, &cancellation); err != nil || calls != 1 {
		t.Fatalf("terminal cancellation replay was not idempotent: calls=%d err=%v", calls, err)
	}

	degradedFixture := newNodeExecutionTestFixture(t)
	degradedBroker, degradedConnection, degradedLease := nodeConnectorAcceptedBroker(t, degradedFixture)
	degradedCancellation := nodeExecutionTestCancellation(t, degradedFixture, degradedLease)
	degradedEvidence := nodeConnectorTestEvidence(t, degradedFixture)
	degradedEvidence.Events[len(degradedEvidence.Events)-1].Event = nodeConnectorOperationEvent(t, degradedFixture.now.Add(2*time.Minute), "fail")
	degradedEvidence.TerminalResult = "degraded"
	degradedEvidence.CancellationID = degradedCancellation.CancellationID
	degradedEvidence.CancellationAcknowledged = true
	degradedEvidence.Cleanup = NodeExecutionCleanupOutcome{Status: "failed", EvidenceDigest: nodeExecutionTestDigest("cleanup-residue")}
	degradedConnector, err := NewNodeValidationConnector(degradedFixture.request.Workflow, degradedFixture.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) { return degradedEvidence, nil })
	if err != nil {
		t.Fatal(err)
	}
	degradedReceipt, err := degradedConnector.Execute(degradedBroker, degradedConnection, degradedFixture.request, degradedLease, &degradedCancellation)
	if err != nil || degradedReceipt.Result != "degraded" || degradedReceipt.Cleanup.Status != "failed" {
		t.Fatalf("cleanup failure was collapsed into success: receipt=%#v err=%v", degradedReceipt, err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*NodeExecutionCancellation, *NodeValidationEvidence)
	}{
		{name: "stale cancellation", mutate: func(value *NodeExecutionCancellation, _ *NodeValidationEvidence) {
			value.LeaseID = "lease-stale-000000000001"
			*value = mustFinalizeNodeExecutionCancellation(t, *value)
		}},
		{name: "missing cleanup evidence", mutate: func(_ *NodeExecutionCancellation, value *NodeValidationEvidence) {
			value.Cleanup = NodeExecutionCleanupOutcome{Status: "succeeded"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newNodeExecutionTestFixture(t)
			broker, connection, lease := nodeConnectorAcceptedBroker(t, fixture)
			cancellation := nodeExecutionTestCancellation(t, fixture, lease)
			evidence := nodeConnectorTestEvidence(t, fixture)
			evidence.Events[len(evidence.Events)-1].Event = nodeConnectorOperationEvent(t, fixture.now.Add(2*time.Minute), "fail")
			evidence.TerminalResult = "cancelled"
			evidence.CancellationID = cancellation.CancellationID
			evidence.CancellationAcknowledged = true
			evidence.Cleanup = NodeExecutionCleanupOutcome{Status: "succeeded", EvidenceDigest: nodeExecutionTestDigest("cleanup-complete")}
			test.mutate(&cancellation, &evidence)
			before := nodeConnectorStateBytes(t, broker.root)
			connector, err := NewNodeValidationConnector(fixture.request.Workflow, fixture.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) { return evidence, nil })
			if err != nil {
				t.Fatal(err)
			}
			if _, err := connector.Execute(broker, connection, fixture.request, lease, &cancellation); err == nil {
				t.Fatal("invalid cancellation evidence was accepted")
			}
			if !nodeConnectorStateBytesEqual(before, nodeConnectorStateBytes(t, broker.root)) {
				t.Fatal("rejected cancellation evidence published partial state")
			}
		})
	}
}

func nodeConnectorAcceptedBroker(t *testing.T, fixture *nodeExecutionTestFixture) (*NodeExecutionFakeBroker, string, NodeExecutionTaskLease) {
	t.Helper()
	broker, err := NewNodeExecutionFakeBroker(t.TempDir(), fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, nil)
	if err != nil {
		t.Fatal(err)
	}
	connection := "connection-connector-manual-001"
	if err := broker.Connect(fixture.machine.MachineID, connection); err != nil {
		t.Fatal(err)
	}
	lease, err := broker.Dispatch(connection, fixture.requestRaw, fixture.now, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return broker, connection, lease
}

func nodeConnectorTestEvidence(t *testing.T, fixture *nodeExecutionTestFixture) NodeValidationEvidence {
	t.Helper()
	localRunID := "local-run-connector-001"
	return NodeValidationEvidence{
		Workflow: fixture.request.Workflow, SourceRevision: fixture.request.SourceRevision, LocalRunID: localRunID,
		Events: []NodeValidationEventEvidence{
			{Sequence: 1, LocalRunID: localRunID, RecordedAt: nodeExecutionTime(fixture.now.Add(time.Minute)), OutputReferences: []NodeExecutionArtifactReference{{Name: "stdout.txt", MediaType: "text/plain", Digest: nodeExecutionTestDigest("bounded output"), Bytes: 14}}, Event: nodeConnectorOperationEvent(t, fixture.now.Add(time.Minute), "start")},
			{Sequence: 2, LocalRunID: localRunID, RecordedAt: nodeExecutionTime(fixture.now.Add(2 * time.Minute)), OutputReferences: []NodeExecutionArtifactReference{}, Event: nodeConnectorOperationEvent(t, fixture.now.Add(2*time.Minute), "done")},
		},
		TerminalResult: "succeeded",
		Artifacts:      []NodeExecutionArtifactReference{{Name: "validation.json", MediaType: "application/json", Digest: nodeExecutionTestDigest("validation evidence"), Bytes: 19}},
		Cleanup:        NodeExecutionCleanupOutcome{Status: "not_required"}, CompletedAt: nodeExecutionTime(fixture.now.Add(3 * time.Minute)),
	}
}

func nodeConnectorOperationEvent(t *testing.T, at time.Time, status string) json.RawMessage {
	t.Helper()
	return mustNodeExecutionJSON(t, map[string]any{
		"schema": "dockpipe.operation_event.v1", "type": "operation_result", "ts": nodeExecutionTime(at),
		"unit": "validation.readonly", "status": status,
	})
}

func nodeConnectorStateBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !nodeExecutionStateName.MatchString(entry.Name()) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = raw
	}
	return result
}

func nodeConnectorStateBytesEqual(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for name, raw := range left {
		if !bytes.Equal(raw, right[name]) {
			return false
		}
	}
	return true
}
