package remoteidentity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func newIdentity(t *testing.T) Identity {
	t.Helper()
	identity, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func TestIdentityPersistenceAndProof(t *testing.T) {
	identity := newIdentity(t)
	serialized, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	var restored Identity
	if err := json.Unmarshal(serialized, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	message := []byte("dockpipe pairing proof: server, client, challenge")
	signature, err := restored.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(identity.PublicKey, message, signature) {
		t.Fatal("persisted identity proof was rejected")
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(identity.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(publicKey)
	if Fingerprint(identity.PublicKey) != hex.EncodeToString(digest[:]) {
		t.Fatal("fingerprint does not identify decoded public key")
	}
}

func TestProofRejectsSubstitution(t *testing.T) {
	identity := newIdentity(t)
	other := newIdentity(t)
	message := []byte("pairing challenge")
	signature, err := identity.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if Verify(other.PublicKey, message, signature) {
		t.Fatal("another installation accepted identity proof")
	}
	if Verify(identity.PublicKey, []byte("replacement challenge"), signature) {
		t.Fatal("changed message accepted identity proof")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	decoded[0] ^= 1
	if Verify(identity.PublicKey, message, base64.RawURLEncoding.EncodeToString(decoded)) {
		t.Fatal("modified signature accepted")
	}
}

func TestIdentityRejectsMalformedAndMismatchedKeys(t *testing.T) {
	identity := newIdentity(t)
	other := newIdentity(t)
	changedSeed := append([]byte{}, identity.PrivateKey...)
	changedSeed[0] ^= 1
	tests := map[string]Identity{
		"empty":                {},
		"short private":        {PublicKey: identity.PublicKey, PrivateKey: identity.PrivateKey[:31]},
		"long private":         {PublicKey: identity.PublicKey, PrivateKey: append(append([]byte{}, identity.PrivateKey...), 0)},
		"mismatched public":    {PublicKey: other.PublicKey, PrivateKey: identity.PrivateKey},
		"changed private seed": {PublicKey: identity.PublicKey, PrivateKey: changedSeed},
		"padded public":        {PublicKey: identity.PublicKey + "=", PrivateKey: identity.PrivateKey},
		"invalid public":       {PublicKey: "invalid", PrivateKey: identity.PrivateKey},
		"short public":         {PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 31)), PrivateKey: identity.PrivateKey},
		"long public":          {PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 33)), PrivateKey: identity.PrivateKey},
	}
	for name, candidate := range tests {
		t.Run(name, func(t *testing.T) {
			if candidate.Validate() == nil {
				t.Fatal("invalid identity accepted")
			}
			if _, err := candidate.Sign([]byte("message")); err == nil {
				t.Fatal("invalid identity signed a message")
			}
		})
	}
}

func TestProofEncodingAndPayloadBounds(t *testing.T) {
	identity := newIdentity(t)
	message := []byte("message")
	signature, err := identity.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "invalid", signature + "=", signature[:len(signature)-1], base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize+1))} {
		if Verify(identity.PublicKey, message, invalid) {
			t.Fatal("invalid signature encoding accepted")
		}
	}
	for _, invalid := range []string{"", "invalid", identity.PublicKey + "=", identity.PublicKey + "\n"} {
		if Fingerprint(invalid) != "" || Verify(invalid, message, signature) {
			t.Fatal("invalid public encoding accepted")
		}
	}
	maximum := make([]byte, MaxSignedPayload)
	maximumSignature, err := identity.Sign(maximum)
	if err != nil || !Verify(identity.PublicKey, maximum, maximumSignature) {
		t.Fatalf("maximum bounded proof failed: %v", err)
	}
	oversized := append(maximum, 0)
	if _, err := identity.Sign(oversized); err == nil {
		t.Fatal("oversized proof signed")
	}
	if Verify(identity.PublicKey, oversized, maximumSignature) {
		t.Fatal("oversized proof accepted")
	}
}
