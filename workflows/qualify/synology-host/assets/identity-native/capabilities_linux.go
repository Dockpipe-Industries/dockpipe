//go:build linux

package main

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	"dockpipe/workflows/internal/synology/hostidentity"
)

const capabilityPair = "00000000000000c0"
const noCapabilities = "0000000000000000"

func admitCapabilities(metadata hostidentity.Identity, inherited bool) bool {
	expectedInherited := noCapabilities
	if inherited {
		expectedInherited = capabilityPair
	}
	return metadata.UID > 0 && metadata.EUID == metadata.UID && metadata.SavedUID == metadata.UID && metadata.FSUID == metadata.UID &&
		metadata.Capabilities["CapEff"] == capabilityPair && metadata.Capabilities["CapPrm"] == capabilityPair &&
		metadata.Capabilities["CapInh"] == expectedInherited && metadata.Capabilities["CapAmb"] == expectedInherited
}

func prepareCapabilities(inherited bool) (hostidentity.Status, error) {
	before, err := hostidentity.Read(schema)
	if err != nil || !admitCapabilities(before.Identity, inherited) {
		return hostidentity.Status{}, errors.New("required non-root CAP_SETUID and CAP_SETGID admission failed")
	}
	if _, _, errno := syscall.AllThreadsSyscall(syscall.SYS_PRCTL, 38, 1, 0); errno != 0 {
		return hostidentity.Status{}, errors.New("cannot enable no-new-privileges on all threads")
	}
	// The ordinary worker receives only the daemon's existing two capabilities
	// through ambient propagation. Remove inheritable/ambient sets immediately.
	header := struct{ Version, PID uint32 }{Version: 0x20080522}
	data := [2]struct{ Effective, Permitted, Inheritable uint32 }{{Effective: 0xc0, Permitted: 0xc0}}
	_, _, errno := syscall.AllThreadsSyscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&header)), uintptr(unsafe.Pointer(&data)), 0)
	runtime.KeepAlive(header)
	runtime.KeepAlive(data)
	if errno != 0 {
		return hostidentity.Status{}, errors.New("cannot restrict worker capability inheritance")
	}
	if _, _, errno := syscall.AllThreadsSyscall6(syscall.SYS_PRCTL, 47, 4, 0, 0, 0, 0); errno != 0 {
		return hostidentity.Status{}, errors.New("ambient capability support is required for bounded worker supervision")
	}
	if _, _, errno := syscall.AllThreadsSyscall(syscall.SYS_PRCTL, 4, 0, 0); errno != 0 {
		return hostidentity.Status{}, errors.New("cannot disable process dumps")
	}
	after, err := hostidentity.Read(schema)
	if err != nil || !admitCapabilities(after.Identity, false) || after.Identity.NoNewPrivs == nil || !*after.Identity.NoNewPrivs {
		return hostidentity.Status{}, errors.New("restricted capability state was not established")
	}
	return after, nil
}

func unprivileged(metadata hostidentity.Identity) bool {
	if metadata.UID <= 0 || metadata.NoNewPrivs == nil || !*metadata.NoNewPrivs {
		return false
	}
	for _, name := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		if metadata.Capabilities[name] != noCapabilities {
			return false
		}
	}
	return true
}
