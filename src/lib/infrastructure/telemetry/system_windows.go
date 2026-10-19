//go:build windows

package telemetry

import (
	"context"
	"errors"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	systemKernel32        = windows.NewLazySystemDLL("kernel32.dll")
	systemTimes           = systemKernel32.NewProc("GetSystemTimes")
	systemMemoryStatus    = systemKernel32.NewProc("GlobalMemoryStatusEx")
	systemBootInformation = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQuerySystemInformation")
)

var systemProcessorGroupCount = systemKernel32.NewProc("GetActiveProcessorGroupCount")

type nativeWindowsSystem struct{}

// System samples local kernel counters. Native calls are synchronous; cancellation
// is checked between calls and interfaces, with bounded enumeration and retries.
func (NativeCollector) System(ctx context.Context) (SystemResources, error) {
	return collectWindowsSystem(ctx, nativeWindowsSystem{})
}

func (nativeWindowsSystem) bootID() (string, error) {
	// SYSTEM_BOOT_ENVIRONMENT_INFORMATION, SystemBootEnvironmentInformation=90.
	// This kernel boot GUID is shared by independent collector processes. The
	// private ABI is isolated and fails closed if unavailable or changed:
	// https://github.com/winsiderss/systeminformer/blob/master/phnt/include/ntexapi.h
	var boot struct {
		Identifier   windows.GUID
		FirmwareType uint32
		Padding      uint32
		Flags        uint64
	}
	if err := systemBootInformation.Find(); err != nil {
		return "", err
	}
	var returned uint32
	status, _, _ := systemBootInformation.Call(90, uintptr(unsafe.Pointer(&boot)), unsafe.Sizeof(boot), uintptr(unsafe.Pointer(&returned)))
	if int32(status) < 0 || returned != uint32(unsafe.Sizeof(boot)) || boot.Identifier == (windows.GUID{}) {
		return "", errors.New("Windows boot information unavailable or changed")
	}
	return boot.Identifier.String(), nil
}

func (nativeWindowsSystem) cpu() (uint32, uint64, uint64, uint64, error) {
	logical := windows.GetActiveProcessorCount(0xffff) // ALL_PROCESSOR_GROUPS
	if err := systemProcessorGroupCount.Find(); err != nil {
		return logical, 0, 0, 0, err
	}
	groups, _, _ := systemProcessorGroupCount.Call()
	if logical == 0 || logical > 64 || groups != 1 {
		return logical, 0, 0, 0, errors.New("CPU group coverage unavailable")
	}
	if err := systemTimes.Find(); err != nil {
		return logical, 0, 0, 0, err
	}
	var idle, kernel, user windows.Filetime
	ok, _, err := systemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return logical, 0, 0, 0, err
	}
	counter := func(value windows.Filetime) uint64 {
		return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
	}
	return logical, counter(kernel), counter(user), counter(idle), nil
}

func (nativeWindowsSystem) memory() (uint64, uint64, error) {
	var memory struct {
		Length, Load                                             uint32
		TotalPhysical, AvailablePhysical                         uint64
		TotalPageFile, AvailablePageFile                         uint64
		TotalVirtual, AvailableVirtual, AvailableExtendedVirtual uint64
	}
	memory.Length = uint32(unsafe.Sizeof(memory))
	if err := systemMemoryStatus.Find(); err != nil {
		return 0, 0, err
	}
	ok, _, err := systemMemoryStatus.Call(uintptr(unsafe.Pointer(&memory)))
	if ok == 0 {
		return 0, 0, err
	}
	return memory.TotalPhysical, memory.AvailablePhysical, nil
}

func (nativeWindowsSystem) disk() (uint64, uint64, error) {
	// Resolve the actual mount point, including Windows installed in a mounted
	// volume. Do not infer the volume from environment variables or drive letters.
	directory, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		return 0, 0, err
	}
	path, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return 0, 0, err
	}
	volume := make([]uint16, 32768) // Maximum Win32 Unicode path including NUL.
	if err := windows.GetVolumePathName(path, &volume[0], uint32(len(volume))); err != nil {
		return 0, 0, err
	}
	var total, available, free uint64
	err = windows.GetDiskFreeSpaceEx(&volume[0], &available, &total, &free)
	return total, available, err
}

func (nativeWindowsSystem) uptime() (float64, error) {
	elapsed := windows.DurationSinceBoot()
	if elapsed < 0 {
		return 0, errors.New("uptime exceeds duration range")
	}
	return float64(elapsed) / float64(time.Second), nil
}

func (nativeWindowsSystem) network(ctx context.Context) (uint64, uint64, error) {
	const maxBytes = 256 * 1024
	const maxAdapters = 256
	const flags = windows.GAA_FLAG_SKIP_UNICAST | windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER | windows.GAA_FLAG_SKIP_FRIENDLY_NAME
	size := uint32(16 * 1024)
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		if size == 0 || size > maxBytes {
			return 0, 0, errors.New("Windows adapter inventory exceeds 256 KiB")
		}
		// uint64 backing storage guarantees native structure alignment.
		buffer := make([]uint64, (size+7)/8)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, first, &size)
		if errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			continue
		}
		if errors.Is(err, windows.ERROR_NO_DATA) {
			return 0, 0, nil
		}
		if err != nil {
			return 0, 0, err
		}
		var received, sent uint64
		seen := make(map[uint64]bool)
		base := uintptr(unsafe.Pointer(first))
		bytes := uintptr(len(buffer) * 8)
		for adapter, count := first, 0; adapter != nil; adapter, count = adapter.Next, count+1 {
			if err := ctx.Err(); err != nil {
				return 0, 0, err
			}
			address := uintptr(unsafe.Pointer(adapter))
			minimum := unsafe.Offsetof(adapter.Luid) + unsafe.Sizeof(adapter.Luid)
			if count >= maxAdapters || address < base || address-base > bytes || minimum > bytes-(address-base) || address%unsafe.Alignof(*adapter) != 0 {
				return 0, 0, errors.New("invalid or oversized Windows adapter list")
			}
			if uintptr(adapter.Length) < minimum {
				return 0, 0, errors.New("Windows adapter record too short")
			}
			if adapter.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK || seen[adapter.Luid] {
				continue
			}
			seen[adapter.Luid] = true
			row := windows.MibIfRow2{InterfaceLuid: adapter.Luid}
			if err := windows.GetIfEntry2Ex(0, &row); err != nil { // MibIfEntryNormal
				return 0, 0, err // A partial sum would corrupt the next rate sample.
			}
			if err := addWindowsNetworkCounters(&received, &sent, row.InOctets, row.OutOctets); err != nil {
				return 0, 0, err
			}
		}
		runtime.KeepAlive(buffer)
		return received, sent, nil
	}
	return 0, 0, errors.New("Windows adapter inventory changed during all three attempts")
}
