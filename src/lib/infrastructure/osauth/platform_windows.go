//go:build windows && (amd64 || arm64)

package osauth

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wtsSessionInfoEx   = 25
	wtsSessionLocked   = 0
	wtsSessionUnlocked = 1
)

var (
	wtsQuerySessionInformation = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSQuerySessionInformationW")
	kernelClock                = windows.NewLazySystemDLL("kernel32.dll")
	getTickCount64             = kernelClock.NewProc("GetTickCount64")
	queryUnbiasedInterruptTime = kernelClock.NewProc("QueryUnbiasedInterruptTime")
)

// The Level 1 WTSINFOEXW data union is eight-byte aligned on amd64 and arm64.
// Only the identity and state fields are needed. The returned byte count is
// checked before the prefix is read.
type windowsSessionInfoPrefix struct {
	Level     uint32
	_         uint32
	SessionID uint32
	State     uint32
	Flags     int32
}

type windowsPlatform struct {
	sleepMutex sync.Mutex
	sleep      suspendTracker
}

func New() (Platform, error) { return &windowsPlatform{}, nil }

func (*windowsPlatform) Authenticate(ctx context.Context, _ int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("OS-authenticated remote unlock is unavailable on Windows")
}

func (*windowsPlatform) SecurityDirectory() (string, error) { return securityDirectory() }

func (platform *windowsPlatform) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{Locked: true}
	generation, err := platform.SuspendGeneration()
	if err == nil {
		snapshot.SleepGeneration, snapshot.SleepKnown = generation, true
	}

	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	snapshot.Locked, snapshot.LockKnown = windowsConsoleLock()
	if !snapshot.LockKnown || !snapshot.SleepKnown {
		snapshot.Detail = "Active Windows console lock or sleep state is unavailable."
	}
	return snapshot, ctx.Err()
}

func windowsConsoleLock() (locked bool, known bool) {
	var session uint32
	if windows.ProcessIdToSessionId(uint32(os.Getpid()), &session) != nil ||
		session == 0 || session == math.MaxUint32 || session != windows.WTSGetActiveConsoleSessionId() {
		return true, false
	}
	if wtsQuerySessionInformation.Find() != nil {
		return true, false
	}

	var information *windowsSessionInfoPrefix
	var size uint32
	result, _, _ := wtsQuerySessionInformation.Call(0, uintptr(session), wtsSessionInfoEx,
		uintptr(unsafe.Pointer(&information)), uintptr(unsafe.Pointer(&size)))
	if information != nil {
		defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(information)))
	}
	if result == 0 || information == nil || uintptr(size) < unsafe.Sizeof(*information) ||
		session != windows.WTSGetActiveConsoleSessionId() {
		return true, false
	}

	return information.consoleLock(session)
}

func (information windowsSessionInfoPrefix) consoleLock(session uint32) (locked bool, known bool) {
	if information.Level != 1 || information.SessionID != session || information.State != windows.WTSActive {
		return true, false
	}
	switch information.Flags {
	case wtsSessionLocked:
		return true, true
	case wtsSessionUnlocked:
		return false, true
	default:
		return true, false
	}
}

func (platform *windowsPlatform) SuspendGeneration() (uint64, error) {
	platform.sleepMutex.Lock()
	defer platform.sleepMutex.Unlock()

	if getTickCount64.Find() != nil || queryUnbiasedInterruptTime.Find() != nil {
		return 0, errors.New("Windows suspend-aware clocks are unavailable")
	}

	// Bracket the working-state sample with elapsed-time reads. If suspension or
	// scheduling spans the reads, leave this observation unknown.
	before, _, _ := getTickCount64.Call()
	var unbiasedTicks uint64
	result, _, _ := queryUnbiasedInterruptTime.Call(uintptr(unsafe.Pointer(&unbiasedTicks)))
	after, _, _ := getTickCount64.Call()
	if result == 0 || after < before || after-before > 50 || uint64(after) > math.MaxInt64/uint64(time.Millisecond) ||
		unbiasedTicks > math.MaxInt64/uint64(100*time.Nanosecond) {
		return 0, errors.New("Windows suspend-aware clock sample is unavailable or inconsistent")
	}

	suspended := time.Duration(after)*time.Millisecond - time.Duration(unbiasedTicks)*100*time.Nanosecond
	if suspended < -50*time.Millisecond {
		return 0, errors.New("Windows suspend-aware clocks returned inconsistent values")
	}
	return platform.sleep.observe(max(suspended, 0)), nil
}
