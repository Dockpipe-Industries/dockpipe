package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type nodeExecutionTestFixture struct {
	root       string
	now        time.Time
	machine    NodeExecutionMachineIdentity
	capability NodeExecutionCapabilitySnapshot
	request    NodeExecutionRequest
	requestRaw []byte
	broker     *NodeExecutionFakeBroker
	connection string
	calls      *int
}

func TestNodeExecutionDispatchReconnectRestartAndTerminalReceipt(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	lease := dispatchNodeExecutionFixture(t, fixture)
	if *fixture.calls != 1 {
		t.Fatalf("fake executor calls = %d, want 1", *fixture.calls)
	}
	if lease.MachineID == fixture.connection || lease.LeaseID == lease.OperationID || lease.CapabilitySnapshotID == lease.LeaseID {
		t.Fatal("machine, connection, lease, operation, and capability identities were not kept separate")
	}

	event1 := nodeExecutionTestEvent(t, fixture, lease, 1, "start")
	mustAcceptNodeExecutionEvent(t, fixture.broker, fixture.connection, event1, fixture.now.Add(time.Minute))
	fixture.broker.Disconnect(fixture.connection)
	beforeRestart := nodeExecutionStateArtifacts(t, fixture.root)

	restarted, err := NewNodeExecutionFakeBroker(fixture.root, fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, func(NodeExecutionRequest, NodeExecutionTaskLease) { *fixture.calls++ })
	if err != nil {
		t.Fatalf("restart broker: %v", err)
	}
	connection2 := "connection-restarted-001"
	if err := restarted.Connect(fixture.machine.MachineID, connection2); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	resume, err := restarted.Resume(connection2, fixture.request.OperationID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resume.Cursor != nodeExecutionCursor(1) || resume.Lease.LeaseID != lease.LeaseID || resume.Receipt != nil {
		t.Fatalf("unexpected resume state: %#v", resume)
	}
	replayedLease, err := restarted.Dispatch(connection2, fixture.requestRaw, fixture.now, 30*time.Minute)
	if err != nil {
		t.Fatalf("exact request replay: %v", err)
	}
	if replayedLease.LeaseID != lease.LeaseID || *fixture.calls != 1 {
		t.Fatalf("reconnect issued a second lease or execution: lease=%s calls=%d", replayedLease.LeaseID, *fixture.calls)
	}
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(beforeRestart, got) {
		t.Fatalf("restart or replay mutated state: before=%v after=%v", beforeRestart, got)
	}

	event2 := nodeExecutionTestEvent(t, fixture, lease, 2, "done")
	mustAcceptNodeExecutionEvent(t, restarted, connection2, event2, fixture.now.Add(2*time.Minute))
	receipt := nodeExecutionTestReceipt(t, fixture, lease, "succeeded", "not_required", "")
	receipt.FinalCursor = nodeExecutionCursor(2)
	receipt, err = FinalizeNodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	mustAcceptNodeExecutionReceipt(t, restarted, connection2, receipt, fixture.now.Add(3*time.Minute))
	terminalArtifacts := nodeExecutionStateArtifacts(t, fixture.root)
	mustAcceptNodeExecutionReceipt(t, restarted, connection2, receipt, fixture.now.Add(3*time.Minute))
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(terminalArtifacts, got) {
		t.Fatal("exact terminal receipt replay published a new state")
	}

	reopened, err := NewNodeExecutionFakeBroker(fixture.root, fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, func(NodeExecutionRequest, NodeExecutionTaskLease) { *fixture.calls++ })
	if err != nil {
		t.Fatalf("terminal reopen: %v", err)
	}
	if err := reopened.Connect(fixture.machine.MachineID, "connection-terminal-001"); err != nil {
		t.Fatal(err)
	}
	terminal, err := reopened.Resume("connection-terminal-001", fixture.request.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Receipt == nil || terminal.Receipt.ReceiptFingerprint != receipt.ReceiptFingerprint || *fixture.calls != 1 {
		t.Fatalf("terminal state was not stable across reopen: %#v calls=%d", terminal, *fixture.calls)
	}
}

func TestNodeExecutionCapabilityRefreshAndLeaseIdentityFailures(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	lease := dispatchNodeExecutionFixture(t, fixture)
	originalLease := lease
	refreshed, err := NewNodeExecutionCapabilitySnapshot(fixture.machine.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "windows", Runtime: "docker", Toolchains: []string{"go1.25"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, fixture.now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.SnapshotID == fixture.capability.SnapshotID {
		t.Fatal("capability refresh reused an immutable snapshot identity")
	}
	if err := fixture.broker.RegisterCapabilitySnapshot(refreshed); err != nil {
		t.Fatal(err)
	}
	resume, err := fixture.broker.Resume(fixture.connection, fixture.request.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if !nodeExecutionEqual(resume.Lease, originalLease) {
		t.Fatal("capability refresh mutated the existing lease")
	}
	stateCount := len(nodeExecutionStateArtifacts(t, fixture.root))
	if err := fixture.broker.RegisterCapabilitySnapshot(refreshed); err != nil {
		t.Fatal(err)
	}
	if len(nodeExecutionStateArtifacts(t, fixture.root)) != stateCount {
		t.Fatal("exact capability replay published a new state")
	}

	base := nodeExecutionTestEvent(t, fixture, lease, 1, "progress")
	tests := []struct {
		name   string
		at     time.Time
		mutate func(*NodeExecutionEventEnvelope)
	}{
		{name: "wrong machine", at: fixture.now.Add(time.Minute), mutate: func(event *NodeExecutionEventEnvelope) { event.MachineID = "machine-substitute-001" }},
		{name: "wrong capability", at: fixture.now.Add(time.Minute), mutate: func(event *NodeExecutionEventEnvelope) { event.CapabilitySnapshotID = refreshed.SnapshotID }},
		{name: "replaced lease", at: fixture.now.Add(time.Minute), mutate: func(event *NodeExecutionEventEnvelope) { event.LeaseID = "lease-replaced-00000001" }},
		{name: "wrong operation", at: fixture.now.Add(time.Minute), mutate: func(event *NodeExecutionEventEnvelope) { event.OperationID = "operation-substitute-001" }},
		{name: "wrong attempt", at: fixture.now.Add(time.Minute), mutate: func(event *NodeExecutionEventEnvelope) { event.Attempt = 2 }},
		{name: "expired", at: fixture.now.Add(30 * time.Minute), mutate: func(*NodeExecutionEventEnvelope) {}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := base
			test.mutate(&event)
			event = mustFinalizeNodeExecutionEvent(t, event)
			raw := mustNodeExecutionJSON(t, event)
			if err := fixture.broker.AcceptEvent(fixture.connection, raw, test.at); err == nil {
				t.Fatalf("expected %s lease rejection", test.name)
			}
		})
	}
	if len(fixture.broker.state.Operations[fixture.request.OperationID].Events) != 0 {
		t.Fatal("rejected lease substitutions mutated durable events")
	}

	badRequest := fixture.request
	badRequest.OperationID = fixture.machine.MachineID
	if _, err := FinalizeNodeExecutionRequest(badRequest); err == nil {
		t.Fatal("machine identity substituted for operation identity")
	}
	badReceipt := nodeExecutionTestReceipt(t, fixture, lease, "succeeded", "not_required", "")
	badReceipt.ReceiptID = lease.LeaseID
	if _, err := FinalizeNodeExecutionReceipt(badReceipt); err == nil {
		t.Fatal("lease identity substituted for receipt identity")
	}
}

func TestNodeExecutionRequestReplayAndStrictJSON(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	dispatchNodeExecutionFixture(t, fixture)
	before := nodeExecutionStateArtifacts(t, fixture.root)
	if _, err := fixture.broker.Dispatch(fixture.connection, fixture.requestRaw, fixture.now, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(before, got) {
		t.Fatal("exact request replay published new evidence")
	}

	changed := fixture.request
	changed.Inputs = []NodeExecutionInput{{Name: "mode", Value: "readonly"}}
	changed, err := FinalizeNodeExecutionRequest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.broker.Dispatch(fixture.connection, mustNodeExecutionJSON(t, changed), fixture.now, 30*time.Minute); err == nil {
		t.Fatal("same operation ID with changed request was accepted")
	}
	if _, err := fixture.broker.Dispatch(fixture.connection, append(fixture.requestRaw, '\n'), fixture.now, 30*time.Minute); err == nil {
		t.Fatal("non-canonical changed request bytes were accepted")
	}

	for _, field := range []string{"command", "shell", "provider", "token", "credential_url"} {
		t.Run("unknown_"+field, func(t *testing.T) {
			var payload map[string]any
			if err := json.Unmarshal(fixture.requestRaw, &payload); err != nil {
				t.Fatal(err)
			}
			payload[field] = "https://user:secret@example.invalid/run"
			if _, err := fixture.broker.Dispatch(fixture.connection, mustNodeExecutionJSON(t, payload), fixture.now, 30*time.Minute); err == nil {
				t.Fatalf("forbidden %s field was accepted", field)
			}
		})
	}

	invalid := fixture.request
	invalid.RequestedAt = "not-a-time"
	if _, err := FinalizeNodeExecutionRequest(invalid); err == nil {
		t.Fatal("invalid request timestamp was accepted")
	}
	invalid = fixture.request
	invalid.TaskID = "machine-office-001"
	if _, err := FinalizeNodeExecutionRequest(invalid); err == nil {
		t.Fatal("invalid typed request identity was accepted")
	}
	invalid = fixture.request
	invalid.RequestedAt = ""
	if _, err := FinalizeNodeExecutionRequest(invalid); err == nil {
		t.Fatal("missing required request field was accepted")
	}
	invalid = fixture.request
	invalid.Inputs = []NodeExecutionInput{{Name: "access_token", Value: "plain"}}
	if _, err := FinalizeNodeExecutionRequest(invalid); err == nil {
		t.Fatal("credential-like input name was accepted")
	}
	invalid = fixture.request
	invalid.Inputs = []NodeExecutionInput{{Name: "endpoint", Value: "https://user:password@example.invalid"}}
	if _, err := FinalizeNodeExecutionRequest(invalid); err == nil {
		t.Fatal("credential-bearing URL input was accepted")
	}
}

func TestNodeExecutionEventOrderingReplayAndPostTerminal(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	lease := dispatchNodeExecutionFixture(t, fixture)
	event1 := nodeExecutionTestEvent(t, fixture, lease, 1, "start")
	mustAcceptNodeExecutionEvent(t, fixture.broker, fixture.connection, event1, fixture.now.Add(time.Minute))
	beforeDuplicate := nodeExecutionStateArtifacts(t, fixture.root)
	mustAcceptNodeExecutionEvent(t, fixture.broker, fixture.connection, event1, fixture.now.Add(time.Minute))
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(beforeDuplicate, got) {
		t.Fatal("exact duplicate event published a new state")
	}
	changedDuplicate := nodeExecutionTestEvent(t, fixture, lease, 1, "fail")
	if err := fixture.broker.AcceptEvent(fixture.connection, mustNodeExecutionJSON(t, changedDuplicate), fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("changed duplicate event was accepted")
	}
	gap := nodeExecutionTestEvent(t, fixture, lease, 3, "progress")
	if err := fixture.broker.AcceptEvent(fixture.connection, mustNodeExecutionJSON(t, gap), fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("event sequence gap was accepted")
	}
	event2 := nodeExecutionTestEvent(t, fixture, lease, 2, "done")
	mustAcceptNodeExecutionEvent(t, fixture.broker, fixture.connection, event2, fixture.now.Add(2*time.Minute))
	receipt := nodeExecutionTestReceipt(t, fixture, lease, "succeeded", "not_required", "")
	mustAcceptNodeExecutionReceipt(t, fixture.broker, fixture.connection, receipt, fixture.now.Add(3*time.Minute))
	postTerminal := nodeExecutionTestEvent(t, fixture, lease, 3, "progress")
	if err := fixture.broker.AcceptEvent(fixture.connection, mustNodeExecutionJSON(t, postTerminal), fixture.now.Add(4*time.Minute)); err == nil {
		t.Fatal("post-terminal event was accepted")
	}
}

func TestNodeExecutionCancellationAndCleanupOutcomes(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	lease := dispatchNodeExecutionFixture(t, fixture)
	base := nodeExecutionTestCancellation(t, fixture, lease)
	mutations := []struct {
		name   string
		at     time.Time
		mutate func(*NodeExecutionCancellation)
	}{
		{name: "wrong machine", at: fixture.now.Add(time.Minute), mutate: func(value *NodeExecutionCancellation) { value.MachineID = "machine-substitute-001" }},
		{name: "wrong capability", at: fixture.now.Add(time.Minute), mutate: func(value *NodeExecutionCancellation) {
			value.CapabilitySnapshotID = strings.Repeat("a", 64)
			value.CapabilitySnapshotID = "sha256:" + value.CapabilitySnapshotID
		}},
		{name: "wrong operation", at: fixture.now.Add(time.Minute), mutate: func(value *NodeExecutionCancellation) { value.OperationID = "operation-substitute-001" }},
		{name: "wrong lease", at: fixture.now.Add(time.Minute), mutate: func(value *NodeExecutionCancellation) { value.LeaseID = "lease-substitute-000001" }},
		{name: "wrong attempt", at: fixture.now.Add(time.Minute), mutate: func(value *NodeExecutionCancellation) { value.Attempt = 2 }},
		{name: "wrong cancellation", at: fixture.now.Add(time.Minute), mutate: func(value *NodeExecutionCancellation) { value.CancellationID = "cancellation-substitute-001" }},
		{name: "expired", at: fixture.now.Add(30 * time.Minute), mutate: func(*NodeExecutionCancellation) {}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			value := base
			mutation.mutate(&value)
			value = mustFinalizeNodeExecutionCancellation(t, value)
			if _, err := fixture.broker.RequestCancellation(fixture.connection, mustNodeExecutionJSON(t, value), mutation.at); err == nil {
				t.Fatalf("expected %s cancellation rejection", mutation.name)
			}
		})
	}

	ack, err := fixture.broker.RequestCancellation(fixture.connection, mustNodeExecutionJSON(t, base), fixture.now.Add(time.Minute))
	if err != nil {
		t.Fatalf("valid cancellation: %v", err)
	}
	if ack.CancellationID != base.CancellationID || ack.AckFingerprint == "" {
		t.Fatalf("invalid cancellation acknowledgement: %#v", ack)
	}
	beforeReplay := nodeExecutionStateArtifacts(t, fixture.root)
	if _, err := fixture.broker.RequestCancellation(fixture.connection, mustNodeExecutionJSON(t, base), fixture.now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(beforeReplay, got) {
		t.Fatal("exact cancellation replay published new state")
	}

	invalid := nodeExecutionTestReceiptBase(t, fixture, lease)
	invalid.Result = "cancelled"
	invalid.CancellationID = base.CancellationID
	invalid.CancellationAcknowledged = true
	invalid.Cleanup = NodeExecutionCleanupOutcome{Status: "succeeded"}
	if _, err := FinalizeNodeExecutionReceipt(invalid); err == nil {
		t.Fatal("cleanup success without evidence was accepted")
	}
	invalid.Cleanup = NodeExecutionCleanupOutcome{Status: "failed", EvidenceDigest: nodeExecutionTestDigest("cleanup-failed")}
	if _, err := FinalizeNodeExecutionReceipt(invalid); err == nil {
		t.Fatal("cleanup failure was collapsed into successful cancellation")
	}
	invalid.Result = "degraded"
	receipt, err := FinalizeNodeExecutionReceipt(invalid)
	if err != nil {
		t.Fatal(err)
	}
	mustAcceptNodeExecutionReceipt(t, fixture.broker, fixture.connection, receipt, fixture.now.Add(2*time.Minute))
}

func TestNodeExecutionReceiptReplayAndArtifactManifestBindings(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	lease := dispatchNodeExecutionFixture(t, fixture)
	manifest, err := NewNodeExecutionArtifactManifest([]NodeExecutionArtifactReference{{Name: "result.json", MediaType: "application/json", Digest: nodeExecutionTestDigest("result"), Bytes: 6}})
	if err != nil {
		t.Fatal(err)
	}
	receipt := nodeExecutionTestReceiptBase(t, fixture, lease)
	receipt.Artifacts = manifest
	receipt, err = FinalizeNodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	mustAcceptNodeExecutionReceipt(t, fixture.broker, fixture.connection, receipt, fixture.now.Add(time.Minute))
	before := nodeExecutionStateArtifacts(t, fixture.root)
	mustAcceptNodeExecutionReceipt(t, fixture.broker, fixture.connection, receipt, fixture.now.Add(time.Minute))
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(before, got) {
		t.Fatal("exact receipt replay published new state")
	}
	changed := receipt
	changed.Result = "failed"
	changed, err = FinalizeNodeExecutionReceipt(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.broker.AcceptReceipt(fixture.connection, mustNodeExecutionJSON(t, changed), fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("conflicting receipt replay was accepted")
	}
	tampered := receipt
	tampered.Artifacts.Entries[0].Digest = nodeExecutionTestDigest("tampered")
	tampered.ReceiptFingerprint, _ = nodeExecutionReceiptFingerprint(tampered)
	if _, err := fixture.broker.AcceptReceipt(fixture.connection, mustNodeExecutionJSON(t, tampered), fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("tampered artifact manifest binding was accepted")
	}
	badManifest := manifest
	badManifest.Entries[0].Name = "C:/remote/result.json"
	badManifest.ManifestFingerprint, _ = nodeExecutionManifestFingerprint(badManifest)
	if err := validateNodeExecutionManifest(badManifest); err == nil {
		t.Fatal("remote path was treated as a transferable artifact")
	}
}

func TestNodeExecutionRecoveryFailsClosedWithoutOverwrite(t *testing.T) {
	mutations := []struct {
		name      string
		firstFile bool
		mutate    func(t *testing.T, raw []byte) []byte
	}{
		{name: "malformed json", mutate: func(_ *testing.T, _ []byte) []byte { return []byte("{") }},
		{name: "unknown field", mutate: func(t *testing.T, raw []byte) []byte {
			return mutateNodeExecutionStateJSON(t, raw, func(state map[string]any) { state["unexpected"] = true })
		}},
		{name: "missing required", mutate: func(t *testing.T, raw []byte) []byte {
			return mutateNodeExecutionStateJSON(t, raw, func(state map[string]any) { delete(state, "schema") })
		}},
		{name: "invalid timestamp", mutate: func(t *testing.T, raw []byte) []byte {
			return mutateNodeExecutionStateJSON(t, raw, func(state map[string]any) { state["machine"].(map[string]any)["enrolled_at"] = "yesterday" })
		}},
		{name: "invalid identifier", mutate: func(t *testing.T, raw []byte) []byte {
			return mutateNodeExecutionStateJSON(t, raw, func(state map[string]any) { state["machine"].(map[string]any)["machine_id"] = "lease-substitute-001" })
		}},
		{name: "tampered earlier artifact", firstFile: true, mutate: func(t *testing.T, raw []byte) []byte {
			return mutateNodeExecutionStateJSON(t, raw, func(state map[string]any) { state["machine"].(map[string]any)["enrolled_at"] = "2026-01-01T00:00:00Z" })
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			fixture := newNodeExecutionTestFixture(t)
			dispatchNodeExecutionFixture(t, fixture)
			artifacts := nodeExecutionStateArtifacts(t, fixture.root)
			target := artifacts[len(artifacts)-1]
			if mutation.firstFile {
				target = artifacts[0]
			}
			path := filepath.Join(fixture.root, target)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tampered := mutation.mutate(t, raw)
			if err := os.WriteFile(path, tampered, 0o644); err != nil {
				t.Fatal(err)
			}
			before := append([]byte{}, tampered...)
			beforeArtifacts := nodeExecutionStateArtifacts(t, fixture.root)
			if _, err := NewNodeExecutionFakeBroker(fixture.root, fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, nil); err == nil {
				t.Fatal("tampered durable state was accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !nodeExecutionStringSlicesEqual(beforeArtifacts, nodeExecutionStateArtifacts(t, fixture.root)) {
				t.Fatal("failed recovery overwrote valid or tampered evidence")
			}
		})
	}
}

func TestNodeExecutionAtomicPersistencePublishesNoPartialState(t *testing.T) {
	fixture := newNodeExecutionTestFixture(t)
	before := nodeExecutionStateArtifacts(t, fixture.root)
	beforeState := cloneNodeExecutionState(fixture.broker.state)
	refreshed, err := NewNodeExecutionCapabilitySnapshot(fixture.machine.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "linux", Runtime: "docker", Toolchains: []string{"go1.25"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, fixture.now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	originalWriter := nodeExecutionWriteAtomic
	nodeExecutionWriteAtomic = func(string, any) error { return errors.New("injected atomic write failure") }
	t.Cleanup(func() { nodeExecutionWriteAtomic = originalWriter })
	if err := fixture.broker.RegisterCapabilitySnapshot(refreshed); err == nil {
		t.Fatal("injected persistence failure was ignored")
	}
	if !nodeExecutionEqual(beforeState, fixture.broker.state) {
		t.Fatal("in-memory state advanced after failed persistence")
	}
	if got := nodeExecutionStateArtifacts(t, fixture.root); !nodeExecutionStringSlicesEqual(before, got) {
		t.Fatalf("failed persistence published partial state: before=%v after=%v", before, got)
	}
	entries, err := os.ReadDir(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".promotion-candidate-") {
			t.Fatalf("temporary atomic artifact remained: %s", entry.Name())
		}
	}
}

func newNodeExecutionTestFixture(t *testing.T) *nodeExecutionTestFixture {
	t.Helper()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	machine := NodeExecutionMachineIdentity{Schema: NodeExecutionMachineIdentitySchema, MachineID: "machine-office-001", EnrolledAt: nodeExecutionTime(now.Add(-time.Hour))}
	capability, err := NewNodeExecutionCapabilitySnapshot(machine.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "windows", Runtime: "host", Toolchains: []string{"go1.25"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	request, err := FinalizeNodeExecutionRequest(NodeExecutionRequest{
		OperationID: "operation-validate-001", GraphRunID: "graph-validation-001", RunID: "run-validation-001", TaskID: "task-validation-001",
		SourceRevision: strings.Repeat("a", 40), Workflow: NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "validate.readonly"},
		CapabilitySnapshotID: capability.SnapshotID, Inputs: []NodeExecutionInput{}, Artifacts: []NodeExecutionArtifactReference{}, RequestedAt: nodeExecutionTime(now.Add(-time.Second)),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	root := t.TempDir()
	broker, err := NewNodeExecutionFakeBroker(root, machine, []NodeExecutionCapabilitySnapshot{capability}, func(NodeExecutionRequest, NodeExecutionTaskLease) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	connection := "connection-session-001"
	if err := broker.Connect(machine.MachineID, connection); err != nil {
		t.Fatal(err)
	}
	return &nodeExecutionTestFixture{root: root, now: now, machine: machine, capability: capability, request: request, requestRaw: mustNodeExecutionJSON(t, request), broker: broker, connection: connection, calls: &calls}
}

func dispatchNodeExecutionFixture(t *testing.T, fixture *nodeExecutionTestFixture) NodeExecutionTaskLease {
	t.Helper()
	lease, err := fixture.broker.Dispatch(fixture.connection, fixture.requestRaw, fixture.now, 30*time.Minute)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	return lease
}

func nodeExecutionTestEvent(t *testing.T, fixture *nodeExecutionTestFixture, lease NodeExecutionTaskLease, sequence int64, status string) NodeExecutionEventEnvelope {
	t.Helper()
	inner := mustNodeExecutionJSON(t, map[string]any{
		"schema": "dockpipe.operation_event.v1", "type": "operation_result", "ts": nodeExecutionTime(fixture.now.Add(time.Duration(sequence) * time.Minute)),
		"unit": "validation.readonly", "status": status,
	})
	return mustFinalizeNodeExecutionEvent(t, NodeExecutionEventEnvelope{
		OperationID: fixture.request.OperationID, GraphRunID: fixture.request.GraphRunID, RunID: fixture.request.RunID, TaskID: fixture.request.TaskID,
		MachineID: lease.MachineID, CapabilitySnapshotID: lease.CapabilitySnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt,
		Sequence: sequence, RecordedAt: nodeExecutionTime(fixture.now.Add(time.Duration(sequence) * time.Minute)), OutputReferences: []NodeExecutionArtifactReference{}, Event: inner,
	})
}

func nodeExecutionTestCancellation(t *testing.T, fixture *nodeExecutionTestFixture, lease NodeExecutionTaskLease) NodeExecutionCancellation {
	t.Helper()
	return mustFinalizeNodeExecutionCancellation(t, NodeExecutionCancellation{
		CancellationID: lease.CancellationID, OperationID: lease.OperationID, MachineID: lease.MachineID,
		CapabilitySnapshotID: lease.CapabilitySnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt,
		RequestedAt: nodeExecutionTime(fixture.now.Add(time.Minute)),
	})
}

func nodeExecutionTestReceipt(t *testing.T, fixture *nodeExecutionTestFixture, lease NodeExecutionTaskLease, result, cleanupStatus, cleanupDigest string) NodeExecutionReceipt {
	t.Helper()
	receipt := nodeExecutionTestReceiptBase(t, fixture, lease)
	receipt.Result = result
	receipt.Cleanup = NodeExecutionCleanupOutcome{Status: cleanupStatus, EvidenceDigest: cleanupDigest}
	receipt, err := FinalizeNodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func nodeExecutionTestReceiptBase(t *testing.T, fixture *nodeExecutionTestFixture, lease NodeExecutionTaskLease) NodeExecutionReceipt {
	t.Helper()
	manifest, err := NewNodeExecutionArtifactManifest([]NodeExecutionArtifactReference{})
	if err != nil {
		t.Fatal(err)
	}
	return NodeExecutionReceipt{
		ReceiptID: nodeExecutionReceiptID(fixture.request.OperationID), OperationID: fixture.request.OperationID,
		MachineID: lease.MachineID, CapabilitySnapshotID: lease.CapabilitySnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt,
		RequestFingerprint: fixture.request.RequestFingerprint, LocalRunID: "local-run-validation-001",
		FinalCursor: nodeExecutionCursor(int64(len(fixture.broker.state.Operations[fixture.request.OperationID].Events))),
		Result:      "succeeded", Artifacts: manifest, Cleanup: NodeExecutionCleanupOutcome{Status: "not_required"}, CompletedAt: nodeExecutionTime(fixture.now.Add(3 * time.Minute)),
	}
}

func mustFinalizeNodeExecutionEvent(t *testing.T, event NodeExecutionEventEnvelope) NodeExecutionEventEnvelope {
	t.Helper()
	value, err := FinalizeNodeExecutionEvent(event)
	if err != nil {
		t.Fatalf("finalize event: %v", err)
	}
	return value
}

func mustFinalizeNodeExecutionCancellation(t *testing.T, value NodeExecutionCancellation) NodeExecutionCancellation {
	t.Helper()
	finalized, err := FinalizeNodeExecutionCancellation(value)
	if err != nil {
		t.Fatalf("finalize cancellation: %v", err)
	}
	return finalized
}

func mustAcceptNodeExecutionEvent(t *testing.T, broker *NodeExecutionFakeBroker, connection string, event NodeExecutionEventEnvelope, at time.Time) {
	t.Helper()
	if err := broker.AcceptEvent(connection, mustNodeExecutionJSON(t, event), at); err != nil {
		t.Fatalf("accept event: %v", err)
	}
}

func mustAcceptNodeExecutionReceipt(t *testing.T, broker *NodeExecutionFakeBroker, connection string, receipt NodeExecutionReceipt, at time.Time) {
	t.Helper()
	if _, err := broker.AcceptReceipt(connection, mustNodeExecutionJSON(t, receipt), at); err != nil {
		t.Fatalf("accept receipt: %v", err)
	}
}

func mustNodeExecutionJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func nodeExecutionStateArtifacts(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := []string{}
	for _, entry := range entries {
		if nodeExecutionStateName.MatchString(entry.Name()) {
			result = append(result, entry.Name())
		}
	}
	return result
}

func mutateNodeExecutionStateJSON(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	mutate(state)
	result, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(result, '\n')
}

func nodeExecutionTestDigest(value string) string {
	fingerprint, _ := nodeExecutionFingerprintValue(value)
	return fingerprint
}

func nodeExecutionStringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
