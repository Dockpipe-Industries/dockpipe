package hardwarekey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"math/big"
	"testing"
	"time"

	"dockpipe/src/lib/internal/testsupport/securitykey"
)

func TestManufacturerIntermediateCompletesOnlyExplicitRootChain(t *testing.T) {
	fixture := securitykey.New(t)
	key := func() *ecdsa.PrivateKey {
		value, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	rootKey, intermediateKey := key(), key()
	certificate := func(serial int64, name string, ca bool) *x509.Certificate {
		usage := x509.KeyUsageDigitalSignature
		if ca {
			usage = x509.KeyUsageCertSign
		}
		return &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{
			CommonName: name, Country: []string{"US"}, Organization: []string{"Fixture"},
			OrganizationalUnit: []string{"Authenticator Attestation"}},
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
			IsCA: ca, BasicConstraintsValid: true, KeyUsage: usage}
	}
	issue := func(template, parent *x509.Certificate, public *ecdsa.PublicKey, signer *ecdsa.PrivateKey) ([]byte, *x509.Certificate) {
		der, err := x509.CreateCertificate(rand.Reader, template, parent, public, signer)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return der, parsed
	}
	rootTemplate := certificate(11, "Explicit root", true)
	rootDER, root := issue(rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	intermediateDER, intermediate := issue(certificate(12, "Manufacturer intermediate", true), root, &intermediateKey.PublicKey, rootKey)
	leafDER, _ := issue(certificate(13, "Hardware attestation", false), intermediate, &fixture.AttestationKey.PublicKey, intermediateKey)
	rootPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))
	intermediatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateDER}))
	if _, err := NewRegistry(intermediatePEM); err == nil {
		t.Fatal("supplemental intermediate became an independent trust anchor")
	}
	for _, test := range []struct {
		name  string
		trust string
		valid bool
	}{
		{"missing intermediate", rootPEM, false},
		{"complete chain", rootPEM + intermediatePEM, true},
		{"intermediate below different root", fixture.RootsPEM + intermediatePEM, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry, err := NewRegistry(test.trust)
			if err != nil {
				t.Fatal(err)
			}
			options, session, err := registry.BeginRegistration()
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(options)
			var response map[string]any
			if err := json.Unmarshal(fixture.Registration(raw), &response); err != nil {
				t.Fatal(err)
			}
			payload := response["response"].(map[string]any)
			objectBytes, _ := base64.RawURLEncoding.DecodeString(payload["attestationObject"].(string))
			var object map[string]any
			if err := cbor.Unmarshal(objectBytes, &object); err != nil {
				t.Fatal(err)
			}
			object["attStmt"].(map[any]any)["x5c"] = []any{leafDER}
			objectBytes, _ = cbor.Marshal(object)
			payload["attestationObject"] = base64.RawURLEncoding.EncodeToString(objectBytes)
			raw, _ = json.Marshal(response)
			err = registry.FinishRegistration(*session, raw)
			if (err == nil) != test.valid {
				t.Fatalf("unexpected manufacturer-chain admission: %v", err)
			}
			if test.valid {
				if err := registry.Validate(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func testRegistry(t *testing.T) (*Registry, *securitykey.Fixture) {
	t.Helper()
	fixture := securitykey.New(t)
	registry, err := NewRegistry(fixture.RootsPEM)
	if err != nil {
		t.Fatal(err)
	}
	options, session, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(options)
	if err := registry.FinishRegistration(*session, fixture.Registration(encoded)); err != nil {
		t.Fatal(err)
	}
	return registry, fixture
}

func TestHardwareAttestationAndAssertion(t *testing.T) {
	registry, fixture := testRegistry(t)
	if err := registry.Validate(); err != nil {
		t.Fatal(err)
	}
	options, session, err := registry.BeginAssertion()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(options)
	response := fixture.Assertion(raw)
	updated, err := registry.FinishAssertion(*session, response)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Keys[0].Credential.Authenticator.SignCount == 0 || registry.Keys[0].Credential.Authenticator.SignCount != 0 {
		t.Fatal("assertion did not return an independent updated counter")
	}
	if _, err := updated.FinishAssertion(*session, response); err == nil {
		t.Fatal("counter replay accepted")
	}
	session.Expires = time.Now().Add(-time.Second)
	if _, err := registry.FinishAssertion(*session, response); err == nil {
		t.Fatal("expired assertion accepted")
	}
}

func TestHardwareRegistrationRejectsUntrustedAndSoftwareCredentials(t *testing.T) {
	for _, scenario := range []string{"wrong-root", "backup", "no-presence", "no-verification", "wrong-origin", "wrong-rp", "tampered-signature", "no-attestation", "self-attestation"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := securitykey.New(t)
			rootsPEM := fixture.RootsPEM
			if scenario == "wrong-root" {
				rootsPEM = securitykey.New(t).RootsPEM
			}
			registry, err := NewRegistry(rootsPEM)
			if err != nil {
				t.Fatal(err)
			}
			options, session, err := registry.BeginRegistration()
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "backup":
				fixture.Flags |= 8
			case "no-presence":
				fixture.Flags &^= 1
			case "no-verification":
				fixture.Flags &^= 4
			case "wrong-origin":
				fixture.Origin = "https://attacker.example"
			case "wrong-rp":
				fixture.RP = "attacker.example"
			}
			raw, _ := json.Marshal(options)
			response := fixture.Registration(raw)
			if scenario == "no-attestation" || scenario == "self-attestation" {
				var browser map[string]any
				if err := json.Unmarshal(response, &browser); err != nil {
					t.Fatal(err)
				}
				payload := browser["response"].(map[string]any)
				objectBytes, err := base64.RawURLEncoding.DecodeString(payload["attestationObject"].(string))
				if err != nil {
					t.Fatal(err)
				}
				var object map[string]any
				if err := cbor.Unmarshal(objectBytes, &object); err != nil {
					t.Fatal(err)
				}
				if scenario == "no-attestation" {
					object["fmt"] = "none"
					object["attStmt"] = map[string]any{}
				} else {
					statement := object["attStmt"].(map[any]any)
					delete(statement, "x5c")
				}
				objectBytes, err = cbor.Marshal(object)
				if err != nil {
					t.Fatal(err)
				}
				payload["attestationObject"] = base64.RawURLEncoding.EncodeToString(objectBytes)
				response, err = json.Marshal(browser)
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "tampered-signature" {
				response[len(response)/2] ^= 1
			}
			if err := registry.FinishRegistration(*session, response); err == nil {
				t.Fatal("invalid hardware credential accepted")
			}
		})
	}
}

func TestHardwareAssertionRejectsInvalidProof(t *testing.T) {
	for _, scenario := range []string{"wrong-key", "backup", "no-presence", "no-verification", "wrong-origin", "wrong-rp", "wrong-challenge"} {
		t.Run(scenario, func(t *testing.T) {
			registry, fixture := testRegistry(t)
			options, session, err := registry.BeginAssertion()
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "wrong-key":
				fixture.Key = securitykey.New(t).Key
			case "backup":
				fixture.Flags |= 8
			case "no-presence":
				fixture.Flags &^= 1
			case "no-verification":
				fixture.Flags &^= 4
			case "wrong-origin":
				fixture.Origin = "https://attacker.example"
			case "wrong-rp":
				fixture.RP = "attacker.example"
			case "wrong-challenge":
				session.Challenge = "not-the-issued-challenge"
			}
			raw, _ := json.Marshal(options)
			if _, err := registry.FinishAssertion(*session, fixture.Assertion(raw)); err == nil {
				t.Fatal("invalid assertion accepted")
			}
		})
	}
}

func TestEnrolledAttestationExpiryDoesNotLockOutTrustedKeys(t *testing.T) {
	enrolledAt := time.Now().Add(-48 * time.Hour)
	fixture := securitykey.NewAt(t, enrolledAt)
	registry, err := NewRegistry(fixture.RootsPEM)
	if err != nil {
		t.Fatal(err)
	}
	options, session, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(options)
	parsed, err := protocol.ParseCredentialCreationResponseBytes(fixture.Registration(raw))
	if err != nil {
		t.Fatal(err)
	}
	_ = session
	digest := sha256.Sum256(parsed.Raw.AttestationResponse.ClientDataJSON)
	credential, err := webauthn.NewCredential(digest[:], parsed)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.verifyHardware(*credential, time.Now(), true); err == nil {
		t.Fatal("expired certificate admitted as new enrollment")
	}
	if err := registry.verifyHardware(*credential, enrolledAt, false); err != nil {
		t.Fatal(err)
	}
	registry.Keys = []RegisteredKey{{Credential: *credential, RegisteredAt: enrolledAt}}
	if err := registry.Validate(); err != nil {
		t.Fatalf("historical trust validation failed: %v", err)
	}
	if _, _, err := registry.BeginAssertion(); err != nil {
		t.Fatalf("existing key locked out: %v", err)
	}
}
