//go:build linux || darwin

package remotecmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestHardwarePairingApprovalRejectsHostedOwner(t *testing.T) {
	root := privateConnectionDirectory(t)
	writeConnectionFixture(t, root, "connection.json", connectionConfig{Schema: contract.Version,
		Mode: "hosted", Endpoint: "https://broker.example.com", Token: strings.Repeat("a", 64),
		Resolver: infrastructure.ResolverMetadata{Name: "example.hosted", RemoteSetup: "hosted"}})
	if err := approveWithHardwareKey(context.Background(), root, "AAAA-BBBB-CCCC"); err == nil || !strings.Contains(err.Error(), "local owner") {
		t.Fatalf("hosted approval must stop before opening a browser or sending credentials: %v", err)
	}
}

func TestHardwarePairingApprovalPinsProtectedLocalBrokerIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "broker")
	if _, err := initialize(root, "127.0.0.1:47831"); err != nil {
		t.Fatal(err)
	}
	if _, err := remoteio.NewBroker(root); err != nil {
		t.Fatal(err)
	}
	var state remoteio.BrokerState
	if err := remoteio.ReadPrivate(filepath.Join(root, "broker.json"), &state); err != nil {
		t.Fatal(err)
	}
	client, key, err := localPairingApprovalClient(root)
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	if key != state.Identity.PublicKey || client.Endpoint != "http://127.0.0.1:47831" {
		t.Fatal("approval did not pin private local identity and loopback endpoint")
	}
	var owner OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &owner); err != nil {
		t.Fatal(err)
	}
	owner.Token = strings.Repeat("b", 64)
	writeConnectionFixture(t, root, "operator.json", owner)
	if _, _, err := localPairingApprovalClient(root); err == nil {
		t.Fatal("mismatched owner credential accepted")
	}
}

type pairingApprovalTestTransport func(*http.Request) (*http.Response, error)

func (transport pairingApprovalTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestHardwarePairingNeverDisplaysUnsignedConsent(t *testing.T) {
	key, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	client, err := remoteio.NewClient("http://127.0.0.1:47831", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	client.HTTP.Transport = pairingApprovalTestTransport(func(request *http.Request) (*http.Response, error) {
		var input contract.PairingChallengeRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			return nil, err
		}
		forged := contract.PairingChallenge{ID: strings.Repeat("e", 64), Code: input.Code, Node: "intended-worker",
			BrokerKey: key.PublicKey, BrokerFingerprint: remoteidentity.Fingerprint(key.PublicKey), RequestNonce: input.Nonce,
			WorkerFingerprint: strings.Repeat("e", 64), ExpiresAt: time.Now().Add(time.Minute), Options: json.RawMessage(`{"publicKey":{}}`)}
		body, err := json.Marshal(forged)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body))), Request: request}, nil
	})
	if _, err := preparePairingApproval(context.Background(), client, key.PublicKey, "AAAA-BBBB-CCCC"); err == nil {
		t.Fatal("unsigned consent reached the browser preparation boundary")
	}
}
