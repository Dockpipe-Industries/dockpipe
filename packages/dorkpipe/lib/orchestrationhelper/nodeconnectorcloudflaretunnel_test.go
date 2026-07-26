package orchestrationhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNodeConnectorCloudflareTunnelCarriesDirectTLSTransportRestartAndIdempotence(t *testing.T) {
	fixture := newNodeConnectorWireFixture(t)
	limits := nodeConnectorTransportTestLimits()
	config := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
		MaxQueuedFrames: 8, MaxQueuedBytes: 8 * NodeConnectorWireMaxBytes,
		MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
	})
	secrets := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	starter := newNodeConnectorCloudflareTunnelTestStarter(t)
	brokerRoot, connectorRoot := t.TempDir(), t.TempDir()
	brokerExchange := mustNodeConnectorDuplex(t, brokerRoot, fixture.wire, config)
	connectorExchange := mustNodeConnectorDuplex(t, connectorRoot, fixture.wire, config)

	brokerConnection, connectorConnection, closePair := openNodeConnectorCloudflareTunnelPair(t, starter, secrets, limits)
	connectorWrites := &nodeConnectorTransportShortWriteConn{Conn: connectorConnection, max: 3}
	brokerWrites := &nodeConnectorTransportShortWriteConn{Conn: brokerConnection, max: 5}
	resumeNodeConnectorTransportDirection(t, connectorWrites, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerWrites, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)

	hello := nodeConnectorSessionHello(t, &nodeConnectorSessionFixture{execution: wireExecutionFixture(fixture), enrollment: fixture.enrollment}, 1,
		"negotiation-cloudflare-initial-001", "replay-negotiation-cloudflare-initial-001", "connection-cloudflare-initial-001", "",
		fixture.enrollment.InitialCredentialID, fixture.capability.SnapshotID, fixture.now.Add(time.Second))
	helloFrame := mustNodeConnectorWireFrame(t, fixture, NodeConnectorWireFrameInput{
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-cloudflare-hello-001", ReplayIdentity: "replay-cloudflare-hello-001",
		CredentialReference: hello.CredentialID, MessageKind: NodeConnectorWireSessionHello, Payload: mustNodeExecutionJSON(t, hello),
		IssuedAt: fixture.now, ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(connectorWrites, config, NodeConnectorWireConnectorToBroker, 1, helloFrame, limits); err != nil {
		t.Fatal(err)
	}
	var negotiation NodeConnectorSessionNegotiation
	brokerCursor, err := nodeConnectorTransportAcceptFrame(brokerConnection, brokerExchange, fixture.now.Add(time.Second), func(frames [][]byte) error {
		if len(frames) != 1 || !bytes.Equal(frames[0], helloFrame) {
			return errors.New("Cloudflare Tunnel changed split direct-TLS transport bytes")
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
		Direction: NodeConnectorWireBrokerToConnector, FrameID: "frame-cloudflare-negotiation-001", ReplayIdentity: "replay-cloudflare-negotiation-001",
		CredentialReference: fixture.wire.brokerCredential, MessageKind: NodeConnectorWireSessionNegotiation, Payload: mustNodeExecutionJSON(t, negotiation),
		IssuedAt: fixture.now.Add(time.Second), ExpiresAt: fixture.now.Add(time.Minute),
	})
	if err := nodeConnectorTransportWriteFrame(brokerWrites, config, NodeConnectorWireBrokerToConnector, 1, negotiationFrame, limits); err != nil {
		t.Fatal(err)
	}
	connectorCursor, err := nodeConnectorTransportAcceptFrame(connectorConnection, connectorExchange, fixture.now.Add(2*time.Second), func(frames [][]byte) error {
		if !bytes.Equal(frames[0], negotiationFrame) {
			return errors.New("Cloudflare Tunnel changed broker direct-TLS transport bytes")
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
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 2, "evidence-cloudflare-connected-001", "replay-cloudflare-connected-001", "presence", "connected", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(2*time.Second)))
	mustRecordNodeConnectorEvidence(t, fixture.session, nodeConnectorSessionEvidence(t, sessionFixture, 3, "evidence-cloudflare-healthy-001", "replay-cloudflare-healthy-001", "health", "healthy", negotiation.SessionID, negotiation.ConnectionID, negotiation.CredentialID, "", negotiation.CapabilitySnapshotID, fixture.now.Add(3*time.Second)))
	acceptNodeConnectorWireBrokerOperation(t, fixture)

	brokerConnection, connectorConnection, closePair = openNodeConnectorCloudflareTunnelPair(t, starter, secrets, limits)
	resumeNodeConnectorTransportDirection(t, connectorConnection, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerConnection, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)
	requestFrame := nodeConnectorDuplexOperationFrame(t, fixture, "cloudflare-request-initial-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(4*time.Second))
	leaseFrame := nodeConnectorDuplexOperationFrame(t, fixture, "cloudflare-lease-initial-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(4*time.Second))
	writeCoalescedNodeConnectorTransportFrames(t, brokerConnection, config, NodeConnectorWireBrokerToConnector, 2, [][]byte{requestFrame, leaseFrame}, limits)
	var receipt NodeExecutionReceipt
	connectorCursor, err = nodeConnectorTransportAcceptFrames(connectorConnection, connectorExchange, 2, fixture.now.Add(5*time.Second), func(frames [][]byte) error {
		if len(frames) != 2 || !bytes.Equal(frames[0], requestFrame) || !bytes.Equal(frames[1], leaseFrame) {
			return errors.New("Cloudflare Tunnel changed coalesced records or directional ordering")
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
		Direction: NodeConnectorWireConnectorToBroker, FrameID: "frame-cloudflare-receipt-001", ReplayIdentity: "replay-cloudflare-receipt-001",
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
	brokerConnection, connectorConnection, closePair = openNodeConnectorCloudflareTunnelPair(t, starter, secrets, limits)
	defer closePair()
	resumeNodeConnectorTransportDirection(t, connectorConnection, brokerConnection, brokerExchange, NodeConnectorWireConnectorToBroker, limits)
	resumeNodeConnectorTransportDirection(t, brokerConnection, connectorConnection, connectorExchange, NodeConnectorWireBrokerToConnector, limits)
	freshRequest := nodeConnectorDuplexOperationFrame(t, fixture, "cloudflare-request-restart-001", NodeConnectorWireExecutionRequest, mustNodeExecutionJSON(t, fixture.request), fixture.now.Add(8*time.Second))
	freshLease := nodeConnectorDuplexOperationFrame(t, fixture, "cloudflare-lease-restart-001", NodeConnectorWireTaskLease, mustNodeExecutionJSON(t, fixture.lease), fixture.now.Add(8*time.Second))
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
		t.Fatal("Cloudflare Tunnel restart changed the receipt or repeated connector, validator, or executor work")
	}
}

func TestNodeConnectorCloudflareTunnelCommandsReferencesAndAuthoritySurface(t *testing.T) {
	limits := nodeConnectorTransportTestLimits()
	secrets := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	starter := newNodeConnectorCloudflareTunnelTestStarter(t)
	brokerConnection, connectorConnection, closePair := openNodeConnectorCloudflareTunnelPair(t, starter, secrets, limits)
	_ = brokerConnection.Close()
	_ = connectorConnection.Close()
	closePair()
	if nodeConnectorCloudflareTunnelTempEntries(t, starter.temporaryRoot) != 0 {
		t.Fatal("successful Cloudflare Tunnel shutdown left temporary configuration or pid state")
	}

	calls := starter.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected one origin and one client process, got %d", len(calls))
	}
	origin, client := calls[0], calls[1]
	if len(origin.arguments) != 8 || origin.arguments[0] != "tunnel" || origin.arguments[1] != "--config" || origin.arguments[3] != "--no-autoupdate" || origin.arguments[4] != "--pidfile" || origin.arguments[6] != "run" || origin.arguments[7] != nodeConnectorCloudflareTunnelTestID {
		t.Fatalf("unexpected documented origin arguments: %#v", origin.arguments)
	}
	if !reflect.DeepEqual(client.arguments, []string{"access", "tcp", "--hostname", nodeConnectorCloudflareTunnelTestHostname, "--url", client.proxyEndpoint}) {
		t.Fatalf("unexpected documented client arguments: %#v", client.arguments)
	}
	credentialPath := filepath.ToSlash(starter.credentialPath)
	for _, required := range []string{
		"tunnel: \"" + nodeConnectorCloudflareTunnelTestID + "\"",
		"credentials-file: \"" + strings.ReplaceAll(credentialPath, "\\", "\\\\") + "\"",
		"hostname: \"" + nodeConnectorCloudflareTunnelTestHostname + "\"",
		"service: \"tcp://" + origin.brokerEndpoint + "\"",
		"service: http_status:404",
	} {
		if !bytes.Contains(origin.configuration, []byte(required)) {
			t.Fatalf("origin configuration omitted %q: %s", required, origin.configuration)
		}
	}
	if bytes.Contains(origin.configuration, []byte(nodeConnectorCloudflareTunnelSecretMarker)) {
		t.Fatal("temporary origin configuration contains credential bytes")
	}
	serialized, err := json.Marshal([]any{starter.originConfig, starter.clientConfig})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(serialized, []byte(nodeConnectorCloudflareTunnelSecretMarker)) || bytes.Contains(serialized, []byte(starter.credentialPath)) {
		t.Fatal("serialized Cloudflare Tunnel configuration contains a resolved path or credential bytes")
	}
	for _, call := range calls {
		joined := strings.Join(append([]string{call.executable}, call.arguments...), " ")
		if strings.Contains(joined, nodeConnectorCloudflareTunnelSecretMarker) || strings.Contains(joined, "--token") || strings.Contains(joined, "TUNNEL_TOKEN") {
			t.Fatal("Cloudflare Tunnel argv contains token or credential material")
		}
	}

	fieldText := ""
	for _, value := range []any{NodeConnectorCloudflareTunnelOriginConfig{}, NodeConnectorCloudflareTunnelClientConfig{}, NodeConnectorCloudflareTunnelOrigin{}, NodeConnectorCloudflareTunnelClient{}} {
		typeOf := reflect.TypeOf(value)
		for index := 0; index < typeOf.NumField(); index++ {
			fieldText += " " + strings.ToLower(typeOf.Field(index).Name+" "+typeOf.Field(index).Tag.Get("json"))
		}
	}
	for _, forbidden := range []string{"command", "executor", "validator", "request", "lease", "receipt", "approval", "mutation", "apply", "git", "checkpoint", "commit", "push", "publication", "authority"} {
		if strings.Contains(fieldText, forbidden) {
			t.Fatalf("Cloudflare Tunnel adapter exposes forbidden authority field %q", forbidden)
		}
	}
}

func TestNodeConnectorCloudflareTunnelFailsClosedAndCleansUp(t *testing.T) {
	limits := nodeConnectorTransportTestLimits()
	secrets := newNodeConnectorDirectTLSSecrets(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	starter := newNodeConnectorCloudflareTunnelTestStarter(t)
	broker := mustNodeConnectorDirectTLSBroker(t, secrets, limits)
	defer broker.Close()
	originConfig := starter.originConfiguration(broker.Endpoint())

	t.Run("credential resolution redacts errors and leaves no state", func(t *testing.T) {
		root := t.TempDir()
		changed := originConfig
		changed.TemporaryRoot = root
		_, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), changed, starter.executableResolver, func(string) (string, error) {
			return "", errors.New(nodeConnectorCloudflareTunnelSecretMarker)
		}, starter.start)
		if err == nil || strings.Contains(err.Error(), nodeConnectorCloudflareTunnelSecretMarker) || nodeConnectorCloudflareTunnelTempEntries(t, root) != 0 {
			t.Fatal("credential resolution failure leaked details or temporary state")
		}
	})

	t.Run("empty and unknown credential references", func(t *testing.T) {
		for _, reference := range []string{"", "ref-unknown-cloudflare-credential"} {
			changed := originConfig
			changed.CredentialReference = reference
			resolver := starter.credentialResolver
			if reference == "" {
				resolver = starter.credentialResolver
			}
			if origin, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), changed, starter.executableResolver, resolver, starter.start); err == nil {
				_ = origin.Close()
				t.Fatalf("credential reference %q was accepted", reference)
			}
		}
	})

	t.Run("missing executable and unsupported declared compatibility", func(t *testing.T) {
		missing := starter.executable
		missing.Path = filepath.Join(t.TempDir(), "missing-cloudflared")
		unsupportedVersion := starter.executable
		unsupportedVersion.Capabilities.Version = "unknown"
		unsupportedCapability := starter.executable
		unsupportedCapability.Capabilities.AccessTCP = false
		for name, resolved := range map[string]NodeConnectorCloudflareTunnelExecutable{"missing": missing, "version": unsupportedVersion, "capability": unsupportedCapability} {
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				changed := originConfig
				changed.TemporaryRoot = root
				if origin, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), changed, func(string) (NodeConnectorCloudflareTunnelExecutable, error) { return resolved, nil }, starter.credentialResolver, starter.start); err == nil {
					_ = origin.Close()
					t.Fatal("missing or incompatible executable was accepted")
				}
				if nodeConnectorCloudflareTunnelTempEntries(t, root) != 0 {
					t.Fatal("incompatible executable left temporary state")
				}
			})
		}
	})

	t.Run("process start failure is redacted", func(t *testing.T) {
		root := t.TempDir()
		changed := originConfig
		changed.TemporaryRoot = root
		_, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), changed, starter.executableResolver, starter.credentialResolver, func(string, []string) (NodeConnectorCloudflareTunnelProcess, error) {
			return nil, errors.New(nodeConnectorCloudflareTunnelSecretMarker)
		})
		if err == nil || strings.Contains(err.Error(), nodeConnectorCloudflareTunnelSecretMarker) || nodeConnectorCloudflareTunnelTempEntries(t, root) != 0 {
			t.Fatal("process start failure leaked details or temporary state")
		}
	})

	for _, test := range []struct {
		name     string
		behavior string
		cancel   bool
	}{
		{name: "readiness timeout", behavior: "no_pid"},
		{name: "readiness cancellation", behavior: "no_pid", cancel: true},
		{name: "early exit", behavior: "exit"},
		{name: "invalid pid evidence", behavior: "bad_pid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			localStarter := newNodeConnectorCloudflareTunnelTestStarter(t)
			localStarter.originBehavior = test.behavior
			changed := localStarter.originConfiguration(broker.Endpoint())
			changed.StartupTimeout = 75 * time.Millisecond
			ctx := context.Background()
			var cancel context.CancelFunc
			if test.cancel {
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if origin, err := StartNodeConnectorCloudflareTunnelOrigin(ctx, changed, localStarter.executableResolver, localStarter.credentialResolver, localStarter.start); err == nil {
				_ = origin.Close()
				t.Fatal("invalid readiness state was accepted")
			}
			if nodeConnectorCloudflareTunnelTempEntries(t, localStarter.temporaryRoot) != 0 {
				t.Fatal("readiness failure left temporary state")
			}
		})
	}

	t.Run("client early exit and connection timeout", func(t *testing.T) {
		for _, behavior := range []string{"exit", "no_listener"} {
			localStarter := newNodeConnectorCloudflareTunnelTestStarter(t)
			localStarter.clientBehavior = behavior
			proxy := nodeConnectorCloudflareTunnelFreeEndpoint(t)
			clientConfig := localStarter.clientConfiguration(proxy)
			clientConfig.StartupTimeout = 100 * time.Millisecond
			directTLS := NodeConnectorDirectTLSConnectorConfig{Endpoint: proxy, ExpectedServerIdentity: "broker.test", TrustRootsReference: secrets.rootsReference, HandshakeTimeout: 50 * time.Millisecond}
			if client, connection, err := StartNodeConnectorCloudflareTunnelClient(context.Background(), clientConfig, directTLS, limits, nodeConnectorDirectTLSMapLoader(secrets.values), localStarter.executableResolver, localStarter.start); err == nil {
				_ = connection.Close()
				_ = client.Close()
				t.Fatalf("client behavior %q unexpectedly connected", behavior)
			}
		}
	})

	t.Run("corruption fails at existing TLS boundary", func(t *testing.T) {
		localStarter := newNodeConnectorCloudflareTunnelTestStarter(t)
		localStarter.originBehavior = "corrupt"
		origin, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), localStarter.originConfiguration(broker.Endpoint()), localStarter.executableResolver, localStarter.credentialResolver, localStarter.start)
		if err != nil {
			t.Fatal(err)
		}
		defer origin.Close()
		accepted := make(chan error, 1)
		go func() {
			connection, acceptErr := broker.Accept()
			if connection != nil {
				_ = connection.Close()
			}
			accepted <- acceptErr
		}()
		proxy := nodeConnectorCloudflareTunnelFreeEndpoint(t)
		directTLS := NodeConnectorDirectTLSConnectorConfig{Endpoint: proxy, ExpectedServerIdentity: "broker.test", TrustRootsReference: secrets.rootsReference, HandshakeTimeout: 250 * time.Millisecond}
		if client, connection, err := StartNodeConnectorCloudflareTunnelClient(context.Background(), localStarter.clientConfiguration(proxy), directTLS, limits, nodeConnectorDirectTLSMapLoader(secrets.values), localStarter.executableResolver, localStarter.start); err == nil {
			_ = connection.Close()
			_ = client.Close()
			t.Fatal("corrupted Cloudflare transport passed direct TLS")
		}
		if err := <-accepted; err == nil {
			t.Fatal("broker accepted corrupted direct TLS bytes")
		}
	})

	t.Run("unexpected exit is redacted and cleanup remains bounded", func(t *testing.T) {
		localStarter := newNodeConnectorCloudflareTunnelTestStarter(t)
		localStarter.originBehavior = "exit_after_pid"
		origin, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), localStarter.originConfiguration(broker.Endpoint()), localStarter.executableResolver, localStarter.credentialResolver, localStarter.start)
		if err != nil {
			t.Fatal(err)
		}
		select {
		case exitErr := <-origin.Done():
			if exitErr == nil || exitErr.Error() != "Cloudflare Tunnel origin process exited" {
				t.Fatalf("unexpected exit was not generic and redacted: %v", exitErr)
			}
		case <-time.After(time.Second):
			t.Fatal("origin did not publish bounded unexpected-exit evidence")
		}
		if err := origin.Close(); err != nil {
			t.Fatal(err)
		}
		if nodeConnectorCloudflareTunnelTempEntries(t, localStarter.temporaryRoot) != 0 {
			t.Fatal("unexpected exit left temporary provider state")
		}
	})

	t.Run("failure before durable acceptance emits no state or acknowledgement", func(t *testing.T) {
		fixture := newNodeConnectorWireFixture(t)
		duplexConfig := nodeConnectorDuplexTestConfig(t, fixture, NodeConnectorDuplexLimits{
			MaxQueuedFrames: 4, MaxQueuedBytes: 4 * NodeConnectorWireMaxBytes,
			MaxInFlightFrames: 4, MaxInFlightBytes: 4 * NodeConnectorWireMaxBytes, MaxFrameBytes: NodeConnectorWireMaxBytes,
		})
		exchange := mustNodeConnectorDuplex(t, t.TempDir(), fixture.wire, duplexConfig)
		before := nodeConnectorDuplexStateBytes(t, exchange.root)
		localStarter := newNodeConnectorCloudflareTunnelTestStarter(t)
		brokerConnection, connectorConnection, closePair := openNodeConnectorCloudflareTunnelPair(t, localStarter, secrets, limits)
		defer closePair()
		frame := nodeConnectorDuplexHelloFrame(t, fixture, "cloudflare-downstream-rejection-001")
		if err := nodeConnectorTransportWriteFrame(connectorConnection, duplexConfig, NodeConnectorWireConnectorToBroker, 1, frame, limits); err != nil {
			t.Fatal(err)
		}
		if _, err := nodeConnectorTransportAcceptFrame(brokerConnection, exchange, fixture.now, func([][]byte) error { return errors.New("rejected") }, limits); err == nil {
			t.Fatal("downstream rejection was accepted through Cloudflare Tunnel")
		}
		if !nodeConnectorDuplexStateBytesEqual(before, nodeConnectorDuplexStateBytes(t, exchange.root)) {
			t.Fatal("rejected Cloudflare Tunnel record mutated durable duplex state")
		}
		short := limits
		short.IOTimeout = 25 * time.Millisecond
		if _, err := readNodeConnectorTransportRecord(connectorConnection, short); err == nil {
			t.Fatal("downstream rejection emitted an acknowledgement")
		}
	})

	t.Run("invalid endpoints hostnames and bounds", func(t *testing.T) {
		for _, endpoint := range []string{"localhost:443", "*:443", "0.0.0.0:443", "[::]:443", "192.0.2.1:443", "127.0.0.1:0", "bad"} {
			changed := starter.clientConfiguration(endpoint)
			if err := validateNodeConnectorCloudflareTunnelClientConfig(changed); err == nil {
				t.Fatalf("invalid proxy endpoint %q was accepted", endpoint)
			}
		}
		for _, hostname := range []string{"", "*.example.com", "EXAMPLE.com", "localhost", "127.0.0.1", "bad_name.example.com"} {
			changed := starter.clientConfiguration("127.0.0.1:443")
			changed.PublicHostname = hostname
			if err := validateNodeConnectorCloudflareTunnelClientConfig(changed); err == nil {
				t.Fatalf("invalid hostname %q was accepted", hostname)
			}
		}
		for _, timeout := range []time.Duration{0, -time.Second, nodeConnectorCloudflareMaxTimeout + time.Second} {
			changed := starter.clientConfiguration("127.0.0.1:443")
			changed.StartupTimeout = timeout
			if err := validateNodeConnectorCloudflareTunnelClientConfig(changed); err == nil {
				t.Fatalf("invalid timeout %s was accepted", timeout)
			}
		}
	})
}

