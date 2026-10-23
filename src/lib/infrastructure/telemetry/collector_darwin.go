//go:build darwin

package telemetry

import (
	"context"
	"errors"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// NativeCollector reads fixed-size BSD process metadata only. It never requests
// the KERN_PROCARGS interfaces containing command arguments or environment.
type NativeCollector struct{ Root string }

func darwinProcessIdentity(info *unix.KinfoProc) (ProcessIdentity, error) {
	start := info.Proc.P_starttime
	if info.Proc.P_pid <= 0 || start.Sec <= 0 || start.Usec < 0 || start.Usec >= 1000000 {
		return ProcessIdentity{}, errors.New("invalid native process identity")
	}
	return ProcessIdentity{
		PID:        int(info.Proc.P_pid),
		StartToken: strconv.FormatInt(start.Sec, 10) + ":" + strconv.FormatInt(int64(start.Usec), 10),
	}, nil
}

func darwinProcessState(state int8) (string, bool) {
	switch state {
	case 1:
		return "I", true
	case 2:
		return "R", true
	case 3:
		return "S", true
	case 4:
		return "T", true
	default:
		// SZOMB (5) and unused slots cannot verify a running execution.
		return "", false
	}
}

func (NativeCollector) Inspect(ctx context.Context, pid int) (Process, error) {
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)>>1) {
		return Process{}, errors.New("invalid process ID")
	}
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return Process{}, err
	}
	identity, err := darwinProcessIdentity(info)
	if err != nil || identity.PID != pid {
		return Process{}, errors.New("native process identity is unavailable")
	}
	state, live := darwinProcessState(info.Proc.P_stat)
	if !live {
		return Process{}, os.ErrNotExist
	}
	name := make([]byte, 0, len(info.Proc.P_comm))
	for _, value := range info.Proc.P_comm {
		if value == 0 {
			break
		}
		name = append(name, byte(value))
	}
	process := Process{
		ProcessIdentity: identity,
		ParentPID:       int(info.Eproc.Ppid),
		Name:            string(name),
		State:           state,
		Machine:         "local",
	}
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	current, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return Process{}, err
	}
	currentIdentity, err := darwinProcessIdentity(current)
	if err != nil {
		return Process{}, err
	}
	_, live = darwinProcessState(current.Proc.P_stat)
	return finishNativeInspection(ctx, process, currentIdentity, live)
}

func boundedDarwinProcessTable(ctx context.Context) ([]unix.KinfoProc, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	// CTL_KERN/KERN_PROC/KERN_PROC_ALL returns fixed-size kinfo_proc records.
	// Supply a fixed buffer rather than the allocating/retrying slice helper:
	// process churn cannot increase memory or retry work beyond this budget.
	mib := [4]int32{1, 14, 0, 0}
	processes := make([]unix.KinfoProc, MaxProcesses*2+1)
	size := uintptr(len(processes)) * unsafe.Sizeof(processes[0])
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&processes[0])), uintptr(unsafe.Pointer(&size)), 0, 0)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if errno == syscall.ENOMEM {
		return nil, true, errors.New("native process inventory exceeds its inspection budget")
	}
	if errno != 0 {
		return nil, false, errno
	}
	recordSize := unsafe.Sizeof(processes[0])
	if size%recordSize != 0 || size/recordSize > uintptr(len(processes)) {
		return nil, false, errors.New("invalid native process inventory")
	}
	count := int(size / recordSize)
	truncated := count > MaxProcesses*2
	if truncated {
		count = MaxProcesses * 2
	}
	return processes[:count], truncated, nil
}

func (collector NativeCollector) Discover(ctx context.Context) ([]Process, bool, error) {
	table, truncated, err := boundedDarwinProcessTable(ctx)
	if err != nil {
		return nil, truncated, err
	}
	processes := make([]Process, 0)
	incomplete := false
	for _, info := range table {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if info.Proc.P_pid <= 0 {
			continue
		}
		if len(processes) == MaxProcesses {
			return processes, true, nil
		}
		process, err := collector.Inspect(ctx, int(info.Proc.P_pid))
		if err == nil {
			processes = append(processes, process)
		} else if !os.IsNotExist(err) && !errors.Is(err, unix.ESRCH) && !errors.Is(err, unix.EIO) {
			incomplete = true
		}
	}
	if incomplete {
		return processes, truncated, errors.New("some native process metadata is inaccessible")
	}
	return processes, truncated, nil
}
