package orchestrationhelper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type nodeConnectorWireFixture struct {
	now              time.Time
	machine          NodeExecutionMachineIdentity
	capability       NodeExecutionCapabilitySnapshot
	request          NodeExecutionRequest
	lease            NodeExecutionTaskLease
	enrollment       NodeConnectorEnrollment
	negotiation      NodeConnectorSessionNegotiation
	broker           *NodeExecutionFakeBroker
	session          *NodeConnectorSessionFake
	wire             *NodeConnectorWireProfile
	connector        *NodeValidationConnector
	brokerRoot       string
	sessionRoot      string
	wireRoot         string
	validationCalls  *int
	transportCalls   *int
	credentialStatus map[string]bool
}

func TestNodeConnectorWireAuthenticatedDispatchReplayReconnectAndRestart(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	if len(fixture.broker.state.Operations) != 0 || *fixture.validationCalls != 0 {
		t.Fatal("authenticated session evidence initiated broker work")
	}
	acceptNodeConnectorWireSession(t, fixture)
	if len(fixture.broker.state.Operations) != 0 || *fixture.validationCalls != 0 {
		t.Fatal("healthy authenticated session framing initiated dispatch")
	}
	acceptNodeConnectorWireBrokerOperation(t, fixture)

	requestRaw := mustNodeExecutionJSON(t, fixture.request)
	leaseRaw := mustNodeExecutionJSON(t, fixture.lease)
	requestFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-request-initial-001", ReplayIdentity: "replay-request-initial-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireExecutionRequest,
		Payload: requestRaw, IssuedAt: fixture.now.Add(4 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	leaseFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-lease-initial-001", ReplayIdentity: "replay-lease-initial-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireTaskLease,
		Payload: leaseRaw, IssuedAt: fixture.now.Add(4 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	beforeWire := nodeConnectorWireStateBytes(t, fixture.wireRoot)
	receipt, err := fixture.wire.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, requestFrame, leaseFrame, fixture.now.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if *fixture.validationCalls != 1 || len(fixture.connector.results) != 1 {
		t.Fatalf("accepted proof did not invoke the validation connector and collaborator exactly once: connector_results=%d validator=%d", len(fixture.connector.results), *fixture.validationCalls)
	}
	if fixture.broker.executor != nil || fixture.broker.state.Operations[fixture.request.OperationID].ExecutionCount != 1 {
		t.Fatal("authenticated proof invoked or synthesized a broker executor")
	}
	if receipt.RequestFingerprint != fixture.request.RequestFingerprint || receipt.LeaseID != fixture.lease.LeaseID || receipt.MachineID != fixture.machine.MachineID || receipt.CapabilitySnapshotID != fixture.capability.SnapshotID {
		t.Fatalf("receipt lost accepted request/lease/session bindings: %#v", receipt)
	}
	if nodeConnectorWireStateBytesEqual(beforeWire, nodeConnectorWireStateBytes(t, fixture.wireRoot)) {
		t.Fatal("accepted request and lease frames did not publish replay evidence")
	}

	receiptFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-receipt-initial-001", ReplayIdentity: "replay-receipt-initial-001",
		CredentialReference: fixture.negotiation.CredentialID, MessageKind: NodeConnectorWireExecutionReceipt,
		Payload: mustNodeExecutionJSON(t, receipt), IssuedAt: fixture.now.Add(6 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	brokerBeforeReceipt := nodeConnectorStateBytes(t, fixture.brokerRoot)
	if err := fixture.wire.AcceptExecutionReceipt(receiptFrame, fixture.negotiation, receipt, fixture.now.Add(7*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !nodeConnectorStateBytesEqual(brokerBeforeReceipt, nodeConnectorStateBytes(t, fixture.brokerRoot)) {
		t.Fatal("authenticated receipt framing created completion or broker authority")
	}

	terminalBroker := nodeConnectorStateBytes(t, fixture.brokerRoot)
	terminalSession := nodeConnectorSessionStateBytes(t, fixture.sessionRoot)
	terminalWire := nodeConnectorWireStateBytes(t, fixture.wireRoot)
	if _, err := fixture.wire.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, requestFrame, leaseFrame, fixture.now.Add(8*time.Second)); err == nil {
		t.Fatal("identical accepted wire frame replay was not rejected")
	}
	if *fixture.validationCalls != 1 || !nodeConnectorStateBytesEqual(terminalBroker, nodeConnectorStateBytes(t, fixture.brokerRoot)) || !nodeConnectorSessionStateBytesEqual(terminalSession, nodeConnectorSessionStateBytes(t, fixture.sessionRoot)) || !nodeConnectorWireStateBytesEqual(terminalWire, nodeConnectorWireStateBytes(t, fixture.wireRoot)) {
		t.Fatal("wire replay rejection invoked validation or published partial state")
	}

	disconnected := nodeConnectorSessionEvidence(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 4,
		"evidence-wire-disconnected-001", "replay-wire-disconnected-001", "presence", "disconnected", fixture.negotiation.SessionID,
		fixture.negotiation.ConnectionID, fixture.negotiation.CredentialID, "", fixture.capability.SnapshotID, fixture.now.Add(8*time.Second))
	mustRecordNodeConnectorEvidence(t, fixture.session, disconnected)
	fixture.broker.Disconnect(fixture.negotiation.ConnectionID)

	reopenedBroker, err := NewNodeExecutionFakeBroker(fixture.brokerRoot, fixture.machine, []NodeExecutionCapabilitySnapshot{fixture.capability}, nil)
	if err != nil {
		t.Fatal(err)
	}
	reopenedTransportCalls := 0
	reopenedSession, err := NewNodeConnectorSessionFake(fixture.sessionRoot, reopenedBroker, fixture.enrollment, nodeConnectorSessionTransport(&reopenedTransportCalls, false))
	if err != nil {
		t.Fatal(err)
	}
	reopenedWire := reopenNodeConnectorWire(t, fixture, reopenedSession)
	fixture.broker, fixture.session, fixture.wire = reopenedBroker, reopenedSession, reopenedWire
	restartHello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 5,
		"negotiation-wire-restart-001", "replay-negotiation-wire-restart-001", "connection-wire-restart-001", fixture.negotiation.SessionID,
		fixture.negotiation.CredentialID, fixture.capability.SnapshotID, fixture.now.Add(9*time.Second))
	restartHelloFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-hello-restart-001", ReplayIdentity: "replay-frame-hello-restart-001",
		CredentialReference: restartHello.CredentialID, MessageKind: NodeConnectorWireSessionHello,
		Payload: mustNodeExecutionJSON(t, restartHello), IssuedAt: fixture.now.Add(9 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	restartNegotiation, err := fixture.wire.NegotiateSession(restartHelloFrame, fixture.now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	fixture.negotiation = restartNegotiation
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 6,
		"evidence-wire-reconnected-001", "replay-wire-reconnected-001", "presence", "connected", restartNegotiation.SessionID, restartNegotiation.ConnectionID,
		restartNegotiation.CredentialID, "", restartNegotiation.CapabilitySnapshotID, fixture.now.Add(10*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 7,
		"evidence-wire-rehealth-001", "replay-wire-rehealth-001", "health", "healthy", restartNegotiation.SessionID, restartNegotiation.ConnectionID,
		restartNegotiation.CredentialID, "", restartNegotiation.CapabilitySnapshotID, fixture.now.Add(11*time.Second)))
	if err := fixture.broker.Connect(fixture.machine.MachineID, restartNegotiation.ConnectionID); err != nil {
		t.Fatal(err)
	}
	replayedLease, err := fixture.broker.Dispatch(restartNegotiation.ConnectionID, requestRaw, fixture.now, 30*time.Minute)
	if err != nil || !nodeExecutionEqual(replayedLease, fixture.lease) {
		t.Fatalf("broker operation replay changed the accepted lease: lease=%#v err=%v", replayedLease, err)
	}

	freshRequestFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-request-resume-001", ReplayIdentity: "replay-request-resume-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireExecutionRequest, Payload: requestRaw,
		IssuedAt: fixture.now.Add(12 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	freshLeaseFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-lease-resume-001", ReplayIdentity: "replay-lease-resume-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireTaskLease, Payload: leaseRaw,
		IssuedAt: fixture.now.Add(12 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	brokerBeforeResume := nodeConnectorStateBytes(t, fixture.brokerRoot)
	resumed, err := fixture.wire.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, freshRequestFrame, freshLeaseFrame, fixture.now.Add(13*time.Second))
	if err != nil || resumed.ReceiptFingerprint != receipt.ReceiptFingerprint {
		t.Fatalf("fresh wire operation replay did not return the durable receipt: receipt=%#v err=%v", resumed, err)
	}
	if *fixture.validationCalls != 1 || len(fixture.connector.results) != 1 || !nodeConnectorStateBytesEqual(brokerBeforeResume, nodeConnectorStateBytes(t, fixture.brokerRoot)) {
		t.Fatal("fresh reconnect frames duplicated validation or durable broker output")
	}
	if _, err := fixture.wire.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, requestFrame, freshLeaseFrame, fixture.now.Add(14*time.Second)); err == nil {
		t.Fatal("receiver restart forgot an already accepted frame identity")
	}
}

func TestNodeConnectorWireCanonicalMutualAuthenticationAndStrictFraming(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-wire-canonical-001", "replay-negotiation-wire-canonical-001", "connection-wire-canonical-001", "",
		fixture.enrollment.InitialCredentialID, fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	input := NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-canonical-001", ReplayIdentity: "replay-frame-canonical-001",
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	}
	first := mustNodeConnectorWireFrame(t, fixture, input)
	second := mustNodeConnectorWireFrame(t, fixture, input)
	if !bytes.Equal(first, second) || len(first) > NodeConnectorWireMaxBytes {
		t.Fatalf("canonical frame bytes are nondeterministic or oversized: bytes=%d", len(first))
	}
	var frame NodeConnectorWireFrame
	if err := decodeNodeExecutionCanonical(first, &frame); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame.Payload, input.Payload) || frame.PayloadFingerprint != nodeConnectorWirePayloadFingerprint(input.Payload) || frame.FrameFingerprint == "" {
		t.Fatal("canonical payload bytes or fingerprints changed across encode/decode")
	}

	for _, invalid := range []struct {
		name string
		raw  []byte
	}{
		{name: "unknown outer field", raw: append(first[:len(first)-1], []byte(`,"unknown":true}`)...)},
		{name: "trailing data", raw: append(append([]byte{}, first...), '\n')},
		{name: "malformed frame", raw: []byte(`{"schema":`)},
		{name: "oversized frame", raw: bytes.Repeat([]byte("x"), NodeConnectorWireMaxBytes+1)},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			before := nodeConnectorWireStateBytes(t, fixture.wireRoot)
			if _, err := fixture.wire.NegotiateSession(invalid.raw, fixture.now.Add(time.Second)); err == nil {
				t.Fatal("invalid framing was accepted")
			}
			if !nodeConnectorWireStateBytesEqual(before, nodeConnectorWireStateBytes(t, fixture.wireRoot)) {
				t.Fatal("invalid framing published replay evidence")
			}
		})
	}
	prettyPayload, _ := json.MarshalIndent(hello, "", "  ")
	if _, err := fixture.wire.EncodeFrame(NodeConnectorWireFrameInput{
		Direction: input.Direction, FrameID: "frame-noncanonical-001", ReplayIdentity: "replay-frame-noncanonical-001", CredentialReference: input.CredentialReference,
		MessageKind: input.MessageKind, Payload: prettyPayload, IssuedAt: input.IssuedAt, ExpiresAt: input.ExpiresAt,
	}); err == nil {
		t.Fatal("noncanonical payload was accepted")
	}
	unknownPayload := append(append([]byte{}, input.Payload[:len(input.Payload)-1]...), []byte(`,"unknown":true}`)...)
	if _, err := fixture.wire.EncodeFrame(NodeConnectorWireFrameInput{
		Direction: input.Direction, FrameID: "frame-payload-unknown-001", ReplayIdentity: "replay-frame-payload-unknown-001", CredentialReference: input.CredentialReference,
		MessageKind: input.MessageKind, Payload: unknownPayload, IssuedAt: input.IssuedAt, ExpiresAt: input.ExpiresAt,
	}); err == nil {
		t.Fatal("payload with an unknown field was accepted")
	}
	if _, err := fixture.wire.EncodeFrame(NodeConnectorWireFrameInput{
		Direction: input.Direction, FrameID: "frame-payload-trailing-001", ReplayIdentity: "replay-frame-payload-trailing-001", CredentialReference: input.CredentialReference,
		MessageKind: input.MessageKind, Payload: append(append([]byte{}, input.Payload...), '\n'), IssuedAt: input.IssuedAt, ExpiresAt: input.ExpiresAt,
	}); err == nil {
		t.Fatal("payload with trailing data was accepted")
	}
	if _, err := fixture.wire.EncodeFrame(NodeConnectorWireFrameInput{
		Direction: input.Direction, FrameID: "frame-payload-oversize-001", ReplayIdentity: "replay-frame-payload-oversize-001", CredentialReference: input.CredentialReference,
		MessageKind: input.MessageKind, Payload: bytes.Repeat([]byte("x"), nodeConnectorWireMaxPayload+1), IssuedAt: input.IssuedAt, ExpiresAt: input.ExpiresAt,
	}); err == nil {
		t.Fatal("oversized payload was accepted")
	}
	if _, err := fixture.wire.EncodeFrame(NodeConnectorWireFrameInput{
		Direction: input.Direction, FrameID: "frame-unknown-kind-001", ReplayIdentity: "replay-frame-unknown-kind-001", CredentialReference: input.CredentialReference,
		MessageKind: "connector.unknown", Payload: input.Payload, IssuedAt: input.IssuedAt, ExpiresAt: input.ExpiresAt,
	}); err == nil {
		t.Fatal("unsupported message kind was accepted")
	}

	connectorFrame := frame
	connectorProof := connectorFrame.AuthenticationProof
	brokerFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-auth-broker-001", ReplayIdentity: "replay-auth-broker-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireExecutionRequest, Payload: mustNodeExecutionJSON(t, fixture.request),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
	var wrongProof NodeConnectorWireFrame
	if err := json.Unmarshal(brokerFrame, &wrongProof); err != nil {
		t.Fatal(err)
	}
	wrongProof.AuthenticationProof = connectorProof
	wrongProof.FrameFingerprint, _ = nodeConnectorWireFrameFingerprint(wrongProof)
	wrongProofRaw, _ := json.Marshal(wrongProof)
	if _, err := fixture.wire.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, wrongProofRaw, brokerFrame, fixture.now.Add(time.Second)); err == nil {
		t.Fatal("connector authentication proof authenticated a broker frame")
	}

	rawFrameType := reflect.TypeOf(NodeConnectorWireFrame{})
	serializedNames := []string{}
	for index := 0; index < rawFrameType.NumField(); index++ {
		serializedNames = append(serializedNames, rawFrameType.Field(index).Tag.Get("json"))
	}
	joined := strings.ToLower(strings.Join(serializedNames, " "))
	for _, forbidden := range []string{"command", "executable", "provider", "scheduler", "approval", "apply", "checkpoint", "commit", "push", "publication", "completion_authority", "credential_material", "password", "private_key", "certificate", "token"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("wire framing profile exposes forbidden authority or credential field %q", forbidden)
		}
	}
}

