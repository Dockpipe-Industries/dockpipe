package remote

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/hardwarekey"
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"dockpipe/src/lib/internal/testsupport/securitykey"
)

func loadPairingHardware(t *testing.T, broker *Broker) *securitykey.Fixture {
	t.Helper()
	fixture := securitykey.New(t)
	registry, err := hardwarekey.NewRegistry(fixture.RootsPEM)
	if err != nil {
		t.Fatal(err)
	}
	options, session, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(options)
	if err := registry.FinishRegistration(*session, fixture.Registration(raw)); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	broker.state.PairingKeys = registry
	if err := broker.save(); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func signedTestPairing(t *testing.T, node, token string) (contract.PairingRequest, remoteidentity.Identity) {
	t.Helper()
	identity, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	request := contract.PairingRequest{Node: node, Token: token}
	if err := SignPairingRequest(&request, identity); err != nil {
		t.Fatal(err)
	}
	return request, identity
}

func testPairingApproval(t *testing.T, admin *Client, fixture *securitykey.Fixture, code string) contract.PairingApproval {
	t.Helper()
	nonce, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	var challenge contract.PairingChallenge
	if err := admin.Call(context.Background(), "/v1/approve-begin", contract.PairingChallengeRequest{Code: code, Nonce: nonce}, &challenge); err != nil {
		t.Fatal(err)
	}
	return contract.PairingApproval{Code: code, ChallengeID: challenge.ID, Response: fixture.Assertion(challenge.Options)}
}

func approveTestPairing(t *testing.T, admin *Client, fixture *securitykey.Fixture, code string) {
	t.Helper()
	approval := testPairingApproval(t, admin, fixture, code)
	if err := admin.Call(context.Background(), "/v1/approve", approval, nil); err != nil {
		t.Fatal(err)
	}
}

func TestHardwarePairingRequiresProofAndBindsBothMachines(t *testing.T) {
	root, broker, admin, server := testBroker(t)
	fixture := loadPairingHardware(t, broker)
	public, _ := NewClient(server.URL, "")
	defer public.HTTP.CloseIdleConnections()
	ctx := context.Background()
	if err := admin.Call(ctx, "/v1/pairing-open", nil, nil); err != nil {
		t.Fatal(err)
	}
	token, _ := Secret()
	request, identity := signedTestPairing(t, "worker", token)
	var status contract.PairingStatus
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPairingStatus(status, identity.PublicKey, token, strings.Repeat("f", 64), ""); err == nil {
		t.Fatal("replayed status accepted for fresh request")
	}
	if err := VerifyPairingStatus(status, identity.PublicKey, token, request.Nonce, ""); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPairingStatus(status, identity.PublicKey, token, request.Nonce, remoteidentityMustNew(t).PublicKey); err == nil {
		t.Fatal("substituted broker accepted")
	}
	if err := admin.Call(ctx, "/v1/approve", map[string]string{"code": status.Code}, nil); err == nil {
		t.Fatal("bearer-only approval accepted")
	}
	if err := admin.Call(ctx, "/v1/invite", map[string]string{"node": "bypass"}, nil); err == nil {
		t.Fatal("file invitation bypass accepted")
	}
	approval := testPairingApproval(t, admin, fixture, status.Code)
	if err := admin.Call(ctx, "/v1/approve", approval, nil); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(ctx, "/v1/approve", approval, nil); err == nil {
		t.Fatal("hardware assertion replay accepted")
	}
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": strings.Repeat("a", 64)}, nil); err == nil {
		t.Fatal("machine token without identity accepted")
	}
	worker.Identity = &identity
	worker.BrokerKey = broker.state.Identity.PublicKey
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": strings.Repeat("a", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	savedBrokerKey := broker.state.Identity.PublicKey
	reloaded, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.state.Identity.PublicKey != savedBrokerKey || reloaded.state.Nodes[request.Node].MachineKey != identity.PublicKey {
		t.Fatal("machine identity changed on reload")
	}
	worker.Identity = pointerIdentity(remoteidentityMustNew(t))
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": strings.Repeat("a", 64)}, nil); err == nil {
		t.Fatal("substituted worker identity accepted")
	}
	worker.Identity = &identity
	worker.BrokerKey = remoteidentityMustNew(t).PublicKey
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": strings.Repeat("a", 64)}, nil); err == nil {
		t.Fatal("substituted server identity accepted")
	}
}

func remoteidentityMustNew(t *testing.T) remoteidentity.Identity {
	t.Helper()
	identity, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	return identity
}
func pointerIdentity(identity remoteidentity.Identity) *remoteidentity.Identity { return &identity }

