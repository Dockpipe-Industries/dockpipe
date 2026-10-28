//go:build linux && (amd64 || arm64)

package osauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func sessionReply(locked bool) []byte {
	return []byte(fmt.Sprintf(`{"type":"a{sv}","data":[{"Active":{"type":"b","data":true},"Remote":{"type":"b","data":false},"CanLock":{"type":"b","data":true},"LockedHint":{"type":"b","data":%t},"Display":{"type":"s","data":":0"}}]}`, locked))
}

func fakePlatform(t *testing.T, locked bool, authenticate func(context.Context, []string) error) *linuxPlatform {
	t.Helper()
	return &linuxPlatform{
		processSession: func(pid, uid int) (string, error) {
			if pid != os.Getpid() || uid != os.Getuid() {
				t.Fatal("OS subject did not use the actual local caller identity")
			}
			return "session", nil
		},
		displaySession: func(int) (string, error) { return "session", nil },
		readClocks:     func() (time.Duration, error) { return 0, nil },
		run: func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
			if program == "/usr/bin/pkcheck" {
				if authenticate == nil {
					t.Fatal("OS authentication should not have been invoked")
				}
				return nil, authenticate(ctx, arguments)
			}
			if program != "/usr/bin/busctl" {
				t.Fatal("security command used a non-system executable")
			}
			switch arguments[len(arguments)-1] {
			case "session":
				return []byte(`{"type":"o","data":["/org/freedesktop/login1/session/session"]}`), nil
			case "org.freedesktop.login1.Session":
				return sessionReply(locked), nil
			case "GetIdletime":
				if arguments[0] != fmt.Sprintf("--address=unix:path=/run/user/%d/bus", os.Getuid()) {
					t.Fatal("idle query trusted a caller-selected session bus")
				}
				return []byte(`{"type":"t","data":[1200]}`), nil
			default:
				t.Fatalf("unexpected OS query: %v", arguments)
				return nil, errors.New("unexpected OS query")
			}
		},
	}
}

func TestAuthenticateRequiresActualOSSubjectAndDedicatedPolicy(t *testing.T) {
	called := false
	platform := fakePlatform(t, false, func(ctx context.Context, arguments []string) error {
		called = true
		identity, err := readProcessIdentity(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{"--action-id", "org.dockpipe.remote.unlock", "--process", identity.subject(), "--allow-user-interaction"}
		if strings.Join(arguments, "|") != strings.Join(expected, "|") {
			t.Fatalf("wrong OS authentication request: %v", arguments)
		}
		return nil
	})
	if err := platform.Authenticate(context.Background(), os.Getpid()); err != nil || !called {
		t.Fatalf("OS authentication did not complete: %v", err)
	}
}

func TestAuthenticateRejectsLockedMissingAndRemoteSessions(t *testing.T) {
	platform := fakePlatform(t, true, nil)
	if err := platform.Authenticate(context.Background(), os.Getpid()); err == nil {
		t.Fatal("locked OS session authenticated")
	}
	platform = fakePlatform(t, false, nil)
	platform.processSession = func(int, int) (string, error) { return "", errors.New("remote or inactive") }
	if err := platform.Authenticate(context.Background(), os.Getpid()); err == nil {
		t.Fatal("remote or inactive OS session authenticated")
	}
	if err := platform.Authenticate(context.Background(), -1); err == nil {
		t.Fatal("invalid caller identity authenticated")
	}
}

func TestAuthenticatePreservesCancellationAndFailsClosedOnMissingPolicy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	platform := fakePlatform(t, false, func(context.Context, []string) error {
		cancel()
		return ctx.Err()
	})
	if err := platform.Authenticate(ctx, os.Getpid()); !errors.Is(err, context.Canceled) {
		t.Fatalf("authentication cancellation was lost: %v", err)
	}
	platform = fakePlatform(t, false, func(context.Context, []string) error { return errors.New("action unavailable") })
	if err := platform.Authenticate(context.Background(), os.Getpid()); err == nil {
		t.Fatal("unavailable native authentication silently succeeded")
	}
}

func TestAuthenticateStillUsesOSWhenLockCapabilityIsUnknown(t *testing.T) {
	called := false
	platform := fakePlatform(t, false, nil)
	platform.run = func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
		if program == "/usr/bin/pkcheck" {
			called = true
			return nil, nil
		}
		return nil, errors.New("lock integration unavailable")
	}
	if err := platform.Authenticate(context.Background(), os.Getpid()); err != nil || !called {
		t.Fatalf("unknown lock capability prevented real OS authentication: %v", err)
	}
}

func TestSnapshotUsesOSInputAndDetectsResume(t *testing.T) {
	platform := fakePlatform(t, false, nil)
	suspended := time.Second
	platform.readClocks = func() (time.Duration, error) { return suspended, nil }
	first, err := platform.Snapshot(context.Background())
	if err != nil || !first.IdleKnown || first.Idle != 1200*time.Millisecond || !first.LockKnown || first.Locked || !first.SleepKnown {
		t.Fatalf("OS snapshot was not preserved: %+v %v", first, err)
	}
	second, err := platform.Snapshot(context.Background())
	if err != nil || second.SleepGeneration != first.SleepGeneration || second.Idle != first.Idle {
		t.Fatal("background snapshot changed activity or sleep generation")
	}
	suspended += 3 * time.Second
	resumed, err := platform.Snapshot(context.Background())
	if err != nil || resumed.SleepGeneration != first.SleepGeneration+1 {
		t.Fatal("resume did not invalidate the prior sleep generation")
	}
}

