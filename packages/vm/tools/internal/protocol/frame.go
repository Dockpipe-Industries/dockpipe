package protocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"
)

const (
	Version              = "dockpipe.vm.v2"
	MaxFrameBytes        = 64 * 1024
	MaxPayloadBytes      = 32 * 1024
	MaxLifetime          = 5 * time.Minute
	MaxClockSkew         = 30 * time.Second
	NonceBytes           = 32
	BootstrapKind        = "bootstrap"
	RequestKind          = "request"
	ResultKind           = "result"
	BootstrapPhase       = "bootstrap"
	BootstrapSequence    = uint64(1)
	FirstRequestSequence = uint64(2)
)

var QualificationCapabilities = map[string]struct{}{
	"identity/v1": {}, "health/v1": {}, "checkpoint/v1": {},
	"recovery/v1": {}, "launch-hash-pinned/v1": {},
}

var (
	contextUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	contextID   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)
	nonceHex    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Context struct {
	MachineUUID        string `json:"machine_uuid"`
	DiskSerial         string `json:"disk_serial"`
	BootID             string `json:"boot_id"`
	Sequence           uint64 `json:"sequence"`
	RunID              string `json:"run_id"`
	Nonce              string `json:"nonce"`
	Scenario           string `json:"scenario"`
	DurabilityBoundary string `json:"durability_boundary"`
	Phase              string `json:"phase"`
}

type UnsignedFrame struct {
	Version       string          `json:"version"`
	Kind          string          `json:"kind"`
	Capability    string          `json:"capability"`
	IssuedAtUnix  int64           `json:"issued_at_unix"`
	ExpiresAtUnix int64           `json:"expires_at_unix"`
	Context       Context         `json:"context"`
	Payload       json.RawMessage `json:"payload"`
}

type SignedFrame struct {
	Version       string          `json:"version"`
	Kind          string          `json:"kind"`
	Capability    string          `json:"capability"`
	IssuedAtUnix  int64           `json:"issued_at_unix"`
	ExpiresAtUnix int64           `json:"expires_at_unix"`
	Context       Context         `json:"context"`
	Payload       json.RawMessage `json:"payload"`
	Signature     string          `json:"signature"`
}

type IdentityBootstrapPayload struct {
	BootIDSource              string `json:"boot_id_source"`
	ControllerPublicKeySHA256 string `json:"controller_public_key_sha256"`
	GuestPublicKeySHA256      string `json:"guest_public_key_sha256"`
	ControllerBinarySHA256    string `json:"controller_binary_sha256"`
	GuestAgentBinarySHA256    string `json:"guest_agent_binary_sha256"`
}

func Sign(kind, capability string, ctx Context, payload any, issuedAt, expiresAt time.Time, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid Ed25519 private key")
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	payloadJSON, err = Canonicalize(payloadJSON)
	if err != nil {
		return nil, fmt.Errorf("canonicalize payload: %w", err)
	}
	if len(payloadJSON) > MaxPayloadBytes {
		return nil, fmt.Errorf("payload exceeds %d bytes", MaxPayloadBytes)
	}
	unsigned := UnsignedFrame{Version: Version, Kind: kind, Capability: capability, IssuedAtUnix: issuedAt.Unix(), ExpiresAtUnix: expiresAt.Unix(), Context: ctx, Payload: payloadJSON}
	if err := validateUnsigned(unsigned); err != nil {
		return nil, err
	}
	unsignedJSON, err := json.Marshal(unsigned)
	if err != nil {
		return nil, err
	}
	unsignedJSON, err = Canonicalize(unsignedJSON)
	if err != nil {
		return nil, err
	}
	signed := SignedFrame{Version: unsigned.Version, Kind: unsigned.Kind, Capability: unsigned.Capability, IssuedAtUnix: unsigned.IssuedAtUnix, ExpiresAtUnix: unsigned.ExpiresAtUnix, Context: unsigned.Context, Payload: unsigned.Payload, Signature: base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey, unsignedJSON))}
	out, err := json.Marshal(signed)
	if err != nil {
		return nil, err
	}
	out, err = Canonicalize(out)
	if err != nil {
		return nil, err
	}
	if len(out) > MaxFrameBytes {
		return nil, fmt.Errorf("frame exceeds %d bytes", MaxFrameBytes)
	}
	return out, nil
}

