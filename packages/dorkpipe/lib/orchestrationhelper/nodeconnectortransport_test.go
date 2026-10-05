package orchestrationhelper

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNodeConnectorTransportLoopbackAuthenticatedDuplexRestartAndIdempotence(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	limits := nodeConnectorTransportTestLimits()
	config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
		MaxQueuedFrames: 8, MaxQueuedBytes: 8 * NodeConnectorWireMaxBytes,
		MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
	})
	brokerRoot, connectorRoot := t.TempDir(), t.TempDir()
	brokerExchange := mustNodeConnectorDuplex(t, brokerRoot, fixture.wire, config)
	connectorExchange := mustNodeConnectorDuplex(t, connectorRoot, fixture.wire, config)

	brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
	connectorWrites := &nodeConnectorTransportShortWriteConn{Conn: connectorConnection, max: 3}
	brokerWrites := &nodeConnectorTransportShortWriteConn{Conn: brokerConnection, max: 5}
	resumeNodeConnectorTransportDirection(t, connectorWrites, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerWrites, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)

	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-transport-initial-001", "replay-negotiation-transport-initial-001", "connection-transport-initial-001", "",
		fixture.enrollment.InitialCredentialID, fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	helloFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-transport-hello-001", ReplayIdentity: "replay-transport-hello-001",
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(connectorWrites, config, NodeConnectorWireConnectorToBroker, 1, helloFrame, limits); err != nil {
		t.Fatal(err)
	}
	var negotiation NodeConnectorSessionNegotiation
	brokerCursor, err := nodeConnectorTransportAcceptFrame(brokerConnection, brokerExchange, fixture.now.Add(time.Second), func(frames [][]byte) error {
		if len(frames) != 1 || !bytes.Equal(frames[0], helloFrame) {
			return errors.New("transport changed hello frame bytes")
		}
		var acceptErr error
		negotiation, acceptErr = fixture.wire.NegotiateSession(frames[0], fixture.now.Add(time.Second))
		return acceptErr
	}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportReadAcknowledgement(connectorConnection, brokerCursor, 1, limits); err != nil {
		t.Fatal(err)
	}

	negotiationFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-transport-negotiation-001", ReplayIdentity: "replay-transport-negotiation-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireSessionNegotiation, Payload: mustNodeExecutionJSON(t, negotiation),
		IssuedAt: fixture.now.Add(time.Second), ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(brokerWrites, config, NodeConnectorWireBrokerToConnector, 1, negotiationFrame, limits); err != nil {
		t.Fatal(err)
	}
	connectorCursor, err := nodeConnectorTransportAcceptFrame(connectorConnection, connectorExchange, fixture.now.Add(2*time.Second), func(frames [][]byte) error {
		if !bytes.Equal(frames[0], negotiationFrame) {
			return errors.New("transport changed negotiation frame bytes")
		}
		return fixture.wire.AcceptSessionNegotiation(frames[0], negotiation, fixture.now.Add(2*time.Second))
	}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportReadAcknowledgement(brokerConnection, connectorCursor, 1, limits); err != nil {
		t.Fatal(err)
	}
	closePair()

	fixture.negotiation = negotiation
	sessionFixture := &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 2, "evidence-transport-connected-001", "replay-transport-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 3, "evidence-transport-healthy-001", "replay-transport-healthy-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(3*time.Second)))
	acceptNodeConnectorWireBrokerOperation(t, fixture)

	brokerConnection, connectorConnection, closePair = openNodeConnectorTransportPair(t, limits)
	resumeNodeConnectorTransportDirection(t, connectorConnection, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerConnection, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)
	requestFrame := nodeConnectorDuplexOperationFrame(t, fixture, "transport-request-initial-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(4*time.Second))
	leaseFrame := nodeConnectorDuplexOperationFrame(t, fixture, "transport-lease-initial-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(4*time.Second))
	writeCoalescedNodeConnectorTransportFrames(t, brokerConnection, config, NodeConnectorWireBrokerToConnector, 2, [][]byte{requestFrame, leaseFrame}, limits)
	var receipt NodeExecutionReceipt
	connectorCursor, err = nodeConnectorTransportAcceptFrames(connectorConnection, connectorExchange, 2, fixture.now.Add(5*time.Second), func(frames [][]byte) error {
		if len(frames) != 2 || !bytes.Equal(frames[0], requestFrame) || !bytes.Equal(frames[1], leaseFrame) {
			return errors.New("transport changed coalesced request and lease bytes or order")
		}
		var acceptErr error
		receipt, acceptErr = fixture.wire.DispatchAcceptedValidation(fixture.connector, negotiation, frames[0], frames[1], fixture.now.Add(5*time.Second))
		return acceptErr
	}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportReadAcknowledgement(brokerConnection, connectorCursor, 3, limits); err != nil {
		t.Fatal(err)
	}
	receiptFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-transport-receipt-001", ReplayIdentity: "replay-transport-receipt-001",
		CredentialReference: negotiation.CredentialID, MessageKind: NodeConnectorWireExecutionReceipt, Payload: mustNodeExecutionJSON(t, receipt),
		IssuedAt: fixture.now.Add(6 * time.Second), ExpiresAt: fixture.now.Add(2 * time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(connectorConnection, config, NodeConnectorWireConnectorToBroker, 2, receiptFrame, limits); err != nil {
		t.Fatal(err)
	}
	brokerCursor, err = nodeConnectorTransportAcceptFrame(brokerConnection, brokerExchange, fixture.now.Add(7*time.Second), func(frames [][]byte) error {
		return fixture.wire.AcceptExecutionReceipt(frames[0], negotiation, receipt, fixture.now.Add(7*time.Second))
	}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportReadAcknowledgement(connectorConnection, brokerCursor, 2, limits); err != nil {
		t.Fatal(err)
	}
	closePair()

	brokerExchange = mustNodeConnectorDuplex(t, brokerRoot, fixture.wire, config)
	connectorExchange = mustNodeConnectorDuplex(t, connectorRoot, fixture.wire, config)
	brokerConnection, connectorConnection, closePair = openNodeConnectorTransportPair(t, limits)
	defer closePair()
	resumeNodeConnectorTransportDirection(t, connectorConnection, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerConnection, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)
	freshRequest := nodeConnectorDuplexOperationFrame(t, fixture, "transport-request-restart-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(8*time.Second))
	freshLease := nodeConnectorDuplexOperationFrame(t, fixture, "transport-lease-restart-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(8*time.Second))
	if err := nodeConnectorTransportWriteFrame(brokerConnection, config, NodeConnectorWireBrokerToConnector, 4, freshRequest, limits); err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportWriteFrame(brokerConnection, config, NodeConnectorWireBrokerToConnector, 5, freshLease, limits); err != nil {
		t.Fatal(err)
	}
	var replayed NodeExecutionReceipt
	connectorCursor, err = nodeConnectorTransportAcceptFrames(connectorConnection, connectorExchange, 2, fixture.now.Add(9*time.Second), func(frames [][]byte) error {
		var acceptErr error
		replayed, acceptErr = fixture.wire.DispatchAcceptedValidation(fixture.connector, negotiation, frames[0], frames[1], fixture.now.Add(9*time.Second))
		return acceptErr
	}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportReadAcknowledgement(brokerConnection, connectorCursor, 5, limits); err != nil {
		t.Fatal(err)
	}
	if replayed.ReceiptFingerprint != receipt.ReceiptFingerprint || *fixture.validationCalls != 1 || len(fixture.connector.results) != 1 || fixture.broker.executor != nil {
		t.Fatal("transport restart changed the durable receipt or repeated connector, validator, or executor work")
	}
	if brokerExchange.state.Config.ConfigFingerprint != connectorExchange.state.Config.ConfigFingerprint || brokerCursor.ExchangeID != connectorCursor.ExchangeID {
		t.Fatal("transport endpoints lost the exact exchange or configuration identity")
	}
}

func TestNodeConnectorTransportRejectsEndpointsRecordsCursorsAndDownstreamFailureWithoutState(t *testing.T) {
	limits := nodeConnectorTransportTestLimits()
	for _, endpoint := range []string{"localhost:0", "*:0", "0.0.0.0:0", "[::]:0", "192.0.2.1:0", "example.test:443", "127.0.0.1:9"} {
		if _, _, err := validateNodeConnectorTransportEndpoint(endpoint, true); err == nil {
			t.Fatalf("invalid broker endpoint %q was accepted", endpoint)
		}
	}
	for _, endpoint := range []string{"localhost:443", "0.0.0.0:443", "[::]:443", "192.0.2.1:443", "127.0.0.1:0"} {
		if _, _, err := validateNodeConnectorTransportEndpoint(endpoint, false); err == nil {
			t.Fatalf("invalid connector endpoint %q was accepted", endpoint)
		}
	}
	if _, _, err := validateNodeConnectorTransportEndpoint("127.0.0.1:0", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validateNodeConnectorTransportEndpoint("[::1]:443", false); err != nil {
		t.Fatal(err)
	}

	fixture := newNodeConnectorWireFixture(t)
	config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
		MaxQueuedFrames: 4, MaxQueuedBytes: 4 * NodeConnectorWireMaxBytes,
		MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
	})
	helloFrame := nodeConnectorDuplexHelloFrame(t, fixture, "transport-rejection-001")

	tests := []struct {
		name string
		raw  func() []byte
	}{
		{name: "empty length", raw: func() []byte { return []byte{0, 0, 0, 0} }},
		{name: "oversized declaration", raw: func() []byte {
			raw := make([]byte, 4)
			binary.BigEndian.PutUint32(raw, uint32(limits.MaxRecordBytes+1))
			return raw
		}},
		{name: "malformed body", raw: func() []byte { return nodeConnectorTransportRawPacket([]byte(`{"schema":`)) }},
		{name: "trailing body", raw: func() []byte { return nodeConnectorTransportRawPacket([]byte("{}\n")) }},
		{name: "unsupported kind", raw: func() []byte {
			return nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: "dispatch", ExchangeID: config.ExchangeID, ConfigFingerprint: config.ConfigFingerprint, Direction: NodeConnectorWireConnectorToBroker, Sequence: 1, Frame: helloFrame})
		}},
		{name: "wrong exchange", raw: func() []byte {
			return nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame, ExchangeID: "exchange-substituted-001", ConfigFingerprint: config.ConfigFingerprint, Direction: NodeConnectorWireConnectorToBroker, Sequence: 1, Frame: helloFrame})
		}},
		{name: "wrong configuration", raw: func() []byte {
			return nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame, ExchangeID: config.ExchangeID, ConfigFingerprint: "sha256:" + strings.Repeat("0", 64), Direction: NodeConnectorWireConnectorToBroker, Sequence: 1, Frame: helloFrame})
		}},
		{name: "wrong direction", raw: func() []byte {
			return nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame, ExchangeID: config.ExchangeID, ConfigFingerprint: config.ConfigFingerprint, Direction: NodeConnectorWireBrokerToConnector, Sequence: 1, Frame: helloFrame})
		}},
		{name: "ahead sequence", raw: func() []byte {
			return nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame, ExchangeID: config.ExchangeID, ConfigFingerprint: config.ConfigFingerprint, Direction: NodeConnectorWireConnectorToBroker, Sequence: 2, Frame: helloFrame})
		}},
		{name: "empty frame", raw: func() []byte {
			return nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame, ExchangeID: config.ExchangeID, ConfigFingerprint: config.ConfigFingerprint, Direction: NodeConnectorWireConnectorToBroker, Sequence: 1})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
			before := nodeConnectorDuplexStateBytes(t, exchange.root)
			brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
			defer closePair()
			raw := test.raw()
			if _, err := connectorConnection.Write(raw); err != nil {
				t.Fatal(err)
			}
			if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return nil }, limits); err == nil {
				t.Fatal("invalid transport record was accepted")
			}
			if !nodeConnectorDuplexStateBytesEqual(before, nodeConnectorDuplexStateBytes(t, exchange.root)) {
				t.Fatal("invalid transport record advanced durable duplex state")
			}
		})
	}

	t.Run("partial prefix", func(t *testing.T) {
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
		brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
		defer closePair()
		_, _ = connectorConnection.Write([]byte{0, 0})
		_ = connectorConnection.(*net.TCPConn).CloseWrite()
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return nil }, limits); err == nil {
			t.Fatal("partial length prefix was accepted")
		}
	})

	t.Run("truncated body", func(t *testing.T) {
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
		brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
		defer closePair()
		_, _ = connectorConnection.Write(append([]byte{0, 0, 0, 10}, []byte("xx")...))
		_ = connectorConnection.(*net.TCPConn).CloseWrite()
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return nil }, limits); err == nil {
			t.Fatal("truncated transport body was accepted")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
		short := limits
		short.IOTimeout = 25 * time.Millisecond
		brokerConnection, _, closePair := openNodeConnectorTransportPair(t, short)
		defer closePair()
		before := nodeConnectorDuplexStateBytes(t, exchange.root)
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return nil }, short); err == nil {
			t.Fatal("transport read timeout was accepted")
		}
		if !nodeConnectorDuplexStateBytesEqual(before, nodeConnectorDuplexStateBytes(t, exchange.root)) {
			t.Fatal("timeout advanced durable state")
		}
	})

	t.Run("downstream rejection", func(t *testing.T) {
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
		brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
		defer closePair()
		before := nodeConnectorDuplexStateBytes(t, exchange.root)
		if err := nodeConnectorTransportWriteFrame(connectorConnection, config, NodeConnectorWireConnectorToBroker, 1, helloFrame, limits); err != nil {
			t.Fatal(err)
		}
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return errors.New("rejected") }, limits); err == nil {
			t.Fatal("downstream transport rejection was accepted")
		}
		if !nodeConnectorDuplexStateBytesEqual(before, nodeConnectorDuplexStateBytes(t, exchange.root)) {
			t.Fatal("downstream rejection advanced durable state")
		}
	})

	t.Run("resume and acknowledgement substitution", func(t *testing.T) {
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
		cursor, _ := exchange.Cursor(NodeConnectorWireConnectorToBroker)
		brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
		defer closePair()
		changed := cursor
		changed.AcceptedSequence++
		if err := nodeConnectorTransportWriteResume(connectorConnection, changed, limits); err != nil {
			t.Fatal(err)
		}
		if _, err := nodeConnectorTransportAcceptResume(brokerConnection, exchange, cursor.Direction, limits); err == nil {
			t.Fatal("stale or ahead resume cursor was accepted")
		}
		ack := nodeConnectorTransportRecord{Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportAck, ExchangeID: changed.ExchangeID, ConfigFingerprint: changed.ConfigFingerprint, Direction: changed.Direction, Sequence: 1, Cursor: &changed}
		if err := writeNodeConnectorTransportRecord(brokerConnection, ack, limits); err != nil {
			t.Fatal(err)
		}
		if err := nodeConnectorTransportReadAcknowledgement(connectorConnection, cursor, 1, limits); err == nil {
			t.Fatal("substituted acknowledgement cursor was accepted")
		}
	})

	t.Run("duplicate authenticated frame", func(t *testing.T) {
		isolated := newNodeConnectorWireFixture(t)
		isolatedConfig := nodeConnectorDuplexTestConfig(t, isolated, config.Limits)
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), isolated.wire, isolatedConfig)
		frame := nodeConnectorDuplexHelloFrame(t, isolated, "transport-duplicate-001")
		brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
		if err := nodeConnectorTransportWriteFrame(connectorConnection, isolatedConfig, NodeConnectorWireConnectorToBroker, 1, frame, limits); err != nil {
			t.Fatal(err)
		}
		cursor, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, isolated.now, func(frames [][]byte) error {
			_, acceptErr := isolated.wire.NegotiateSession(frames[0], isolated.now)
			return acceptErr
		}, limits)
		if err != nil {
			t.Fatal(err)
		}
		if err := nodeConnectorTransportReadAcknowledgement(connectorConnection, cursor, 1, limits); err != nil {
			t.Fatal(err)
		}
		closePair()
		before := nodeConnectorDuplexStateBytes(t, exchange.root)
		brokerConnection, connectorConnection, closePair = openNodeConnectorTransportPair(t, limits)
		defer closePair()
		if err := nodeConnectorTransportWriteFrame(connectorConnection, isolatedConfig, NodeConnectorWireConnectorToBroker, 2, frame, limits); err != nil {
			t.Fatal(err)
		}
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, isolated.now, func([][]byte) error { return nil }, limits); err == nil {
			t.Fatal("duplicate authenticated transport frame was accepted")
		}
		if !nodeConnectorDuplexStateBytesEqual(before, nodeConnectorDuplexStateBytes(t, exchange.root)) {
			t.Fatal("duplicate frame rejection advanced durable state")
		}
	})

	t.Run("durable state tamper", func(t *testing.T) {
		isolated := newNodeConnectorWireFixture(t)
		isolatedConfig := nodeConnectorDuplexTestConfig(t, isolated, config.Limits)
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), isolated.wire, isolatedConfig)
		tamperLatestNodeConnectorTransportState(t, exchange)
		brokerConnection, connectorConnection, closePair := openNodeConnectorTransportPair(t, limits)
		defer closePair()
		frame := nodeConnectorDuplexHelloFrame(t, isolated, "transport-tamper-001")
		if err := nodeConnectorTransportWriteFrame(connectorConnection, isolatedConfig, NodeConnectorWireConnectorToBroker, 1, frame, limits); err != nil {
			t.Fatal(err)
		}
		called := false
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, isolated.now, func([][]byte) error { called = true; return nil }, limits); err == nil {
			t.Fatal("tampered durable transport state was accepted")
		}
		if called {
			t.Fatal("durable state tamper reached downstream acceptance")
		}
	})
}