const (
	nodeConnectorCloudflareTunnelTestID            = "6ff42ae2-765d-4adf-8112-31c55c1551ef"
	nodeConnectorCloudflareTunnelTestHostname      = "broker.example.test"
	nodeConnectorCloudflareTunnelSecretMarker      = "raw-cloudflare-credential-secret-marker"
	nodeConnectorCloudflareTunnelHelperEnvironment = "DORKPIPE_CLOUDFLARE_HELPER_PROCESS"
)

type nodeConnectorCloudflareTunnelTestCall struct {
	executable     string
	arguments      []string
	configuration  []byte
	brokerEndpoint string
	proxyEndpoint  string
}

type nodeConnectorCloudflareTunnelTestStarter struct {
	t                  *testing.T
	executable         NodeConnectorCloudflareTunnelExecutable
	credentialPath     string
	temporaryRoot      string
	edgePath           string
	originConfig       NodeConnectorCloudflareTunnelOriginConfig
	clientConfig       NodeConnectorCloudflareTunnelClientConfig
	originBehavior     string
	clientBehavior     string
	mu                 sync.Mutex
	calls              []nodeConnectorCloudflareTunnelTestCall
	executableResolver NodeConnectorCloudflareTunnelExecutableResolver
	credentialResolver NodeConnectorCloudflareTunnelCredentialFileResolver
	start              NodeConnectorCloudflareTunnelProcessStarter
}

