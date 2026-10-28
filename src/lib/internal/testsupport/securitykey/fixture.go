// Package securitykey provides signed WebAuthn responses for security boundary tests.
// It is a virtual authenticator and does not establish physical hardware proof.
package securitykey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// Fixture exposes authenticator inputs so tests can sign adversarial responses.
// Counter advances before each assertion; set it before Assertion to test rollback.
type Fixture struct {
	RootsPEM       string
	Origin         string
	RP             string
	Flags          byte
	Counter        uint32
	Key            *ecdsa.PrivateKey
	AttestationKey *ecdsa.PrivateKey
	CredentialID   []byte

	testing     testing.TB
	certificate []byte
	userHandle  string
}

// New creates an isolated CA, packed attestation identity and ES256 credential.
func New(t testing.TB) *Fixture {
	return NewAt(t, time.Now())
}

// NewAt creates historical attestation material for expiry regression tests.
func NewAt(t testing.TB, now time.Time) *Fixture {
	t.Helper()
	fixture := &Fixture{
		testing: t,
		Origin:  "http://localhost:47832",
		RP:      "localhost",
		Flags:   0x05,
	}
	rootKey := fixture.newKey()
	fixture.Key = fixture.newKey()
	fixture.AttestationKey = fixture.newKey()
	fixture.CredentialID = make([]byte, 32)
	if _, err := rand.Read(fixture.CredentialID); err != nil {
		t.Fatal(err)
	}
	root := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Dockpipe test authenticator CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootCertificate, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:            []string{"US"},
			Organization:       []string{"Dockpipe test"},
			OrganizationalUnit: []string{"Authenticator Attestation"},
			CommonName:         "test",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	fixture.certificate, err = x509.CreateCertificate(rand.Reader, leaf, rootCertificate, &fixture.AttestationKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	fixture.RootsPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))
	return fixture
}

// Registration returns a browser credential response with a CA-signed packed
// attestation. Options may be an inner publicKey object or its JSON wrapper.
func (fixture *Fixture) Registration(options []byte) []byte {
	fixture.testing.Helper()
	credentialIDLength := len(fixture.CredentialID)
	if credentialIDLength > math.MaxUint16 {
		fixture.testing.Fatal("test credential ID exceeds the WebAuthn length field")
		return nil
	}
	request := fixture.options(options)
	var user struct {
		ID string `json:"id"`
	}
	if raw := request["user"]; len(raw) > 0 {
		fixture.decode(raw, &user)
	}
	fixture.userHandle = user.ID
	clientData := fixture.clientData(request, "webauthn.create")
	authenticatorData := fixture.authenticatorData(fixture.Flags | 0x40)
	authenticatorData = append(authenticatorData, make([]byte, 16)...)
	authenticatorData = binary.BigEndian.AppendUint16(authenticatorData, uint16(credentialIDLength))
	authenticatorData = append(authenticatorData, fixture.CredentialID...)
	publicKey := map[int]any{
		1:  2,
		3:  -7,
		-1: 1,
		-2: fixture.Key.PublicKey.X.FillBytes(make([]byte, 32)),
		-3: fixture.Key.PublicKey.Y.FillBytes(make([]byte, 32)),
	}
	authenticatorData = append(authenticatorData, fixture.cbor(publicKey)...)
	attestation := fixture.cbor(map[string]any{
		"fmt":      "packed",
		"authData": authenticatorData,
		"attStmt": map[string]any{
			"alg": -7,
			"sig": fixture.sign(fixture.AttestationKey, authenticatorData, clientData),
			"x5c": [][]byte{fixture.certificate},
		},
	})
	return fixture.response(map[string]any{
		"clientDataJSON":    encode(clientData),
		"attestationObject": encode(attestation),
		"transports":        []string{"usb"},
	})
}

// Assertion signs the supplied challenge using the credential key. Mutation of
// Origin, RP, Flags, Counter or Key permits cryptographically signed negatives.
func (fixture *Fixture) Assertion(options []byte) []byte {
	fixture.testing.Helper()
	request := fixture.options(options)
	clientData := fixture.clientData(request, "webauthn.get")
	fixture.Counter++
	authenticatorData := fixture.authenticatorData(fixture.Flags)
	return fixture.response(map[string]any{
		"clientDataJSON":    encode(clientData),
		"authenticatorData": encode(authenticatorData),
		"signature":         encode(fixture.sign(fixture.Key, authenticatorData, clientData)),
		"userHandle":        fixture.userHandle,
	})
}

func (fixture *Fixture) options(raw []byte) map[string]json.RawMessage {
	fixture.testing.Helper()
	var options map[string]json.RawMessage
	fixture.decode(raw, &options)
	if publicKey := options["publicKey"]; len(publicKey) > 0 {
		fixture.decode(publicKey, &options)
	}
	if len(options["challenge"]) == 0 {
		fixture.testing.Fatal("WebAuthn options omit challenge")
	}
	return options
}

func (fixture *Fixture) clientData(options map[string]json.RawMessage, ceremony string) []byte {
	var challenge string
	fixture.decode(options["challenge"], &challenge)
	return fixture.json(map[string]any{
		"type":        ceremony,
		"challenge":   challenge,
		"origin":      fixture.Origin,
		"crossOrigin": false,
	})
}

func (fixture *Fixture) authenticatorData(flags byte) []byte {
	rpHash := sha256.Sum256([]byte(fixture.RP))
	data := append([]byte{}, rpHash[:]...)
	data = append(data, flags)
	return binary.BigEndian.AppendUint32(data, fixture.Counter)
}

func (fixture *Fixture) sign(key *ecdsa.PrivateKey, authenticatorData, clientData []byte) []byte {
	fixture.testing.Helper()
	clientHash := sha256.Sum256(clientData)
	message := append(append([]byte{}, authenticatorData...), clientHash[:]...)
	digest := sha256.Sum256(message)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		fixture.testing.Fatal(err)
	}
	return signature
}

func (fixture *Fixture) response(response map[string]any) []byte {
	return fixture.json(map[string]any{
		"id":                      encode(fixture.CredentialID),
		"rawId":                   encode(fixture.CredentialID),
		"type":                    "public-key",
		"authenticatorAttachment": "cross-platform",
		"clientExtensionResults":  map[string]any{},
		"response":                response,
	})
}

func (fixture *Fixture) newKey() *ecdsa.PrivateKey {
	fixture.testing.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fixture.testing.Fatal(err)
	}
	return key
}

func (fixture *Fixture) cbor(value any) []byte {
	fixture.testing.Helper()
	data, err := cbor.Marshal(value)
	if err != nil {
		fixture.testing.Fatal(err)
	}
	return data
}

func (fixture *Fixture) json(value any) []byte {
	fixture.testing.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		fixture.testing.Fatal(err)
	}
	return data
}

func (fixture *Fixture) decode(data []byte, value any) {
	fixture.testing.Helper()
	if err := json.Unmarshal(data, value); err != nil {
		fixture.testing.Fatal(err)
	}
}

func encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
