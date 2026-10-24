//go:build windows

package telemetry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

// NativeCollector reads native process metadata without arguments or environment.
type NativeCollector struct{ Root string }

func windowsCreationTicks(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}

func (NativeCollector) Inspect(ctx context.Context, pid int) (Process, error) {
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) {
		return Process{}, errors.New("invalid process ID")
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return Process{}, err
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		return Process{}, os.ErrNotExist
	}
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return Process{}, err
	}
	var basic windows.PROCESS_BASIC_INFORMATION
	var returned uint32
	if err := windows.NtQueryInformationProcess(handle, windows.ProcessBasicInformation,
		unsafe.Pointer(&basic), uint32(unsafe.Sizeof(basic)), &returned); err != nil {
		return Process{}, err
	}
	if int(basic.UniqueProcessId) != pid {
		return Process{}, errors.New("process identity mismatch")
	}
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	// The native API accepts a 32,768 UTF-16-unit image-path buffer. Only its
	// basename enters telemetry; directories, arguments and environment do not.
	path := make([]uint16, 32768)
	length := uint32(len(path))
	if err := windows.QueryFullProcessImageName(handle, 0, &path[0], &length); err != nil {
		return Process{}, err
	}
	if length == 0 || length > uint32(len(path)) || windowsCreationTicks(creation) == 0 {
		return Process{}, errors.New("invalid native process metadata")
	}
	seconds := float64(windowsCreationTicks(kernel))/10000000 + float64(windowsCreationTicks(user))/10000000
	process := Process{
		ProcessIdentity: ProcessIdentity{
			PID:        pid,
			StartToken: strconv.FormatUint(windowsCreationTicks(creation), 10),
		},
		ParentPID:  int(basic.InheritedFromUniqueProcessId),
		Name:       filepath.Base(windows.UTF16ToString(path[:length])),
		State:      "active",
		Machine:    "local",
		CPUSeconds: &seconds,
	}
	// Holding the process handle pins the kernel object, even after PID reuse.
	// Recheck liveness and creation time before publishing the complete sample.
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return Process{}, err
	}
	state, err = windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return Process{}, err
	}
	current := ProcessIdentity{PID: pid, StartToken: strconv.FormatUint(windowsCreationTicks(creation), 10)}
	return finishNativeInspection(ctx, process, current, state == uint32(windows.WAIT_TIMEOUT))
}

func (collector NativeCollector) Discover(ctx context.Context) ([]Process, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	// Match the aggregate inspection budget used by the Linux collector. One
	// extra PID distinguishes a full budget from an exactly fitting inventory.
	pids := make([]uint32, MaxProcesses*2+1)
	var bytes uint32
	if err := windows.EnumProcesses(pids, &bytes); err != nil {
		return nil, false, err
	}
	count := int(bytes / 4)
	if bytes%4 != 0 || count > len(pids) {
		return nil, false, errors.New("invalid native process inventory")
	}
	truncated := count > MaxProcesses*2
	if truncated {
		count = MaxProcesses * 2
	}
	processes := make([]Process, 0)
	incomplete := false
	for _, pid := range pids[:count] {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if pid == 0 {
			continue
		}
		if len(processes) == MaxProcesses {
			return processes, true, nil
		}
		process, err := collector.Inspect(ctx, int(pid))
		if err == nil {
			processes = append(processes, process)
		} else if !os.IsNotExist(err) && !errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			incomplete = true
		}
	}
	if incomplete {
		return processes, truncated, errors.New("some native process metadata is inaccessible")
	}
	return processes, truncated, nil
}