func TestNodeConnectorWireRejectsChangedBindingsFreshnessRevocationAndTamper(t *testing.T) {
	sharedFixture := newNodeConnectorWireFixture(t)
	acceptNodeConnectorWireSession(t, sharedFixture)
	acceptNodeConnectorWireBrokerOperation(t, sharedFixture)
	sharedRequestFrame, sharedLeaseFrame := nodeConnectorWireOperationFrames(t, sharedFixture)

	tests := []struct {
		name   string
		mutate func(*nodeConnectorWireFixture, []byte, []byte) ([]byte, []byte, time.Time)
	}{
		{name: "frame identity", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return mutateNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.FrameID = "frame-substituted-001" }, false), lease, f.now.Add(5 * time.Second)
		}},
		{name: "replay identity", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return mutateNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.ReplayIdentity = "replay-substituted-001" }, false), lease, f.now.Add(5 * time.Second)
		}},
		{name: "direction", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.Direction = NodeConnectorWireConnectorToBroker }), lease, f.now.Add(5 * time.Second)
		}},
		{name: "sender receiver role", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) {
				value.SenderRole, value.ReceiverRole = value.ReceiverRole, value.SenderRole
			}), lease, f.now.Add(5 * time.Second)
		}},
		{name: "peer authentication identity", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.SenderPeerID = "peer-broker-substituted-001" }), lease, f.now.Add(5 * time.Second)
		}},
		{name: "credential reference", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.CredentialReference = "cred-broker-substituted-001" }), lease, f.now.Add(5 * time.Second)
		}},
		{name: "message schema", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.MessageSchema = NodeExecutionLeaseSchema }), lease, f.now.Add(5 * time.Second)
		}},
		{name: "message kind", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.MessageKind = NodeConnectorWireTaskLease }), lease, f.now.Add(5 * time.Second)
		}},
		{name: "canonical payload bytes fingerprint", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return mutateNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.PayloadFingerprint = nodeExecutionTestDigest("substituted") }, false), lease, f.now.Add(5 * time.Second)
		}},
		{name: "request substitution", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			changed := f.request
			changed.TaskID = "task-wire-substituted-001"
			changed, _ = FinalizeNodeExecutionRequest(changed)
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) {
				value.Payload = mustNodeExecutionJSON(t, changed)
				value.PayloadFingerprint = nodeConnectorWirePayloadFingerprint(value.Payload)
			}), lease, f.now.Add(5 * time.Second)
		}},
		{name: "capability snapshot substitution", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			changed := f.request
			changed.CapabilitySnapshotID = "sha256:" + strings.Repeat("b", 64)
			changed, _ = FinalizeNodeExecutionRequest(changed)
			return resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) {
				value.Payload = mustNodeExecutionJSON(t, changed)
				value.PayloadFingerprint = nodeConnectorWirePayloadFingerprint(value.Payload)
			}), lease, f.now.Add(5 * time.Second)
		}},
		{name: "lease substitution", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			changed := f.lease
			changed.LeaseID = "lease-wire-substituted-001"
			return request, resignNodeConnectorWireFrame(t, f, lease, func(value *NodeConnectorWireFrame) {
				value.Payload = mustNodeExecutionJSON(t, changed)
				value.PayloadFingerprint = nodeConnectorWirePayloadFingerprint(value.Payload)
			}), f.now.Add(5 * time.Second)
		}},
		{name: "lease expiry despite fresh frame", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			request = resignNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) {
				value.IssuedAt = nodeExecutionTime(f.leaseExpiry().Add(-time.Second))
				value.ExpiresAt = nodeExecutionTime(f.leaseExpiry().Add(time.Minute))
			})
			lease = resignNodeConnectorWireFrame(t, f, lease, func(value *NodeConnectorWireFrame) {
				value.IssuedAt = nodeExecutionTime(f.leaseExpiry().Add(-time.Second))
				value.ExpiresAt = nodeExecutionTime(f.leaseExpiry().Add(time.Minute))
			})
			return request, lease, f.leaseExpiry()
		}},
		{name: "stale frame freshness", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return request, lease, f.now.Add(3 * time.Minute)
		}},
		{name: "freshness timestamp tamper", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			return mutateNodeConnectorWireFrame(t, f, request, func(value *NodeConnectorWireFrame) { value.IssuedAt = nodeExecutionTime(f.now.Add(time.Second)) }, false), lease, f.now.Add(5 * time.Second)
		}},
		{name: "revoked broker credential", mutate: func(f *nodeConnectorWireFixture, request, lease []byte) ([]byte, []byte, time.Time) {
			f.credentialStatus[f.wire.brokerCredential] = true
			return request, lease, f.now.Add(5 * time.Second)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := sharedFixture
			requestFrame := append([]byte{}, sharedRequestFrame...)
			leaseFrame := append([]byte{}, sharedLeaseFrame...)
			if test.name == "revoked broker credential" {
				fixture = newNodeConnectorWireFixture(t)
				acceptNodeConnectorWireSession(t, fixture)
				acceptNodeConnectorWireBrokerOperation(t, fixture)
				requestFrame, leaseFrame = nodeConnectorWireOperationFrames(t, fixture)
			}
			requestFrame, leaseFrame, at := test.mutate(fixture, requestFrame, leaseFrame)
			assertNodeConnectorWireDispatchRejected(t, fixture, requestFrame, leaseFrame, at)
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorSessionNegotiation)
	}{
		{name: "enrollment identity", mutate: func(value *NodeConnectorSessionNegotiation) { value.EnrollmentID = "enrollment-substituted-001" }},
		{name: "machine identity", mutate: func(value *NodeConnectorSessionNegotiation) { value.MachineID = "machine-substituted-001" }},
		{name: "connection identity", mutate: func(value *NodeConnectorSessionNegotiation) { value.ConnectionID = "connection-substituted-001" }},
		{name: "session identity", mutate: func(value *NodeConnectorSessionNegotiation) { value.SessionID = "session-substituted-001" }},
		{name: "negotiated capability identity", mutate: func(value *NodeConnectorSessionNegotiation) {
			value.CapabilitySnapshotID = "sha256:" + strings.Repeat("c", 64)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := *sharedFixture
			fixture.negotiation = sharedFixture.negotiation
			test.mutate(&fixture.negotiation)
			assertNodeConnectorWireDispatchRejected(t, &fixture, sharedRequestFrame, sharedLeaseFrame, fixture.now.Add(5*time.Second))
		})
	}

	t.Run("revoked connector credential", func(t *testing.T) {
		fixture := newNodeConnectorWireFixture(t)
		acceptNodeConnectorWireSession(t, fixture)
		acceptNodeConnectorWireBrokerOperation(t, fixture)
		revocation := mustFinalizeNodeConnectorCredentialEvidence(t, NodeConnectorCredentialEvidence{
			Sequence: 4, EvidenceID: "evidence-wire-revoke-001", ReplayIdentity: "replay-wire-revoke-001", EnrollmentID: fixture.enrollment.EnrollmentID,
			MachineID: fixture.machine.MachineID, Action: "revoke", CredentialID: fixture.negotiation.CredentialID, ObservedAt: nodeExecutionTime(fixture.now.Add(4 * time.Second)),
		})
		if err := fixture.session.RecordCredential(mustNodeExecutionJSON(t, revocation)); err != nil {
			t.Fatal(err)
		}
		requestFrame, leaseFrame := nodeConnectorWireOperationFrames(t, fixture)
		assertNodeConnectorWireDispatchRejected(t, fixture, requestFrame, leaseFrame, fixture.now.Add(5*time.Second))
	})
}

