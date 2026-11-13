package remote

import (
	"errors"
	"time"
)

const DesktopRelayPath = "/v1/desktop-relay"
const DesktopRelayTicketPath = "/v1/desktop-relay-ticket"
const DesktopRelayProtocol = "dockpipe.desktop.relay.v1"
const MaxDesktopRelayFrame = 64 << 10
const MaxDesktopRelayBytes = uint64(16) << 30
const MaxDesktopRelayFrames = uint64(1) << 24
const DesktopRelayTicketLifetime = 20 * time.Second

// DesktopRelayRequest selects an already authorized session and both ephemeral
// endpoint identities. Tickets never confer desktop access independently of its
// original owner admission, machine identity, policy generation and lease.
type DesktopRelayRequest struct {
	Reference        DesktopReference `json:"reference"`
	Role             string           `json:"role"`
	LocalFingerprint string           `json:"local_fingerprint"`
	PeerFingerprint  string           `json:"peer_fingerprint"`
}

func (request DesktopRelayRequest) Validate() error {
	if request.Reference.Validate() != nil || request.Role != "viewer" && request.Role != "worker" ||
		!featureDigest(request.LocalFingerprint) || !featureDigest(request.PeerFingerprint) || request.LocalFingerprint == request.PeerFingerprint {
		return errors.New("invalid exact desktop relay endpoint binding")
	}
	return nil
}

type DesktopRelayTicket struct {
	Mode        string              `json:"mode"`
	Provider    DesktopProvider     `json:"provider"`
	Request     DesktopRelayRequest `json:"request"`
	Secret      string              `json:"secret"`
	LeaseMillis int64               `json:"lease_millis"`
	ExpiresAt   time.Time           `json:"expires_at"`
}
