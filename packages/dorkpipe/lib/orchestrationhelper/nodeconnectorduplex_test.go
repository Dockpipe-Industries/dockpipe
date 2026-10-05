package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNodeConnectorDuplexAuthenticatedExchangeDispatchesOnceAndResumesBeforeAcknowledgement(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	exchangeRoot := t.TempDir()
	config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
		MaxQueuedFrames: 8, MaxQueuedBytes: 8 * NodeConnectorWireMaxBytes,
		MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
	})
	exchange := mustNodeConnectorDuplex(t, exchangeRoot, fixture.wire, config)
	initialBroker := nodeConnectorStateBytes(t, fixture.brokerRoot)
	initialSession := nodeConnectorSessionStateBytes(t, fixture.sessionRoot)

	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-duplex-initial-001", "replay-negotiation-duplex-initial-001", "connection-duplex-initial-001", "", fixture.enrollment.InitialCredentialID,
		fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	helloFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-duplex-hello-001", ReplayIdentity: "replay-duplex-hello-001",
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 1, helloFrame, fixture.now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if *fixture.validationCalls != 0 || len(fixture.broker.state.Operations) != 0 || !nodeConnectorStateBytesEqual(initialBroker, nodeConnectorStateBytes(t, fixture.brokerRoot)) || !nodeConnectorSessionStateBytesEqual(initialSession, nodeConnectorSessionStateBytes(t, fixture.sessionRoot)) {
		t.Fatal("exchange establishment or queued credit initiated session, broker, connector, validator, or executor work")
	}
	var negotiation NodeConnectorSessionNegotiation
	if err := exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func(frames [][]byte) error {
		if len(frames) != 1 || !bytes.Equal(frames[0], helloFrame) {
			return errors.New("hello bytes changed in duplex delivery")
		}
		var err error
		negotiation, err = fixture.wire.NegotiateSession(frames[0], fixture.now.Add(time.Second))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := exchange.Acknowledge(NodeConnectorWireConnectorToBroker, 1); err != nil {
		t.Fatal(err)
	}

	negotiationFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-duplex-negotiation-001", ReplayIdentity: "replay-duplex-negotiation-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireSessionNegotiation, Payload: mustNodeExecutionJSON(t, negotiation),
		IssuedAt: fixture.now.Add(time.Second), ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 1, negotiationFrame, fixture.now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := exchange.Deliver(NodeConnectorWireBrokerToConnector, 1, func(frames [][]byte) error {
		return fixture.wire.AcceptSessionNegotiation(frames[0], negotiation, fixture.now.Add(2*time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	if err := exchange.Acknowledge(NodeConnectorWireBrokerToConnector, 1); err != nil {
		t.Fatal(err)
	}
	fixture.negotiation = negotiation
	sessionFixture := &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 2, "evidence-duplex-connected-001", "replay-duplex-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 3, "evidence-duplex-healthy-001", "replay-duplex-healthy-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(3*time.Second)))
	acceptNodeConnectorWireBrokerOperation(t, fixture)
	originalLeaseExpiry := fixture.lease.ExpiresAt

	requestFrame := nodeConnectorDuplexOperationFrame(t, fixture, "request-initial-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(4*time.Second))
	leaseFrame := nodeConnectorDuplexOperationFrame(t, fixture, "lease-initial-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(4*time.Second))
	if err := exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 2, requestFrame, fixture.now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 3, leaseFrame, fixture.now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	beforeRejectedDelivery := mustNodeConnectorDuplexSnapshot(t, exchange)
	if err := exchange.Deliver(NodeConnectorWireBrokerToConnector, 2, func([][]byte) error { return errors.New("acceptance interrupted") }); err == nil {
		t.Fatal("interruption before downstream acceptance was not rejected")
	}
	if !reflect.DeepEqual(beforeRejectedDelivery, mustNodeConnectorDuplexSnapshot(t, exchange)) || *fixture.validationCalls != 0 {
		t.Fatal("failed downstream acceptance advanced state or invoked validation")
	}
	var receipt NodeExecutionReceipt
	if err := exchange.Deliver(NodeConnectorWireBrokerToConnector, 2, func(frames [][]byte) error {
		if len(frames) != 2 || !bytes.Equal(frames[0], requestFrame) || !bytes.Equal(frames[1], leaseFrame) {
			return errors.New("request/lease bytes or ordering changed")
		}
		var err error
		receipt, err = fixture.wire.DispatchAcceptedValidation(fixture.connector, negotiation, frames[0], frames[1], fixture.now.Add(5*time.Second))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if *fixture.validationCalls != 1 || len(fixture.connector.results) != 1 || fixture.broker.executor != nil {
		t.Fatalf("dispatch counts differ from connector=1 validator=1 executor=0: connector=%d validator=%d executor=%v", len(fixture.connector.results), *fixture.validationCalls, fixture.broker.executor != nil)
	}
	if fixture.lease.ExpiresAt != originalLeaseExpiry {
		t.Fatal("exchange delivery or credit extended the broker lease")
	}

	// This is the durable interruption boundary: downstream accepted both
	// frames, but neither acknowledgement was published yet.
	beforeRestart := mustNodeConnectorDuplexSnapshot(t, exchange)
	cursor, err := exchange.Cursor(NodeConnectorWireBrokerToConnector)
	if err != nil {
		t.Fatal(err)
	}
	exchange = mustNodeConnectorDuplex(t, exchangeRoot, fixture.wire, config)
	resumed, err := exchange.Resume(cursor)
	if err != nil || !reflect.DeepEqual(resumed, beforeRestart) {
		t.Fatalf("restart did not restore exact delivery/acknowledgement state: snapshot=%#v err=%v", resumed, err)
	}
	if err := exchange.Deliver(NodeConnectorWireBrokerToConnector, 1, func([][]byte) error {
		t.Fatal("already delivered frame was invoked twice")
		return nil
	}); err == nil {
		t.Fatal("restart offered an already accepted in-flight frame for redelivery")
	}
	if err := exchange.Acknowledge(NodeConnectorWireBrokerToConnector, 2); err != nil {
		t.Fatal(err)
	}
	if err := exchange.Acknowledge(NodeConnectorWireBrokerToConnector, 3); err != nil {
		t.Fatal(err)
	}
	if *fixture.validationCalls != 1 || len(fixture.connector.results) != 1 {
		t.Fatal("acknowledgement after restart duplicated connector or validation invocation")
	}

	receiptFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-duplex-receipt-001", ReplayIdentity: "replay-duplex-receipt-001",
		CredentialReference: negotiation.CredentialID, MessageKind: NodeConnectorWireExecutionReceipt, Payload: mustNodeExecutionJSON(t, receipt),
		IssuedAt: fixture.now.Add(6 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	if err := exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 2, receiptFrame, fixture.now.Add(7*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func(frames [][]byte) error {
		return fixture.wire.AcceptExecutionReceipt(frames[0], negotiation, receipt, fixture.now.Add(7*time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	if err := exchange.Acknowledge(NodeConnectorWireConnectorToBroker, 2); err != nil {
		t.Fatal(err)
	}

	// A new authenticated wire identity for the same completed request/lease
	// returns the same durable receipt without reinvocation.
	freshRequest := nodeConnectorDuplexOperationFrame(t, fixture, "request-fresh-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(8*time.Second))
	freshLease := nodeConnectorDuplexOperationFrame(t, fixture, "lease-fresh-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(8*time.Second))
	if err := exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 4, freshRequest, fixture.now.Add(9*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 5, freshLease, fixture.now.Add(9*time.Second)); err != nil {
		t.Fatal(err)
	}
	var replayedReceipt NodeExecutionReceipt
	if err := exchange.Deliver(NodeConnectorWireBrokerToConnector, 2, func(frames [][]byte) error {
		var err error
		replayedReceipt, err = fixture.wire.DispatchAcceptedValidation(fixture.connector, negotiation, frames[0], frames[1], fixture.now.Add(9*time.Second))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if replayedReceipt.ReceiptFingerprint != receipt.ReceiptFingerprint || *fixture.validationCalls != 1 || len(fixture.connector.results) != 1 {
		t.Fatal("fresh completed-operation frames changed the receipt or repeated execution")
	}
	if err := exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 6, requestFrame, fixture.now.Add(10*time.Second)); err == nil {
		t.Fatal("exact authenticated frame replay was accepted after exchange restart")
	}

	snapshot := mustNodeConnectorDuplexSnapshot(t, exchange)
	if snapshot.ConnectorToBroker.AcceptedSequence != 2 || snapshot.ConnectorToBroker.AcknowledgedSequence != 2 || snapshot.BrokerToConnector.AcceptedSequence != 5 || snapshot.BrokerToConnector.AcknowledgedSequence != 3 {
		t.Fatalf("directional sequence and acknowledgement frontiers are not independent: %#v", snapshot)
	}
}

func TestNodeConnectorDuplexFlowControlOrderingAndRejectionPublishNoPartialState(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	raw1 := nodeConnectorDuplexHelloFrame(t, fixture, "flow-0001")
	raw2 := nodeConnectorDuplexHelloFrame(t, fixture, "flow-0002")

	t.Run("queue-full", func(t *testing.T) {
		limits := NodeConnectorDuplexLimits{MaxQueuedFrames: 1, MaxQueuedBytes: 2 * NodeConnectorWireMaxBytes, MaxInFlightFrames: 2, MaxInFlightBytes: 2 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes}
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, nodeConnectorDuplexTestConfig(t, fixture, limits))
		mustAcceptNodeConnectorDuplexFrame(t, exchange, 1, raw1, fixture.now)
		assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 2, raw2, fixture.now) })
	})

	t.Run("byte-full", func(t *testing.T) {
		limits := NodeConnectorDuplexLimits{MaxQueuedFrames: 3, MaxQueuedBytes: len(raw1), MaxInFlightFrames: 3, MaxInFlightBytes: 3 * len(raw1), MaxFrameBytes: len(raw1)}
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, nodeConnectorDuplexTestConfig(t, fixture, limits))
		mustAcceptNodeConnectorDuplexFrame(t, exchange, 1, raw1, fixture.now)
		assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 2, raw2, fixture.now) })
	})

	t.Run("in-flight-full", func(t *testing.T) {
		limits := NodeConnectorDuplexLimits{MaxQueuedFrames: 3, MaxQueuedBytes: 3 * NodeConnectorWireMaxBytes, MaxInFlightFrames: 1, MaxInFlightBytes: NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes}
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, nodeConnectorDuplexTestConfig(t, fixture, limits))
		mustAcceptNodeConnectorDuplexFrame(t, exchange, 1, raw1, fixture.now)
		mustAcceptNodeConnectorDuplexFrame(t, exchange, 2, raw2, fixture.now)
		if err := exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func(frames [][]byte) error {
			if !bytes.Equal(frames[0], raw1) {
				return errors.New("frame bytes changed")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error {
			return exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func([][]byte) error { return nil })
		})
	})

	t.Run("oversized", func(t *testing.T) {
		limits := NodeConnectorDuplexLimits{MaxQueuedFrames: 2, MaxQueuedBytes: len(raw1) - 1, MaxInFlightFrames: 2, MaxInFlightBytes: len(raw1) - 1, MaxFrameBytes: len(raw1) - 1}
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, nodeConnectorDuplexTestConfig(t, fixture, limits))
		assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 1, raw1, fixture.now) })
	})

	limits := NodeConnectorDuplexLimits{MaxQueuedFrames: 3, MaxQueuedBytes: 3 * NodeConnectorWireMaxBytes, MaxInFlightFrames: 3, MaxInFlightBytes: 3 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes}
	exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, nodeConnectorDuplexTestConfig(t, fixture, limits))
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 2, raw1, fixture.now) })
	mustAcceptNodeConnectorDuplexFrame(t, exchange, 1, raw1, fixture.now)
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 1, raw2, fixture.now) })
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.AcceptFrame(NodeConnectorWireBrokerToConnector, 1, raw2, fixture.now) })
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.Acknowledge(NodeConnectorWireConnectorToBroker, 1) })
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error {
		return exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func([][]byte) error { return errors.New("downstream rejected") })
	})
	if err := exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func(frames [][]byte) error {
		if !bytes.Equal(frames[0], raw1) {
			return errors.New("frame content changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error { return exchange.Acknowledge(NodeConnectorWireConnectorToBroker, 2) })
	if err := exchange.Acknowledge(NodeConnectorWireConnectorToBroker, 1); err != nil {
		t.Fatal(err)
	}
	cursor, err := exchange.Cursor(NodeConnectorWireConnectorToBroker)
	if err != nil {
		t.Fatal(err)
	}
	cursor.AcceptedSequence++
	if _, err := exchange.Resume(cursor); err == nil {
		t.Fatal("conflicting resume cursor was accepted")
	}

	tampered := mutateNodeConnectorWireFrame(t, fixture, raw2, func(frame *NodeConnectorWireFrame) { frame.FrameID = "frame-tampered-flow-0002" }, false)
	assertNodeConnectorDuplexRejectedWithoutState(t, exchange, func() error {
		return exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, 2, tampered, fixture.now)
	})
}

func TestNodeConnectorDuplexRestartConfigurationAndDurableTamperFailClosed(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	root := t.TempDir()
	limits := NodeConnectorDuplexLimits{MaxQueuedFrames: 4, MaxQueuedBytes: 4 * NodeConnectorWireMaxBytes, MaxInFlightFrames: 2, MaxInFlightBytes: 2 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes}
	config := nodeConnectorDuplexTestConfig(t, fixture, limits)
	exchange := mustNodeConnectorDuplex(t, root, fixture.wire, config)
	raw1 := nodeConnectorDuplexHelloFrame(t, fixture, "restart-0001")
	raw2 := nodeConnectorDuplexHelloFrame(t, fixture, "restart-0002")
	mustAcceptNodeConnectorDuplexFrame(t, exchange, 1, raw1, fixture.now)
	mustAcceptNodeConnectorDuplexFrame(t, exchange, 2, raw2, fixture.now)
	if err := exchange.Deliver(NodeConnectorWireConnectorToBroker, 1, func([][]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	before := mustNodeConnectorDuplexSnapshot(t, exchange)
	if before.ConnectorToBroker.QueuedFrames != 1 || before.ConnectorToBroker.InFlightFrames != 1 {
		t.Fatalf("test did not create bounded queued/in-flight state: %#v", before)
	}
	reopened := mustNodeConnectorDuplex(t, root, fixture.wire, config)
	if after := mustNodeConnectorDuplexSnapshot(t, reopened); !reflect.DeepEqual(before, after) {
		t.Fatalf("restart changed queue, byte counters, or frontiers: before=%#v after=%#v", before, after)
	}

	changed := config
	changed.Limits.MaxQueuedFrames++
	changed, err := FinalizeNodeConnectorDuplexConfig(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewNodeConnectorDuplex(root, fixture.wire, changed); err == nil {
		t.Fatal("restart accepted incompatible configuration identity")
	}

	latest := filepath.Join(root, nodeConnectorDuplexStateFileName(reopened.state.Generation))
	raw, err := os.ReadFile(latest)
	if err != nil {
		t.Fatal(err)
	}
	var durable map[string]any
	if err := json.Unmarshal(raw, &durable); err != nil {
		t.Fatal(err)
	}
	durable["state_fingerprint"] = strings.Repeat("0", 64)
	tampered, err := json.Marshal(durable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(latest, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewNodeConnectorDuplex(root, fixture.wire, config); err == nil {
		t.Fatal("durable duplex state tamper was accepted")
	}
	if _, err := reopened.Snapshot(); err == nil {
		t.Fatal("existing collaborator ignored durable state tamper")
	}
}

func TestNodeConnectorDuplexCarriesNoExecutionOrLifecycleAuthority(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
		MaxQueuedFrames: 2, MaxQueuedBytes: 2 * NodeConnectorWireMaxBytes,
		MaxInFlightFrames: 2, MaxInFlightBytes: 2 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
	})
	exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
	raw, err := json.Marshal(struct {
		Config   NodeConnectorDuplexConfig   `json:"config"`
		Snapshot NodeConnectorDuplexSnapshot `json:"snapshot"`
	}{config, mustNodeConnectorDuplexSnapshot(t, exchange)})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"command", "execution_authorized", "lease_granted", "completion_authorized", "mutation_authorized", "publication_authorized"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("duplex contract exposes forbidden generic command or lifecycle authority field %q", forbidden)
		}
	}
	if len(fixture.broker.state.Operations) != 0 || *fixture.validationCalls != 0 || fixture.broker.executor != nil {
		t.Fatal("constructing exchange/config/cursors initiated execution")
	}
	if config.ExchangeID == fixture.machine.MachineID || config.ExchangeID == fixture.capability.SnapshotID || config.ExchangeID == fixture.enrollment.EnrollmentID || config.ExchangeID == fixture.wire.connectorPeerID || config.ExchangeID == fixture.wire.brokerPeerID {
		t.Fatal("exchange identity was conflated with machine, capability, enrollment, or peer identity")
	}
}

func nodeConnectorDuplexTestConfig(t *testing.T, fixture *nodeConnectorWireFixture, limits NodeConnectorDuplexLimits) NodeConnectorDuplexConfig {
	t.Helper()
	config, err := FinalizeNodeConnectorDuplexConfig(NodeConnectorDuplexConfig{
		ExchangeID: "exchange-duplex-proof-001", ConnectorPeerID: fixture.wire.connectorPeerID, BrokerPeerID: fixture.wire.brokerPeerID, Limits: limits,
	})
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func mustNodeConnectorDuplex(t *testing.T, root string, wire *NodeConnectorWireProfile, config NodeConnectorDuplexConfig) *NodeConnectorDuplex {
	t.Helper()
	exchange, err := NewNodeConnectorDuplex(root, wire, config)
	if err != nil {
		t.Fatal(err)
	}
	return exchange
}

func nodeConnectorDuplexHelloFrame(t *testing.T, fixture *nodeConnectorWireFixture, suffix string) []byte {
	t.Helper()
	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-"+suffix, "replay-negotiation-"+suffix, "connection-"+suffix, "", fixture.enrollment.InitialCredentialID, fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	return mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-hello-" + suffix, ReplayIdentity: "replay-frame-hello-" + suffix,
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello), IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
}

func nodeConnectorDuplexOperationFrame(t *testing.T, fixture *nodeConnectorWireFixture, suffix, kind string, payload []byte, issuedAt time.Time) []byte {
	t.Helper()
	return mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-duplex-" + suffix, ReplayIdentity: "replay-duplex-" + suffix,
		CredentialReference: fixture.wire.brokerCredential, MessageKind: kind, Payload: payload, IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(2 * time.Minute),
	})
}

