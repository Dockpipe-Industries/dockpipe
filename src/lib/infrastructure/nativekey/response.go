package nativekey

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/fxamacker/cbor/v2"
)

var strictCBOR = func() cbor.DecMode {
	mode, err := (cbor.DecOptions{
		DupMapKey:        cbor.DupMapKeyEnforcedAPF,
		MaxNestedLevels:  8,
		MaxArrayElements: 16,
		MaxMapPairs:      16,
		IndefLength:      cbor.IndefLengthForbidden,
		TagsMd:           cbor.TagsForbidden,
	}).DecMode()
	if err != nil {
		panic(err)
	}
	return mode
}()

// libfido2's *_authdata_ptr returns a CBOR byte string, not authenticatorData.
// Decode it once, including on versions without assertion *_authdata_raw_ptr.
func rawAuthenticatorData(wrapped []byte, id []byte, creation bool) ([]byte, error) {
	if len(wrapped) == 0 || len(wrapped) > maxNativeBytes {
		return nil, errors.New("invalid native authenticator data length")
	}
	var raw []byte
	if err := strictCBOR.Unmarshal(wrapped, &raw); err != nil || len(raw) < 37 || len(raw) > 4096 {
		return nil, errors.New("invalid CBOR native authenticator data")
	}
	rpHash := sha256.Sum256([]byte("localhost"))
	expectedFlags := byte(0x05) // UP and UV; no backup, extensions, or reserved flags.
	if creation {
		expectedFlags |= 0x40 // Attested credential data.
	}
	if !bytes.Equal(raw[:32], rpHash[:]) || raw[32] != expectedFlags {
		return nil, errors.New("security key did not provide required presence, verification, and device-bound authenticator data")
	}
	if !creation {
		if len(raw) != 37 {
			return nil, errors.New("unexpected native assertion authenticator data")
		}
		return raw, nil
	}
	if len(raw) < 55 {
		return nil, errors.New("native attested credential data is truncated")
	}
	idLength := int(binary.BigEndian.Uint16(raw[53:55]))
	if idLength != len(id) || idLength == 0 || idLength > maxCredentialID || len(raw) <= 55+idLength || !bytes.Equal(raw[55:55+idLength], id) {
		return nil, errors.New("native attested credential identifier does not match")
	}
	var publicKey map[int]cbor.RawMessage
	if err := strictCBOR.Unmarshal(raw[55+idLength:], &publicKey); err != nil || len(publicKey) != 5 {
		return nil, errors.New("invalid native ES256 credential public key")
	}
	for label, expected := range map[int]int{1: 2, 3: -7, -1: 1} {
		var actual int
		if err := strictCBOR.Unmarshal(publicKey[label], &actual); err != nil || actual != expected {
			return nil, errors.New("security key did not return an ES256 P-256 public key")
		}
	}
	for _, label := range []int{-2, -3} {
		var coordinate []byte
		if err := strictCBOR.Unmarshal(publicKey[label], &coordinate); err != nil || len(coordinate) != 32 {
			return nil, errors.New("invalid native ES256 public key coordinate")
		}
	}
	return raw, nil
}

func registrationResponse(clientJSON, id, wrapped []byte, format string, statement []byte) ([]byte, error) {
	if len(id) == 0 || len(id) > maxCredentialID || len(statement) == 0 || len(statement) > maxNativeBytes || (format != "packed" && format != "fido-u2f") {
		return nil, errors.New("security key did not return supported manufacturer attestation")
	}
	raw, err := rawAuthenticatorData(wrapped, id, true)
	if err != nil {
		return nil, err
	}
	var fields map[string]cbor.RawMessage
	if err := strictCBOR.Unmarshal(statement, &fields); err != nil {
		return nil, errors.New("invalid native attestation statement")
	}
	var signature []byte
	var certificates [][]byte
	if strictCBOR.Unmarshal(fields["sig"], &signature) != nil || len(signature) == 0 || len(signature) > 256 ||
		strictCBOR.Unmarshal(fields["x5c"], &certificates) != nil || len(certificates) == 0 || len(certificates) > 8 {
		return nil, errors.New("native attestation requires a bounded signature and manufacturer certificate chain")
	}
	for _, certificate := range certificates {
		if len(certificate) == 0 || len(certificate) > 8192 {
			return nil, errors.New("invalid native attestation certificate length")
		}
	}
	expectedFields := 2
	if format == "packed" {
		expectedFields = 3
		var algorithm int
		if strictCBOR.Unmarshal(fields["alg"], &algorithm) != nil || algorithm != -7 {
			return nil, errors.New("native packed attestation must use ES256")
		}
	}
	if len(fields) != expectedFields {
		return nil, errors.New("unsupported native attestation statement fields")
	}
	attestation, err := cbor.Marshal(struct {
		AuthData  []byte          `cbor:"authData"`
		Format    string          `cbor:"fmt"`
		Statement cbor.RawMessage `cbor:"attStmt"`
	}{raw, format, statement})
	if err != nil {
		return nil, err
	}
	return marshalResponse(id, map[string]any{
		"clientDataJSON":    encode(clientJSON),
		"attestationObject": encode(attestation),
	})
}

func assertionResponse(clientJSON, id, wrapped, signature, user []byte, options ceremonyOptions) ([]byte, error) {
	if len(id) == 0 || len(id) > maxCredentialID || len(signature) == 0 || len(signature) > 256 || len(user) > 64 {
		return nil, errors.New("invalid native assertion fields")
	}
	allowed := false
	for _, credential := range options.assertion.AllowedCredentials {
		if bytes.Equal(credential.CredentialID, id) {
			allowed = true
		}
	}
	if !allowed {
		return nil, errors.New("security key selected a credential outside the allowlist")
	}
	raw, err := rawAuthenticatorData(wrapped, nil, false)
	if err != nil {
		return nil, err
	}
	var userHandle any
	if len(user) != 0 {
		userHandle = encode(user)
	}
	return marshalResponse(id, map[string]any{
		"clientDataJSON":    encode(clientJSON),
		"authenticatorData": encode(raw),
		"signature":         encode(signature),
		"userHandle":        userHandle,
	})
}

func encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func marshalResponse(id []byte, response any) ([]byte, error) {
	value, err := json.Marshal(struct {
		ID         string         `json:"id"`
		RawID      string         `json:"rawId"`
		Type       string         `json:"type"`
		Attachment string         `json:"authenticatorAttachment"`
		Extensions map[string]any `json:"clientExtensionResults"`
		Response   any            `json:"response"`
	}{encode(id), encode(id), "public-key", "cross-platform", map[string]any{}, response})
	if err == nil && len(value) > 64<<10 {
		return nil, errors.New("native security-key response exceeds its size bound")
	}
	return value, err
}
