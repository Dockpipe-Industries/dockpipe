package remote

import "time"

// PairingRequest binds approval to a worker-generated secret, never to a name alone.
type PairingRequest struct {
	Node  string `json:"node"`
	Token string `json:"token"`
}

// PairingStatus is safe to display. The code verifies a request; it is not a credential.
type PairingStatus struct {
	Code      string    `json:"code"`
	Node      string    `json:"node"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

type NodeStatus struct {
	Node   string `json:"node"`
	Status string `json:"status"`
}