func TestHardwarePairingRejectsInvalidatedChallengeAndPersistenceFailure(t *testing.T) {
	for _, scenario := range []string{"denied", "closed", "expired", "changed-worker", "write-failure", "bad-proof"} {
		t.Run(scenario, func(t *testing.T) {
			_, broker, admin, server := testBroker(t)
			fixture := loadPairingHardware(t, broker)
			public, _ := NewClient(server.URL, "")
			defer public.HTTP.CloseIdleConnections()
			ctx := context.Background()
			if err := admin.Call(ctx, "/v1/pairing-open", nil, nil); err != nil {
				t.Fatal(err)
			}
			token, _ := Secret()
			request, _ := signedTestPairing(t, "worker", token)
			var status contract.PairingStatus
			if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
				t.Fatal(err)
			}
			approval := testPairingApproval(t, admin, fixture, status.Code)
			switch scenario {
			case "denied":
				if err := admin.Call(ctx, "/v1/deny", map[string]string{"code": status.Code}, nil); err != nil {
					t.Fatal(err)
				}
			case "closed":
				if err := admin.Call(ctx, "/v1/pairing-close", nil, nil); err != nil {
					t.Fatal(err)
				}
			case "expired":
				broker.mutex.Lock()
				challenge := broker.pairingChallenges[approval.ChallengeID]
				challenge.Session.Expires = time.Now().Add(-time.Second)
				broker.pairingChallenges[approval.ChallengeID] = challenge
				broker.mutex.Unlock()
			case "changed-worker":
				broker.mutex.Lock()
				pairing := broker.state.Pairings[status.Code]
				pairing.MachineKey = remoteidentityMustNew(t).PublicKey
				broker.state.Pairings[status.Code] = pairing
				broker.mutex.Unlock()
			case "write-failure":
				broker.mutex.Lock()
				broker.path = filepath.Join(t.TempDir(), "missing", "broker.json")
				if err := os.WriteFile(filepath.Dir(broker.path), []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
				broker.mutex.Unlock()
			case "bad-proof":
				approval.Response = json.RawMessage(`{}`)
			}
			if err := admin.Call(ctx, "/v1/approve", approval, nil); err == nil {
				t.Fatal("invalidated approval succeeded")
			}
			if scenario == "write-failure" && !broker.poisoned {
				t.Fatal("uncertain persistence did not poison broker")
			}
		})
	}
}

func TestPairingConsentAuthenticatesIntentOptionsAndFreshness(t *testing.T) {
	_, broker, admin, server := testBroker(t)
	_ = loadPairingHardware(t, broker)
	public, err := NewClient(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer public.HTTP.CloseIdleConnections()
	ctx := context.Background()
	if err := admin.Call(ctx, "/v1/pairing-open", nil, nil); err != nil {
		t.Fatal(err)
	}
	token, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	request, _ := signedTestPairing(t, "intended-worker", token)
	var status contract.PairingStatus
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
		t.Fatal(err)
	}
	nonce, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	var challenge contract.PairingChallenge
	if err := admin.Call(ctx, "/v1/approve-begin", contract.PairingChallengeRequest{Code: status.Code, Nonce: nonce}, &challenge); err != nil {
		t.Fatal(err)
	}
	brokerKey := broker.state.Identity.PublicKey
	if err := VerifyPairingChallenge(challenge, brokerKey, status.Code, nonce); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"node", "worker", "broker", "code", "options", "challenge-id", "expiry", "fresh-request", "different-intent", "signature"} {
		t.Run(scenario, func(t *testing.T) {
			changed := challenge
			expectedCode := status.Code
			expectedNonce := nonce
			switch scenario {
			case "node":
				changed.Node = "substituted-worker"
			case "worker":
				changed.WorkerFingerprint = strings.Repeat("e", 64)
			case "broker":
				changed.BrokerKey = remoteidentityMustNew(t).PublicKey
			case "code":
				changed.Code = "AAAA-BBBB-CCCC"
			case "options":
				changed.Options = json.RawMessage(`{"publicKey":{"challenge":"substituted"}}`)
			case "challenge-id":
				changed.ID = strings.Repeat("e", 64)
			case "expiry":
				changed.ExpiresAt = changed.ExpiresAt.Add(time.Minute)
			case "fresh-request":
				expectedNonce = strings.Repeat("e", 64)
			case "different-intent":
				expectedCode = "AAAA-BBBB-CCCC"
			case "signature":
				changed.Signature = ""
			}
			if err := VerifyPairingChallenge(changed, brokerKey, expectedCode, expectedNonce); err == nil {
				t.Fatal("substituted or replayed consent accepted")
			}
		})
	}
}
