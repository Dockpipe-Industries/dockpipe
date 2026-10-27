// Package hardwarekey verifies hardware-attested WebAuthn credentials for local
// owner ceremonies. It never treats a browser attachment hint as hardware proof.
package hardwarekey

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	Origin           = "http://localhost:47832"
	CeremonyLifetime = 2 * time.Minute
	MaxResponseBytes = 64 << 10
	MaxKeys          = 8
)

// Registry contains public credentials and explicit manufacturer trust anchors.
// The surrounding private owner store protects their integrity.
type Registry struct {
	UserID   []byte          `json:"user_id"`
	RootsPEM string          `json:"roots_pem"`
	Keys     []RegisteredKey `json:"keys"`
}

// RegisteredKey keeps enrollment-time provenance separate from current key use.
// An attestation leaf expiring later must not lock out an already trusted broker.
type RegisteredKey struct {
	Credential   webauthn.Credential `json:"credential"`
	RegisteredAt time.Time           `json:"registered_at"`
}

func NewRegistry(rootsPEM string) (*Registry, error) {
	if _, err := roots(rootsPEM); err != nil {
		return nil, err
	}
	userID := make([]byte, 32)
	if _, err := rand.Read(userID); err != nil {
		return nil, err
	}
	return &Registry{UserID: userID, RootsPEM: rootsPEM}, nil
}

func authenticator() (*webauthn.WebAuthn, error) {
	timeout := webauthn.TimeoutConfig{Enforce: true, Timeout: CeremonyLifetime, TimeoutUVD: CeremonyLifetime}
	return webauthn.New(&webauthn.Config{
		RPID:                  "localhost",
		RPDisplayName:         "Dockpipe pairing",
		RPOrigins:             []string{Origin},
		AttestationPreference: protocol.PreferDirectAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.CrossPlatform,
			UserVerification:        protocol.VerificationRequired,
			ResidentKey:             protocol.ResidentKeyRequirementDiscouraged,
		},
		Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
}

func (r Registry) WebAuthnID() []byte          { return r.UserID }
func (r Registry) WebAuthnName() string        { return "dockpipe-owner" }
func (r Registry) WebAuthnDisplayName() string { return "Dockpipe owner" }
func (r Registry) WebAuthnCredentials() []webauthn.Credential {
	credentials := make([]webauthn.Credential, 0, len(r.Keys))
	for _, key := range r.Keys {
		credentials = append(credentials, key.Credential)
	}
	return credentials
}

func (r Registry) Validate() error {
	if len(r.UserID) != 32 || len(r.Keys) == 0 || len(r.Keys) > MaxKeys {
		return errors.New("invalid pairing hardware-key registry")
	}
	if _, err := roots(r.RootsPEM); err != nil {
		return err
	}
	for index, key := range r.Keys {
		if key.RegisteredAt.IsZero() || key.RegisteredAt.After(time.Now().Add(time.Minute)) {
			return errors.New("invalid hardware-key enrollment time")
		}
		if err := r.verifyHardware(key.Credential, key.RegisteredAt, false); err != nil {
			return err
		}
		for _, previous := range r.Keys[:index] {
			if bytes.Equal(previous.Credential.ID, key.Credential.ID) {
				return errors.New("duplicate pairing hardware key")
			}
		}
	}
	return nil
}

func (r Registry) BeginRegistration() (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	if len(r.Keys) >= MaxKeys {
		return nil, nil, errors.New("pairing hardware-key capacity reached")
	}
	verifier, err := authenticator()
	if err != nil {
		return nil, nil, err
	}
	exclusions := make([]protocol.CredentialDescriptor, 0, len(r.Keys))
	for _, key := range r.Keys {
		exclusions = append(exclusions, key.Credential.Descriptor())
	}
	return verifier.BeginRegistration(r, webauthn.WithExclusions(exclusions))
}

func (r *Registry) FinishRegistration(session webauthn.SessionData, response []byte) error {
	if len(response) > MaxResponseBytes || len(r.Keys) >= MaxKeys {
		return errors.New("pairing hardware-key registration exceeds its bounds")
	}
	verifier, err := authenticator()
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return errors.New("invalid hardware-key registration response")
	}
	credential, err := verifier.CreateCredential(*r, session, parsed)
	if err != nil {
		return errors.New("hardware-key registration verification failed")
	}
	registeredAt := time.Now().UTC()
	if err := r.verifyHardware(*credential, registeredAt, true); err != nil {
		return err
	}
	for _, previous := range r.Keys {
		if bytes.Equal(previous.Credential.ID, credential.ID) {
			return errors.New("pairing hardware key is already registered")
		}
	}
	r.Keys = append(r.Keys, RegisteredKey{Credential: *credential, RegisteredAt: registeredAt})
	return nil
}

func (r Registry) BeginAssertion() (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	if err := r.Validate(); err != nil {
		return nil, nil, err
	}
	verifier, err := authenticator()
	if err != nil {
		return nil, nil, err
	}
	return verifier.BeginLogin(r)
}

