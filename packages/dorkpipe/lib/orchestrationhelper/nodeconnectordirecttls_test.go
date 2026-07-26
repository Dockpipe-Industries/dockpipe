package orchestrationhelper

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNodeConnectorDirectTLSCarriesTransportDuplexRestartAndIdempotence(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	limits := nodeConnectorTransportTestLimits()
	config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
		MaxQueuedFrames: 8, MaxQueuedBytes: 8 * NodeConnectorWireMaxBytes,
		MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
	})
	secrets := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	brokerRoot, connectorRoot := t.TempDir(), t.TempDir()
	brokerExchange := mustNodeConnectorDuplex(t, brokerRoot, fixture.wire, config)
	connectorExchange := mustNodeConnectorDuplex(t, connectorRoot, fixture.wire, config)

	brokerConnection, connectorConnection, closePair := openNodeConnectorDirectTLSPair(t, secrets, limits)
	connectorTLS, connectorOK := connectorConnection.(*tls.Conn)
	brokerTLS, brokerOK := brokerConnection.(*tls.Conn)
	if !connectorOK || !brokerOK || connectorTLS.ConnectionState().Version != tls.VersionTLS13 || brokerTLS.ConnectionState().Version != tls.VersionTLS13 {
		t.Fatal("direct TLS pair did not negotiate TLS 1.3")
	}
	connectorWrites := &nodeConnectorTransportShortWriteConn{Conn: connectorConnection, max: 3}
	brokerWrites := &nodeConnectorTransportShortWriteConn{Conn: brokerConnection, max: 5}
	resumeNodeConnectorTransportDirection(t, connectorWrites, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerWrites, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)

	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-direct-tls-initial-001", "replay-negotiation-direct-tls-initial-001", "connection-direct-tls-initial-001", "",
		fixture.enrollment.InitialCredentialID, fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	helloFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-direct-tls-hello-001", ReplayIdentity: "replay-direct-tls-hello-001",
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(connectorWrites, config, NodeConnectorWireConnectorToBroker, 1, helloFrame, limits); err != nil {
		t.Fatal(err)
	}
	var negotiation NodeConnectorSessionNegotiation
	brokerCursor, err := nodeConnectorTransportAcceptFrame(brokerConnection, brokerExchange, fixture.now.Add(time.Second), func(frames [][]byte) error {
		if len(frames) != 1 || !bytes.Equal(frames[0], helloFrame) {
			return errors.New("direct TLS changed split transport record bytes")
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
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-direct-tls-negotiation-001", ReplayIdentity: "replay-direct-tls-negotiation-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireSessionNegotiation, Payload: mustNodeExecutionJSON(t, negotiation),
		IssuedAt: fixture.now.Add(time.Second), ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(brokerWrites, config, NodeConnectorWireBrokerToConnector, 1, negotiationFrame, limits); err != nil {
		t.Fatal(err)
	}
	connectorCursor, err := nodeConnectorTransportAcceptFrame(connectorConnection, connectorExchange, fixture.now.Add(2*time.Second), func(frames [][]byte) error {
		if !bytes.Equal(frames[0], negotiationFrame) {
			return errors.New("direct TLS changed negotiation bytes")
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
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 2, "evidence-direct-tls-connected-001", "replay-direct-tls-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 3, "evidence-direct-tls-healthy-001", "replay-direct-tls-healthy-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(3*time.Second)))
	acceptNodeConnectorWireBrokerOperation(t, fixture)

	brokerConnection, connectorConnection, closePair = openNodeConnectorDirectTLSPair(t, secrets, limits)
	resumeNodeConnectorTransportDirection(t, connectorConnection, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerConnection, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)
	requestFrame := nodeConnectorDuplexOperationFrame(t, fixture, "direct-tls-request-initial-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(4*time.Second))
	leaseFrame := nodeConnectorDuplexOperationFrame(t, fixture, "direct-tls-lease-initial-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(4*time.Second))
	writeCoalescedNodeConnectorTransportFrames(t, brokerConnection, config, NodeConnectorWireBrokerToConnector, 2, [][]byte{requestFrame, leaseFrame}, limits)
	var receipt NodeExecutionReceipt
	connectorCursor, err = nodeConnectorTransportAcceptFrames(connectorConnection, connectorExchange, 2, fixture.now.Add(5*time.Second), func(frames [][]byte) error {
		if len(frames) != 2 || !bytes.Equal(frames[0], requestFrame) || !bytes.Equal(frames[1], leaseFrame) {
			return errors.New("direct TLS changed coalesced records or ordering")
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
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-direct-tls-receipt-001", ReplayIdentity: "replay-direct-tls-receipt-001",
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
	brokerConnection, connectorConnection, closePair = openNodeConnectorDirectTLSPair(t, secrets, limits)
	defer closePair()
	resumeNodeConnectorTransportDirection(t, connectorConnection, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerConnection, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)
	freshRequest := nodeConnectorDuplexOperationFrame(t, fixture, "direct-tls-request-restart-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(8*time.Second))
	freshLease := nodeConnectorDuplexOperationFrame(t, fixture, "direct-tls-lease-restart-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(8*time.Second))
	writeCoalescedNodeConnectorTransportFrames(t, brokerConnection, config, NodeConnectorWireBrokerToConnector, 4, [][]byte{freshRequest, freshLease}, limits)
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
		t.Fatal("direct TLS restart changed the receipt or repeated connector, validator, or executor work")
	}
}

func TestNodeConnectorDirectTLSFailsClosedWithoutStateOrPlaintextFallback(t *testing.T) {
	limits := nodeConnectorTransportTestLimits()
	valid := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	t.Run("wrong CA", func(t *testing.T) {
		other := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
		expectNodeConnectorDirectTLSHandshakeFailure(t, valid, other.values[other.rootsReference], "broker.test", limits)
	})
	t.Run("wrong server identity", func(t *testing.T) {
		expectNodeConnectorDirectTLSHandshakeFailure(t, valid, valid.values[valid.rootsReference], "wrong.test", limits)
	})
	t.Run("expired certificate", func(t *testing.T) {
		expired := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
		expectNodeConnectorDirectTLSHandshakeFailure(t, expired, expired.values[expired.rootsReference], "broker.test", limits)
	})
	t.Run("not yet valid certificate", func(t *testing.T) {
		future := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
		expectNodeConnectorDirectTLSHandshakeFailure(t, future, future.values[future.rootsReference], "broker.test", limits)
	})

	t.Run("handshake timeout", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		closed := make(chan struct{})
		go func() {
			connection, acceptErr := listener.Accept()
			if acceptErr == nil {
				<-closed
				_ = connection.Close()
			}
		}()
		connector := mustNodeConnectorDirectTLSConnector(t, listener.Addr().String(), "broker.test", valid.values[valid.rootsReference], 25*time.Millisecond, limits)
		if connection, dialErr := connector.Dial(context.Background()); dialErr == nil {
			_ = connection.Close()
			t.Fatal("stalled direct TLS handshake succeeded")
		}
		close(closed)
	})

	t.Run("unexpected peer closure", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		go func() {
			connection, _ := listener.Accept()
			if connection != nil {
				_ = connection.Close()
			}
		}()
		connector := mustNodeConnectorDirectTLSConnector(t, listener.Addr().String(), "broker.test", valid.values[valid.rootsReference], time.Second, limits)
		if connection, dialErr := connector.Dial(context.Background()); dialErr == nil {
			_ = connection.Close()
			t.Fatal("closed non-TLS peer was accepted")
		}
	})

	t.Run("non TLS peer and no plaintext fallback", func(t *testing.T) {
		broker := mustNodeConnectorDirectTLSBroker(t, valid, limits)
		defer broker.Close()
		accepted := make(chan error, 1)
		go func() {
			connection, acceptErr := broker.Accept()
			if connection != nil {
				_ = connection.Close()
			}
			accepted <- acceptErr
		}()
		raw, err := net.DialTimeout("tcp", broker.Endpoint(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer raw.Close()
		_, _ = raw.Write([]byte("not tls"))
		_ = raw.SetReadDeadline(time.Now().Add(time.Second))
		response := make([]byte, 1024)
		count, _ := raw.Read(response)
		if strings.Contains(string(response[:count]), NodeConnectorTransportRecordSchema) {
			t.Fatal("raw TCP received a plaintext transport record fallback")
		}
		if acceptErr := <-accepted; acceptErr == nil {
			t.Fatal("non-TLS peer was accepted")
		}
	})

	t.Run("downstream rejection emits no acknowledgement or state", func(t *testing.T) {
		fixture := newNodeConnectorWireFixture(t)
		config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
			MaxQueuedFrames: 4, MaxQueuedBytes: 4 * NodeConnectorWireMaxBytes,
			MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
		})
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, config)
		before := nodeConnectorDuplexStateBytes(t, exchange.root)
		brokerConnection, connectorConnection, closePair := openNodeConnectorDirectTLSPair(t, valid, limits)
		defer closePair()
		frame := nodeConnectorDuplexHelloFrame(t, fixture, "direct-tls-downstream-rejection-001")
		if err := nodeConnectorTransportWriteFrame(connectorConnection, config, NodeConnectorWireConnectorToBroker, 1, frame, limits); err != nil {
			t.Fatal(err)
		}
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return errors.New("rejected") }, limits); err == nil {
			t.Fatal("downstream rejection was accepted over direct TLS")
		}
		if !nodeConnectorDuplexStateBytesEqual(before, nodeConnectorDuplexStateBytes(t, exchange.root)) {
			t.Fatal("rejected direct TLS record mutated durable duplex state")
		}
		short := limits
		short.IOTimeout = 25 * time.Millisecond
		if _, err := readNodeConnectorTransportRecord(connectorConnection, short); err == nil {
			t.Fatal("downstream rejection emitted an acknowledgement")
		}
	})
}

func TestNodeConnectorDirectTLSSecretReferencesErrorsAndAuthoritySurface(t *testing.T) {
	limits := nodeConnectorTransportTestLimits()
	valid := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	brokerConfig := valid.brokerConfig("127.0.0.1:0", time.Second)
	connectorConfig := NodeConnectorDirectTLSConnectorConfig{Endpoint: "127.0.0.1:443", ExpectedServerIdentity: "broker.test", TrustRootsReference: valid.rootsReference, HandshakeTimeout: time.Second}

	serialized, err := json.Marshal([]any{brokerConfig, connectorConfig})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range valid.values {
		if bytes.Contains(serialized, secret) {
			t.Fatal("serialized direct TLS configuration contains resolved certificate or private-key bytes")
		}
	}

	leakingLoader := func(string) ([]byte, error) { return nil, errors.New("resolved-secret-marker") }
	if _, err := NewNodeConnectorDirectTLSBrokerListener(brokerConfig, limits, leakingLoader); err == nil || strings.Contains(err.Error(), "resolved-secret-marker") {
		t.Fatal("broker loader failure was accepted or leaked secret-bearing error text")
	}
	if _, err := NewNodeConnectorDirectTLSConnector(connectorConfig, limits, leakingLoader); err == nil || strings.Contains(err.Error(), "resolved-secret-marker") {
		t.Fatal("connector loader failure was accepted or leaked secret-bearing error text")
	}

	tests := []struct {
		name   string
		values map[string][]byte
		config NodeConnectorDirectTLSBrokerConfig
	}{
		{name: "missing reference", values: valid.values, config: NodeConnectorDirectTLSBrokerConfig{ListenEndpoint: "127.0.0.1:0", PrivateKeyReference: valid.keyReference, HandshakeTimeout: time.Second}},
		{name: "unknown reference", values: valid.values, config: NodeConnectorDirectTLSBrokerConfig{ListenEndpoint: "127.0.0.1:0", CertificateChainReference: "ref-unknown-certificate", PrivateKeyReference: valid.keyReference, HandshakeTimeout: time.Second}},
		{name: "empty resolved certificate", values: map[string][]byte{valid.certificateReference: {}, valid.keyReference: valid.values[valid.keyReference]}, config: brokerConfig},
		{name: "malformed certificate", values: map[string][]byte{valid.certificateReference: []byte("not a certificate"), valid.keyReference: valid.values[valid.keyReference]}, config: brokerConfig},
		{name: "malformed private key", values: map[string][]byte{valid.certificateReference: valid.values[valid.certificateReference], valid.keyReference: []byte("not a private key")}, config: brokerConfig},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if broker, createErr := NewNodeConnectorDirectTLSBrokerListener(test.config, limits, nodeConnectorDirectTLSMapLoader(test.values)); createErr == nil {
				_ = broker.Close()
				t.Fatal("invalid direct TLS secret reference or material was accepted")
			}
		})
	}

	mismatch := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	mismatchValues := cloneNodeConnectorDirectTLSValues(valid.values)
	mismatchValues[valid.keyReference] = mismatch.values[mismatch.keyReference]
	if broker, err := NewNodeConnectorDirectTLSBrokerListener(brokerConfig, limits, nodeConnectorDirectTLSMapLoader(mismatchValues)); err == nil {
		_ = broker.Close()
		t.Fatal("mismatched direct TLS key pair was accepted")
	}
	badRoots := cloneNodeConnectorDirectTLSValues(valid.values)
	badRoots[valid.rootsReference] = []byte("not trust roots")
	if _, err := NewNodeConnectorDirectTLSConnector(connectorConfig, limits, nodeConnectorDirectTLSMapLoader(badRoots)); err == nil {
		t.Fatal("malformed direct TLS trust roots were accepted")
	}
	for name, changed := range map[string]NodeConnectorDirectTLSConnectorConfig{
		"missing trust reference": {Endpoint: connectorConfig.Endpoint, ExpectedServerIdentity: connectorConfig.ExpectedServerIdentity, HandshakeTimeout: time.Second},
		"unknown trust reference": {Endpoint: connectorConfig.Endpoint, ExpectedServerIdentity: connectorConfig.ExpectedServerIdentity, TrustRootsReference: "ref-unknown-trust-roots", HandshakeTimeout: time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewNodeConnectorDirectTLSConnector(changed, limits, nodeConnectorDirectTLSMapLoader(valid.values)); err == nil {
				t.Fatal("missing or unknown direct TLS trust reference was accepted")
			}
		})
	}

	for _, endpoint := range []string{"localhost:443", "*:443", "0.0.0.0:443", "[::]:443", "example.test:443", "127.0.0.1:0"} {
		if _, _, err := validateNodeConnectorDirectTLSEndpoint(endpoint, false); err == nil {
			t.Fatalf("invalid direct TLS connector endpoint %q was accepted", endpoint)
		}
	}

	fieldText := ""
	for _, value := range []any{NodeConnectorDirectTLSBrokerConfig{}, NodeConnectorDirectTLSConnectorConfig{}, NodeConnectorDirectTLSBrokerListener{}, NodeConnectorDirectTLSConnector{}} {
		typeOf := reflect.TypeOf(value)
		for index := 0; index < typeOf.NumField(); index++ {
			fieldText += " " + strings.ToLower(typeOf.Field(index).Name+" "+typeOf.Field(index).Tag.Get("json"))
		}
	}
	for _, forbidden := range []string{"command", "executor", "validator", "request", "lease", "receipt", "approval", "mutation", "apply", "git", "checkpoint", "commit", "push", "publication", "authority"} {
		if strings.Contains(fieldText, forbidden) {
			t.Fatalf("direct TLS adapter exposes forbidden authority field %q", forbidden)
		}
	}
}

type nodeConnectorDirectTLSSecrets struct {
	values               map[string][]byte
	certificateReference string
	keyReference         string
	rootsReference       string
}

func newNodeConnectorDirectTLSSecrets(t *testing.T, notBefore, notAfter time.Time) nodeConnectorDirectTLSSecrets {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "direct TLS test root"},
		NotBefore: time.Now().Add(-24 * time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, IsCA: true, BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "broker.test"}, DNSNames: []string{"broker.test"},
		NotBefore: notBefore, NotAfter: notAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	certificateReference, keyReference, rootsReference := "ref-broker-certificate", "ref-broker-private-key", "ref-broker-trust-roots"
	return nodeConnectorDirectTLSSecrets{
		certificateReference: certificateReference, keyReference: keyReference, rootsReference: rootsReference,
		values: map[string][]byte{
			certificateReference: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}),
			keyReference:         pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
			rootsReference:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		},
	}
}

