//go:build linux || darwin

package terminalsession

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

func testManager(t *testing.T, args ...string) *Manager {
	t.Helper()
	manager, err := New(Profile{ID: "owner-shell", Executable: "/bin/sh", Args: args, Env: []string{"PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1="}, Dir: t.TempDir()}, localAuthority())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return manager
}

func openTestSession(t *testing.T, manager *Manager) *Session {
	t.Helper()
	session, err := manager.Open(context.Background(), Size{Columns: 80, Rows: 24}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func waitOutput(t *testing.T, session *Session, expected string) Output {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		output, err := session.Read(context.Background(), 0, MaxOutput)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(output.Data, []byte(expected)) {
			return output
		}
		time.Sleep(5 * time.Millisecond)
	}
	output, _ := session.Read(context.Background(), 0, MaxOutput)
	t.Fatalf("missing %q in PTY output: %q", expected, output.Data)
	return Output{}
}

func waitSession(t *testing.T, session *Session) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := session.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.CleanupError != nil {
		t.Fatal(result.CleanupError)
	}
	return result
}

func TestRealPTYInputResizeAndExit(t *testing.T) {
	manager := testManager(t, "-i")
	session := openTestSession(t, manager)
	if err := session.Input(context.Background(), 1, []byte("stty -echo; test -t 0 && test -t 1 && printf '__%s__\\n' REAL_PTY\n")); err != nil {
		t.Fatal(err)
	}
	waitOutput(t, session, "__REAL_PTY__\r\n")
	if err := session.Resize(context.Background(), Size{Columns: 113, Rows: 37}); err != nil {
		t.Fatal(err)
	}
	if err := session.Input(context.Background(), 2, []byte("stty size; printf '__%s__\\n' SECOND; exit 17\n")); err != nil {
		t.Fatal(err)
	}
	waitOutput(t, session, "37 113\r\n")
	result := waitSession(t, session)
	if result.ExitCode != 17 {
		t.Fatalf("exit code: %+v", result)
	}
	output, _ := session.Read(context.Background(), 0, MaxOutput)
	if !output.Closed || !bytes.Contains(output.Data, []byte("__SECOND__\r\n")) {
		t.Fatal("final output not retained")
	}
	if err := manager.Release(context.Background(), session.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Read(context.Background(), 0, 1); !errors.Is(err, ErrClosed) {
		t.Fatal("released output remained readable")
	}
}

func TestPTYDenialCancellationAndMalformedLaunchBeforeEffects(t *testing.T) {
	directory := t.TempDir()
	marker := filepath.Join(directory, "started")
	denied := errors.New("operator denied")
	authority := localAuthority()
	authority.Authorize = func(context.Context, invocation.Binding) error { return denied }
	profile := Profile{ID: "shell", Executable: "/bin/sh", Args: []string{"-c", "touch \"$1\"", "sh", marker}, Env: []string{"PATH=/usr/bin:/bin"}, Dir: directory}
	manager, err := New(profile, authority)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Open(context.Background(), Size{80, 24}, 0); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Open(ctx, Size{80, 24}, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("denied launch had effects")
	}
	profile.Args = []string{"bad\x00argument"}
	if _, err := New(profile, localAuthority()); !errors.Is(err, ErrInvalid) {
		t.Fatal("NUL accepted")
	}
	if _, err := manager.Open(context.Background(), Size{1001, 24}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid dimensions accepted")
	}
	if len(manager.sessions) != 0 {
		t.Fatal("failed launch retained session")
	}
}

func TestPTYSessionCapacityExplicitRelease(t *testing.T) {
	manager := testManager(t, "-c", "printf finished")
	var sessions []*Session
	for index := 0; index < MaxSessions; index++ {
		session := openTestSession(t, manager)
		waitSession(t, session)
		sessions = append(sessions, session)
	}
	if _, err := manager.Open(context.Background(), Size{80, 24}, 0); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	if err := manager.Release(context.Background(), sessions[0].ID()); err != nil {
		t.Fatal(err)
	}
	waitSession(t, openTestSession(t, manager))
}

func TestPTYContextStopsForegroundAndBackgroundGroups(t *testing.T) {
	manager := testManager(t, "-i")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := manager.Open(ctx, Size{80, 24}, 0)
	if err != nil {
		t.Fatal(err)
	}
	command := "stty -echo; sh -c 'trap \"\" HUP TERM; printf \"__BACKGROUND__%s\\n\" $$; exec sleep 30' &\n"
	if err := session.Input(context.Background(), 1, []byte(command)); err != nil {
		t.Fatal(err)
	}
	waitOutput(t, session, "\r\n__BACKGROUND__")
	if err := session.Input(context.Background(), 2, []byte("sh -c 'printf \"__FOREGROUND__%s\\n\" $$; exec sleep 30'\n")); err != nil {
		t.Fatal(err)
	}
	output := waitOutput(t, session, "__FOREGROUND__")
	identities := make([]processIdentity, 0, 2)
	for _, label := range []string{"__BACKGROUND__", "__FOREGROUND__"} {
		start := bytes.LastIndex(output.Data, []byte(label)) + len(label)
		line := strings.TrimSpace(strings.SplitN(string(output.Data[start:]), "\n", 2)[0])
		pid, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("bad child identity %q", line)
		}
		identity, err := inspectProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, identity)
	}
	cancel()
	result := waitSession(t, session)
	if !errors.Is(result.Cause, context.Canceled) {
		t.Fatal(result.Cause)
	}
	for _, original := range identities {
		current, err := inspectProcess(original.pid)
		if err == nil && current.birth == original.birth && !current.zombie {
			t.Fatalf("job %d survived cancellation", original.pid)
		}
	}
}

