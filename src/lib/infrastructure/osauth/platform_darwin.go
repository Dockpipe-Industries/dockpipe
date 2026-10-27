//go:build darwin

package osauth

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type darwinPlatform struct {
	run        commandRunner
	executable func() (string, error)
	readClocks func() (time.Duration, error)
	mutex      sync.Mutex
	sleep      suspendTracker
}

func New() (Platform, error) {
	return &darwinPlatform{run: runSystemCommand, executable: os.Executable, readClocks: darwinSuspendedTime}, nil
}

func (platform *darwinPlatform) SuspendGeneration() (uint64, error) {
	platform.mutex.Lock()
	defer platform.mutex.Unlock()
	suspended, err := platform.readClocks()
	if err != nil {
		return 0, err
	}
	return platform.sleep.observe(suspended), nil
}

func darwinSuspendedTime() (time.Duration, error) {
	// Apple's mach_continuous_time and mach_absolute_time documentation maps
	// these clocks to their nanosecond equivalents respectively. The first
	// includes sleep; CLOCK_UPTIME_RAW does not. Both avoid wall-clock changes.
	var continuous, awake unix.Timespec
	if unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &continuous) != nil ||
		unix.ClockGettime(unix.CLOCK_UPTIME_RAW, &awake) != nil {
		return 0, errors.New("native macOS suspend-aware clocks are unavailable")
	}
	difference := time.Duration(continuous.Nano() - awake.Nano())
	if difference < -time.Second {
		return 0, errors.New("native macOS clocks returned inconsistent values")
	}
	return difference, nil
}

func (platform *darwinPlatform) SecurityDirectory() (string, error) {
	helper, err := platform.helper()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := platform.verifyHelper(ctx, helper); err != nil {
		return "", err
	}
	raw, err := platform.run(ctx, helper, "--security-directory")
	var response struct {
		Schema string `json:"schema"`
		Home   string `json:"home"`
	}
	if err != nil || len(raw) > 4096 || json.Unmarshal(raw, &response) != nil || response.Schema != "dockpipe.osauth/v1" ||
		!filepath.IsAbs(response.Home) || response.Home == "/" || filepath.Clean(response.Home) != response.Home {
		return "", errors.New("native macOS account home directory is unavailable")
	}
	return filepath.Join(response.Home, "Library", "Application Support", "Dockpipe", "security"), nil
}

func (platform *darwinPlatform) verifyHelper(ctx context.Context, helper string) error {
	if _, err := platform.run(ctx, "/usr/bin/codesign", "--verify", "--strict", "--test-requirement", `identifier "com.dockpipe.system-auth"`, helper); err != nil {
		return errors.New("Dockpipe system authentication helper signature is unavailable or invalid")
	}
	return nil
}

func (platform *darwinPlatform) helper() (string, error) {
	executable, err := platform.executable()
	if err != nil || !filepath.IsAbs(executable) {
		return "", errors.New("installed Dockpipe system authentication helper is unavailable")
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", errors.New("installed Dockpipe CLI location could not be verified")
	}
	directory := filepath.Dir(executable)
	if filepath.Base(directory) == "MacOS" && filepath.Base(filepath.Dir(directory)) == "Contents" {
		directory = filepath.Join(filepath.Dir(directory), "Helpers")
	}
	path := filepath.Join(directory, "dockpipe-system-auth")
	if err := validateDarwinHelper(path); err != nil {
		return "", err
	}
	return path, nil
}

func validateDarwinHelper(path string) error {
	for current := path; current != "/"; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0002 != 0 {
			return errors.New("Dockpipe system authentication helper must be installed without linked or shared-writable paths")
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || (owner.Uid != 0 && owner.Uid != uint32(os.Getuid())) {
			return errors.New("Dockpipe system authentication helper ownership is invalid")
		}
		// /Applications is normally root:admin 0775. Its administrators already
		// control installation. Other shared-writable paths remain inadmissible.
		if info.Mode().Perm()&0020 != 0 && !(info.IsDir() && owner.Uid == 0 && owner.Gid == 80) {
			return errors.New("Dockpipe system authentication helper has a shared-writable installation path")
		}
		if current == path && (!info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0) {
			return errors.New("Dockpipe system authentication helper is not executable")
		}
	}
	return nil
}

func (platform *darwinPlatform) Authenticate(ctx context.Context, peerPID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if peerPID <= 0 || syscall.Kill(peerPID, 0) != nil {
		return errors.New("OS authentication caller is unavailable")
	}
	helper, err := platform.helper()
	if err != nil {
		return err
	}
	// Validate the packaged helper's signature before allowing it to present UI.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := platform.verifyHelper(ctx, helper); err != nil {
		return err
	}
	if _, err := platform.run(ctx, helper, "--authenticate"); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("native OS authentication was cancelled, unavailable or rejected")
	}
	return ctx.Err()
}

func (platform *darwinPlatform) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	helper, err := platform.helper()
	if err != nil {
		return Snapshot{Locked: true, Detail: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := platform.verifyHelper(ctx, helper); err != nil {
		return Snapshot{Locked: true, Detail: "Native macOS security helper signature is unavailable or invalid."}, ctx.Err()
	}
	raw, err := platform.run(ctx, helper, "--snapshot")
	var response struct {
		Schema         string  `json:"schema"`
		IdleSeconds    float64 `json:"idle_seconds"`
		IdleKnown      bool    `json:"idle_known"`
		Locked         bool    `json:"locked"`
		LockKnown      bool    `json:"lock_known"`
		SleepKnown     bool    `json:"sleep_known"`
		SuspendedNanos uint64  `json:"suspended_nanoseconds"`
	}
	if err != nil || len(raw) > 4096 || json.Unmarshal(raw, &response) != nil || response.Schema != "dockpipe.osauth/v1" ||
		math.IsNaN(response.IdleSeconds) || math.IsInf(response.IdleSeconds, 0) || response.IdleSeconds < 0 ||
		response.IdleSeconds > (365*24*time.Hour).Seconds() || response.SuspendedNanos > math.MaxInt64 {
		return Snapshot{Locked: true, Detail: "Native macOS security snapshot is unavailable."}, ctx.Err()
	}
	snapshot := Snapshot{Idle: time.Duration(response.IdleSeconds * float64(time.Second)), IdleKnown: response.IdleKnown,
		Locked: response.Locked, LockKnown: response.LockKnown, SleepKnown: response.SleepKnown}
	if snapshot.SleepKnown {
		platform.mutex.Lock()
		snapshot.SleepGeneration = platform.sleep.observe(time.Duration(response.SuspendedNanos))
		platform.mutex.Unlock()
	}
	var details []string
	if !snapshot.IdleKnown {
		details = append(details, "Native human-input idle detection is unavailable.")
	}
	if !snapshot.LockKnown {
		details = append(details, "This macOS session does not expose a verifiable lock state.")
	}
	if !snapshot.SleepKnown {
		details = append(details, "Suspend-aware OS clocks are unavailable.")
	}
	snapshot.Detail = strings.Join(details, " ")
	return snapshot, nil
}
