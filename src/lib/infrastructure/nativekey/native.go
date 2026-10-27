package nativekey

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Device is an enumerated authenticator. Its transport path is intentionally
// private: JSON or UI callers select a retained Device by ID, never a path.
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	path string
}

// Serializing native access prevents overlapping ceremonies on the same key.
// Waiting for admission is cancellable; library object lifetimes stay synchronous.
var nativeAdmission = make(chan struct{}, 1)

func enter(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case nativeAdmission <- struct{}{}:
		return func() { <-nativeAdmission }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func prepareLibrary() (*library, error) {
	// libfido2 enables debugging from this environment variable even when its
	// caller passes no debug flag. Do not risk native device/PIN diagnostics.
	if _, enabled := os.LookupEnv("FIDO_DEBUG"); enabled {
		return nil, errors.New("unset FIDO_DEBUG before using native security keys")
	}
	return getLibrary()
}

// Devices enumerates at most eight devices without opening them or attempting
// PIN authentication. Device names are bounded public display labels.
func Devices(ctx context.Context) ([]Device, error) {
	release, err := enter(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	lib, err := prepareLibrary()
	if err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// The disable-fallback setting is thread-local in libfido2.
	lib.initialize(2)
	manifest := lib.infoNew(maxDevices)
	if manifest == 0 {
		return nil, errors.New("cannot allocate native security-key device list")
	}
	defer lib.infoFree(&manifest, maxDevices)
	var count uintptr
	if err := nativeError(lib.manifest(manifest, maxDevices, &count)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if count > maxDevices {
		return nil, errors.New("native security-key manifest exceeds its capacity")
	}
	devices := make([]Device, 0, int(count))
	for index := uintptr(0); index < count; index++ {
		info := lib.infoAt(manifest, index)
		if info == 0 {
			return nil, errors.New("invalid native security-key manifest entry")
		}
		path, err := nativeString(lib.infoPath(info), 4096)
		if err != nil || path == "" || !utf8.ValidString(path) {
			return nil, errors.New("invalid native security-key device path")
		}
		manufacturer, err := nativeString(lib.infoManufacturer(info), 128)
		if err != nil {
			return nil, err
		}
		product, err := nativeString(lib.infoProduct(info), 128)
		if err != nil {
			return nil, err
		}
		name := displayName(manufacturer, product)
		devices = append(devices, Device{ID: strconv.Itoa(int(index)), Name: name, path: path})
	}
	return devices, nil
}

func displayName(manufacturer, product string) string {
	name := strings.TrimSpace(manufacturer + " " + product)
	name = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) || unicode.Is(unicode.Cf, character) || character == utf8.RuneError {
			return -1
		}
		return character
	}, name)
	characters := []rune(name)
	if len(characters) > 80 {
		name = string(characters[:80])
	}
	if name == "" {
		return "FIDO2 security key"
	}
	return name
}

// Create consumes and clears pin, then returns a standard WebAuthn response.
// No PIN configuration, key reset, or automatic PIN retry is performed. A
// cancelled call joins its synchronous native operation before freeing anything;
// cancellation can therefore take up to the remaining ceremony timeout.
func Create(ctx context.Context, device Device, optionsJSON []byte, pin []byte) ([]byte, error) {
	return run(ctx, device, optionsJSON, pin, true)
}

// Assert has the same PIN ownership and cancellation contract as Create.
func Assert(ctx context.Context, device Device, optionsJSON []byte, pin []byte) ([]byte, error) {
	return run(ctx, device, optionsJSON, pin, false)
}

func run(ctx context.Context, device Device, value, pin []byte, creation bool) ([]byte, error) {
	defer clear(pin)
	options, err := parseOptions(value, creation)
	if err != nil {
		return nil, err
	}
	if device.path == "" || len(device.path) > 4096 || strings.ContainsRune(device.path, 0) {
		return nil, errors.New("select a device returned by native security-key enumeration")
	}
	if len(pin) > 63 || !utf8.Valid(pin) || utf8.RuneCount(pin) < 4 || bytes.IndexByte(pin, 0) >= 0 {
		return nil, errors.New("supply the existing FIDO2 PIN (at least four characters and at most 63 UTF-8 bytes)")
	}
	ctx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()
	release, err := enter(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	lib, err := prepareLibrary()
	if err != nil {
		return nil, err
	}
	clientJSON, hash, err := clientData(options, creation)
	if err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	lib.initialize(2)
	dev := lib.devNew()
	if dev == 0 {
		return nil, errors.New("cannot allocate native security-key device")
	}
	defer lib.devFree(&dev)
	if err := setTimeout(ctx, lib, dev, 5*time.Second); err != nil {
		return nil, err
	}
	if err := nativeError(lib.devOpen(dev, device.path)); err != nil {
		return nil, err
	}
	defer lib.devClose(dev)
	if !lib.devFIDO2(dev) {
		return nil, errors.New("the selected security key must support FIDO2; U2F fallback is disabled")
	}
	if !lib.devHasPIN(dev) {
		return nil, errors.New("configure a FIDO2 PIN using the manufacturer's management tool before pairing")
	}
	if err := setTimeout(ctx, lib, dev, maxCeremony); err != nil {
		return nil, err
	}
	secret := make([]byte, len(pin)+1)
	copy(secret, pin)
	defer clear(secret)
	var response []byte
	if creation {
		response, err = create(lib, dev, options, clientJSON, hash, &secret[0])
	} else {
		response, err = assert(lib, dev, options, clientJSON, hash, &secret[0])
	}
	runtime.KeepAlive(secret)
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	return response, err
}

func setTimeout(ctx context.Context, lib *library, dev uintptr, maximum time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < maximum {
			maximum = remaining
		}
	}
	milliseconds := maximum.Milliseconds()
	if milliseconds < 1 {
		return context.DeadlineExceeded
	}
	return nativeError(lib.devTimeout(dev, int32(milliseconds)))
}

