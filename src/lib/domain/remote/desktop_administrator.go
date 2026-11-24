package remote

import "encoding/json"

const DesktopAdministratorApprovalPath = "/v1/desktop-administrator-approval"
const DesktopAdministratorServicePath = "/v1/desktop-administrator-service"

const DesktopAdministratorGrantPath = "/v1/desktop-administrator-grant"
const DesktopAdministratorConsentVersion = "dockpipe.remote.desktop-admin-consent/v1"
const DesktopAdministratorGrantVersion = "dockpipe.remote.desktop-admin-grant/v1"

type DesktopAdministratorEnvelope struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

// DesktopAdministratorConsent is issued by the independently trusted Windows
// service after local approval. The ordinary desktop sender does not own that
// service's signing key.
type DesktopAdministratorConsent struct {
	Schema             string           `json:"schema"`
	Reference          DesktopReference `json:"reference"`
	Operator           string           `json:"operator"`
	Challenge          string           `json:"challenge"`
	ServiceFingerprint string           `json:"service_fingerprint"`
	PeerFingerprint    string           `json:"peer_fingerprint"`
	WindowsUserSID     string           `json:"windows_user_sid"`
	WindowsSession     uint32           `json:"windows_session"`
	Deadline           int64            `json:"deadline"`
}

type DesktopAdministratorGrant struct {
	Schema       string          `json:"schema"`
	Consent      json.RawMessage `json:"consent"`
	Serial       uint64          `json:"serial"`
	Operator     string          `json:"operator"`
	Generation   uint64          `json:"generation"`
	LeaseExpires int64           `json:"lease_expires"`
}

type DesktopAdministratorApprovalRequest struct {
	Reference DesktopReference `json:"reference"`
	Start     bool             `json:"start,omitempty"`
}

type DesktopAdministratorApproval struct {
	Reference         DesktopReference              `json:"reference"`
	Operator          string                        `json:"operator"`
	ViewerFingerprint string                        `json:"viewer_fingerprint"`
	DesktopDeadline   int64                         `json:"desktop_deadline"`
	Expires           int64                         `json:"expires"`
	Consent           *DesktopAdministratorEnvelope `json:"consent,omitempty"`
	BrokerKey         string                        `json:"broker_key,omitempty"`
	Denied            bool                          `json:"denied,omitempty"`
}

type DesktopAdministratorServiceRequest struct {
	Nonce   string                        `json:"nonce"`
	Node    string                        `json:"node"`
	ReplyTo *DesktopReference             `json:"reply_to,omitempty"`
	Consent *DesktopAdministratorEnvelope `json:"consent,omitempty"`
	Denied  bool                          `json:"denied,omitempty"`
}