func (secrets nodeConnectorDirectTLSSecrets) brokerConfig(endpoint string, timeout time.Duration) NodeConnectorDirectTLSBrokerConfig {
	return NodeConnectorDirectTLSBrokerConfig{ListenEndpoint: endpoint, CertificateChainReference: secrets.certificateReference, PrivateKeyReference: secrets.keyReference, HandshakeTimeout: timeout}
}

func nodeConnectorDirectTLSMapLoader(values map[string][]byte) NodeConnectorDirectTLSSecretReferenceLoader {
	return func(reference string) ([]byte, error) {
		value, ok := values[reference]
		if !ok || len(value) == 0 {
			return nil, errors.New("unavailable local secret reference")
		}
		return append([]byte{}, value...), nil
	}
}

func cloneNodeConnectorDirectTLSValues(values map[string][]byte) map[string][]byte {
	result := map[string][]byte{}
	for key, value := range values {
		result[key] = append([]byte{}, value...)
	}
	return result
}

func mustNodeConnectorDirectTLSBroker(t *testing.T, secrets nodeConnectorDirectTLSSecrets, limits NodeConnectorTransportLimits) *NodeConnectorDirectTLSBrokerListener {
	t.Helper()
	broker, err := NewNodeConnectorDirectTLSBrokerListener(secrets.brokerConfig("127.0.0.1:0", time.Second), limits, nodeConnectorDirectTLSMapLoader(secrets.values))
	if err != nil {
		t.Fatal(err)
	}
	return broker
}