func TestNodeConnectorTransportCarriesNoExecutionOrLifecycleAuthority(t *testing.T) {
	typeOfRecord := reflect.TypeOf(nodeConnectorTransportRecord{})
	names := []string{}
	for index := 0; index < typeOfRecord.NumField(); index++ {
		names = append(names, typeOfRecord.Field(index).Tag.Get("json"))
	}
	serialized := strings.ToLower(strings.Join(names, " "))
	for _, forbidden := range []string{"command", "executor", "validator", "request", "lease", "receipt", "credential", "secret", "token", "approval", "apply", "checkpoint", "commit", "push", "publication", "authority"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("transport record exposes forbidden execution or lifecycle field %q", forbidden)
		}
	}
	limits := nodeConnectorTransportTestLimits()
	broker, err := NewNodeConnectorTransportBrokerListener("127.0.0.1:0", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	address, ok := broker.listener.Addr().(*net.TCPAddr)
	if !ok || address.Port == 0 || !address.IP.IsLoopback() || address.IP.IsUnspecified() {
		t.Fatal("broker listener is not an explicit ephemeral loopback endpoint")
	}
	connector, err := NewNodeConnectorTransportConnector(limits)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.TypeOf(*connector).NumField() != 1 {
		t.Fatal("outbound connector unexpectedly owns listener or lifecycle state")
	}
}

