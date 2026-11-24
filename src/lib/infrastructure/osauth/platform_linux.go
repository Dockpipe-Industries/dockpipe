//go:build linux && (amd64 || arm64)

package osauth

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const UnlockAction = "org.dockpipe.remote.unlock"

type linuxPlatform struct {
	run               commandRunner
	processSession    func(int, int) (string, error)
	displaySession    func(int) (string, error)
	readClocks        func() (time.Duration, error)
	readSuspendCount  func() (uint64, bool, error)
	sleepMutex        sync.Mutex
	sleep             suspendTracker
	suspendCount      uint64
	suspendCountKnown bool
}

func New() (Platform, error) {
	return &linuxPlatform{run: runSystemCommand, processSession: nativeProcessSession,
		displaySession: nativeDisplaySession, readClocks: linuxSuspendedTime,
		readSuspendCount: linuxSuccessfulSuspends}, nil
}

func (*linuxPlatform) SecurityDirectory() (string, error) { return securityDirectory() }

func (platform *linuxPlatform) SuspendGeneration() (uint64, error) {
	platform.sleepMutex.Lock()
	defer platform.sleepMutex.Unlock()
	suspended, err := platform.readClocks()
	if err != nil {
		return 0, err
	}

	count, known, err := platform.readSuspendCount()
	if err != nil {
		return 0, err
	}
	if platform.suspendCountKnown && !known {
		return 0, errors.New("Linux kernel suspend statistics became unavailable")
	}

	// Virtualized suspend-to-idle can leave the clock difference unchanged.
	// Read the kernel counter at final admission as well as on each snapshot.
	resumed := known && (!platform.suspendCountKnown || count != platform.suspendCount)
	if known {
		platform.suspendCount = count
		platform.suspendCountKnown = true
	}

	return platform.sleep.observeWithResumeEvent(suspended, resumed), nil
}

func linuxSuccessfulSuspends() (uint64, bool, error) {
	file, err := os.Open("/sys/power/suspend_stats/success")
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, 32))
	if err != nil {
		return 0, false, err
	}
	if len(raw) == 32 {
		return 0, false, errors.New("Linux kernel suspend statistics exceeded their bound")
	}

	count, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	return count, err == nil, err
}

func (platform *linuxPlatform) Authenticate(ctx context.Context, peerPID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	identity, err := readProcessIdentity(peerPID)
	if err != nil {
		return err
	}
	session, err := platform.processSession(identity.pid, identity.uid)
	if err != nil {
		return err
	}
	desktop, err := readDesktopSession(ctx, platform.run, session)
	if err == nil && desktop.locked {
		return errors.New("unlock the active local OS session before unlocking remote access")
	}
	// Native account/session checks remain mandatory. An unknown screen-lock
	// adapter is a capability decision for the authority's policy, not a reason
	// to skip real OS authentication (including a deliberate policy change).
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// A real registered OS agent owns the password/biometric UI. This authority
	// does not receive passwords or accept a caller's claimed authentication.
	_, err = platform.run(ctx, "/usr/bin/pkcheck", "--action-id", UnlockAction,
		"--process", identity.subject(), "--allow-user-interaction")
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var exit systemCommandExitError
		if errors.As(err, &exit) {
			// pkcheck documents distinct results for denial, no agent, and dismissal.
			switch exit.code {
			case 1:
				return errors.New("OS authentication was denied")
			case 2:
				return errors.New("OS authentication requires an active local authentication agent")
			case 3:
				return errors.New("OS authentication was cancelled")
			}
		}

		return errors.New("OS authentication did not complete; install the Dockpipe polkit action and use an active local authentication agent")
	}
	current, err := readProcessIdentity(peerPID)
	if err != nil || current != identity {
		return errors.New("OS authentication caller changed while authenticating")
	}
	currentSession, err := platform.processSession(identity.pid, identity.uid)
	if err != nil || currentSession != session {
		return errors.New("OS authentication login session changed")
	}
	desktop, err = readDesktopSession(ctx, platform.run, session)
	if err == nil && desktop.locked {
		return errors.New("OS session locked while authenticating")
	}
	return ctx.Err()
}

func (platform *linuxPlatform) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Locked: true}
	var details []string
	generation, err := platform.SuspendGeneration()
	if err == nil {
		snapshot.SleepGeneration = generation
		snapshot.SleepKnown = true
	} else {
		details = append(details, "Suspend-aware OS clock is unavailable.")
	}
	session, err := platform.displaySession(os.Getuid())
	if err != nil {
		details = append(details, "Active local owner desktop and lock state are unavailable.")
		snapshot.Detail = strings.Join(details, " ")
		return snapshot, ctx.Err()
	}
	desktop, err := readDesktopSession(ctx, platform.run, session)
	if err == nil {
		snapshot.Locked, snapshot.LockKnown = desktop.locked, true
	} else {
		details = append(details, "OS session lock integration is unavailable.")
	}
	idle, idleErr := readGnomeIdle(ctx, platform.run, os.Getuid())
	if idleErr == nil {
		snapshot.Idle, snapshot.IdleKnown = idle, true
	} else {
		details = append(details, "Trusted desktop input idle detection is unavailable; only the bounded absolute session lifetime can apply.")
	}
	snapshot.Detail = strings.Join(details, " ")
	return snapshot, ctx.Err()
}

func linuxSuspendedTime() (time.Duration, error) {
	var boot, monotonic unix.Timespec
	if unix.ClockGettime(unix.CLOCK_BOOTTIME, &boot) != nil || unix.ClockGettime(unix.CLOCK_MONOTONIC, &monotonic) != nil {
		return 0, errors.New("suspend-aware OS clocks are unavailable")
	}
	difference := time.Duration(boot.Nano() - monotonic.Nano())
	if difference < -time.Second {
		return 0, errors.New("OS clocks returned inconsistent values")
	}
	return difference, nil
}