func mustNodeConnectorDirectTLSConnector(t *testing.T, endpoint, identity string, roots []byte, timeout time.Duration, limits NodeConnectorTransportLimits) *NodeConnectorDirectTLSConnector {
	t.Helper()
	values := map[string][]byte{"ref-broker-trust-roots": roots}
	connector, err := NewNodeConnectorDirectTLSConnector(NodeConnectorDirectTLSConnectorConfig{
		Endpoint: endpoint, ExpectedServerIdentity: identity, TrustRootsReference: "ref-broker-trust-roots", HandshakeTimeout: timeout,
	}, limits, nodeConnectorDirectTLSMapLoader(values))
	if err != nil {
		t.Fatal(err)
	}
	return connector
}

func openNodeConnectorDirectTLSPair(t *testing.T, secrets nodeConnectorDirectTLSSecrets, limits NodeConnectorTransportLimits) (net.Conn, net.Conn, func()) {
	t.Helper()
	broker := mustNodeConnectorDirectTLSBroker(t, secrets, limits)
	connector := mustNodeConnectorDirectTLSConnector(t, broker.Endpoint(), "broker.test", secrets.values[secrets.rootsReference], time.Second, limits)
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
	connectorConnection, err := connector.Dial(context.Background())
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
	return result.connection, connectorConnection, func() {
		if closed {
			return
		}
		closed = true
		_ = connectorConnection.Close()
		_ = result.connection.Close()
		_ = broker.Close()
	}
}

func expectNodeConnectorDirectTLSHandshakeFailure(t *testing.T, server nodeConnectorDirectTLSSecrets, roots []byte, identity string, limits NodeConnectorTransportLimits) {
	t.Helper()
	broker := mustNodeConnectorDirectTLSBroker(t, server, limits)
	defer broker.Close()
	accepted := make(chan error, 1)
	go func() {
		connection, err := broker.Accept()
		if connection != nil {
			_ = connection.Close()
		}
		accepted <- err
	}()
	connector := mustNodeConnectorDirectTLSConnector(t, broker.Endpoint(), identity, roots, time.Second, limits)
	if connection, err := connector.Dial(context.Background()); err == nil {
		_ = connection.Close()
		t.Fatal("invalid direct TLS server was accepted")
	}
	if err := <-accepted; err == nil {
		t.Fatal("broker unexpectedly completed rejected direct TLS handshake")
	}
}
