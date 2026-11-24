package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const DesktopVersion = "dockpipe.remote.desktop/v1"

// DesktopDiscoveryVersion opts into display inventory and broker-owned
// administrator-input discovery without changing desktop session references.
const DesktopDiscoveryVersion = "dockpipe.remote.desktop-discovery/v2"

// One existing-desktop session per enrolled node bounds simultaneous capture and
// input ownership. 64 nodes each retain at most 512 KiB of ephemeral signaling.
const MaxDesktopSessions = 64
const MaxDesktopSignals = 64
const MaxDesktopSignalBytes = 512 << 10
const MaxDesktopSDP = 64 << 10
const MaxDesktopICE = 4 << 10
const MaxDesktopParameters = 16 << 10
const MaxDesktopBody = 1 << 20
const DesktopLease = 10 * time.Second
const DesktopLifetime = time.Hour
const DesktopCatalogFreshness = 10 * time.Second

// DesktopProvider identifies a versioned external implementation, not an
// executable, network destination, codec or credential selected by a caller.
type DesktopProvider struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Artifact string `json:"artifact"`
}

func (provider DesktopProvider) Validate() error {
	if !desktopIdentifier(provider.ID, 128) || !desktopIdentifier(provider.Version, 32) || !featureDigest(provider.Artifact) {
		return errors.New("desktop provider requires a bounded identity and version")
	}
	return nil
}

func desktopIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 'a' || character > 'z' {
			if character < 'A' || character > 'Z' {
				if character < '0' || character > '9' {
					if character != '.' && character != '-' && character != '_' {
						return false
					}
				}
			}
		}
	}
	return true
}

type DesktopCatalog struct {
	Schema             string           `json:"schema"`
	Target             FeatureTarget    `json:"target"`
	Provider           DesktopProvider  `json:"provider"`
	View               bool             `json:"view"`
	Input              bool             `json:"input"`
	AdministratorInput bool             `json:"administrator_input,omitempty"`
	Displays           []DesktopDisplay `json:"displays,omitempty"`
	Epoch              string           `json:"epoch,omitempty"`
	ReceivedAt         time.Time        `json:"received_at"`
}

func (catalog DesktopCatalog) Validate() error {
	if catalog.Schema != DesktopVersion || catalog.Target.Validate() != nil || catalog.Provider.Validate() != nil || catalog.Input && !catalog.View ||
		catalog.Epoch != "" && !featureDigest(catalog.Epoch) {
		return errors.New("invalid desktop capability catalog")
	}
	return ValidateDesktopDisplays(catalog.View, catalog.Displays)
}

