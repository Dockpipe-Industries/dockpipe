package remote

import (
	"encoding/json"
	"time"
)

// PairingRequest binds approval to a worker-generated secret, never to a name alone.
type PairingRequest struct {
	Nonce        string `json:"nonce"`
	Node         string `json:"node"`
	Token        string `json:"token"`
	MachineKey   string `json:"machine_key"`
	MachineProof string `json:"machine_proof"`
}

// PairingStatus is safe to display. The code verifies a request; it is not a credential.
type PairingStatus struct {
	RequestNonce string    `json:"request_nonce,omitempty"`
	Code         string    `json:"code"`
	Node         string    `json:"node"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	BrokerKey    string    `json:"broker_key,omitempty"`
	BrokerProof  string    `json:"broker_proof,omitempty"`
}

type NodeStatus struct {
	Node   string `json:"node"`
	Status string `json:"status"`
}

type PairingChallengeRequest struct {
	Code  string `json:"code"`
	Nonce string `json:"nonce"`
}

// PairingChallenge binds a hardware-key assertion to one immutable pairing.
type PairingChallenge struct {
	RequestNonce      string          `json:"request_nonce"`
	BrokerKey         string          `json:"broker_key"`
	Signature         string          `json:"signature,omitempty"`
	ID                string          `json:"id"`
	Code              string          `json:"code"`
	Node              string          `json:"node"`
	WorkerFingerprint string          `json:"worker_fingerprint"`
	BrokerFingerprint string          `json:"broker_fingerprint"`
	ExpiresAt         time.Time       `json:"expires_at"`
	Options           json.RawMessage `json:"options"`
}

type PairingApproval struct {
	Code        string          `json:"code"`
	ChallengeID string          `json:"challenge_id"`
	Response    json.RawMessage `json:"response"`
}