type nodeConnectorTransportShortWriteConn struct {
	net.Conn
	max int
}

func (connection *nodeConnectorTransportShortWriteConn) Write(raw []byte) (int, error) {
	if len(raw) > connection.max {
		raw = raw[:connection.max]
	}
	return connection.Conn.Write(raw)
}

func nodeConnectorTransportTestLimits() NodeConnectorTransportLimits {
	return NodeConnectorTransportLimits{MaxRecordBytes: 96 * 1024, ConnectTimeout: 2 * time.Second, IOTimeout: 2 * time.Second}
}

func openNodeConnectorTransportPair(t *testing.T, limits NodeConnectorTransportLimits) (net.Conn, net.Conn, func()) {
	t.Helper()
	broker, err := NewNodeConnectorTransportBrokerListener("127.0.0.1:0", limits)
	if err != nil {
		t.Fatal(err)
	}
	connector, err := NewNodeConnectorTransportConnector(limits)
	if err != nil {
		_ = broker.Close()
		t.Fatal(err)
	}
	accepted := make(chan struct {
		connection net.Conn
		err        error
	}, 1)
	go func() {
		connection, acceptErr := broker.Accept()
		accepted <- struct {
			connection net.Conn
			err        error
		}{connection, acceptErr}
	}()
	connectorConnection, err := connector.Dial(context.Background(), broker.Endpoint())
	if err != nil {
		_ = broker.Close()
		t.Fatal(err)
	}
	result := <-accepted
	if result.err != nil {
		_ = connectorConnection.Close()
		_ = broker.Close()
		t.Fatal(result.err)
	}
	closed := false
	closePair := func() {
		if closed {
			return
		}
		closed = true
		_ = connectorConnection.Close()
		_ = result.connection.Close()
		_ = broker.Close()
	}
	return result.connection, connectorConnection, closePair
}

