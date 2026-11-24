//go:build windows

package hostboot

import (
	"encoding/binary"
	"errors"

	"golang.org/x/sys/windows"
)

// WindowsSequence reads the boot sequence the kernel publishes to user mode.
func WindowsSequence() (uint32, error) {
	// KUSER_SHARED_DATA is mapped read-only in user space. BootId is a ULONG
	// at offset 0x2c4 on Windows NT 10, including Windows 11.
	const bootSequenceAddress = 0x7ffe0000 + 0x2c4
	if windows.RtlGetVersion().MajorVersion != 10 {
		return 0, errors.New("Windows boot sequence layout is unsupported")
	}

	var data [4]byte
	var read uintptr
	err := windows.ReadProcessMemory(windows.CurrentProcess(), bootSequenceAddress, &data[0], uintptr(len(data)), &read)
	if err != nil {
		return 0, err
	}

	sequence := binary.LittleEndian.Uint32(data[:])
	if read != uintptr(len(data)) || sequence == 0 {
		return 0, errors.New("Windows boot sequence is unavailable")
	}

	return sequence, nil
}