func Verify(data []byte, publicKey ed25519.PublicKey, now time.Time) (SignedFrame, error) {
	return verify(data, publicKey, now, true)
}

// VerifyRecorded authenticates a previously accepted canonical frame without
// treating its historical validity window as current authority. The signed
// issuance and expiry policy is still validated by validateUnsigned.
func VerifyRecorded(data []byte, publicKey ed25519.PublicKey) (SignedFrame, error) {
	return verify(data, publicKey, time.Time{}, false)
}

func verify(data []byte, publicKey ed25519.PublicKey, now time.Time, requireFresh bool) (SignedFrame, error) {
	var frame SignedFrame
	if len(data) == 0 || len(data) > MaxFrameBytes {
		return frame, fmt.Errorf("invalid frame length")
	}
	if err := RequireCanonical(data); err != nil {
		return frame, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&frame); err != nil {
		return frame, fmt.Errorf("decode signed frame: %w", err)
	}
	if err := requireEOF(dec); err != nil {
		return frame, err
	}
	unsigned := UnsignedFrame{Version: frame.Version, Kind: frame.Kind, Capability: frame.Capability, IssuedAtUnix: frame.IssuedAtUnix, ExpiresAtUnix: frame.ExpiresAtUnix, Context: frame.Context, Payload: frame.Payload}
	if err := validateUnsigned(unsigned); err != nil {
		return frame, err
	}
	issued, expires := time.Unix(frame.IssuedAtUnix, 0), time.Unix(frame.ExpiresAtUnix, 0)
	if requireFresh && (now.Add(MaxClockSkew).Before(issued) || now.Add(-MaxClockSkew).After(expires)) {
		return frame, fmt.Errorf("frame is stale or not yet valid")
	}
	sig, err := base64.RawStdEncoding.DecodeString(frame.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return frame, fmt.Errorf("invalid signature encoding")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return frame, fmt.Errorf("invalid pinned public key")
	}
	unsignedJSON, _ := json.Marshal(unsigned)
	unsignedJSON, _ = Canonicalize(unsignedJSON)
	if !ed25519.Verify(publicKey, unsignedJSON, sig) {
		return frame, fmt.Errorf("signature verification failed")
	}
	return frame, nil
}

func validateUnsigned(frame UnsignedFrame) error {
	if frame.Version != Version {
		return fmt.Errorf("unsupported protocol version or empty kind")
	}
	if frame.Kind != BootstrapKind && frame.Kind != RequestKind && frame.Kind != ResultKind {
		return fmt.Errorf("unsupported signed frame kind %q", frame.Kind)
	}
	if _, ok := QualificationCapabilities[frame.Capability]; !ok {
		return fmt.Errorf("capability %q is not permitted in qualification", frame.Capability)
	}
	if frame.Capability == "exec/v1" {
		return fmt.Errorf("arbitrary execution is prohibited in qualification")
	}
	if !contextUUID.MatchString(frame.Context.MachineUUID) || !contextUUID.MatchString(frame.Context.BootID) || !contextID.MatchString(frame.Context.DiskSerial) || !contextID.MatchString(frame.Context.RunID) || !nonceHex.MatchString(frame.Context.Nonce) || !contextID.MatchString(frame.Context.Scenario) || !contextID.MatchString(frame.Context.DurabilityBoundary) || !contextID.MatchString(frame.Context.Phase) {
		return fmt.Errorf("authenticated context is incomplete or malformed")
	}
	if frame.Kind == BootstrapKind {
		if frame.Capability != "identity/v1" || frame.Context.Sequence != BootstrapSequence || frame.Context.Phase != BootstrapPhase {
			return fmt.Errorf("identity bootstrap must be sequence %d in phase %q", BootstrapSequence, BootstrapPhase)
		}
	} else if frame.Context.Sequence < FirstRequestSequence || frame.Context.Phase == BootstrapPhase {
		return fmt.Errorf("request and result sequences must start at %d outside bootstrap phase", FirstRequestSequence)
	}
	if frame.ExpiresAtUnix <= frame.IssuedAtUnix || time.Duration(frame.ExpiresAtUnix-frame.IssuedAtUnix)*time.Second > MaxLifetime {
		return fmt.Errorf("frame lifetime exceeds policy")
	}
	if len(frame.Payload) == 0 || len(frame.Payload) > MaxPayloadBytes {
		return fmt.Errorf("invalid payload length")
	}
	if err := RequireCanonical(frame.Payload); err != nil {
		return fmt.Errorf("payload: %w", err)
	}
	return nil
}

