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

type nodeConnectorSessionFixture struct {
	execution  *nodeExecutionTestFixture
	root       string
	enrollment NodeConnectorEnrollment
	fake       *NodeConnectorSessionFake
	calls      *int
}

type nodeConnectorSessionDispatchFixture struct {
	session         *nodeConnectorSessionFixture
	negotiation     NodeConnectorSessionNegotiation
	lease           NodeExecutionTaskLease
	connector       *NodeValidationConnector
	validationCalls *int
}

func TestNodeConnectorSessionDispatchesAcceptedValidationOnceAcrossReplayReconnectAndRestart(t *testing.T) {
	fixture := newNodeConnectorSessionDispatchFixture(t)
	receipt, err := fixture.session.fake.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, fixture.session.execution.request, fixture.lease, fixture.session.execution.now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if *fixture.validationCalls != 1 || *fixture.session.execution.calls != 1 {
		t.Fatalf("accepted proof invocation counts are invalid: validation=%d broker_executor=%d", *fixture.validationCalls, *fixture.session.execution.calls)
	}
	if receipt.RequestFingerprint != fixture.session.execution.request.RequestFingerprint || receipt.LeaseID != fixture.lease.LeaseID || receipt.CapabilitySnapshotID != fixture.negotiation.CapabilitySnapshotID || receipt.MachineID != fixture.negotiation.MachineID {
		t.Fatalf("connector receipt lost the accepted request or lease binding: %#v", receipt)
	}
	identities := []string{
		fixture.negotiation.MachineID,
		fixture.negotiation.CapabilitySnapshotID,
		fixture.lease.LeaseID,
		receipt.ReceiptID,
		fixture.negotiation.ConnectionID,
		fixture.negotiation.EnrollmentID,
		fixture.negotiation.CredentialID,
		fixture.negotiation.SessionID,
	}
	unique := map[string]bool{}
	for _, identity := range identities {
		unique[identity] = true
	}
	if len(unique) != len(identities) {
		t.Fatalf("machine, capability, lease, receipt, connection, enrollment, credential, and session identities are not distinct: %#v", identities)
	}

	terminalBroker := nodeConnectorStateBytes(t, fixture.session.execution.root)
	terminalSession := nodeConnectorSessionStateBytes(t, fixture.session.root)
	replayed, err := fixture.session.fake.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, fixture.session.execution.request, fixture.lease, fixture.session.execution.now.Add(11*time.Second))
	if err != nil || replayed.ReceiptFingerprint != receipt.ReceiptFingerprint || *fixture.validationCalls != 1 || !nodeConnectorStateBytesEqual(terminalBroker, nodeConnectorStateBytes(t, fixture.session.execution.root)) || !nodeConnectorSessionStateBytesEqual(terminalSession, nodeConnectorSessionStateBytes(t, fixture.session.root)) {
		t.Fatalf("exact replay invoked validation or published duplicate output: receipt=%#v err=%v calls=%d", replayed, err, *fixture.validationCalls)
	}
	changedRequest := fixture.session.execution.request
	changedRequest.TaskID = "task-validation-changed-001"
	changedRequest, err = FinalizeNodeExecutionRequest(changedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.session.fake.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, changedRequest, fixture.lease, fixture.session.execution.now.Add(12*time.Second)); err == nil {
		t.Fatal("changed terminal request replay was accepted")
	}
	if *fixture.validationCalls != 1 || !nodeConnectorStateBytesEqual(terminalBroker, nodeConnectorStateBytes(t, fixture.session.execution.root)) || !nodeConnectorSessionStateBytesEqual(terminalSession, nodeConnectorSessionStateBytes(t, fixture.session.root)) {
		t.Fatal("changed replay invoked validation or published partial state")
	}

	disconnected := nodeConnectorSessionEvidence(t, fixture.session, 4, "evidence-dispatch-disconnected-001", "replay-dispatch-disconnected-001", "presence", "disconnected", fixture.negotiation.SessionID, fixture.negotiation.ConnectionID, fixture.negotiation.CredentialID, "", fixture.negotiation.CapabilitySnapshotID, fixture.session.execution.now.Add(13*time.Second))
	mustRecordNodeConnectorEvidence(t, fixture.session.fake, disconnected)
	fixture.session.execution.broker.Disconnect(fixture.negotiation.ConnectionID)
	disconnectedBroker := nodeConnectorStateBytes(t, fixture.session.execution.root)
	disconnectedSession := nodeConnectorSessionStateBytes(t, fixture.session.root)
	if _, err := fixture.session.fake.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, fixture.session.execution.request, fixture.lease, fixture.session.execution.now.Add(14*time.Second)); err == nil {
		t.Fatal("disconnected session transported validation")
	}
	if *fixture.validationCalls != 1 || !nodeConnectorStateBytesEqual(disconnectedBroker, nodeConnectorStateBytes(t, fixture.session.execution.root)) || !nodeConnectorSessionStateBytesEqual(disconnectedSession, nodeConnectorSessionStateBytes(t, fixture.session.root)) {
		t.Fatal("disconnect rejection invoked validation or published partial state")
	}

	restartedExecutions := 0
	reopenedBroker, err := NewNodeExecutionFakeBroker(fixture.session.execution.root, fixture.session.execution.machine, []NodeExecutionCapabilitySnapshot{fixture.session.execution.capability}, func(NodeExecutionRequest, NodeExecutionTaskLease) { restartedExecutions++ })
	if err != nil {
		t.Fatal(err)
	}
	reopenedSession, err := NewNodeConnectorSessionFake(fixture.session.root, reopenedBroker, fixture.session.enrollment, nodeConnectorSessionTransport(fixture.session.calls, false))
	if err != nil {
		t.Fatal(err)
	}
	reconnect := nodeConnectorSessionHello(t, fixture.session, 5, "negotiation-dispatch-restart-001", "replay-dispatch-restart-001", "connection-dispatch-restart-001", fixture.negotiation.SessionID, fixture.negotiation.CredentialID, fixture.negotiation.CapabilitySnapshotID, fixture.session.execution.now.Add(15*time.Second))
	restartedNegotiation, err := reopenedSession.Negotiate(mustNodeExecutionJSON(t, reconnect))
	if err != nil {
		t.Fatal(err)
	}
	mustRecordNodeConnectorEvidence(t, reopenedSession, nodeConnectorSessionEvidence(t, fixture.session, 6, "evidence-dispatch-reconnected-001", "replay-dispatch-reconnected-001", "presence", "connected", restartedNegotiation.SessionID, restartedNegotiation.ConnectionID, restartedNegotiation.CredentialID, "", restartedNegotiation.CapabilitySnapshotID, fixture.session.execution.now.Add(16*time.Second)))
	mustRecordNodeConnectorEvidence(t, reopenedSession, nodeConnectorSessionEvidence(t, fixture.session, 7, "evidence-dispatch-restart-health-001", "replay-dispatch-restart-health-001", "health", "healthy", restartedNegotiation.SessionID, restartedNegotiation.ConnectionID, restartedNegotiation.CredentialID, "", restartedNegotiation.CapabilitySnapshotID, fixture.session.execution.now.Add(17*time.Second)))
	if err := reopenedBroker.Connect(restartedNegotiation.MachineID, restartedNegotiation.ConnectionID); err != nil {
		t.Fatal(err)
	}
	restartedValidationCalls := 0
	restartedConnector, err := NewNodeValidationConnector(fixture.session.execution.request.Workflow, fixture.session.execution.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		restartedValidationCalls++
		return nodeConnectorTestEvidence(t, fixture.session.execution), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	restartBroker := nodeConnectorStateBytes(t, fixture.session.execution.root)
	restartSession := nodeConnectorSessionStateBytes(t, fixture.session.root)
	expiresAt, err := parseNodeExecutionTime(fixture.lease.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	for _, replayAt := range []time.Time{fixture.session.execution.now.Add(18 * time.Second), expiresAt.Add(time.Second)} {
		resumed, err := reopenedSession.DispatchAcceptedValidation(restartedConnector, restartedNegotiation, fixture.session.execution.request, fixture.lease, replayAt)
		if err != nil || resumed.ReceiptFingerprint != receipt.ReceiptFingerprint {
			t.Fatalf("restart resume lost the accepted receipt: receipt=%#v err=%v", resumed, err)
		}
	}
	if restartedValidationCalls != 0 || restartedExecutions != 0 || *fixture.validationCalls != 1 || !nodeConnectorStateBytesEqual(restartBroker, nodeConnectorStateBytes(t, fixture.session.execution.root)) || !nodeConnectorSessionStateBytesEqual(restartSession, nodeConnectorSessionStateBytes(t, fixture.session.root)) {
		t.Fatalf("restart replay duplicated validation, execution, or durable output: validation=%d execution=%d", restartedValidationCalls, restartedExecutions)
	}
}

func TestNodeConnectorSessionDispatchRejectsChangedBindingsExpiryStaleStateRevocationAndTamper(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time)
	}{
		{name: "machine identity", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			value := f.negotiation
			value.MachineID = "machine-conflict-001"
			return mustFinalizeNodeConnectorSessionNegotiation(t, value), f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "connection identity", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			value := f.negotiation
			value.ConnectionID = "connection-conflict-001"
			return mustFinalizeNodeConnectorSessionNegotiation(t, value), f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "enrollment identity", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			value := f.negotiation
			value.EnrollmentID = "enrollment-conflict-001"
			return mustFinalizeNodeConnectorSessionNegotiation(t, value), f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "credential identity", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			value := f.negotiation
			value.CredentialID = "cred-conflict-identity-001"
			return mustFinalizeNodeConnectorSessionNegotiation(t, value), f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "session identity", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			value := f.negotiation
			value.SessionID = "session-conflict-001"
			return mustFinalizeNodeConnectorSessionNegotiation(t, value), f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "capability substitution", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			capability, err := NewNodeExecutionCapabilitySnapshot(f.negotiation.MachineID, NodeExecutionObservedCapabilities{HostOS: "windows", Runtime: "docker", Toolchains: []string{"go1.26"}}, NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, f.session.execution.now.Add(4*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if err := f.session.execution.broker.RegisterCapabilitySnapshot(capability); err != nil {
				t.Fatal(err)
			}
			request := f.session.execution.request
			request.CapabilitySnapshotID = capability.SnapshotID
			request, err = FinalizeNodeExecutionRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			lease := f.lease
			lease.CapabilitySnapshotID = capability.SnapshotID
			return f.negotiation, request, lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "lease identity", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			lease := f.lease
			lease.LeaseID = "lease-conflict-000000000001"
			return f.negotiation, f.session.execution.request, lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "lease expiry", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			expiresAt, err := parseNodeExecutionTime(f.lease.ExpiresAt)
			if err != nil {
				t.Fatal(err)
			}
			return f.negotiation, f.session.execution.request, f.lease, expiresAt
		}},
		{name: "request substitution", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			request := f.session.execution.request
			request.RunID = "run-substituted-001"
			request, err := FinalizeNodeExecutionRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			return f.negotiation, request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "revoked credential", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			revocation := mustFinalizeNodeConnectorCredentialEvidence(t, NodeConnectorCredentialEvidence{
				Sequence: 4, EvidenceID: "evidence-dispatch-revocation-001", ReplayIdentity: "replay-dispatch-revocation-001", EnrollmentID: f.negotiation.EnrollmentID, MachineID: f.negotiation.MachineID, Action: "revoke", CredentialID: f.negotiation.CredentialID, ObservedAt: nodeExecutionTime(f.session.execution.now.Add(4 * time.Second)),
			})
			if err := f.session.fake.RecordCredential(mustNodeExecutionJSON(t, revocation)); err != nil {
				t.Fatal(err)
			}
			return f.negotiation, f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
		{name: "stale in-memory session state", mutate: func(f *nodeConnectorSessionDispatchFixture) (NodeConnectorSessionNegotiation, NodeExecutionRequest, NodeExecutionTaskLease, time.Time) {
			reopened, err := NewNodeConnectorSessionFake(f.session.root, f.session.execution.broker, f.session.enrollment, nodeConnectorSessionTransport(f.session.calls, false))
			if err != nil {
				t.Fatal(err)
			}
			mustRecordNodeConnectorEvidence(t, reopened, nodeConnectorSessionEvidence(t, f.session, 4, "evidence-dispatch-external-disconnect-001", "replay-dispatch-external-disconnect-001", "presence", "disconnected", f.negotiation.SessionID, f.negotiation.ConnectionID, f.negotiation.CredentialID, "", f.negotiation.CapabilitySnapshotID, f.session.execution.now.Add(4*time.Second)))
			return f.negotiation, f.session.execution.request, f.lease, f.session.execution.now.Add(10 * time.Second)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newNodeConnectorSessionDispatchFixture(t)
			negotiation, request, lease, at := test.mutate(fixture)
			assertNodeConnectorSessionDispatchRejected(t, fixture, negotiation, request, lease, at)
		})
	}

	t.Run("missing accepted session state", func(t *testing.T) {
		session := newNodeConnectorSessionFixture(t, nil)
		lease := dispatchNodeExecutionFixture(t, session.execution)
		calls := 0
		connector, err := NewNodeValidationConnector(session.execution.request.Workflow, session.execution.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
			calls++
			return nodeConnectorTestEvidence(t, session.execution), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		fixture := &nodeConnectorSessionDispatchFixture{session: session, lease: lease, connector: connector, validationCalls: &calls}
		assertNodeConnectorSessionDispatchRejected(t, fixture, NodeConnectorSessionNegotiation{}, session.execution.request, lease, session.execution.now.Add(10*time.Second))
	})

	t.Run("healthy presence without accepted request and lease", func(t *testing.T) {
		session := newNodeConnectorSessionFixture(t, nil)
		hello := nodeConnectorSessionHello(t, session, 1, "negotiation-presence-only-001", "replay-presence-only-001", "connection-presence-only-001", "", session.enrollment.InitialCredentialID, session.execution.capability.SnapshotID, session.execution.now.Add(time.Second))
		negotiation, err := session.fake.Negotiate(mustNodeExecutionJSON(t, hello))
		if err != nil {
			t.Fatal(err)
		}
		mustRecordNodeConnectorEvidence(t, session.fake, nodeConnectorSessionEvidence(t, session, 2, "evidence-presence-only-connected-001", "replay-presence-only-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, session.execution.now.Add(2*time.Second)))
		mustRecordNodeConnectorEvidence(t, session.fake, nodeConnectorSessionEvidence(t, session, 3, "evidence-presence-only-health-001", "replay-presence-only-health-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, session.execution.now.Add(3*time.Second)))
		if err := session.execution.broker.Connect(negotiation.MachineID, negotiation.ConnectionID); err != nil {
			t.Fatal(err)
		}
		calls := 0
		connector, err := NewNodeValidationConnector(session.execution.request.Workflow, session.execution.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
			calls++
			return nodeConnectorTestEvidence(t, session.execution), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		lease := newNodeExecutionLease(session.execution.request, negotiation.MachineID, session.execution.now, session.execution.now.Add(30*time.Minute))
		brokerBefore := nodeConnectorStateBytes(t, session.execution.root)
		sessionBefore := nodeConnectorSessionStateBytes(t, session.root)
		if _, err := session.fake.DispatchAcceptedValidation(connector, negotiation, session.execution.request, lease, session.execution.now.Add(10*time.Second)); err == nil {
			t.Fatal("healthy presence initiated an unaccepted handoff")
		}
		if calls != 0 || *session.execution.calls != 0 || len(session.execution.broker.state.Operations) != 0 || !nodeConnectorStateBytesEqual(brokerBefore, nodeConnectorStateBytes(t, session.execution.root)) || !nodeConnectorSessionStateBytesEqual(sessionBefore, nodeConnectorSessionStateBytes(t, session.root)) {
			t.Fatal("session evidence created authority, invoked execution, or published state")
		}
	})

	for _, target := range []string{"session", "broker"} {
		t.Run(target+" durable tamper", func(t *testing.T) {
			fixture := newNodeConnectorSessionDispatchFixture(t)
			var path string
			var oldValue, newValue []byte
			if target == "session" {
				artifacts := nodeConnectorSessionStateArtifacts(t, fixture.session.root)
				path = filepath.Join(fixture.session.root, artifacts[len(artifacts)-1])
				oldValue, newValue = []byte(fixture.negotiation.ConnectionID), []byte("connection-tampered-001")
			} else {
				artifacts := nodeExecutionStateArtifacts(t, fixture.session.execution.root)
				path = filepath.Join(fixture.session.execution.root, artifacts[len(artifacts)-1])
				oldValue, newValue = []byte(fixture.session.execution.request.TaskID), []byte("task-tampered-0001")
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tampered := bytes.Replace(raw, oldValue, newValue, 1)
			if bytes.Equal(raw, tampered) {
				t.Fatal("tamper fixture did not change durable state")
			}
			if err := os.WriteFile(path, tampered, 0o644); err != nil {
				t.Fatal(err)
			}
			assertNodeConnectorSessionDispatchRejected(t, fixture, fixture.negotiation, fixture.session.execution.request, fixture.lease, fixture.session.execution.now.Add(10*time.Second))
		})
	}
}

func TestNodeConnectorSessionEnrollmentRotationPresenceHealthRefreshReconnectAndRestart(t *testing.T) {
	fixture := newNodeConnectorSessionFixture(t, nil)
	hello := nodeConnectorSessionHello(t, fixture, 1, "negotiation-initial-001", "replay-negotiation-initial-001", "connection-transport-001", "", "cred-fixture-initial-001", fixture.execution.capability.SnapshotID, fixture.execution.now.Add(time.Second))
	negotiation, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, hello))
	if err != nil {
		t.Fatal(err)
	}
	if negotiation.SessionID == fixture.enrollment.MachineID || negotiation.SessionID == hello.ConnectionID || negotiation.SessionID == fixture.enrollment.EnrollmentID || negotiation.SessionID == hello.CredentialID {
		t.Fatal("machine, enrollment, credential, connection, and session identities were not kept distinct")
	}
	if negotiation.RestartNegotiated || !nodeExecutionEqual(negotiation.Authority, NodeConnectorSessionAuthority{}) {
		t.Fatalf("initial negotiation granted authority or claimed restart: %#v", negotiation)
	}
	mustRecordNodeConnectorEvidence(t, fixture.fake, nodeConnectorSessionEvidence(t, fixture, 2, "evidence-presence-connected-001", "replay-presence-connected-001", "presence", "connected", negotiation.SessionID, hello.ConnectionID, hello.CredentialID, "", hello.CapabilitySnapshotID, fixture.execution.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.fake, nodeConnectorSessionEvidence(t, fixture, 3, "evidence-health-healthy-001", "replay-health-healthy-001", "health", "healthy", negotiation.SessionID, hello.ConnectionID, hello.CredentialID, "", hello.CapabilitySnapshotID, fixture.execution.now.Add(3*time.Second)))

	refreshed, err := NewNodeExecutionCapabilitySnapshot(fixture.enrollment.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "windows", Runtime: "host", Toolchains: []string{"go1.26"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, fixture.execution.now.Add(3500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.execution.broker.RegisterCapabilitySnapshot(refreshed); err != nil {
		t.Fatal(err)
	}
	mustRecordNodeConnectorEvidence(t, fixture.fake, nodeConnectorSessionEvidence(t, fixture, 4, "evidence-capability-refresh-001", "replay-capability-refresh-001", "capability_refresh", "refreshed", negotiation.SessionID, hello.ConnectionID, hello.CredentialID, hello.CapabilitySnapshotID, refreshed.SnapshotID, fixture.execution.now.Add(4*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.fake, nodeConnectorSessionEvidence(t, fixture, 5, "evidence-presence-disconnected-001", "replay-presence-disconnected-001", "presence", "disconnected", negotiation.SessionID, hello.ConnectionID, hello.CredentialID, "", refreshed.SnapshotID, fixture.execution.now.Add(5*time.Second)))

	rotation := mustFinalizeNodeConnectorCredentialEvidence(t, NodeConnectorCredentialEvidence{
		Sequence: 6, EvidenceID: "evidence-rotation-001", ReplayIdentity: "replay-rotation-001",
		EnrollmentID: fixture.enrollment.EnrollmentID, MachineID: fixture.enrollment.MachineID, Action: "rotate",
		PreviousCredentialID: hello.CredentialID, CredentialID: "cred-fixture-rotated-001", ObservedAt: nodeExecutionTime(fixture.execution.now.Add(6 * time.Second)),
	})
	if err := fixture.fake.RecordCredential(mustNodeExecutionJSON(t, rotation)); err != nil {
		t.Fatal(err)
	}

	// Reopen before reconnect to prove restart negotiation uses only durable
	// evidence and the injected transport.
	reopened, err := NewNodeConnectorSessionFake(fixture.root, fixture.execution.broker, fixture.enrollment, nodeConnectorSessionTransport(fixture.calls, false))
	if err != nil {
		t.Fatal(err)
	}
	reconnect := nodeConnectorSessionHello(t, fixture, 7, "negotiation-restart-001", "replay-negotiation-restart-001", "connection-transport-002", negotiation.SessionID, rotation.CredentialID, refreshed.SnapshotID, fixture.execution.now.Add(7*time.Second))
	restarted, err := reopened.Negotiate(mustNodeExecutionJSON(t, reconnect))
	if err != nil {
		t.Fatal(err)
	}
	if !restarted.RestartNegotiated || restarted.SessionID != negotiation.SessionID || restarted.ConnectionID == hello.ConnectionID {
		t.Fatalf("restart did not preserve session and replace only connection evidence: %#v", restarted)
	}
	if *fixture.calls != 2 {
		t.Fatalf("transport calls = %d, want 2", *fixture.calls)
	}

	terminalArtifacts := nodeConnectorSessionStateBytes(t, fixture.root)
	reopenedAgain, err := NewNodeConnectorSessionFake(fixture.root, fixture.execution.broker, fixture.enrollment, nodeConnectorSessionTransport(fixture.calls, false))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := reopenedAgain.Negotiate(mustNodeExecutionJSON(t, reconnect))
	if err != nil {
		t.Fatal(err)
	}
	if replayed.NegotiationFingerprint != restarted.NegotiationFingerprint || *fixture.calls != 2 || !nodeConnectorSessionStateBytesEqual(terminalArtifacts, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("exact restart replay invoked transport or published new durable state")
	}

	mustRecordNodeConnectorEvidence(t, reopenedAgain, nodeConnectorSessionEvidence(t, fixture, 8, "evidence-presence-connected-002", "replay-presence-connected-002", "presence", "connected", restarted.SessionID, reconnect.ConnectionID, reconnect.CredentialID, "", refreshed.SnapshotID, fixture.execution.now.Add(8*time.Second)))
	mustRecordNodeConnectorEvidence(t, reopenedAgain, nodeConnectorSessionEvidence(t, fixture, 9, "evidence-health-degraded-001", "replay-health-degraded-001", "health", "degraded", restarted.SessionID, reconnect.ConnectionID, reconnect.CredentialID, "", refreshed.SnapshotID, fixture.execution.now.Add(9*time.Second)))
	mustRecordNodeConnectorEvidence(t, reopenedAgain, nodeConnectorSessionEvidence(t, fixture, 10, "evidence-presence-disconnected-002", "replay-presence-disconnected-002", "presence", "disconnected", restarted.SessionID, reconnect.ConnectionID, reconnect.CredentialID, "", refreshed.SnapshotID, fixture.execution.now.Add(10*time.Second)))
	revocation := mustFinalizeNodeConnectorCredentialEvidence(t, NodeConnectorCredentialEvidence{
		Sequence: 11, EvidenceID: "evidence-revocation-001", ReplayIdentity: "replay-revocation-001",
		EnrollmentID: fixture.enrollment.EnrollmentID, MachineID: fixture.enrollment.MachineID, Action: "revoke",
		CredentialID: reconnect.CredentialID, ObservedAt: nodeExecutionTime(fixture.execution.now.Add(11 * time.Second)),
	})
	if err := reopenedAgain.RecordCredential(mustNodeExecutionJSON(t, revocation)); err != nil {
		t.Fatal(err)
	}
	beforeRevoked := nodeConnectorSessionStateBytes(t, fixture.root)
	revokedHello := nodeConnectorSessionHello(t, fixture, 12, "negotiation-revoked-001", "replay-negotiation-revoked-001", "connection-transport-003", restarted.SessionID, reconnect.CredentialID, refreshed.SnapshotID, fixture.execution.now.Add(12*time.Second))
	if _, err := reopenedAgain.Negotiate(mustNodeExecutionJSON(t, revokedHello)); err == nil {
		t.Fatal("revoked credential was accepted")
	}
	if _, err := reopenedAgain.Negotiate(mustNodeExecutionJSON(t, reconnect)); err == nil {
		t.Fatal("accepted negotiation replay bypassed later credential revocation")
	}
	if *fixture.calls != 2 || !nodeConnectorSessionStateBytesEqual(beforeRevoked, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("revoked credential invoked transport or published partial state")
	}

	derived, err := deriveNodeConnectorSessionState(reopenedAgain.state, fixture.execution.broker)
	if err != nil {
		t.Fatal(err)
	}
	session := derived.sessions[restarted.SessionID]
	if session.Present || session.Health != "" || session.CapabilitySnapshot != refreshed.SnapshotID || !derived.revoked[reconnect.CredentialID] {
		t.Fatalf("durable session evidence is incomplete: %#v revoked=%v", session, derived.revoked)
	}
	if len(fixture.execution.broker.state.Operations) != 0 || *fixture.execution.calls != 0 {
		t.Fatal("session presence or health granted a lease or executed broker work")
	}
}

func TestNodeConnectorSessionRejectsStaleEnrollmentDuplicatesCapabilitySubstitutionAndConflictingSession(t *testing.T) {
	fixture := newNodeConnectorSessionFixture(t, nil)
	hello := nodeConnectorSessionHello(t, fixture, 1, "negotiation-initial-001", "replay-negotiation-initial-001", "connection-transport-001", "", "cred-fixture-initial-001", fixture.execution.capability.SnapshotID, fixture.execution.now.Add(time.Second))
	accepted, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, hello))
	if err != nil {
		t.Fatal(err)
	}
	before := nodeConnectorSessionStateBytes(t, fixture.root)

	changed := hello
	changed.ConnectionID = "connection-transport-changed-001"
	changed = mustFinalizeNodeConnectorSessionHello(t, changed)
	if _, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, changed)); err == nil {
		t.Fatal("changed duplicate negotiation was accepted")
	}
	replayed := hello
	replayed.NegotiationID = "negotiation-replayed-001"
	replayed = mustFinalizeNodeConnectorSessionHello(t, replayed)
	if _, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, replayed)); err == nil {
		t.Fatal("replayed negotiation identity was accepted")
	}
	stale := hello
	stale.Sequence = 2
	stale.NegotiationID = "negotiation-stale-enroll-001"
	stale.ReplayIdentity = "replay-stale-enroll-001"
	stale.EnrollmentID = "enrollment-stale-001"
	stale = mustFinalizeNodeConnectorSessionHello(t, stale)
	if _, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, stale)); err == nil {
		t.Fatal("stale enrollment was accepted")
	}
	if *fixture.calls != 1 || !nodeConnectorSessionStateBytesEqual(before, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("rejected negotiation changed state or invoked transport")
	}

	mustRecordNodeConnectorEvidence(t, fixture.fake, nodeConnectorSessionEvidence(t, fixture, 2, "evidence-presence-connected-001", "replay-presence-connected-001", "presence", "connected", accepted.SessionID, hello.ConnectionID, hello.CredentialID, "", hello.CapabilitySnapshotID, fixture.execution.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.fake, nodeConnectorSessionEvidence(t, fixture, 3, "evidence-presence-disconnected-001", "replay-presence-disconnected-001", "presence", "disconnected", accepted.SessionID, hello.ConnectionID, hello.CredentialID, "", hello.CapabilitySnapshotID, fixture.execution.now.Add(3*time.Second)))
	refreshed, err := NewNodeExecutionCapabilitySnapshot(fixture.enrollment.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "windows", Runtime: "docker", Toolchains: []string{"go1.26"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, fixture.execution.now.Add(3500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.execution.broker.RegisterCapabilitySnapshot(refreshed); err != nil {
		t.Fatal(err)
	}
	before = nodeConnectorSessionStateBytes(t, fixture.root)
	substitute := nodeConnectorSessionHello(t, fixture, 4, "negotiation-capability-substitute-001", "replay-capability-substitute-001", "connection-transport-002", accepted.SessionID, hello.CredentialID, refreshed.SnapshotID, fixture.execution.now.Add(4*time.Second))
	if _, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, substitute)); err == nil {
		t.Fatal("capability substitution without refresh evidence was accepted")
	}
	if *fixture.calls != 1 || !nodeConnectorSessionStateBytesEqual(before, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("capability substitution published partial state")
	}

	conflicting, err := NewNodeConnectorSessionFake(fixture.root, fixture.execution.broker, fixture.enrollment, nodeConnectorSessionTransport(fixture.calls, true))
	if err != nil {
		t.Fatal(err)
	}
	reconnect := nodeConnectorSessionHello(t, fixture, 4, "negotiation-conflicting-session-001", "replay-conflicting-session-001", "connection-transport-003", accepted.SessionID, hello.CredentialID, hello.CapabilitySnapshotID, fixture.execution.now.Add(4*time.Second))
	if _, err := conflicting.Negotiate(mustNodeExecutionJSON(t, reconnect)); err == nil {
		t.Fatal("conflicting restart session identity was accepted")
	}
	if !nodeConnectorSessionStateBytesEqual(before, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("conflicting session identity published partial state")
	}
}

func TestNodeConnectorSessionEvidenceIsNonAuthoritativeAndStrict(t *testing.T) {
	fixture := newNodeConnectorSessionFixture(t, nil)
	hello := nodeConnectorSessionHello(t, fixture, 1, "negotiation-initial-001", "replay-negotiation-initial-001", "connection-transport-001", "", "cred-fixture-initial-001", fixture.execution.capability.SnapshotID, fixture.execution.now.Add(time.Second))
	negotiation, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, hello))
	if err != nil {
		t.Fatal(err)
	}
	before := nodeConnectorSessionStateBytes(t, fixture.root)
	health := nodeConnectorSessionEvidence(t, fixture, 2, "evidence-health-offline-001", "replay-health-offline-001", "health", "healthy", negotiation.SessionID, hello.ConnectionID, hello.CredentialID, "", hello.CapabilitySnapshotID, fixture.execution.now.Add(2*time.Second))
	if err := fixture.fake.RecordEvidence(mustNodeExecutionJSON(t, health)); err == nil {
		t.Fatal("health without presence was accepted")
	}

	presence := nodeConnectorSessionEvidence(t, fixture, 2, "evidence-presence-connected-001", "replay-presence-connected-001", "presence", "connected", negotiation.SessionID, hello.ConnectionID, hello.CredentialID, "", hello.CapabilitySnapshotID, fixture.execution.now.Add(2*time.Second))
	authorized := presence
	authorized.Authority.LeaseGranted = true
	if _, err := FinalizeNodeConnectorSessionEvidence(authorized); err == nil {
		t.Fatal("presence evidence granted lease authority")
	}
	unknown := mustNodeExecutionJSON(t, presence)
	unknown = bytes.Replace(unknown, []byte(`"schema":`), []byte(`"credential_material":"plain-value","schema":`), 1)
	if err := fixture.fake.RecordEvidence(unknown); err == nil {
		t.Fatal("unknown credential material was accepted")
	}
	if !nodeConnectorSessionStateBytesEqual(before, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("rejected evidence published partial state")
	}

	mustRecordNodeConnectorEvidence(t, fixture.fake, presence)
	accepted := nodeConnectorSessionStateBytes(t, fixture.root)
	changed := presence
	changed.Status = "disconnected"
	changed = mustFinalizeNodeConnectorSessionEvidence(t, changed)
	if err := fixture.fake.RecordEvidence(mustNodeExecutionJSON(t, changed)); err == nil {
		t.Fatal("changed duplicate evidence was accepted")
	}
	replay := presence
	replay.Sequence = 3
	replay.EvidenceID = "evidence-presence-replayed-001"
	replay = mustFinalizeNodeConnectorSessionEvidence(t, replay)
	if err := fixture.fake.RecordEvidence(mustNodeExecutionJSON(t, replay)); err == nil {
		t.Fatal("replayed evidence identity was accepted")
	}
	if !nodeConnectorSessionStateBytesEqual(accepted, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("duplicate or replay rejection changed durable state")
	}
	if len(fixture.execution.broker.state.Operations) != 0 {
		t.Fatal("presence evidence created broker authority")
	}
}

func TestNodeConnectorSessionRestartTamperAndAtomicFailurePublishNothing(t *testing.T) {
	fixture := newNodeConnectorSessionFixture(t, nil)
	before := nodeConnectorSessionStateBytes(t, fixture.root)
	rotation := mustFinalizeNodeConnectorCredentialEvidence(t, NodeConnectorCredentialEvidence{
		Sequence: 1, EvidenceID: "evidence-rotation-001", ReplayIdentity: "replay-rotation-001",
		EnrollmentID: fixture.enrollment.EnrollmentID, MachineID: fixture.enrollment.MachineID, Action: "rotate",
		PreviousCredentialID: fixture.enrollment.InitialCredentialID, CredentialID: "cred-fixture-rotated-001", ObservedAt: nodeExecutionTime(fixture.execution.now.Add(time.Second)),
	})
	originalWriter := nodeConnectorSessionWriteAtomic
	nodeConnectorSessionWriteAtomic = func(string, any) error { return errors.New("injected connector session write failure") }
	if err := fixture.fake.RecordCredential(mustNodeExecutionJSON(t, rotation)); err == nil {
		t.Fatal("injected atomic failure was ignored")
	}
	nodeConnectorSessionWriteAtomic = originalWriter
	t.Cleanup(func() { nodeConnectorSessionWriteAtomic = originalWriter })
	if len(fixture.fake.state.Transitions) != 0 || !nodeConnectorSessionStateBytesEqual(before, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("atomic failure advanced memory or durable state")
	}

	hello := nodeConnectorSessionHello(t, fixture, 1, "negotiation-initial-001", "replay-negotiation-initial-001", "connection-transport-001", "", fixture.enrollment.InitialCredentialID, fixture.execution.capability.SnapshotID, fixture.execution.now.Add(time.Second))
	if _, err := fixture.fake.Negotiate(mustNodeExecutionJSON(t, hello)); err != nil {
		t.Fatal(err)
	}
	artifacts := nodeConnectorSessionStateArtifacts(t, fixture.root)
	path := filepath.Join(fixture.root, artifacts[len(artifacts)-1])
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(raw, []byte("connection-transport-001"), []byte("connection-tampered-001"), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("tamper fixture did not change state")
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	beforeTamper := nodeConnectorSessionStateBytes(t, fixture.root)
	if _, err := NewNodeConnectorSessionFake(fixture.root, fixture.execution.broker, fixture.enrollment, nodeConnectorSessionTransport(fixture.calls, false)); err == nil {
		t.Fatal("tampered restart state was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, tampered) || !nodeConnectorSessionStateBytesEqual(beforeTamper, nodeConnectorSessionStateBytes(t, fixture.root)) {
		t.Fatal("tamper rejection overwrote durable evidence")
	}
}

func newNodeConnectorSessionDispatchFixture(t *testing.T) *nodeConnectorSessionDispatchFixture {
	t.Helper()
	session := newNodeConnectorSessionFixture(t, nil)
	hello := nodeConnectorSessionHello(t, session, 1, "negotiation-dispatch-initial-001", "replay-dispatch-initial-001", "connection-dispatch-initial-001", "", session.enrollment.InitialCredentialID, session.execution.capability.SnapshotID, session.execution.now.Add(time.Second))
	negotiation, err := session.fake.Negotiate(mustNodeExecutionJSON(t, hello))
	if err != nil {
		t.Fatal(err)
	}
	mustRecordNodeConnectorEvidence(t, session.fake, nodeConnectorSessionEvidence(t, session, 2, "evidence-dispatch-connected-001", "replay-dispatch-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, session.execution.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, session.fake, nodeConnectorSessionEvidence(t, session, 3, "evidence-dispatch-health-001", "replay-dispatch-health-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, session.execution.now.Add(3*time.Second)))
	if err := session.execution.broker.Connect(negotiation.MachineID, negotiation.ConnectionID); err != nil {
		t.Fatal(err)
	}
	lease, err := session.execution.broker.Dispatch(negotiation.ConnectionID, session.execution.requestRaw, session.execution.now, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	validationCalls := 0
	connector, err := NewNodeValidationConnector(session.execution.request.Workflow, session.execution.request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		validationCalls++
		return nodeConnectorTestEvidence(t, session.execution), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return &nodeConnectorSessionDispatchFixture{session: session, negotiation: negotiation, lease: lease, connector: connector, validationCalls: &validationCalls}
}

func assertNodeConnectorSessionDispatchRejected(t *testing.T, fixture *nodeConnectorSessionDispatchFixture, negotiation NodeConnectorSessionNegotiation, request NodeExecutionRequest, lease NodeExecutionTaskLease, at time.Time) {
	t.Helper()
	brokerBefore := nodeConnectorStateBytes(t, fixture.session.execution.root)
	sessionBefore := nodeConnectorSessionStateBytes(t, fixture.session.root)
	executionsBefore := *fixture.session.execution.calls
	if _, err := fixture.session.fake.DispatchAcceptedValidation(fixture.connector, negotiation, request, lease, at); err == nil {
		t.Fatal("changed or unauthorized connector session dispatch was accepted")
	}
	if *fixture.validationCalls != 0 || *fixture.session.execution.calls != executionsBefore || !nodeConnectorStateBytesEqual(brokerBefore, nodeConnectorStateBytes(t, fixture.session.execution.root)) || !nodeConnectorSessionStateBytesEqual(sessionBefore, nodeConnectorSessionStateBytes(t, fixture.session.root)) {
		t.Fatalf("rejected handoff invoked validation or execution or published partial state: validation=%d executions_before=%d executions_after=%d", *fixture.validationCalls, executionsBefore, *fixture.session.execution.calls)
	}
}

func newNodeConnectorSessionFixture(t *testing.T, transport NodeConnectorSessionTransport) *nodeConnectorSessionFixture {
	t.Helper()
	execution := newNodeExecutionTestFixture(t)
	enrollment, err := FinalizeNodeConnectorEnrollment(NodeConnectorEnrollment{
		EnrollmentID: "enrollment-fixture-001", MachineID: execution.machine.MachineID, InitialCredentialID: "cred-fixture-initial-001", EnrolledAt: nodeExecutionTime(execution.now.Add(-2 * time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	if transport == nil {
		transport = nodeConnectorSessionTransport(&calls, false)
	}
	root := t.TempDir()
	fake, err := NewNodeConnectorSessionFake(root, execution.broker, enrollment, transport)
	if err != nil {
		t.Fatal(err)
	}
	return &nodeConnectorSessionFixture{execution: execution, root: root, enrollment: enrollment, fake: fake, calls: &calls}
}

func nodeConnectorSessionTransport(calls *int, conflictingRestart bool) NodeConnectorSessionTransport {
	return func(hello NodeConnectorSessionHello) (NodeConnectorSessionNegotiation, error) {
		*calls++
		sessionID := "session-fixture-001"
		if hello.PreviousSessionID != "" {
			sessionID = hello.PreviousSessionID
			if conflictingRestart {
				sessionID = "session-conflict-001"
			}
		}
		return FinalizeNodeConnectorSessionNegotiation(NodeConnectorSessionNegotiation{
			Sequence: hello.Sequence, NegotiationID: hello.NegotiationID, SessionID: sessionID, ConnectionID: hello.ConnectionID,
			EnrollmentID: hello.EnrollmentID, MachineID: hello.MachineID, CredentialID: hello.CredentialID, CapabilitySnapshotID: hello.CapabilitySnapshotID,
			PreviousSessionID: hello.PreviousSessionID, RestartNegotiated: hello.PreviousSessionID != "", NegotiatedAt: hello.ObservedAt, HelloFingerprint: hello.HelloFingerprint,
		})
	}
}

func nodeConnectorSessionHello(t *testing.T, fixture *nodeConnectorSessionFixture, sequence int64, negotiationID, replayID, connectionID, previousSessionID, credentialID, capabilityID string, at time.Time) NodeConnectorSessionHello {
	t.Helper()
	return mustFinalizeNodeConnectorSessionHello(t, NodeConnectorSessionHello{
		Sequence: sequence, NegotiationID: negotiationID, ReplayIdentity: replayID, EnrollmentID: fixture.enrollment.EnrollmentID,
		MachineID: fixture.enrollment.MachineID, CredentialID: credentialID, ConnectionID: connectionID, PreviousSessionID: previousSessionID,
		CapabilitySnapshotID: capabilityID, ObservedAt: nodeExecutionTime(at),
	})
}

func nodeConnectorSessionEvidence(t *testing.T, fixture *nodeConnectorSessionFixture, sequence int64, evidenceID, replayID, kind, status, sessionID, connectionID, credentialID, previousCapabilityID, capabilityID string, at time.Time) NodeConnectorSessionEvidence {
	t.Helper()
	return mustFinalizeNodeConnectorSessionEvidence(t, NodeConnectorSessionEvidence{
		Sequence: sequence, EvidenceID: evidenceID, ReplayIdentity: replayID, Kind: kind, Status: status, SessionID: sessionID, ConnectionID: connectionID,
		EnrollmentID: fixture.enrollment.EnrollmentID, MachineID: fixture.enrollment.MachineID, CredentialID: credentialID,
		PreviousCapabilitySnapshotID: previousCapabilityID, CapabilitySnapshotID: capabilityID, ObservedAt: nodeExecutionTime(at),
	})
}

func mustFinalizeNodeConnectorSessionHello(t *testing.T, value NodeConnectorSessionHello) NodeConnectorSessionHello {
	t.Helper()
	result, err := FinalizeNodeConnectorSessionHello(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func mustFinalizeNodeConnectorSessionNegotiation(t *testing.T, value NodeConnectorSessionNegotiation) NodeConnectorSessionNegotiation {
	t.Helper()
	result, err := FinalizeNodeConnectorSessionNegotiation(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func mustFinalizeNodeConnectorSessionEvidence(t *testing.T, value NodeConnectorSessionEvidence) NodeConnectorSessionEvidence {
	t.Helper()
	result, err := FinalizeNodeConnectorSessionEvidence(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func mustFinalizeNodeConnectorCredentialEvidence(t *testing.T, value NodeConnectorCredentialEvidence) NodeConnectorCredentialEvidence {
	t.Helper()
	result, err := FinalizeNodeConnectorCredentialEvidence(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func mustRecordNodeConnectorEvidence(t *testing.T, fake *NodeConnectorSessionFake, value NodeConnectorSessionEvidence) {
	t.Helper()
	if err := fake.RecordEvidence(mustNodeExecutionJSON(t, value)); err != nil {
		t.Fatal(err)
	}
}

func nodeConnectorSessionStateArtifacts(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := []string{}
	for _, entry := range entries {
		if nodeConnectorSessionStateName.MatchString(entry.Name()) {
			result = append(result, entry.Name())
		}
	}
	return result
}

func nodeConnectorSessionStateBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range nodeConnectorSessionStateArtifacts(t, root) {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		result[name] = raw
	}
	return result
}

func nodeConnectorSessionStateBytesEqual(left, right map[string][]byte) bool {
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

func TestNodeConnectorSessionContractsContainNoCredentialMaterialField(t *testing.T) {
	fixture := newNodeConnectorSessionFixture(t, nil)
	if _, err := FinalizeNodeConnectorEnrollment(NodeConnectorEnrollment{
		EnrollmentID: "enrollment-invalid-001", MachineID: fixture.enrollment.MachineID,
		InitialCredentialID: "cred-password-value-001", EnrolledAt: fixture.enrollment.EnrolledAt,
	}); err == nil {
		t.Fatal("credential-like opaque identity was accepted")
	}
	raw, err := json.Marshal(fixture.fake.state)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"plaintext", "password", "private_key", "credential_material", "token"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("durable session state contains forbidden credential material field %q", forbidden)
		}
	}
}
