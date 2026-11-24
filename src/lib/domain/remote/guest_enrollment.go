package remote

import "time"

// GuestGrantRequest authorizes workflow delivery to disposable guests in one run.
type GuestGrantRequest struct {
	Host            string `json:"host"`
	Run             string `json:"run"`
	Guests          int    `json:"guests"`
	LifetimeSeconds int    `json:"lifetime_seconds"`
}

type GuestGrantReceipt struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Run       string    `json:"run"`
	Guests    int       `json:"guests"`
	ExpiresAt time.Time `json:"expires_at"`
	BrokerKey string    `json:"broker_key"`
}

// GuestEnrollment contains only public material. The guest keeps its token and key.
type GuestEnrollment struct {
	Grant     string `json:"grant"`
	Node      string `json:"node"`
	PublicKey string `json:"public_key"`
	TokenHash string `json:"token_hash"`
}

func (request GuestEnrollment) ProofMessage(brokerKey string) []byte {
	return []byte("dockpipe.guest-enrollment.v1\n" + brokerKey + "\n" + request.Grant +
		"\n" + request.Node + "\n" + request.PublicKey + "\n" + request.TokenHash)
}

type GuestEnrollmentRequest struct {
	GuestEnrollment
	Proof string `json:"proof"`
}

type GuestEnrollmentReceipt struct {
	Node      string    `json:"node"`
	Grant     string    `json:"grant"`
	ExpiresAt time.Time `json:"expires_at"`
}

type GuestReleaseReceipt struct {
	Grant    string   `json:"grant"`
	Removed  []string `json:"removed"`
	Retained []string `json:"retained"`
}