func TestSuspendAdmissionCheckNeverQueriesDesktopServices(t *testing.T) {
	platform := fakePlatform(t, false, nil)
	suspended := time.Second
	platform.readClocks = func() (time.Duration, error) { return suspended, nil }
	first, err := platform.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	platform.run = func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("final admission started an OS security subprocess")
		return nil, errors.New("unexpected OS command")
	}
	suspended += time.Second
	generation, err := platform.SuspendGeneration()
	if err != nil || generation != first.SleepGeneration+1 {
		t.Fatalf("final admission missed resume: %d %v", generation, err)
	}
	platform.readClocks = func() (time.Duration, error) { return 0, errors.New("clock unavailable") }
	if _, err := platform.SuspendGeneration(); err == nil {
		t.Fatal("unavailable fast OS clock was accepted")
	}
}

func TestSnapshotReportsUnknownCapabilities(t *testing.T) {
	platform := fakePlatform(t, false, nil)
	platform.run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("OS service unavailable")
	}
	snapshot, err := platform.Snapshot(context.Background())
	if err != nil || snapshot.IdleKnown || snapshot.LockKnown || snapshot.Detail == "" || !snapshot.Locked {
		t.Fatalf("missing integrations became unlocked activity: %+v %v", snapshot, err)
	}
}

func TestProcessStartTimeHandlesUnusualCommandNames(t *testing.T) {
	fields := append([]string{"S"}, make([]string, 19)...)
	for index := 1; index < len(fields); index++ {
		fields[index] = "0"
	}
	fields[19] = "1234567"
	stat := []byte("101 (a command ) with ( parentheses)) " + strings.Join(fields, " "))
	start, err := parseProcessStartTime(stat)
	if err != nil || start != 1234567 {
		t.Fatalf("wrong process identity: %d %v", start, err)
	}
	for _, invalid := range []string{"", "101 (short) S 1", "101 (zero) " + strings.ReplaceAll(strings.Join(fields, " "), "1234567", "0")} {
		if _, err := parseProcessStartTime([]byte(invalid)); err == nil {
			t.Fatal("invalid process start time accepted")
		}
	}
}

func TestBusReplyRequiresTypedBoundedValue(t *testing.T) {
	var value uint64
	for _, raw := range []string{`{"type":"s","data":[1]}`, `{"type":"t","data":[1,2]}`, `{"type":"t","data":[-1]}`, `{"type":"t","data":["1"]}`} {
		if err := decodeBusReply([]byte(raw), "t", &value); err == nil {
			t.Fatalf("invalid native reply accepted: %s", raw)
		}
	}
}

func TestNativeSessionRequiresActiveLocalOwner(t *testing.T) {
	library := &sessionLibrary{
		sessionUID: func(name string, owner *uint32) int32 {
			*owner = uint32(os.Getuid())
			return 0
		},
		sessionActive: func(string) int32 { return 1 },
		sessionRemote: func(string) int32 { return 0 },
	}
	if err := verifyNativeSession(library, "session", os.Getuid()); err != nil {
		t.Fatal(err)
	}
	library.sessionRemote = func(string) int32 { return 1 }
	if err := verifyNativeSession(library, "session", os.Getuid()); err == nil {
		t.Fatal("remote OS session was treated as local")
	}
	library.sessionRemote = func(string) int32 { return 0 }
	library.sessionActive = func(string) int32 { return 0 }
	if err := verifyNativeSession(library, "session", os.Getuid()); err == nil {
		t.Fatal("inactive OS session was treated as active")
	}
	library.sessionActive = func(string) int32 { return 1 }
	if err := verifyNativeSession(library, "session", os.Getuid()+1); err == nil {
		t.Fatal("another OS account's session was treated as the operator")
	}
}

func TestDefaultFalseLockHintDoesNotInventIntegration(t *testing.T) {
	run := func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
		switch arguments[len(arguments)-1] {
		case "session":
			return []byte(`{"type":"o","data":["/org/freedesktop/login1/session/session"]}`), nil
		case "org.freedesktop.login1.Session":
			return []byte(strings.ReplaceAll(string(sessionReply(false)), `"CanLock":{"type":"b","data":true},`, "")), nil
		default:
			return nil, errors.New("screen-lock service unavailable")
		}
	}
	if _, err := readDesktopSession(context.Background(), run, "session"); err == nil {
		t.Fatal("default false lock hint bypassed missing screen-lock integration")
	}
}

func TestGnomeShellOwnedScreenLockEndpoint(t *testing.T) {
	for _, locked := range []bool{false, true} {
		run := func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
			for _, argument := range arguments {
				if argument == "org.gnome.Shell" {
					return []byte(fmt.Sprintf(`{"type":"b","data":[%t]}`, locked)), nil
				}
			}
			return nil, errors.New("standalone screen saver service unavailable")
		}
		actual, err := readScreenSaverLock(context.Background(), run, os.Getuid())
		if err != nil || actual != locked {
			t.Fatalf("shell screen-lock state = %t, %v; want %t", actual, err, locked)
		}
	}
}
