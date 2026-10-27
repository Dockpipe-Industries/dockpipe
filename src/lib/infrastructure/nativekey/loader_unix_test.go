//go:build (linux || darwin) && (amd64 || arm64)

package nativekey

import (
	"runtime"
	"testing"
)

func TestMissingNativeLibraryFailsClosed(t *testing.T) {
	if _, err := loadFromPaths([]string{"/nonexistent/dockpipe-native-key/libfido2.so.1"}); err == nil {
		t.Fatal("missing native library was accepted")
	}
}

// This exercises the installed ABI using memory-only credential operations.
// It never enumerates, opens, authenticates, or changes a physical device.
func TestInstalledLibraryCredentialABI(t *testing.T) {
	lib, err := loadLibrary()
	if err != nil {
		t.Skip("system libfido2 is unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	lib.initialize(2)
	credential := lib.credNew()
	if credential == 0 {
		t.Fatal("native allocation failed")
	}
	defer lib.credFree(&credential)
	for _, code := range []int32{lib.credType(credential, -7), lib.credRP(credential, "localhost", "Dockpipe"), lib.credRK(credential, 1), lib.credUV(credential, 2)} {
		if err := nativeError(code); err != nil {
			t.Fatal(err)
		}
	}
	assertion := lib.assertNew()
	if assertion == 0 {
		t.Fatal("native assertion allocation failed")
	}
	defer lib.assertFree(&assertion)
	if lib.assertCount(assertion) != 0 {
		t.Fatal("new assertion is not empty")
	}
}
