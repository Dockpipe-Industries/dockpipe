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