func newNodeConnectorCloudflareTunnelTestStarter(t *testing.T) *nodeConnectorCloudflareTunnelTestStarter {
	t.Helper()
	executablePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	credentialPath := filepath.Join(t.TempDir(), "tunnel-credential.json")
	if err := os.WriteFile(credentialPath, []byte(nodeConnectorCloudflareTunnelSecretMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	starter := &nodeConnectorCloudflareTunnelTestStarter{
		t: t, credentialPath: credentialPath, temporaryRoot: t.TempDir(), edgePath: filepath.Join(t.TempDir(), "edge-endpoint"),
		executable: NodeConnectorCloudflareTunnelExecutable{Path: executablePath, Capabilities: NodeConnectorCloudflareTunnelCapabilities{
			Version: "2026.7.0", LocallyManagedCredentialFile: true, TCPIngress: true, AccessTCP: true, PIDFileReadiness: true,
		}},
	}
	starter.executableResolver = func(reference string) (NodeConnectorCloudflareTunnelExecutable, error) {
		if reference != "ref-cloudflared-executable" {
			return NodeConnectorCloudflareTunnelExecutable{}, errors.New("unknown executable reference")
		}
		return starter.executable, nil
	}
	starter.credentialResolver = func(reference string) (string, error) {
		if reference != "ref-cloudflare-tunnel-credential" {
			return "", errors.New("unknown credential reference")
		}
		return starter.credentialPath, nil
	}
	starter.start = starter.startProcess
	return starter
}

func (starter *nodeConnectorCloudflareTunnelTestStarter) originConfiguration(brokerEndpoint string) NodeConnectorCloudflareTunnelOriginConfig {
	config := NodeConnectorCloudflareTunnelOriginConfig{
		ExecutableReference: "ref-cloudflared-executable", CredentialReference: "ref-cloudflare-tunnel-credential",
		TunnelID: nodeConnectorCloudflareTunnelTestID, PublicHostname: nodeConnectorCloudflareTunnelTestHostname, BrokerEndpoint: brokerEndpoint,
		StartupTimeout: 2 * time.Second, ShutdownTimeout: time.Second, TemporaryRoot: starter.temporaryRoot,
	}
	starter.originConfig = config
	return config
}

func (starter *nodeConnectorCloudflareTunnelTestStarter) clientConfiguration(proxyEndpoint string) NodeConnectorCloudflareTunnelClientConfig {
	config := NodeConnectorCloudflareTunnelClientConfig{
		ExecutableReference: "ref-cloudflared-executable", PublicHostname: nodeConnectorCloudflareTunnelTestHostname, ProxyEndpoint: proxyEndpoint,
		StartupTimeout: 2 * time.Second, ShutdownTimeout: time.Second,
	}
	starter.clientConfig = config
	return config
}

func (starter *nodeConnectorCloudflareTunnelTestStarter) startProcess(executable string, arguments []string) (NodeConnectorCloudflareTunnelProcess, error) {
	call := nodeConnectorCloudflareTunnelTestCall{executable: executable, arguments: append([]string{}, arguments...)}
	mode, behavior := "", ""
	if len(arguments) > 0 && arguments[0] == "tunnel" {
		mode, behavior = "origin", starter.originBehavior
		for index := range arguments {
			if arguments[index] == "--config" && index+1 < len(arguments) {
				call.configuration, _ = os.ReadFile(arguments[index+1])
				call.brokerEndpoint = nodeConnectorCloudflareTunnelTestConfigurationValue(call.configuration, "service: ", "tcp://")
			}
		}
	} else {
		mode, behavior = "client", starter.clientBehavior
		for index := range arguments {
			if arguments[index] == "--url" && index+1 < len(arguments) {
				call.proxyEndpoint = arguments[index+1]
			}
		}
	}
	starter.mu.Lock()
	starter.calls = append(starter.calls, call)
	starter.mu.Unlock()
	helperArguments := append([]string{"-test.run=^TestNodeConnectorCloudflareTunnelHelperProcess$", "--"}, arguments...)
	command := exec.Command(executable, helperArguments...)
	command.Env = append(os.Environ(),
		nodeConnectorCloudflareTunnelHelperEnvironment+"="+mode,
		"DORKPIPE_CLOUDFLARE_HELPER_EDGE_FILE="+starter.edgePath,
		"DORKPIPE_CLOUDFLARE_HELPER_BEHAVIOR="+behavior,
	)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return startNodeConnectorCloudflareTunnelCommand(command)
}

func (starter *nodeConnectorCloudflareTunnelTestStarter) snapshot() []nodeConnectorCloudflareTunnelTestCall {
	starter.mu.Lock()
	defer starter.mu.Unlock()
	result := make([]nodeConnectorCloudflareTunnelTestCall, len(starter.calls))
	copy(result, starter.calls)
	return result
}

func openNodeConnectorCloudflareTunnelPair(t *testing.T, starter *nodeConnectorCloudflareTunnelTestStarter, secrets nodeConnectorDirectTLSSecrets, limits NodeConnectorTransportLimits) (net.Conn, net.Conn, func()) {
	t.Helper()
	broker := mustNodeConnectorDirectTLSBroker(t, secrets, limits)
	origin, err := StartNodeConnectorCloudflareTunnelOrigin(context.Background(), starter.originConfiguration(broker.Endpoint()), starter.executableResolver, starter.credentialResolver, starter.start)
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
	proxyEndpoint := nodeConnectorCloudflareTunnelFreeEndpoint(t)
	directTLS := NodeConnectorDirectTLSConnectorConfig{Endpoint: proxyEndpoint, ExpectedServerIdentity: "broker.test", TrustRootsReference: secrets.rootsReference, HandshakeTimeout: time.Second}
	client, connectorConnection, err := StartNodeConnectorCloudflareTunnelClient(context.Background(), starter.clientConfiguration(proxyEndpoint), directTLS, limits, nodeConnectorDirectTLSMapLoader(secrets.values), starter.executableResolver, starter.start)
	if err != nil {
		_ = origin.Close()
		_ = broker.Close()
		t.Fatal(err)
	}
	result := <-accepted
	if result.err != nil {
		_ = client.Close()
		_ = origin.Close()
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
		_ = client.Close()
		_ = origin.Close()
		_ = broker.Close()
	}
}

func nodeConnectorCloudflareTunnelFreeEndpoint(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func nodeConnectorCloudflareTunnelTempEntries(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), nodeConnectorCloudflareTempPrefix) {
			count++
		}
	}
	return count
}