func mustAcceptNodeConnectorDuplexFrame(t *testing.T, exchange *NodeConnectorDuplex, sequence int64, raw []byte, at time.Time) {
	t.Helper()
	if err := exchange.AcceptFrame(NodeConnectorWireConnectorToBroker, sequence, raw, at); err != nil {
		t.Fatal(err)
	}
}

func mustNodeConnectorDuplexSnapshot(t *testing.T, exchange *NodeConnectorDuplex) NodeConnectorDuplexSnapshot {
	t.Helper()
	snapshot, err := exchange.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertNodeConnectorDuplexRejectedWithoutState(t *testing.T, exchange *NodeConnectorDuplex, action func() error) {
	t.Helper()
	before := mustNodeConnectorDuplexSnapshot(t, exchange)
	beforeFiles := nodeConnectorDuplexStateBytes(t, exchange.root)
	if err := action(); err == nil {
		t.Fatal("duplex invalid, overflow, reordered, or downstream-rejected action succeeded")
	}
	if after := mustNodeConnectorDuplexSnapshot(t, exchange); !reflect.DeepEqual(before, after) || !nodeConnectorDuplexStateBytesEqual(beforeFiles, nodeConnectorDuplexStateBytes(t, exchange.root)) {
		t.Fatal("rejected duplex action published partial state, counters, cursor, or fingerprints")
	}
}

func nodeConnectorDuplexStateBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, entry := range entries {
		if !nodeConnectorDuplexStateName.MatchString(entry.Name()) {
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

func nodeConnectorDuplexStateBytesEqual(left, right map[string][]byte) bool {
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
