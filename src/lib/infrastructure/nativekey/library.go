package nativekey

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// These signatures mirror Yubico's public fido.h. Opaque objects are never
// dereferenced in Go. C int/enum is int32; size_t is uintptr; C buffers use *byte.
// https://github.com/Yubico/libfido2/blob/1.15.0/src/fido.h
type library struct {
	initialize         func(int32)
	infoNew            func(uintptr) uintptr
	infoFree           func(*uintptr, uintptr)
	manifest           func(uintptr, uintptr, *uintptr) int32
	infoAt             func(uintptr, uintptr) uintptr
	infoPath           func(uintptr) *byte
	infoManufacturer   func(uintptr) *byte
	infoProduct        func(uintptr) *byte
	devNew             func() uintptr
	devFree            func(*uintptr)
	devOpen            func(uintptr, string) int32
	devClose           func(uintptr) int32
	devTimeout         func(uintptr, int32) int32
	devFIDO2           func(uintptr) bool
	devHasPIN          func(uintptr) bool
	makeCredential     func(uintptr, uintptr, *byte) int32
	getAssertion       func(uintptr, uintptr, *byte) int32
	credNew            func() uintptr
	credFree           func(*uintptr)
	credType           func(uintptr, int32) int32
	credRP             func(uintptr, string, string) int32
	credUser           func(uintptr, *byte, uintptr, string, string, *byte) int32
	credHash           func(uintptr, *byte, uintptr) int32
	credRK             func(uintptr, int32) int32
	credUV             func(uintptr, int32) int32
	credExclude        func(uintptr, *byte, uintptr) int32
	credID             func(uintptr) *byte
	credIDLen          func(uintptr) uintptr
	credAuthdata       func(uintptr) *byte
	credAuthdataLen    func(uintptr) uintptr
	credFormat         func(uintptr) *byte
	credStatement      func(uintptr) *byte
	credStatementLen   func(uintptr) uintptr
	assertNew          func() uintptr
	assertFree         func(*uintptr)
	assertRP           func(uintptr, string) int32
	assertHash         func(uintptr, *byte, uintptr) int32
	assertUP           func(uintptr, int32) int32
	assertUV           func(uintptr, int32) int32
	assertAllow        func(uintptr, *byte, uintptr) int32
	assertCount        func(uintptr) uintptr
	assertID           func(uintptr, uintptr) *byte
	assertIDLen        func(uintptr, uintptr) uintptr
	assertAuthdata     func(uintptr, uintptr) *byte
	assertAuthdataLen  func(uintptr, uintptr) uintptr
	assertSignature    func(uintptr, uintptr) *byte
	assertSignatureLen func(uintptr, uintptr) uintptr
	assertUser         func(uintptr, uintptr) *byte
	assertUserLen      func(uintptr, uintptr) uintptr
}

var libraryOnce sync.Once
var loadedLibrary *library
var libraryError error

func getLibrary() (*library, error) {
	libraryOnce.Do(func() {
		loadedLibrary, libraryError = loadLibrary()
	})
	return loadedLibrary, libraryError
}