// DesktopDisplay describes a source the native provider can capture.
// The ID remains opaque to the broker, which never receives display positions.
type DesktopDisplay struct {
	Source  string `json:"source"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Primary bool   `json:"primary"`
}

func ValidateDesktopDisplays(view bool, displays []DesktopDisplay) error {
	if len(displays) > 32 || len(displays) != 0 && !view {
		return errors.New("invalid desktop display inventory")
	}

	sources := make(map[string]bool, len(displays))
	var primary bool
	for _, display := range displays {
		if !desktopIdentifier(display.Source, 64) || sources[display.Source] ||
			display.Width < 2 || display.Width > 16384 || display.Height < 2 || display.Height > 16384 ||
			display.Primary && primary {
			return errors.New("invalid desktop display inventory")
		}

		sources[display.Source] = true
		primary = primary || display.Primary
	}
	return nil
}

// DesktopOpen authorizes one monitor of the existing desktop. Audio, clipboard,
// media/input transport and arbitrary provider options are absent from this API.
type DesktopOpen struct {
	Schema   string          `json:"schema"`
	ID       string          `json:"id"`
	Target   FeatureTarget   `json:"target"`
	Epoch    string          `json:"epoch"`
	Provider DesktopProvider `json:"provider"`
	Mode     string          `json:"mode"`
	Nonce    string          `json:"nonce"`
	Deadline time.Time       `json:"deadline"`
}

func (request DesktopOpen) Digest() (string, error) {
	request.ID = ""
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (request DesktopOpen) Validate() error {
	digest, err := request.Digest()
	if err != nil || request.Schema != DesktopVersion || request.ID != digest || request.Target.Validate() != nil ||
		!featureDigest(request.Epoch) || !featureDigest(request.Nonce) || request.Provider.Validate() != nil ||
		request.Mode != "view" && request.Mode != "control" || request.Deadline.IsZero() {
		return errors.New("invalid desktop open intent")
	}
	return nil
}

type DesktopReference struct {
	Schema string        `json:"schema"`
	ID     string        `json:"id"`
	Target FeatureTarget `json:"target"`
	Epoch  string        `json:"epoch"`
}

func (reference DesktopReference) Validate() error {
	if reference.Schema != DesktopVersion || !featureDigest(reference.ID) || !featureDigest(reference.Epoch) || reference.Target.Validate() != nil {
		return errors.New("invalid desktop session reference")
	}
	return nil
}

func (request DesktopOpen) Reference() DesktopReference {
	return DesktopReference{Schema: DesktopVersion, ID: request.ID, Target: request.Target, Epoch: request.Epoch}
}

// Signaling is ephemeral negotiation only. The native endpoint must verify the
// authenticated endpoint fingerprint and enforce mode/lease on its encrypted
// media and input channels. No signaling message itself grants media/input authority.
type DesktopSignal struct {
	Reference     DesktopReference `json:"reference"`
	Sequence      uint64           `json:"sequence"`
	Type          string           `json:"type"`
	SDP           string           `json:"sdp,omitempty"`
	Fingerprint   string           `json:"fingerprint,omitempty"`
	Parameters    json.RawMessage  `json:"parameters,omitempty"`
	Candidate     string           `json:"candidate,omitempty"`
	SDPMid        string           `json:"sdp_mid,omitempty"`
	SDPMLineIndex *uint16          `json:"sdp_mline_index,omitempty"`
}

func (signal DesktopSignal) Validate() error {
	if signal.Reference.Validate() != nil || signal.Sequence == 0 || len(signal.SDPMid) > 64 || strings.ContainsAny(signal.SDPMid, "\r\n\x00") {
		return errors.New("invalid desktop signaling reference")
	}
	if signal.Type != "binding" && signal.Fingerprint != "" {
		return errors.New("desktop endpoint identity requires a binding message")
	}
	if signal.Type != "parameters" && len(signal.Parameters) != 0 {
		return errors.New("desktop parameters require their own signaling message")
	}
	switch signal.Type {
	case "binding":
		if !featureDigest(signal.Fingerprint) || signal.SDP != "" || len(signal.Parameters) != 0 || signal.Candidate != "" || signal.SDPMid != "" || signal.SDPMLineIndex != nil {
			return errors.New("invalid ephemeral desktop endpoint identity binding")
		}
	case "parameters":
		if len(signal.Parameters) == 0 || len(signal.Parameters) > MaxDesktopParameters || !json.Valid(signal.Parameters) || signal.Parameters[0] != '{' || signal.SDP != "" || signal.Candidate != "" || signal.SDPMid != "" || signal.SDPMLineIndex != nil {
			return errors.New("invalid bounded desktop provider connection parameters")
		}
	case "offer", "answer":
		if len(signal.SDP) > MaxDesktopSDP || !strings.HasPrefix(signal.SDP, "v=0\r\n") || strings.ContainsRune(signal.SDP, 0) ||
			signal.Candidate != "" || signal.SDPMid != "" || signal.SDPMLineIndex != nil {
			return errors.New("invalid bounded desktop SDP")
		}
		for _, line := range strings.Split(signal.SDP, "\r\n") {
			if strings.ContainsAny(line, "\r\n") {
				return errors.New("desktop SDP requires canonical CRLF framing")
			}
			if strings.HasPrefix(line, "m=") && !strings.HasPrefix(line, "m=video ") && !strings.HasPrefix(line, "m=application ") {
				return errors.New("desktop SDP permits video and input data channels only")
			}
		}
	case "ice":
		if signal.SDP != "" || len(signal.Candidate) > MaxDesktopICE || !strings.HasPrefix(signal.Candidate, "candidate:") || strings.ContainsAny(signal.Candidate, "\r\n\x00") {
			return errors.New("invalid bounded desktop ICE candidate")
		}
	case "ice-complete":
		if signal.SDP != "" || signal.Candidate != "" || signal.SDPMid != "" || signal.SDPMLineIndex != nil {
			return errors.New("invalid desktop ICE completion")
		}
	default:
		return errors.New("unsupported desktop signaling type")
	}
	return nil
}

type DesktopRead struct {
	MetadataOnly bool             `json:"metadata_only,omitempty"`
	Reference    DesktopReference `json:"reference"`
	After        uint64           `json:"after"`
}

type DesktopView struct {
	LeaseMillis int64            `json:"lease_millis"`
	Reference   DesktopReference `json:"reference"`
	State       string           `json:"state"`
	Reason      string           `json:"reason,omitempty"`
	Ack         uint64           `json:"ack"`
	Signals     []DesktopSignal  `json:"signals,omitempty"`
}

type DesktopReport struct {
	Reference DesktopReference `json:"reference"`
	State     string           `json:"state"`
	Ack       uint64           `json:"ack"`
	Signals   []DesktopSignal  `json:"signals,omitempty"`
}

type DesktopExchange struct {
	Schema  string          `json:"schema"`
	Epoch   string          `json:"epoch,omitempty"`
	Catalog DesktopCatalog  `json:"catalog"`
	Reports []DesktopReport `json:"reports"`
}

type DesktopDirective struct {
	Open        DesktopOpen     `json:"open"`
	Operator    string          `json:"operator"`
	LeaseMillis int64           `json:"lease_millis"`
	Close       bool            `json:"close"`
	Ack         uint64          `json:"ack"`
	Signals     []DesktopSignal `json:"signals,omitempty"`
}

type DesktopExchangeResult struct {
	Schema     string             `json:"schema"`
	Epoch      string             `json:"epoch"`
	Directives []DesktopDirective `json:"directives"`
	Forget     []DesktopReference `json:"forget,omitempty"`
}