func VerifyIdentityBootstrap(data []byte, publicKey ed25519.PublicKey, now time.Time, expected Context, bootstrapNonce string, expectedPayload IdentityBootstrapPayload) (SignedFrame, error) {
	if expected.BootID != "" {
		return SignedFrame{}, fmt.Errorf("controller must not prescribe a pre-launch boot ID")
	}
	frame, err := Verify(data, publicKey, now)
	if err != nil {
		return SignedFrame{}, err
	}
	if frame.Kind != BootstrapKind || frame.Capability != "identity/v1" || frame.Context.Sequence != BootstrapSequence || frame.Context.Phase != BootstrapPhase {
		return SignedFrame{}, fmt.Errorf("first guest frame is not the signed identity bootstrap")
	}
	if !nonceHex.MatchString(bootstrapNonce) || frame.Context.Nonce != bootstrapNonce {
		return SignedFrame{}, fmt.Errorf("identity bootstrap nonce mismatch")
	}
	if frame.Context.MachineUUID != expected.MachineUUID || frame.Context.DiskSerial != expected.DiskSerial || frame.Context.RunID != expected.RunID || frame.Context.Scenario != expected.Scenario || frame.Context.DurabilityBoundary != expected.DurabilityBoundary {
		return SignedFrame{}, fmt.Errorf("identity bootstrap context substitution rejected")
	}
	var payload IdentityBootstrapPayload
	dec := json.NewDecoder(bytes.NewReader(frame.Payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return SignedFrame{}, fmt.Errorf("decode identity bootstrap payload: %w", err)
	}
	if err := requireEOF(dec); err != nil {
		return SignedFrame{}, err
	}
	if payload != expectedPayload || payload.BootIDSource == "" {
		return SignedFrame{}, fmt.Errorf("identity bootstrap pin or boot-ID source mismatch")
	}
	for _, sum := range []string{payload.ControllerPublicKeySHA256, payload.GuestPublicKeySHA256, payload.ControllerBinarySHA256, payload.GuestAgentBinarySHA256} {
		if !nonceHex.MatchString(sum) {
			return SignedFrame{}, fmt.Errorf("identity bootstrap requires exact SHA-256 pins")
		}
	}
	guestPublicSHA256 := sha256.Sum256(publicKey)
	if hex.EncodeToString(guestPublicSHA256[:]) != payload.GuestPublicKeySHA256 {
		return SignedFrame{}, fmt.Errorf("identity bootstrap signature key does not match the guest public-key pin")
	}
	return frame, nil
}

func WriteFramed(w io.Writer, frame []byte) error {
	if len(frame) == 0 || len(frame) > MaxFrameBytes {
		return fmt.Errorf("invalid frame length")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(frame)))
	if _, err := w.Write(prefix[:]); err != nil {
		return err
	}
	_, err := w.Write(frame)
	return err
}

func ReadFramed(r io.Reader) ([]byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 || size > MaxFrameBytes {
		return nil, fmt.Errorf("invalid frame size %d", size)
	}
	frame := make([]byte, size)
	if _, err := io.ReadFull(r, frame); err != nil {
		return nil, err
	}
	return frame, nil
}
