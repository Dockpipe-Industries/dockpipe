// Package nativekey adapts native FIDO2 ceremonies to the WebAuthn verifier.
// libfido2 owns CTAP framing, PIN authentication, and device cryptography.
package nativekey

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"

	"dockpipe/src/lib/infrastructure/hardwarekey"
)

const (
	maxDevices      = 8
	maxOptionsBytes = 32 << 10
	maxCredentialID = 1024
	maxNativeBytes  = 32 << 10
	maxCeremony     = 2 * time.Minute
)

type ceremonyOptions struct {
	challenge string
	timeout   time.Duration
	creation  *protocol.PublicKeyCredentialCreationOptions
	assertion *protocol.PublicKeyCredentialRequestOptions
	userID    []byte
}

func parseOptions(value []byte, creation bool) (ceremonyOptions, error) {
	var result ceremonyOptions
	if len(value) == 0 || len(value) > maxOptionsBytes {
		return result, errors.New("native security-key options exceed their size bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	var challenge []byte
	var timeout int
	var credentials []protocol.CredentialDescriptor
	if creation {
		var options protocol.CredentialCreation
		if err := decoder.Decode(&options); err != nil {
			return result, errors.New("invalid native security-key registration options")
		}
		public := &options.Response
		userID, ok := public.User.ID.(string)
		decodedUser, userErr := base64.RawURLEncoding.Strict().DecodeString(userID)
		selection := public.AuthenticatorSelection
		if options.Mediation != "" || public.RelyingParty.ID != "localhost" ||
			!validText(public.RelyingParty.Name, 128) || !ok || userErr != nil || len(decodedUser) != 32 ||
			!validText(public.User.Name, 128) || !validText(public.User.DisplayName, 128) ||
			selection.AuthenticatorAttachment != protocol.CrossPlatform ||
			selection.UserVerification != protocol.VerificationRequired ||
			selection.ResidentKey != protocol.ResidentKeyRequirementDiscouraged ||
			(selection.RequireResidentKey != nil && *selection.RequireResidentKey) ||
			public.Attestation != protocol.PreferDirectAttestation || len(public.Extensions) != 0 ||
			len(public.Hints) != 0 || len(public.AttestationFormats) != 0 {
			return result, errors.New("native registration requires localhost, hardware attachment, direct attestation, required verification, and nonresident credentials")
		}
		if len(public.Parameters) == 0 || len(public.Parameters) > 16 {
			return result, errors.New("invalid native security-key credential parameters")
		}
		es256 := false
		for _, parameter := range public.Parameters {
			if parameter.Type != protocol.PublicKeyCredentialType {
				return result, errors.New("native security-key credential type must be public-key")
			}
			if parameter.Algorithm == -7 {
				es256 = true
			}
		}
		if !es256 {
			return result, errors.New("native security-key registration requires ES256")
		}
		result.creation = public
		result.userID = decodedUser
		challenge, timeout, credentials = public.Challenge, public.Timeout, public.CredentialExcludeList
	} else {
		var options protocol.CredentialAssertion
		if err := decoder.Decode(&options); err != nil {
			return result, errors.New("invalid native security-key assertion options")
		}
		public := &options.Response
		if options.Mediation != "" || public.RelyingPartyID != "localhost" ||
			public.UserVerification != protocol.VerificationRequired || len(public.Extensions) != 0 ||
			len(public.Hints) != 0 || len(public.AllowedCredentials) == 0 {
			return result, errors.New("native assertion requires localhost, required verification, and an explicit credential allowlist")
		}
		result.assertion = public
		challenge, timeout, credentials = public.Challenge, public.Timeout, public.AllowedCredentials
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return result, errors.New("native security-key options contain trailing data")
	}
	if len(challenge) < 16 || len(challenge) > 64 || timeout < 1 || timeout > int(maxCeremony/time.Millisecond) {
		return result, errors.New("native security-key challenge or timeout is outside its bounds")
	}
	if len(credentials) > maxDevices {
		return result, errors.New("native security-key credential list exceeds eight entries")
	}
	for index, credential := range credentials {
		if credential.Type != protocol.PublicKeyCredentialType || len(credential.CredentialID) == 0 || len(credential.CredentialID) > maxCredentialID || len(credential.Transport) > 8 {
			return result, errors.New("invalid native security-key credential descriptor")
		}
		for _, previous := range credentials[:index] {
			if bytes.Equal(previous.CredentialID, credential.CredentialID) {
				return result, errors.New("duplicate native security-key credential descriptor")
			}
		}
	}
	result.challenge = base64.RawURLEncoding.EncodeToString(challenge)
	result.timeout = time.Duration(timeout) * time.Millisecond
	return result, nil
}

func validText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func clientData(options ceremonyOptions, creation bool) ([]byte, [32]byte, error) {
	kind := "webauthn.get"
	if creation {
		kind = "webauthn.create"
	}
	value, err := json.Marshal(struct {
		Type        string `json:"type"`
		Challenge   string `json:"challenge"`
		Origin      string `json:"origin"`
		CrossOrigin bool   `json:"crossOrigin"`
	}{kind, options.challenge, hardwarekey.Origin, false})
	return value, sha256.Sum256(value), err
}