func TestPTYOutputStaysBounded(t *testing.T) {
	manager := testManager(t, "-c", "head -c 600000 /dev/zero")
	session := openTestSession(t, manager)
	waitSession(t, session)
	output, err := session.Read(context.Background(), 0, MaxOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !output.Truncated || len(output.Data) != MaxOutput || output.Cursor != 600000 {
		t.Fatalf("unexpected output bounds/cursor: len=%d cursor=%d truncated=%t", len(output.Data), output.Cursor, output.Truncated)
	}
}

func TestPTYBlockingInputCancellationClosesSession(t *testing.T) {
	manager := testManager(t, "-c", "stty raw -echo; printf READY; sleep 30")
	session := openTestSession(t, manager)
	waitOutput(t, session, "READY")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := session.Input(ctx, 1, bytes.Repeat([]byte("x"), MaxInput)); !errors.Is(err, ErrInputUncertain) {
		t.Fatalf("blocked input result: %v", err)
	}
	result := waitSession(t, session)
	if !errors.Is(result.Cause, ErrInputUncertain) || session.lastSequence != 0 {
		t.Fatalf("uncertain input was acknowledged or session survived: %+v", result)
	}
}

func TestPTYInterruptUsesForegroundTerminalGroup(t *testing.T) {
	manager := testManager(t, "-i")
	session := openTestSession(t, manager)
	if err := session.Input(context.Background(), 1, []byte("stty -echo; sh -c 'printf __JOB_READY__; exec sleep 30'\n")); err != nil {
		t.Fatal(err)
	}
	waitOutput(t, session, "__JOB_READY__")
	if err := session.Input(context.Background(), 2, []byte{3}); err != nil {
		t.Fatal(err)
	}
	if err := session.Input(context.Background(), 3, []byte("printf '__%s__\\n' AFTER_INTERRUPT\n")); err != nil {
		t.Fatal(err)
	}
	waitOutput(t, session, "__AFTER_INTERRUPT__\r\n")
}

func TestPTYLeaseExpiryStopsNativeShell(t *testing.T) {
	manager := testManager(t, "-c", "sleep 30")
	session, err := manager.Open(context.Background(), Size{80, 24}, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	result := waitSession(t, session)
	if !errors.Is(result.Cause, ErrLeaseExpired) {
		t.Fatal(result.Cause)
	}
}

func TestPTYProfileIsOwnedBeforeAuthorization(t *testing.T) {
	args := []string{"-c", "printf '%s' \"$OWNED\""}
	environment := []string{"OWNED=accepted"}
	manager, err := New(Profile{ID: "owner", Executable: "/bin/sh", Args: args, Env: environment, Dir: t.TempDir()}, localAuthority())
	if err != nil {
		t.Fatal(err)
	}
	args[1] = "printf wrong"
	environment[0] = "OWNED=wrong"
	session := openTestSession(t, manager)
	defer manager.Shutdown(context.Background())
	waitSession(t, session)
	output, err := session.Read(context.Background(), 0, MaxOutput)
	if err != nil || string(output.Data) != "accepted" {
		t.Fatalf("caller changed owned profile: %q, %v", output.Data, err)
	}
}

func TestPTYOpenAsRetainsEachVerifiedOwner(t *testing.T) {
	manager := testManager(t, "-i")
	for _, owner := range []string{"first-operator", "second-operator"} {
		identity := invocation.Identity{Issuer: "verified-transport", Subject: owner}
		authorize := func(_ context.Context, binding invocation.Binding) error {
			if binding.Identity != identity {
				t.Error("session authority crossed operator boundary")
			}
			return nil
		}
		session, err := manager.OpenAs(context.Background(), Size{80, 24}, 0, identity, authorize)
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Input(context.Background(), 1, []byte("exit\n")); err != nil {
			t.Fatal(err)
		}
		waitSession(t, session)
	}
	if len(manager.sessions) != 2 {
		t.Fatal("per-session authority escaped manager quota")
	}
}
