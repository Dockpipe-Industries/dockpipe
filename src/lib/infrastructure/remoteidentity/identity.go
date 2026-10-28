// Package remoteidentity provides installation identities for remote pairing.
// Possession of an identity key does not authorize an operation or authenticate
// the operating-system account using that installation.
package remoteidentity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// MaxSignedPayload bounds identity proof messages before cryptographic work.
const MaxSignedPayload = 1 << 20

// Identity is a persisted installation identity. Callers must store its private
// key using private-state storage and share only PublicKey with peers.
type Identity struct {
	PublicKey  string `json:"public_key"`
	PrivateKey []byte `json:"private_key"`
}

// New generates an independent Ed25519 identity.
func New() (Identity, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		PublicKey:  base64.RawURLEncoding.EncodeToString(publicKey),
		PrivateKey: privateKey,
	}, nil
}

// Validate verifies both the public encoding and the entire private key. Checking
// the private key's cached public suffix alone would miss a substituted seed.
func (identity Identity) Validate() error {
	publicKey, ok := decodeCanonical(identity.PublicKey, ed25519.PublicKeySize)
	if !ok || len(identity.PrivateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid installation identity key")
	}
	derived := ed25519.NewKeyFromSeed(identity.PrivateKey[:ed25519.SeedSize])
	if subtle.ConstantTimeCompare(derived, identity.PrivateKey) != 1 ||
		subtle.ConstantTimeCompare(derived[ed25519.SeedSize:], publicKey) != 1 {
		return errors.New("installation identity keys do not match")
	}
	return nil
}

// Fingerprint returns the lowercase SHA256 digest of the decoded public key.
// Invalid or noncanonical public keys have no fingerprint.
func Fingerprint(public string) string {
	publicKey, ok := decodeCanonical(public, ed25519.PublicKeySize)
	if !ok {
		return ""
	}
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:])
}

// Sign signs a bounded protocol message. Callers own domain separation and must
// include peer identities, purpose and freshness in their handshake message.
func (identity Identity) Sign(data []byte) (string, error) {
	if len(data) > MaxSignedPayload {
		return "", errors.New("installation identity proof exceeds payload limit")
	}
	if err := identity.Validate(); err != nil {
		return "", err
	}
	signature := ed25519.Sign(ed25519.PrivateKey(identity.PrivateKey), data)
	return base64.RawURLEncoding.EncodeToString(signature), nil
}

// Verify checks a bounded message and canonical public key and signature.
func Verify(public string, data []byte, signature string) bool {
	if len(data) > MaxSignedPayload {
		return false
	}
	publicKey, ok := decodeCanonical(public, ed25519.PublicKeySize)
	if !ok {
		return false
	}
	decodedSignature, ok := decodeCanonical(signature, ed25519.SignatureSize)
	if !ok {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), data, decodedSignature)
}

func decodeCanonical(encoded string, size int) ([]byte, bool) {
	if len(encoded) != base64.RawURLEncoding.EncodedLen(size) {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(decoded) != size {
		return nil, false
	}
	return decoded, base64.RawURLEncoding.EncodeToString(decoded) == encoded
}
