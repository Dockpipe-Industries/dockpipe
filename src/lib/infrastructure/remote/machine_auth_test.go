package remote

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func recordedWorkerCall(t *testing.T, broker *Broker, token, path, nonce, signature string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://broker.example"+path, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set(machineNonceHeader, nonce)
	request.Header.Set(machineSignatureHeader, signature)
	recorder := httptest.NewRecorder()
	broker.ServeHTTP(recorder, request)
	return recorder
}

func testMachineNonce(t *testing.T, broker *Broker, token string) string {
	t.Helper()
	requestNonce, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(machineChallengeRequest{Nonce: requestNonce})
	if err != nil {
		t.Fatal(err)
	}
	response := recordedWorkerCall(t, broker, token, "/v1/machine-challenge", "", "", payload)
	if response.Code != http.StatusOK {
		t.Fatalf("challenge failed: %d", response.Code)
	}
	var challenge machineChallengeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	return challenge.Nonce
}

func TestMachineProofRejectsReplayExpiryAndScopeChanges(t *testing.T) {
	for _, scenario := range []string{"replay", "expired", "different-path", "different-body", "different-node", "after-restart"} {
		t.Run(scenario, func(t *testing.T) {
			root, broker, _, _ := testBroker(t)
			identity := remoteidentityMustNew(t)
			token, _ := Secret()
			otherToken, _ := Secret()
			broker.state.Nodes["worker"] = Enrollment{TokenHash: Hash(token), MachineKey: identity.PublicKey}
			broker.state.Nodes["other"] = Enrollment{TokenHash: Hash(otherToken), MachineKey: identity.PublicKey}
			if err := broker.save(); err != nil {
				t.Fatal(err)
			}
			nonce := testMachineNonce(t, broker, token)
			body := []byte(`{"session":"` + strings.Repeat("a", 64) + `"}`)
			path := "/v1/next"
			signature, err := identity.Sign(machineMessage("request", nonce, path, 0, body))
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "replay":
				if response := recordedWorkerCall(t, broker, token, path, nonce, signature, body); response.Code != http.StatusOK {
					t.Fatal("valid request rejected")
				}
			case "expired":
				broker.machineChallenges[nonce] = machineChallenge{Node: "worker", ExpiresAt: time.Now().Add(-time.Second)}
			case "different-path":
				path = "/v1/result"
			case "different-body":
				body = []byte(`{}`)
			case "different-node":
				token = otherToken
			case "after-restart":
				broker, err = NewBroker(root)
				if err != nil {
					t.Fatal(err)
				}
			}
			if response := recordedWorkerCall(t, broker, token, path, nonce, signature, body); response.Code != http.StatusUnauthorized {
				t.Fatalf("invalid machine request accepted: %d", response.Code)
			}
		})
	}
}

func TestMachineChallengeCapacityIsPerNode(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	identity := remoteidentityMustNew(t)
	attacker, _ := Secret()
	healthy, _ := Secret()
	broker.state.Nodes["attacker"] = Enrollment{TokenHash: Hash(attacker), MachineKey: identity.PublicKey}
	broker.state.Nodes["healthy"] = Enrollment{TokenHash: Hash(healthy), MachineKey: identity.PublicKey}
	for i := 0; i < 140; i++ {
		_ = testMachineNonce(t, broker, attacker)
	}
	if len(broker.machineChallenges) != 4 {
		t.Fatalf("one node retained %d nonces", len(broker.machineChallenges))
	}
	_ = testMachineNonce(t, broker, healthy)
	if len(broker.machineChallenges) != 5 {
		t.Fatal("one machine starved another machine's challenge capacity")
	}
}
