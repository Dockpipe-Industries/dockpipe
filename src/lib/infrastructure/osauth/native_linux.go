//go:build linux && (amd64 || arm64)

package osauth

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

type sessionLibrary struct {
	pidSession    func(int32, **byte) int32
	pidOwner      func(int32, *uint32) int32
	uidDisplay    func(uint32, **byte) int32
	sessionUID    func(string, *uint32) int32
	sessionActive func(string) int32
	sessionRemote func(string) int32
	free          func(*byte)
}

var systemdOnce sync.Once
var systemdLibrary *sessionLibrary
var systemdError error

func loadSystemd() (*sessionLibrary, error) {
	systemdOnce.Do(func() {
		library := &sessionLibrary{}
		bindings := []nativeBinding{
			{"sd_pid_get_session", &library.pidSession},
			{"sd_pid_get_owner_uid", &library.pidOwner},
			{"sd_uid_get_display", &library.uidDisplay},
			{"sd_session_get_uid", &library.sessionUID},
			{"sd_session_is_active", &library.sessionActive},
			{"sd_session_is_remote", &library.sessionRemote},
			{"free", &library.free},
		}
		systemdError = bindSystemLibrary("libsystemd.so.0", bindings)
		if systemdError == nil {
			systemdLibrary = library
		}
	})
	return systemdLibrary, systemdError
}

type nativeBinding struct {
	name        string
	destination any
}

// Handles stay mapped for the lifetime of their published function pointers.
func bindSystemLibrary(name string, bindings []nativeBinding) error {
	multiarch := "x86_64-linux-gnu"
	if runtime.GOARCH == "arm64" {
		multiarch = "aarch64-linux-gnu"
	}
	roots := []string{"/usr/lib/" + multiarch, "/lib/" + multiarch, "/usr/lib64", "/lib64", "/usr/lib", "/lib"}
	for _, root := range roots {
		handle, err := purego.Dlopen(root+"/"+name, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			continue
		}
		for _, binding := range bindings {
			symbol, err := purego.Dlsym(handle, binding.name)
			if err != nil {
				_ = purego.Dlclose(handle)
				return errors.New("installed native OS library lacks a required security API")
			}
			purego.RegisterFunc(binding.destination, symbol)
		}
		return nil
	}
	return errors.New("required native OS security library is unavailable")
}

func copyNativeSession(value *byte) (string, error) {
	if value == nil {
		return "", errors.New("OS session identity is unavailable")
	}
	var data []byte
	for index := 0; index < 128; index++ {
		character := *(*byte)(unsafe.Add(unsafe.Pointer(value), index))
		if character == 0 {
			if len(data) == 0 {
				break
			}
			return string(data), nil
		}
		if character < 33 || character > 126 {
			break
		}
		data = append(data, character)
	}
	return "", errors.New("OS session identity is invalid or exceeds its bound")
}

func nativeProcessSession(pid, uid int) (string, error) {
	library, err := loadSystemd()
	if err != nil {
		return "", err
	}
	var session *byte
	if library.pidSession(int32(pid), &session) < 0 || session == nil {
		// DBus-activated GUI applications may live in the user's service manager
		// rather than a login-session scope. Verify that native owner association
		// before selecting its active local desktop. A process with a known remote
		// login session never reaches this fallback.
		var owner uint32
		if library.pidOwner(int32(pid), &owner) < 0 || owner != uint32(uid) {
			return "", errors.New("caller does not belong to a local login session")
		}
		return nativeDisplaySession(uid)
	}
	defer library.free(session)
	name, err := copyNativeSession(session)
	if err != nil {
		return "", err
	}
	return name, verifyNativeSession(library, name, uid)
}

func nativeDisplaySession(uid int) (string, error) {
	library, err := loadSystemd()
	if err != nil {
		return "", err
	}
	var session *byte
	if library.uidDisplay(uint32(uid), &session) < 0 || session == nil {
		return "", errors.New("no local owner desktop session is available")
	}
	defer library.free(session)
	name, err := copyNativeSession(session)
	if err != nil {
		return "", err
	}
	return name, verifyNativeSession(library, name, uid)
}

func verifyNativeSession(library *sessionLibrary, name string, uid int) error {
	var owner uint32
	if library.sessionUID(name, &owner) < 0 || owner != uint32(uid) ||
		library.sessionActive(name) != 1 || library.sessionRemote(name) != 0 {
		return errors.New("remote unlock requires the current account's active local login session")
	}
	return nil
}
