//go:build linux && (amd64 || arm64)

package osauth

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const UnlockAction = "org.dockpipe.remote.unlock"

type linuxPlatform struct {
	run            commandRunner
	processSession func(int, int) (string, error)
	displaySession func(int) (string, error)
	readClocks     func() (time.Duration, error)
	sleepMutex     sync.Mutex
	sleep          suspendTracker
}

func New() (Platform, error) {
	return &linuxPlatform{run: runSystemCommand, processSession: nativeProcessSession,
		displaySession: nativeDisplaySession, readClocks: linuxSuspendedTime}, nil
}

func (*linuxPlatform) SecurityDirectory() (string, error) { return securityDirectory() }

func (platform *linuxPlatform) SuspendGeneration() (uint64, error) {
	platform.sleepMutex.Lock()
	defer platform.sleepMutex.Unlock()
	suspended, err := platform.readClocks()
	if err != nil {
		return 0, err
	}
	return platform.sleep.observe(suspended), nil
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