func TestNodeConnectorWireDurableStateTamperFailsClosed(t *testing.T) {
	for _, target := range []string{"wire", "session", "broker"} {
		t.Run(target, func(t *testing.T) {
			fixture := newNodeConnectorWireFixture(t)
			acceptNodeConnectorWireSession(t, fixture)
			acceptNodeConnectorWireBrokerOperation(t, fixture)
			requestFrame, leaseFrame := nodeConnectorWireOperationFrames(t, fixture)
			var root string
			var names []string
			switch target {
			case "wire":
				root, names = fixture.wireRoot, nodeConnectorWireStateArtifacts(t, fixture.wireRoot)
			case "session":
				root, names = fixture.sessionRoot, nodeConnectorSessionStateArtifacts(t, fixture.sessionRoot)
			default:
				root, names = fixture.brokerRoot, nodeExecutionStateArtifacts(t, fixture.brokerRoot)
			}
			path := filepath.Join(root, names[len(names)-1])
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tampered := bytes.Replace(raw, []byte("sha256:"), []byte("sha256:0"), 1)
			if bytes.Equal(raw, tampered) {
				t.Fatal("tamper fixture did not change durable bytes")
			}
			if err := os.WriteFile(path, tampered, 0o644); err != nil {
				t.Fatal(err)
			}
			assertNodeConnectorWireDispatchRejected(t, fixture, requestFrame, leaseFrame, fixture.now.Add(5*time.Second))
		})
	}
}