func nodeConnectorCloudflareTunnelTestConfigurationValue(raw []byte, prefix, trimPrefix string) string {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		var value string
		if json.Unmarshal([]byte(strings.TrimPrefix(line, prefix)), &value) == nil {
			return strings.TrimPrefix(value, trimPrefix)
		}
	}
	return ""
}

func TestNodeConnectorCloudflareTunnelHelperProcess(t *testing.T) {
	mode := os.Getenv(nodeConnectorCloudflareTunnelHelperEnvironment)
	if mode == "" {
		return
	}
	arguments := nodeConnectorCloudflareTunnelHelperArguments(os.Args)
	behavior := os.Getenv("DORKPIPE_CLOUDFLARE_HELPER_BEHAVIOR")
	if behavior == "exit" {
		os.Exit(19)
	}
	switch mode {
	case "origin":
		nodeConnectorCloudflareTunnelRunOriginHelper(t, arguments, behavior)
	case "client":
		nodeConnectorCloudflareTunnelRunClientHelper(t, arguments, behavior)
	default:
		os.Exit(20)
	}
}

func nodeConnectorCloudflareTunnelHelperArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" {
			return arguments[index+1:]
		}
	}
	return nil
}

func nodeConnectorCloudflareTunnelRunOriginHelper(t *testing.T, arguments []string, behavior string) {
	if len(arguments) != 8 || arguments[0] != "tunnel" || arguments[1] != "--config" || arguments[3] != "--no-autoupdate" || arguments[4] != "--pidfile" || arguments[6] != "run" {
		os.Exit(21)
	}
	raw, err := os.ReadFile(arguments[2])
	if err != nil {
		os.Exit(22)
	}
	brokerEndpoint := nodeConnectorCloudflareTunnelTestConfigurationValue(raw, "service: ", "tcp://")
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		os.Exit(23)
	}
	defer listener.Close()
	if err := os.WriteFile(os.Getenv("DORKPIPE_CLOUDFLARE_HELPER_EDGE_FILE"), []byte(listener.Addr().String()), 0o600); err != nil {
		os.Exit(24)
	}
	if behavior != "no_pid" {
		pid := strconv.Itoa(os.Getpid())
		if behavior == "bad_pid" {
			pid = "1"
		}
		if err := os.WriteFile(arguments[5], []byte(pid), 0o600); err != nil {
			os.Exit(25)
		}
	}
	if behavior == "exit_after_pid" {
		time.Sleep(50 * time.Millisecond)
		os.Exit(29)
	}
	for {
		edge, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		origin, dialErr := net.DialTimeout("tcp", brokerEndpoint, time.Second)
		if dialErr != nil {
			_ = edge.Close()
			continue
		}
		if behavior == "corrupt" {
			go nodeConnectorCloudflareTunnelCorruptCopy(origin, edge)
			go nodeConnectorCloudflareTunnelCopy(edge, origin)
		} else {
			go nodeConnectorCloudflareTunnelCopy(origin, edge)
			go nodeConnectorCloudflareTunnelCopy(edge, origin)
		}
	}
}

