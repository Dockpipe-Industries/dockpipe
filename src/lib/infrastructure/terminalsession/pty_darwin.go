//go:build darwin

package terminalsession

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Darwin's grantpt/unlockpt/ptsname are these native terminal ioctls. The name
// buffer is the 128-byte ABI of TIOCPTYGNAME, not caller-provided storage.
func openPTY() (*os.File, *os.File, error) {
	descriptor, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	master := os.NewFile(uintptr(descriptor), "terminal-master")
	var name [128]byte
	for _, request := range []uintptr{unix.TIOCPTYGRANT, unix.TIOCPTYUNLK, unix.TIOCPTYGNAME} {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(descriptor), request, uintptr(unsafe.Pointer(&name[0])))
		if errno != 0 {
			master.Close()
			return nil, nil, errno
		}
	}
	slave, err := os.OpenFile(unix.ByteSliceToString(name[:]), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	return master, slave, nil
}

func inspectProcess(pid int) (processIdentity, error) {
	var info unix.KinfoProc
	record := unsafe.Slice((*byte)(unsafe.Pointer(&info)), unix.SizeofKinfoProc)
	err := readFixedProcessRecord(record, func(buffer []byte) (uintptr, error) {
		// CTL_KERN/KERN_PROC/KERN_PROC_PID from Darwin sys/sysctl.h. Unlike
		// SysctlKinfoProc, retain the successful empty result for an exited PID.
		mib := [4]int32{unix.CTL_KERN, 14, 1, int32(pid)}
		length := uintptr(len(buffer))
		_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
			uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&length)), 0, 0)
		if errno != 0 {
			return length, errno
		}
		return length, nil
	})
	if err != nil {
		return processIdentity{}, fmt.Errorf("Darwin kern.proc.pid pid=%d: %w", pid, err)
	}
	if info.Proc.P_pid == 0 {
		return processIdentity{}, os.ErrNotExist
	}
	session, err := unix.Getsid(pid)
	if err != nil {
		return processIdentity{}, fmt.Errorf("Darwin getsid pid=%d after kernel identity: %w", pid, err)
	}
	return processIdentity{pid: pid, group: int(info.Eproc.Pgid), session: session,
		birth:  strconv.FormatInt(info.Proc.P_starttime.Sec, 10) + ":" + strconv.FormatInt(int64(info.Proc.P_starttime.Usec), 10),
		zombie: info.Proc.P_stat == 5}, nil
}

func sessionMembers(session int) ([]processIdentity, error) {
	// CTL_KERN/KERN_PROC/KERN_PROC_ALL from Darwin sys/sysctl.h. A fixed buffer
	// prevents an unbounded allocation/retry loop as the process table grows.
	mib := [3]int32{unix.CTL_KERN, 14, 0}
	processes := make([]unix.KinfoProc, maxHostProcesses)
	length := uintptr(len(processes) * unix.SizeofKinfoProc)
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&processes[0])), uintptr(unsafe.Pointer(&length)), 0, 0)
	if errno != 0 {
		return nil, fmt.Errorf("%w: Darwin process enumeration session=%d: %w", ErrCleanupIncomplete, session, errno)
	}
	if length > uintptr(len(processes)*unix.SizeofKinfoProc) || length%unix.SizeofKinfoProc != 0 {
		return nil, fmt.Errorf("%w: Darwin process enumeration session=%d length=%d capacity=%d record_size=%d", ErrCleanupIncomplete, session, length, len(processes), unix.SizeofKinfoProc)
	}
	deadline := time.Now().Add(50 * time.Millisecond)
	members := make([]processIdentity, 0)
	for _, info := range processes[:length/unix.SizeofKinfoProc] {
		if len(members) >= maxSessionMembers || time.Now().After(deadline) {
			return members, fmt.Errorf("%w: Darwin session scan bound session=%d members=%d deadline_reached=%t", ErrCleanupIncomplete, session, len(members), time.Now().After(deadline))
		}
		pid := int(info.Proc.P_pid)
		identity, member, err := inspectEnumeratedSessionMember(pid, session, unix.Getsid, inspectProcess)
		if err != nil {
			return members, err
		}
		if member {
			members = append(members, identity)
		}
	}
	return members, nil
}