func create(lib *library, dev uintptr, options ceremonyOptions, clientJSON []byte, hash [32]byte, pin *byte) ([]byte, error) {
	credential := lib.credNew()
	if credential == 0 {
		return nil, errors.New("cannot allocate native security-key credential")
	}
	defer lib.credFree(&credential)
	public := options.creation
	setters := []func() int32{
		func() int32 { return lib.credType(credential, -7) },
		func() int32 { return lib.credRP(credential, public.RelyingParty.ID, public.RelyingParty.Name) },
		func() int32 {
			return lib.credUser(credential, &options.userID[0], uintptr(len(options.userID)), public.User.Name, public.User.DisplayName, nil)
		},
		func() int32 { return lib.credHash(credential, &hash[0], uintptr(len(hash))) },
		func() int32 { return lib.credRK(credential, 1) }, // FIDO_OPT_FALSE.
		func() int32 { return lib.credUV(credential, 2) }, // FIDO_OPT_TRUE.
	}
	for _, setter := range setters {
		if err := nativeError(setter()); err != nil {
			return nil, err
		}
	}
	for _, excluded := range public.CredentialExcludeList {
		if err := nativeError(lib.credExclude(credential, &excluded.CredentialID[0], uintptr(len(excluded.CredentialID)))); err != nil {
			return nil, err
		}
	}
	if err := nativeError(lib.makeCredential(dev, credential, pin)); err != nil {
		return nil, err
	}
	id, err := nativeBytes(lib.credID(credential), lib.credIDLen(credential), maxCredentialID)
	if err != nil {
		return nil, err
	}
	authdata, err := nativeBytes(lib.credAuthdata(credential), lib.credAuthdataLen(credential), maxNativeBytes)
	if err != nil {
		return nil, err
	}
	statement, err := nativeBytes(lib.credStatement(credential), lib.credStatementLen(credential), maxNativeBytes)
	if err != nil {
		return nil, err
	}
	format, err := nativeString(lib.credFormat(credential), 32)
	if err != nil {
		return nil, err
	}
	return registrationResponse(clientJSON, id, authdata, format, statement)
}

func assert(lib *library, dev uintptr, options ceremonyOptions, clientJSON []byte, hash [32]byte, pin *byte) ([]byte, error) {
	assertion := lib.assertNew()
	if assertion == 0 {
		return nil, errors.New("cannot allocate native security-key assertion")
	}
	defer lib.assertFree(&assertion)
	setters := []func() int32{
		func() int32 { return lib.assertRP(assertion, options.assertion.RelyingPartyID) },
		func() int32 { return lib.assertHash(assertion, &hash[0], uintptr(len(hash))) },
		func() int32 { return lib.assertUP(assertion, 2) },
		func() int32 { return lib.assertUV(assertion, 2) },
	}
	for _, setter := range setters {
		if err := nativeError(setter()); err != nil {
			return nil, err
		}
	}
	for _, allowed := range options.assertion.AllowedCredentials {
		if err := nativeError(lib.assertAllow(assertion, &allowed.CredentialID[0], uintptr(len(allowed.CredentialID)))); err != nil {
			return nil, err
		}
	}
	if err := nativeError(lib.getAssertion(dev, assertion, pin)); err != nil {
		return nil, err
	}
	if lib.assertCount(assertion) != 1 {
		return nil, errors.New("security key must return exactly one assertion")
	}
	id, err := nativeBytes(lib.assertID(assertion, 0), lib.assertIDLen(assertion, 0), maxCredentialID)
	if err != nil {
		return nil, err
	}
	authdata, err := nativeBytes(lib.assertAuthdata(assertion, 0), lib.assertAuthdataLen(assertion, 0), maxNativeBytes)
	if err != nil {
		return nil, err
	}
	signature, err := nativeBytes(lib.assertSignature(assertion, 0), lib.assertSignatureLen(assertion, 0), 256)
	if err != nil {
		return nil, err
	}
	user, err := nativeBytes(lib.assertUser(assertion, 0), lib.assertUserLen(assertion, 0), 64)
	if err != nil {
		return nil, err
	}
	return assertionResponse(clientJSON, id, authdata, signature, user, options)
}