func nodeConnectorCloudflareTunnelRunClientHelper(t *testing.T, arguments []string, behavior string) {
	if len(arguments) != 6 || !reflect.DeepEqual(arguments[:5], []string{"access", "tcp", "--hostname", nodeConnectorCloudflareTunnelTestHostname, "--url"}) {
		os.Exit(26)
	}
	if behavior == "no_listener" {
		select {}
	}
	edgeRaw, err := os.ReadFile(os.Getenv("DORKPIPE_CLOUDFLARE_HELPER_EDGE_FILE"))
	if err != nil {
		os.Exit(27)
	}
	listener, err := net.Listen("tcp", arguments[5])
	if err != nil {
		os.Exit(28)
	}
	defer listener.Close()
	for {
		client, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		edge, dialErr := net.DialTimeout("tcp", strings.TrimSpace(string(edgeRaw)), time.Second)
		if dialErr != nil {
			_ = client.Close()
			continue
		}
		go nodeConnectorCloudflareTunnelCopy(edge, client)
		go nodeConnectorCloudflareTunnelCopy(client, edge)
	}
}

func nodeConnectorCloudflareTunnelCopy(destination, source net.Conn) {
	_, _ = io.Copy(destination, source)
	_ = destination.Close()
	_ = source.Close()
}

func nodeConnectorCloudflareTunnelCorruptCopy(destination, source net.Conn) {
	buffer := make([]byte, 32*1024)
	count, err := source.Read(buffer)
	if count > 0 {
		buffer[0] ^= 0xff
		_, _ = destination.Write(buffer[:count])
	}
	if err == nil {
		_, _ = io.Copy(destination, source)
	}
	_ = destination.Close()
	_ = source.Close()
}
