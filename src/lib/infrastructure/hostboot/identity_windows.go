//go:build windows

package hostboot

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var querySystemInformation = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQuerySystemInformation")

// WindowsIdentifier returns the boot environment GUID. It can persist across
// kernel lifetimes and must be paired with the boot sequence when fencing one.
func WindowsIdentifier() (windows.GUID, error) {
	// SystemBootEnvironmentInformation=90. Fail closed on changes to this ABI:
	// https://github.com/winsiderss/systeminformer/blob/master/phnt/include/ntexapi.h
	var boot struct {
		Identifier   windows.GUID
		FirmwareType uint32
		Padding      uint32
		Flags        uint64
	}

	if err := querySystemInformation.Find(); err != nil {
		return windows.GUID{}, err
	}

	var returned uint32
	status, _, _ := querySystemInformation.Call(90, uintptr(unsafe.Pointer(&boot)), unsafe.Sizeof(boot), uintptr(unsafe.Pointer(&returned)))
	if int32(status) < 0 || returned != uint32(unsafe.Sizeof(boot)) || boot.Identifier == (windows.GUID{}) {
		return windows.GUID{}, errors.New("Windows boot information unavailable or changed")
	}

	return boot.Identifier, nil
}
