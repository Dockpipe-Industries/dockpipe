package nativekey

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"

	"dockpipe/src/lib/infrastructure/hardwarekey"
	"dockpipe/src/lib/internal/testsupport/securitykey"
)

func marshal(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func cborValue(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := cbor.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func decode64(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func signNative(t *testing.T, key *ecdsa.PrivateKey, raw, clientJSON []byte) []byte {
	t.Helper()
	clientHash := sha256.Sum256(clientJSON)
	message := append(append([]byte(nil), raw...), clientHash[:]...)
	digest := sha256.Sum256(message)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

func registrationParts(t *testing.T, fixture *securitykey.Fixture, options []byte) ([]byte, map[string]any) {
	t.Helper()
	var response struct {
		Response struct {
			Attestation string `json:"attestationObject"`
		} `json:"response"`
	}
	if err := json.Unmarshal(fixture.Registration(options), &response); err != nil {
		t.Fatal(err)
	}
	var object struct {
		AuthData  []byte         `cbor:"authData"`
		Statement map[string]any `cbor:"attStmt"`
	}
	if err := cbor.Unmarshal(decode64(t, response.Response.Attestation), &object); err != nil {
		t.Fatal(err)
	}
	return object.AuthData, object.Statement
}

func TestNativeResponsesPassExistingVerifier(t *testing.T) {
	fixture := securitykey.New(t)
	registry, err := hardwarekey.NewRegistry(fixture.RootsPEM)
	if err != nil {
		t.Fatal(err)
	}
	request, session, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	encoded := marshal(t, request)
	options, err := parseOptions(encoded, true)
	if err != nil {
		t.Fatal(err)
	}
	clientJSON, hash, err := clientData(options, true)
	if err != nil || hash != sha256.Sum256(clientJSON) {
		t.Fatal("client data hashing failed", err)
	}
	raw, statement := registrationParts(t, fixture, encoded)
	statement["sig"] = signNative(t, fixture.AttestationKey, raw, clientJSON)
	response, err := registrationResponse(clientJSON, fixture.CredentialID, cborValue(t, raw), "packed", cborValue(t, statement))
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.FinishRegistration(*session, response); err != nil {
		t.Fatalf("native registration did not pass unchanged WebAuthn verifier: %v", err)
	}
	assertion, assertionSession, err := registry.BeginAssertion()
	if err != nil {
		t.Fatal(err)
	}
	options, err = parseOptions(marshal(t, assertion), false)
	if err != nil {
		t.Fatal(err)
	}
	clientJSON, _, err = clientData(options, false)
	if err != nil {
		t.Fatal(err)
	}
	raw = append([]byte(nil), raw[:37]...)
	raw[32] = 0x05
	raw[36] = 1
	response, err = assertionResponse(clientJSON, fixture.CredentialID, cborValue(t, raw), signNative(t, fixture.Key, raw, clientJSON), registry.UserID, options)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := registry.FinishAssertion(*assertionSession, response)
	if err != nil || updated.Keys[0].Credential.Authenticator.SignCount != 1 {
		t.Fatalf("native assertion did not pass unchanged WebAuthn verifier: %v", err)
	}
}

func TestOptionsRejectDowngradesAndBoundsBeforeDeviceAccess(t *testing.T) {
	registry := hardwarekey.Registry{UserID: make([]byte, 32)}
	request, _, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	valid := marshal(t, request)
	if _, err := parseOptions(valid, true); err != nil {
		t.Fatal(err)
	}
	scenarios := map[string]func(*protocol.CredentialCreation){
		"rp": func(value *protocol.CredentialCreation) { value.Response.RelyingParty.ID = "example.com" },
		"uv": func(value *protocol.CredentialCreation) {
			value.Response.AuthenticatorSelection.UserVerification = protocol.VerificationPreferred
		},
		"rk": func(value *protocol.CredentialCreation) {
			value.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementRequired
		},
		"attestation": func(value *protocol.CredentialCreation) { value.Response.Attestation = protocol.PreferNoAttestation },
		"challenge":   func(value *protocol.CredentialCreation) { value.Response.Challenge = []byte{1} },
		"timeout":     func(value *protocol.CredentialCreation) { value.Response.Timeout = 120001 },
		"algorithm": func(value *protocol.CredentialCreation) {
			value.Response.Parameters = []protocol.CredentialParameter{{Type: "public-key", Algorithm: -257}}
		},
		"user": func(value *protocol.CredentialCreation) { value.Response.User.ID = "invalid" },
		"nul":  func(value *protocol.CredentialCreation) { value.Response.User.Name = "bad\x00name" },
		"extension": func(value *protocol.CredentialCreation) {
			value.Response.Extensions = protocol.AuthenticationExtensions{"appid": "https://example.com"}
		},
		"credential": func(value *protocol.CredentialCreation) {
			value.Response.CredentialExcludeList = []protocol.CredentialDescriptor{{Type: "public-key", CredentialID: make([]byte, maxCredentialID+1)}}
		},
	}
	for name, mutate := range scenarios {
		t.Run(name, func(t *testing.T) {
			var modified protocol.CredentialCreation
			if err := json.Unmarshal(valid, &modified); err != nil {
				t.Fatal(err)
			}
			mutate(&modified)
			if _, err := parseOptions(marshal(t, modified), true); err == nil {
				t.Fatal("invalid options accepted")
			}
		})
	}
	for _, invalid := range [][]byte{nil, []byte("{}"), append(valid, []byte("{}")...), []byte(strings.Repeat(" ", maxOptionsBytes+1))} {
		if _, err := parseOptions(invalid, true); err == nil {
			t.Fatal("malformed options accepted")
		}
	}
	pin := []byte("1234")
	if _, err := Create(context.Background(), Device{}, valid, pin); err == nil || !bytes.Equal(pin, make([]byte, 4)) {
		t.Fatal("forged device accepted or PIN retained")
	}
}

func TestAuthenticatorDataRejectsMalformedFlagsAndCBOR(t *testing.T) {
	rpHash := sha256.Sum256([]byte("localhost"))
	raw := append(append([]byte(nil), rpHash[:]...), 5, 0, 0, 0, 0)
	if _, err := rawAuthenticatorData(cborValue(t, raw), nil, false); err != nil {
		t.Fatal(err)
	}
	for _, flags := range []byte{0, 1, 4, 0x0d, 0x15, 0x45, 0x85} {
		modified := append([]byte(nil), raw...)
		modified[32] = flags
		if _, err := rawAuthenticatorData(cborValue(t, modified), nil, false); err == nil {
			t.Fatalf("invalid flags %x accepted", flags)
		}
	}
	for _, malformed := range [][]byte{raw, {0x58}, {0x5f, 0x41, 1, 0xff}, append(cborValue(t, raw), 0), cborValue(t, make([]byte, 4097))} {
		if _, err := rawAuthenticatorData(malformed, nil, false); err == nil {
			t.Fatal("malformed native authenticator data accepted")
		}
	}
}

func TestNativeAttestationRequiresManufacturerStatement(t *testing.T) {
	fixture := securitykey.New(t)
	registry := hardwarekey.Registry{UserID: make([]byte, 32)}
	request, _, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	raw, statement := registrationParts(t, fixture, marshal(t, request))
	wrapped := cborValue(t, raw)
	for _, scenario := range []string{"no-chain", "wrong-algorithm", "none-format", "wrong-id", "unknown-field"} {
		t.Run(scenario, func(t *testing.T) {
			modified := make(map[string]any, len(statement))
			for key, value := range statement {
				modified[key] = value
			}
			format := "packed"
			id := fixture.CredentialID
			switch scenario {
			case "no-chain":
				delete(modified, "x5c")
			case "wrong-algorithm":
				modified["alg"] = -257
			case "none-format":
				format = "none"
			case "wrong-id":
				id = []byte{1}
			case "unknown-field":
				modified["ecdaaKeyId"] = []byte{1}
			}
			if _, err := registrationResponse(nil, id, wrapped, format, cborValue(t, modified)); err == nil {
				t.Fatal("unsupported attestation was accepted")
			}
		})
	}
	if name := displayName("key\u202e", "product\u2066"); strings.ContainsAny(name, "\u202e\u2066") {
		t.Fatal("device display name retained Unicode formatting controls")
	}
}

func TestAdmissionAndTimeoutAreBounded(t *testing.T) {
	release, err := enter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := enter(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled admission did not fail")
	}
	release()
	lib := &library{devTimeout: func(_ uintptr, timeout int32) int32 {
		if timeout <= 0 || timeout > 50 {
			t.Fatalf("unexpected native timeout: %d", timeout)
		}
		return 0
	}}
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := setTimeout(ctx, lib, 1, maxCeremony); err != nil {
		t.Fatal(err)
	}
	for _, code := range []int32{0x31, 0x32, 0x34, 0x35, 0x36} {
		if err := nativeError(code); err == nil || strings.Contains(err.Error(), "/dev/") {
			t.Fatal("PIN error was not safely translated")
		}
	}
	encoded := marshal(t, Device{ID: "0", Name: "key", path: "/private/device"})
	if bytes.Contains(encoded, []byte("private")) {
		t.Fatal("device path leaked into JSON")
	}
}

func TestNativeTimeoutRejectsInvalidBoundsBeforeDeviceAccess(t *testing.T) {
	lib := &library{devTimeout: func(uintptr, int32) int32 {
		t.Fatal("invalid timeout reached the native device")
		return 0
	}}
	for _, maximum := range []time.Duration{0, -time.Second, time.Duration(math.MaxInt32+1) * time.Millisecond} {
		if err := setTimeout(context.Background(), lib, 1, maximum); err == nil {
			t.Fatalf("invalid timeout %v was accepted", maximum)
		}
	}
	var received int32
	lib.devTimeout = func(_ uintptr, milliseconds int32) int32 {
		received = milliseconds
		return 0
	}
	if err := setTimeout(context.Background(), lib, 1, time.Duration(math.MaxInt32)*time.Millisecond); err != nil || received != math.MaxInt32 {
		t.Fatalf("maximum native timeout was changed or rejected: %d, %v", received, err)
	}
}

func TestNativeCopiesRejectInvalidBounds(t *testing.T) {
	value := []byte{'a', 0}
	if _, err := nativeBytes(&value[0], 1, -1); err == nil {
		t.Fatal("negative native buffer bound was accepted")
	}
	if _, err := nativeBytes(&value[0], 2, 1); err == nil {
		t.Fatal("oversized native buffer was accepted")
	}
	for _, maximum := range []int{-1, maxNativeBytes + 1} {
		if _, err := nativeString(&value[0], maximum); err == nil {
			t.Fatalf("invalid native string bound %d was accepted", maximum)
		}
	}
	copied, err := nativeBytes(&value[0], 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	value[0] = 'b'
	if string(copied) != "a" {
		t.Fatal("native buffer escaped without a Go-owned copy")
	}
}

func TestNativeRegistrationPreservesExclusionsAndDoesNotRetryPIN(t *testing.T) {
	registry := hardwarekey.Registry{UserID: make([]byte, 32)}
	request, _, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	excluded := []byte{1, 2, 3}
	request.Response.CredentialExcludeList = []protocol.CredentialDescriptor{{Type: "public-key", CredentialID: excluded}}
	options, err := parseOptions(marshal(t, request), true)
	if err != nil {
		t.Fatal(err)
	}
	calls, freed, exclusions := 0, false, 0
	lib := &library{
		credNew: func() uintptr { return 1 },
		credFree: func(pointer *uintptr) {
			freed = true
			*pointer = 0
		},
		credType: func(_ uintptr, algorithm int32) int32 {
			if algorithm != -7 {
				t.Fatal("wrong algorithm")
			}
			return 0
		},
		credRP: func(_ uintptr, id, _ string) int32 {
			if id != "localhost" {
				t.Fatal("wrong relying party")
			}
			return 0
		},
		credUser: func(_ uintptr, _ *byte, length uintptr, _, _ string, icon *byte) int32 {
			if length != 32 || icon != nil {
				t.Fatal("wrong user parameters")
			}
			return 0
		},
		credHash: func(_ uintptr, _ *byte, length uintptr) int32 {
			if length != 32 {
				t.Fatal("wrong hash length")
			}
			return 0
		},
		credRK: func(_ uintptr, option int32) int32 {
			if option != 1 {
				t.Fatal("resident key was not disabled")
			}
			return 0
		},
		credUV: func(_ uintptr, option int32) int32 {
			if option != 2 {
				t.Fatal("verification was not required")
			}
			return 0
		},
		credExclude: func(_ uintptr, pointer *byte, length uintptr) int32 {
			actual, err := nativeBytes(pointer, length, maxCredentialID)
			if err != nil || !bytes.Equal(actual, excluded) {
				t.Fatal("excluded credential was changed")
			}
			exclusions++
			return 0
		},
		makeCredential: func(_, _ uintptr, _ *byte) int32 {
			calls++
			return 0x31
		},
	}
	if _, err := create(lib, 1, options, nil, [32]byte{}, nil); err == nil || !strings.Contains(err.Error(), "no automatic retry") {
		t.Fatal("incorrect PIN did not fail safely")
	}
	if calls != 1 || !freed || exclusions != 1 {
		t.Fatalf("unexpected native lifecycle: calls=%d freed=%t exclusions=%d", calls, freed, exclusions)
	}
}