func newNodeConnectorWireFixture(t *testing.T) *nodeConnectorWireFixture {
	t.Helper()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	machine := NodeExecutionMachineIdentity{Schema: NodeExecutionMachineIdentitySchema, MachineID: "machine-wire-001", EnrolledAt: nodeExecutionTime(now.Add(-time.Hour))}
	capability, err := NewNodeExecutionCapabilitySnapshot(machine.MachineID,
		NodeExecutionObservedCapabilities{HostOS: "windows", Runtime: "host", Toolchains: []string{"go1.25"}},
		NodeExecutionApprovedCapabilities{PolicyClass: "validation", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	request, err := FinalizeNodeExecutionRequest(NodeExecutionRequest{
		OperationID: "operation-wire-001", GraphRunID: "graph-wire-001", RunID: "run-wire-001", TaskID: "task-wire-001", SourceRevision: strings.Repeat("a", 40),
		Workflow: NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "validate.readonly"}, CapabilitySnapshotID: capability.SnapshotID,
		Inputs: []NodeExecutionInput{}, Artifacts: []NodeExecutionArtifactReference{}, RequestedAt: nodeExecutionTime(now.Add(-time.Second)),
	})
	if err != nil {
		t.Fatal(err)
	}
	brokerRoot := t.TempDir()
	broker, err := NewNodeExecutionFakeBroker(brokerRoot, machine, []NodeExecutionCapabilitySnapshot{capability}, nil)
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := FinalizeNodeConnectorEnrollment(NodeConnectorEnrollment{
		EnrollmentID: "enrollment-wire-001", MachineID: machine.MachineID, InitialCredentialID: "cred-connector-wire-001", EnrolledAt: nodeExecutionTime(now.Add(-2 * time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	transportCalls := 0
	sessionRoot := t.TempDir()
	session, err := NewNodeConnectorSessionFake(sessionRoot, broker, enrollment, nodeConnectorSessionTransport(&transportCalls, false))
	if err != nil {
		t.Fatal(err)
	}
	credentialStatus := map[string]bool{}
	wireRoot := t.TempDir()
	fixture := &nodeConnectorWireFixture{now: now, machine: machine, capability: capability, request: request, enrollment: enrollment, broker: broker, session: session, brokerRoot: brokerRoot, sessionRoot: sessionRoot, wireRoot: wireRoot, transportCalls: &transportCalls, credentialStatus: credentialStatus}
	wire, err := NewNodeConnectorWireProfile(wireRoot, session, "peer-connector-wire-001", "peer-broker-wire-001", "cred-broker-wire-001",
		nodeConnectorWireTestAuthentication("connector-secret"), nodeConnectorWireTestAuthentication("broker-secret"),
		func(direction, peerID, credentialReference string, _ time.Time) error {
			if credentialStatus[credentialReference] {
				return errors.New("revoked")
			}
			if direction == NodeConnectorWireConnectorToBroker && peerID != "peer-connector-wire-001" || direction == NodeConnectorWireBrokerToConnector && (peerID != "peer-broker-wire-001" || credentialReference != "cred-broker-wire-001") {
				return errors.New("identity mismatch")
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	fixture.wire = wire
	validationCalls := 0
	connector, err := NewNodeValidationConnector(request.Workflow, request.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		validationCalls++
		return nodeConnectorWireValidationEvidence(t, fixture), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.connector, fixture.validationCalls = connector, &validationCalls
	return fixture
}

func acceptNodeConnectorWireSession(t *testing.T, fixture *nodeConnectorWireFixture) {
	t.Helper()
	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-wire-initial-001", "replay-negotiation-wire-initial-001", "connection-wire-initial-001", "", fixture.enrollment.InitialCredentialID,
		fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	helloFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-hello-initial-001", ReplayIdentity: "replay-frame-hello-initial-001",
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
	negotiation, err := fixture.wire.NegotiateSession(helloFrame, fixture.now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	negotiationFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-negotiation-initial-001", ReplayIdentity: "replay-frame-negotiation-initial-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireSessionNegotiation, Payload: mustNodeExecutionJSON(t, negotiation),
		IssuedAt: fixture.now.Add(time.Second), ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := fixture.wire.AcceptSessionNegotiation(negotiationFrame, negotiation, fixture.now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	fixture.negotiation = negotiation
	sessionFixture := &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 2, "evidence-wire-connected-001", "replay-wire-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 3, "evidence-wire-health-001", "replay-wire-health-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(3*time.Second)))
}

func acceptNodeConnectorWireBrokerOperation(t *testing.T, fixture *nodeConnectorWireFixture) {
	t.Helper()
	if err := fixture.broker.Connect(fixture.machine.MachineID, fixture.negotiation.ConnectionID); err != nil {
		t.Fatal(err)
	}
	lease, err := fixture.broker.Dispatch(fixture.negotiation.ConnectionID, mustNodeExecutionJSON(t, fixture.request), fixture.now, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	fixture.lease = lease
}

func nodeConnectorWireOperationFrames(t *testing.T, fixture *nodeConnectorWireFixture) ([]byte, []byte) {
	t.Helper()
	return mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
			Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-request-reject-001", ReplayIdentity: "replay-request-reject-001", CredentialReference: fixture.wire.brokerCredential,
			MessageKind: NodeConnectorWireExecutionRequest, Payload: mustNodeExecutionJSON(t, fixture.request), IssuedAt: fixture.now.Add(4 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
		}), mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
			Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-lease-reject-001", ReplayIdentity: "replay-lease-reject-001", CredentialReference: fixture.wire.brokerCredential,
			MessageKind: NodeConnectorWireTaskLease, Payload: mustNodeExecutionJSON(t, fixture.lease), IssuedAt: fixture.now.Add(4 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
		})
}

func assertNodeConnectorWireDispatchRejected(t *testing.T, fixture *nodeConnectorWireFixture, requestFrame, leaseFrame []byte, at time.Time) {
	t.Helper()
	brokerBefore := nodeConnectorStateBytes(t, fixture.brokerRoot)
	sessionBefore := nodeConnectorSessionStateBytes(t, fixture.sessionRoot)
	wireBefore := nodeConnectorWireStateBytes(t, fixture.wireRoot)
	if _, err := fixture.wire.DispatchAcceptedValidation(fixture.connector, fixture.negotiation, requestFrame, leaseFrame, at); err == nil {
		t.Fatal("changed, stale, revoked, or tampered wire dispatch was accepted")
	}
	if *fixture.validationCalls != 0 || len(fixture.connector.results) != 0 || !nodeConnectorStateBytesEqual(brokerBefore, nodeConnectorStateBytes(t, fixture.brokerRoot)) || !nodeConnectorSessionStateBytesEqual(sessionBefore, nodeConnectorSessionStateBytes(t, fixture.sessionRoot)) || !nodeConnectorWireStateBytesEqual(wireBefore, nodeConnectorWireStateBytes(t, fixture.wireRoot)) {
		t.Fatal("rejected wire frame invoked connector/validator/executor or published partial durable output")
	}
}

func wireExecutionFixture(fixture *nodeConnectorWireFixture) *nodeExecutionTestFixture {
	return &nodeExecutionTestFixture{root: fixture.brokerRoot, now: fixture.now, machine: fixture.machine, capability: fixture.capability, request: fixture.request, requestRaw: mustJSONWithoutTest(fixture.request), broker: fixture.broker, connection: fixture.negotiation.ConnectionID}
}

func mustJSONWithoutTest(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func nodeConnectorWireValidationEvidence(t *testing.T, fixture *nodeConnectorWireFixture) NodeValidationEvidence {
	t.Helper()
	base := &nodeExecutionTestFixture{now: fixture.now, request: fixture.request}
	return nodeConnectorTestEvidence(t, base)
}

func nodeConnectorWireTestAuthentication(secret string) NodeConnectorWireAuthentication {
	proof := func(raw []byte) string {
		sum := sha256.Sum256(append(append([]byte{}, raw...), []byte(secret)...))
		return "proof-" + hex.EncodeToString(sum[:])
	}
	return NodeConnectorWireAuthentication{
		Authenticate: func(raw []byte) (string, error) { return proof(raw), nil },
		Verify: func(raw []byte, got string) error {
			if got != proof(raw) {
				return errors.New("deterministic proof mismatch")
			}
			return nil
		},
	}
}

func mustNodeConnectorWireFrame(t *testing.T, fixture *nodeConnectorWireFixture, input NodeConnectorWireFrameInput) []byte {
	t.Helper()
	raw, err := fixture.wire.EncodeFrame(input)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mutateNodeConnectorWireFrame(t *testing.T, fixture *nodeConnectorWireFixture, raw []byte, mutate func(*NodeConnectorWireFrame), resign bool) []byte {
	t.Helper()
	var value NodeConnectorWireFrame
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	mutate(&value)
	if resign {
		value.AuthenticationProof = ""
		value.FrameFingerprint = ""
		binding, _ := nodeConnectorWireAuthenticationBytes(value)
		value.AuthenticationProof, _ = fixture.wire.authentication(value.Direction).Authenticate(binding)
		value.FrameFingerprint, _ = nodeConnectorWireFrameFingerprint(value)
	}
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func resignNodeConnectorWireFrame(t *testing.T, fixture *nodeConnectorWireFixture, raw []byte, mutate func(*NodeConnectorWireFrame)) []byte {
	return mutateNodeConnectorWireFrame(t, fixture, raw, mutate, true)
}

func (fixture *nodeConnectorWireFixture) leaseExpiry() time.Time {
	result, _ := parseNodeExecutionTime(fixture.lease.ExpiresAt)
	return result
}

func reopenNodeConnectorWire(t *testing.T, fixture *nodeConnectorWireFixture, session *NodeConnectorSessionFake) *NodeConnectorWireProfile {
	t.Helper()
	result, err := NewNodeConnectorWireProfile(fixture.wireRoot, session, fixture.wire.connectorPeerID, fixture.wire.brokerPeerID, fixture.wire.brokerCredential,
		fixture.wire.connectorAuthentication, fixture.wire.brokerAuthentication, fixture.wire.credentialCheck)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func nodeConnectorWireStateArtifacts(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := []string{}
	for _, entry := range entries {
		if nodeConnectorWireStateName.MatchString(entry.Name()) {
			result = append(result, entry.Name())
		}
	}
	return result
}

func nodeConnectorWireStateBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range nodeConnectorWireStateArtifacts(t, root) {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		result[name] = raw
	}
	return result
}

func nodeConnectorWireStateBytesEqual(left, right map[string][]byte) bool {
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