func (lib *library) bindings() []struct {
	name        string
	destination any
} {
	return []struct {
		name        string
		destination any
	}{
		{"fido_init", &lib.initialize},
		{"fido_dev_info_new", &lib.infoNew},
		{"fido_dev_info_free", &lib.infoFree},
		{"fido_dev_info_manifest", &lib.manifest},
		{"fido_dev_info_ptr", &lib.infoAt},
		{"fido_dev_info_path", &lib.infoPath},
		{"fido_dev_info_manufacturer_string", &lib.infoManufacturer},
		{"fido_dev_info_product_string", &lib.infoProduct},
		{"fido_dev_new", &lib.devNew},
		{"fido_dev_free", &lib.devFree},
		{"fido_dev_open", &lib.devOpen},
		{"fido_dev_close", &lib.devClose},
		{"fido_dev_set_timeout", &lib.devTimeout},
		{"fido_dev_is_fido2", &lib.devFIDO2},
		{"fido_dev_has_pin", &lib.devHasPIN},
		{"fido_dev_make_cred", &lib.makeCredential},
		{"fido_dev_get_assert", &lib.getAssertion},
		{"fido_cred_new", &lib.credNew},
		{"fido_cred_free", &lib.credFree},
		{"fido_cred_set_type", &lib.credType},
		{"fido_cred_set_rp", &lib.credRP},
		{"fido_cred_set_user", &lib.credUser},
		{"fido_cred_set_clientdata_hash", &lib.credHash},
		{"fido_cred_set_rk", &lib.credRK},
		{"fido_cred_set_uv", &lib.credUV},
		{"fido_cred_exclude", &lib.credExclude},
		{"fido_cred_id_ptr", &lib.credID},
		{"fido_cred_id_len", &lib.credIDLen},
		{"fido_cred_authdata_ptr", &lib.credAuthdata},
		{"fido_cred_authdata_len", &lib.credAuthdataLen},
		{"fido_cred_fmt", &lib.credFormat},
		{"fido_cred_attstmt_ptr", &lib.credStatement},
		{"fido_cred_attstmt_len", &lib.credStatementLen},
		{"fido_assert_new", &lib.assertNew},
		{"fido_assert_free", &lib.assertFree},
		{"fido_assert_set_rp", &lib.assertRP},
		{"fido_assert_set_clientdata_hash", &lib.assertHash},
		{"fido_assert_set_up", &lib.assertUP},
		{"fido_assert_set_uv", &lib.assertUV},
		{"fido_assert_allow_cred", &lib.assertAllow},
		{"fido_assert_count", &lib.assertCount},
		{"fido_assert_id_ptr", &lib.assertID},
		{"fido_assert_id_len", &lib.assertIDLen},
		{"fido_assert_authdata_ptr", &lib.assertAuthdata},
		{"fido_assert_authdata_len", &lib.assertAuthdataLen},
		{"fido_assert_sig_ptr", &lib.assertSignature},
		{"fido_assert_sig_len", &lib.assertSignatureLen},
		{"fido_assert_user_id_ptr", &lib.assertUser},
		{"fido_assert_user_id_len", &lib.assertUserLen},
	}
}

func nativeBytes(pointer *byte, length uintptr, maximum int) ([]byte, error) {
	if length > uintptr(maximum) || (length != 0 && pointer == nil) {
		return nil, errors.New("security key returned an invalid or oversized field")
	}
	if length == 0 {
		return nil, nil
	}
	return append([]byte(nil), unsafe.Slice(pointer, int(length))...), nil
}

func nativeString(pointer *byte, maximum int) (string, error) {
	if pointer == nil {
		return "", nil
	}
	value := make([]byte, 0, maximum)
	for offset := 0; offset <= maximum; offset++ {
		character := *(*byte)(unsafe.Add(unsafe.Pointer(pointer), offset))
		if character == 0 {
			return string(value), nil
		}
		value = append(value, character)
	}
	return "", errors.New("security key returned an oversized string")
}

func nativeError(code int32) error {
	switch code {
	case 0:
		return nil
	case 0x19:
		return errors.New("this security key is already enrolled")
	case 0x2e:
		return errors.New("the selected security key has no enrolled credential for this owner")
	case 0x31:
		return errors.New("security-key PIN was incorrect; no automatic retry was attempted")
	case 0x32:
		return errors.New("security-key PIN is blocked; use the manufacturer's management tool")
	case 0x34:
		return errors.New("security-key PIN is temporarily blocked; reconnect the key before a deliberate retry")
	case 0x35:
		return errors.New("configure a FIDO2 PIN using the manufacturer's management tool before pairing")
	case 0x36:
		return errors.New("the security key requires its existing FIDO2 PIN")
	case 0x2d, 0x27:
		return errors.New("security-key operation was cancelled or denied")
	case 0x05, 0x2f, 0x3a:
		return errors.New("security-key operation timed out; reconnect or touch the selected key and retry deliberately")
	case -2:
		return errors.New("security key did not respond; it may have disconnected or reached its timeout")
	default:
		return fmt.Errorf("native FIDO2 operation failed (code %d); verify key access and FIDO2 support", code)
	}
}