// FinishAssertion returns an updated registry without changing the receiver.
// The caller commits its counter and the authorized effect atomically.
func (r Registry) FinishAssertion(session webauthn.SessionData, response []byte) (*Registry, error) {
	if len(response) > MaxResponseBytes {
		return nil, errors.New("hardware-key assertion exceeds its bound")
	}
	verifier, err := authenticator()
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return nil, errors.New("invalid hardware-key assertion")
	}
	credential, err := verifier.ValidateLogin(r, session, parsed)
	if err != nil || credential.Authenticator.CloneWarning || credential.Flags.BackupEligible || credential.Flags.BackupState {
		return nil, errors.New("hardware-key assertion verification failed")
	}
	updated := r
	updated.Keys = append([]RegisteredKey(nil), r.Keys...)
	for index := range updated.Keys {
		if bytes.Equal(updated.Keys[index].Credential.ID, credential.ID) {
			updated.Keys[index].Credential = *credential
			return &updated, nil
		}
	}
	return nil, errors.New("hardware key is not registered")
}

func roots(value string) (*x509.CertPool, error) {
	trusted, _, err := manufacturerTrust(value)
	return trusted, err
}

func manufacturerTrust(value string) (*x509.CertPool, *x509.CertPool, error) {
	if len(value) == 0 || len(value) > MaxResponseBytes {
		return nil, nil, errors.New("supply a bounded PEM bundle of trusted hardware-key manufacturer roots")
	}
	trusted := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	remaining := []byte(value)
	rootCount, intermediateCount := 0, 0
	for len(bytes.TrimSpace(remaining)) != 0 {
		block, rest := pem.Decode(remaining)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, nil, errors.New("invalid hardware-key trust bundle")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !certificate.IsCA || !certificate.BasicConstraintsValid {
			return nil, nil, errors.New("hardware-key trust bundle entries must be CA certificates")
		}
		if bytes.Equal(certificate.RawSubject, certificate.RawIssuer) && certificate.CheckSignatureFrom(certificate) == nil {
			trusted.AddCert(certificate)
			rootCount++
		} else {
			// Supplemental intermediates help build the chain but never become
			// independent trust anchors. The leaf must reach an explicit root.
			intermediates.AddCert(certificate)
			intermediateCount++
		}
		if rootCount > 16 || intermediateCount > 32 {
			return nil, nil, errors.New("hardware-key trust bundle exceeds 16 roots or 32 intermediates")
		}
		remaining = rest
	}
	if rootCount == 0 {
		return nil, nil, errors.New("hardware-key trust bundle requires an explicit self-signed manufacturer root")
	}
	return trusted, intermediates, nil
}

func (r Registry) verifyHardware(credential webauthn.Credential, registeredAt time.Time, verifyAttestation bool) error {
	if credential.Flags.BackupEligible || credential.Flags.BackupState {
		return errors.New("synced or backup-eligible credentials cannot authorize pairing")
	}
	response := protocol.AuthenticatorAttestationResponse{}
	response.ClientDataJSON = credential.Attestation.ClientDataJSON
	response.AttestationObject = credential.Attestation.Object
	parsed, err := response.Parse()
	if err != nil {
		return errors.New("invalid stored hardware-key attestation")
	}
	object := parsed.AttestationObject
	if object.Format != "packed" && object.Format != "fido-u2f" {
		return errors.New("pairing requires packed or FIDO-U2F manufacturer attestation")
	}
	if !bytes.Equal(object.AuthData.AttData.CredentialPublicKey, credential.PublicKey) ||
		!bytes.Equal(object.AuthData.AttData.CredentialID, credential.ID) ||
		object.AuthData.Flags.HasBackupEligible() || object.AuthData.Flags.HasBackupState() ||
		!object.AuthData.Flags.UserPresent() || !object.AuthData.Flags.UserVerified() {
		return errors.New("hardware-key attestation does not bind the required credential and user verification")
	}
	// Full format/signature and current certificate validity were checked at
	// admission. Repeating the library check for stored keys would reapply today's
	// certificate expiry and lock out previously enrolled hardware. The private
	// registry preserves that admission; structural binding and the historical CA
	// path are still checked on load.
	if verifyAttestation {
		digest := sha256.Sum256(credential.Attestation.ClientDataJSON)
		if err := object.VerifyAttestation(digest[:], nil); err != nil {
			return errors.New("hardware-key manufacturer signature verification failed")
		}
	}
	chain, ok := object.AttStatement["x5c"].([]any)
	if !ok || len(chain) == 0 || len(chain) > 8 {
		return errors.New("pairing requires a hardware-key manufacturer certificate chain")
	}
	certificates := make([]*x509.Certificate, 0, len(chain))
	for _, entry := range chain {
		der, ok := entry.([]byte)
		if !ok {
			return errors.New("invalid hardware-key certificate chain")
		}
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return errors.New("invalid hardware-key certificate")
		}
		certificates = append(certificates, certificate)
	}
	trusted, intermediates, err := manufacturerTrust(r.RootsPEM)
	if err != nil {
		return err
	}
	for _, certificate := range certificates[1:] {
		intermediates.AddCert(certificate)
	}
	if _, err := certificates[0].Verify(x509.VerifyOptions{Roots: trusted, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}, CurrentTime: registeredAt}); err != nil {
		return fmt.Errorf("hardware-key attestation is not trusted by the configured manufacturer roots: %w", err)
	}
	return nil
}