func resumeNodeConnectorTransportDirection(t *testing.T, writer net.Conn, reader net.Conn, exchange *NodeConnectorDuplex, direction string, limits NodeConnectorTransportLimits) {
	t.Helper()
	cursor, err := exchange.Cursor(direction)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeConnectorTransportWriteResume(writer, cursor, limits); err != nil {
		t.Fatal(err)
	}
	accepted, err := nodeConnectorTransportAcceptResume(reader, exchange, direction, limits)
	if err != nil || !nodeExecutionEqual(accepted, cursor) {
		t.Fatalf("exact durable transport resume failed: cursor=%#v err=%v", accepted, err)
	}
}

func writeCoalescedNodeConnectorTransportFrames(t *testing.T, connection net.Conn, config NodeConnectorDuplexConfig, direction string, firstSequence int64, frames [][]byte, limits NodeConnectorTransportLimits) {
	t.Helper()
	combined := []byte{}
	for index, frame := range frames {
		combined = append(combined, nodeConnectorTransportPacket(t, nodeConnectorTransportRecord{
			Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame,
			ExchangeID: config.ExchangeID, ConfigFingerprint: config.ConfigFingerprint,
			Direction: direction, Sequence: firstSequence + int64(index), Frame: frame,
		})...)
	}
	if err := connection.SetWriteDeadline(time.Now().Add(limits.IOTimeout)); err != nil {
		t.Fatal(err)
	}
	if err := writeNodeConnectorTransportBytes(connection, combined); err != nil {
		t.Fatal(err)
	}
}

func nodeConnectorTransportPacket(t *testing.T, record nodeConnectorTransportRecord) []byte {
	t.Helper()
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return nodeConnectorTransportRawPacket(raw)
}

func nodeConnectorTransportRawPacket(raw []byte) []byte {
	result := make([]byte, 4, 4+len(raw))
	binary.BigEndian.PutUint32(result, uint32(len(raw)))
	return append(result, raw...)
}

func tamperLatestNodeConnectorTransportState(t *testing.T, exchange *NodeConnectorDuplex) {
	t.Helper()
	path := filepath.Join(exchange.root, nodeConnectorDuplexStateFileName(exchange.state.Generation))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	state["state_fingerprint"] = strings.Repeat("0", 64)
	tampered, _ := json.Marshal(state)
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
}
