//go:build darwin

package osauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func darwinFixture(t *testing.T, run commandRunner) (*darwinPlatform, string) {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dockpipe", "dockpipe-system-auth"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("test fixture; never executed"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return &darwinPlatform{run: run, executable: func() (string, error) { return filepath.Join(directory, "dockpipe"), nil }}, directory
}

func TestDarwinUsesSignedFixedSiblingAndNativeAccountHome(t *testing.T) {
	var programs []string
	platform, directory := darwinFixture(t, func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
		programs = append(programs, program)
		if program == "/usr/bin/codesign" {
			if !strings.Contains(strings.Join(arguments, " "), `identifier "com.dockpipe.system-auth"`) {
				t.Fatal("native helper signature did not require its installed identifier")
			}
			return nil, nil
		}
		if len(arguments) != 1 || arguments[0] != "--security-directory" {
			t.Fatal("unexpected native account request")
		}
		return []byte(`{"schema":"dockpipe.osauth/v1","home":"/Users/ExampleUser"}`), nil
	})
	t.Setenv("HOME", "/tmp/project-home")
	t.Setenv("PATH", "/tmp/project-bin")
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", "/tmp/project-security")
	path, err := platform.SecurityDirectory()
	if err != nil || path != "/Users/ExampleUser/Library/Application Support/Dockpipe/security" {
		t.Fatalf("native account home was not used: %q %v", path, err)
	}
	if len(programs) != 2 || programs[1] != filepath.Join(directory, "dockpipe-system-auth") {
		t.Fatal("native helper was discovered outside the fixed installed sibling")
	}
}

func TestDarwinRejectsLinkedOrSharedWritableNativeHelper(t *testing.T) {
	platform, directory := darwinFixture(t, nil)
	helper := filepath.Join(directory, "dockpipe-system-auth")
	if err := os.Chmod(helper, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.helper(); err == nil {
		t.Fatal("shared-writable native authentication helper accepted")
	}
	if err := os.Remove(helper); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(directory, "dockpipe"), helper); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.helper(); err == nil {
		t.Fatal("linked native authentication helper accepted")
	}
}

func TestDarwinMissingSignatureNeverInvokesAuthentication(t *testing.T) {
	platform, _ := darwinFixture(t, func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
		if program != "/usr/bin/codesign" {
			t.Fatal("invalid native helper was executed")
		}
		return nil, errors.New("invalid signature")
	})
	if err := platform.Authenticate(context.Background(), os.Getpid()); err == nil {
		t.Fatal("unverified native helper authenticated")
	}
}

func TestDarwinReportsUnknownLockStateAndNativeResume(t *testing.T) {
	suspended := "1000000000"
	platform, _ := darwinFixture(t, func(ctx context.Context, program string, arguments ...string) ([]byte, error) {
		if program == "/usr/bin/codesign" {
			return nil, nil
		}
		return []byte(`{"schema":"dockpipe.osauth/v1","idle_seconds":1.2,"idle_known":true,"locked":true,"lock_known":false,"sleep_known":true,"suspended_nanoseconds":` + suspended + `}`), nil
	})
	first, err := platform.Snapshot(context.Background())
	if err != nil || first.LockKnown || first.Detail == "" || !first.IdleKnown || !first.SleepKnown {
		t.Fatalf("unknown lock capability was hidden: %+v %v", first, err)
	}
	suspended = "3000000000"
	second, err := platform.Snapshot(context.Background())
	if err != nil || second.SleepGeneration != first.SleepGeneration+1 {
		t.Fatal("native suspend clock did not invalidate the generation")
	}
}

func TestDarwinSuspendAdmissionUsesOnlyNativeClock(t *testing.T) {
	platform, _ := darwinFixture(t, func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("final admission invoked a helper subprocess")
		return nil, errors.New("unexpected process")
	})
	suspended := time.Second
	platform.readClocks = func() (time.Duration, error) { return suspended, nil }
	first, err := platform.SuspendGeneration()
	if err != nil {
		t.Fatal(err)
	}
	suspended += time.Second
	second, err := platform.SuspendGeneration()
	if err != nil || second != first+1 {
		t.Fatalf("native admission missed resume: %d %v", second, err)
	}
}
