package localterminal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

func localAuthority() terminalsession.Authority {
	target, identity := nativeinvocation.Local()
	return terminalsession.Authority{Target: target, Identity: identity}
}

func testProfile() terminalsession.Profile {
	executable, _ := os.Executable()
	dir, _ := os.Getwd()
	return terminalsession.Profile{ID: "owned", Executable: executable, Dir: dir, Args: []string{"argument"}, Env: []string{"TERM=xterm-256color", "PRIVATE_SENTINEL=never-persist"}, Route: "native", EnvironmentMode: "complete"}
}

func startFrame() Command {
	profile := testProfile()
	return Command{Schema: Schema, Type: "start", ID: 1, Profile: &profile, Columns: 80, Rows: 24}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestStreamInvalidDeniedCancelledLaunchHasNoProviderEffect(t *testing.T) {
	for _, mode := range []string{"invalid", "denied", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			authority := localAuthority()
			start := startFrame()
			if mode == "invalid" {
				start.Profile.Executable += "\x00"
			}
			if mode == "denied" {
				authority.Authorize = func(context.Context, invocation.Binding) error { return errors.New("denied") }
			}
			if mode == "cancelled" {
				cancel()
			}
			data, _ := json.Marshal(start)
			var output bytes.Buffer
			called := false
			err := Stream(ctx, io.NopCloser(bytes.NewReader(append(data, '\n'))), nopWriteCloser{&output}, authority,
				func(context.Context, terminalsession.Profile, terminalsession.Size) (terminalsession.Backend, error) {
					called = true
					return nil, errors.New("unexpected")
				})
			if err == nil || called {
				t.Fatalf("err=%v provider called=%v", err, called)
			}
			if strings.Contains(output.String(), "PRIVATE_SENTINEL") {
				t.Fatal("private profile in diagnostics")
			}
		})
	}
}

func TestProviderProfileSnapshotAndAuthorityBinding(t *testing.T) {
	profile := testProfile()
	authority := localAuthority()
	var binding invocation.Binding
	authority.Authorize = func(_ context.Context, value invocation.Binding) error {
		binding = value
		profile.Args[0] = "changed"
		profile.Env[0] = "TERM=changed"
		return nil
	}
	called := false
	manager, err := terminalsession.NewWithProvider(profile, authority, func(_ context.Context, owned terminalsession.Profile, _ terminalsession.Size) (terminalsession.Backend, error) {
		called = true
		if owned.Args[0] != "argument" || owned.Env[0] != "TERM=xterm-256color" {
			t.Fatal("profile aliased caller")
		}
		return nil, errors.New("controlled stop")
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Open(context.Background(), terminalsession.Size{Columns: 80, Rows: 24}, 0)
	if err == nil || !called || binding.Contract.Action != "open" || binding.Digest == "" || binding.Revision != manager.Generation() {
		t.Fatalf("missing owned binding: %+v %v", binding, err)
	}
}

func TestStrictLocalTerminalFrames(t *testing.T) {
	valid := `{"schema":"dockpipe.local-terminal/v1","type":"input","id":2,"data":"YWJj"}`
	if _, err := DecodeCommand([]byte(valid), false); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(valid, `"id":2`, `"id":2,"id":3`, 1),
		strings.Replace(valid, `"id":2`, `"ID":2`, 1),
		strings.Replace(valid, `"id":2`, `"id":9007199254740992`, 1),
		strings.Replace(valid, `"id":2`, `"id":2,"argv":[]`, 1),
		valid + `{}`, strings.Replace(valid, "input", "execute", 1),
		strings.Replace(valid, `"data":"YWJj"`, `"data":""`, 1),
	} {
		if _, err := DecodeCommand([]byte(invalid), false); err == nil {
			t.Fatal("accepted malformed frame")
		}
	}
}

// The test subprocess is a protocol peer, not a PTY substitute. Native PTY
// behavior remains covered by terminalsession and the actual Qt provider tests.
func TestMain(m *testing.M) {
	if mode := os.Getenv("DOCKPIPE_LOCAL_TERMINAL_TEST_PROVIDER"); mode != "" {
		fakeProvider(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeProvider(mode string) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), MaxStartLine)
	encoder := json.NewEncoder(os.Stdout)
	first := true
	for scanner.Scan() {
		command, err := DecodeCommand(scanner.Bytes(), first)
		if err != nil {
			return
		}
		first = false
		event := Event{Schema: Schema, ID: command.ID}
		switch command.Type {
		case "start":
			event.Type = "ready"
		case "input":
			if mode == "missing-ack" {
				continue
			}
			if mode == "burst" {
				for index := 0; index < 24; index++ {
					_ = encoder.Encode(Event{Schema: Schema, Type: "output", Data: bytes.Repeat([]byte{byte(index)}, MaxOutputChunk)})
				}
			}
			event.Type, event.Bytes = "ack", len(command.Data)
			if mode == "partial-ack" {
				event.Bytes--
			}
		case "resize":
			event.Type = "ack"
		case "close":
			if mode == "crash" {
				return
			}
			event.Type, event.ID, event.Cleanup = "closed", 0, "complete"
			_ = encoder.Encode(event)
			return
		}
		_ = encoder.Encode(event)
	}
}

func TestStreamDrainsBoundedBurstBeforeMetadataClosed(t *testing.T) {
	t.Setenv("DOCKPIPE_LOCAL_TERMINAL_TEST_PROVIDER", "burst")
	self, _ := os.Executable()
	t.Setenv("DOCKPIPE_TERMINAL_PROVIDER", self)
	input, commands := io.Pipe()
	events, output := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- Stream(ctx, input, output, localAuthority(), InstalledProvider) }()
	encoder, decoder := json.NewEncoder(commands), json.NewDecoder(events)
	if err := encoder.Encode(startFrame()); err != nil {
		t.Fatal(err)
	}
	var ready Event
	if err := decoder.Decode(&ready); err != nil || ready.Type != "ready" {
		t.Fatal(err)
	}
	if err := encoder.Encode(Command{Schema: Schema, Type: "input", ID: 2, Data: []byte("produce")}); err != nil {
		t.Fatal(err)
	}
	var cursor uint64
	var tail []byte
	for {
		var event Event
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		switch event.Type {
		case "ack":
			if err := encoder.Encode(Command{Schema: Schema, Type: "close", ID: 3}); err != nil {
				t.Fatal(err)
			}
		case "output":
			start := event.Cursor - uint64(len(event.Data))
			if len(event.Data) > MaxOutputChunk || start < cursor || start > cursor && !event.Truncated || event.Oldest > event.Cursor {
				t.Fatal("invalid output continuity")
			}
			cursor = event.Cursor
			tail = append(tail, event.Data...)
			if len(tail) > MaxOutputChunk {
				tail = append([]byte(nil), tail[len(tail)-MaxOutputChunk:]...)
			}
		case "closed":
			if event.Cleanup != "complete" || event.Cursor != 24*MaxOutputChunk || event.Cursor != cursor || len(event.Data) != 0 || !bytes.Equal(tail, bytes.Repeat([]byte{23}, MaxOutputChunk)) {
				t.Fatal("tail lost or final output duplicated")
			}
			if err := <-finished; err != nil {
				t.Fatal(err)
			}
			_ = commands.Close()
			_ = events.Close()
			return
		default:
			t.Fatalf("unexpected event type %q", event.Type)
		}
	}
}

func TestInstalledProviderFullWriteAckAndCleanup(t *testing.T) {
	for _, mode := range []string{"normal", "partial-ack", "missing-ack", "crash"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DOCKPIPE_LOCAL_TERMINAL_TEST_PROVIDER", mode)
			self, _ := os.Executable()
			t.Setenv("DOCKPIPE_TERMINAL_PROVIDER", self)
			backend, err := InstalledProvider(context.Background(), testProfile(), terminalsession.Size{Columns: 80, Rows: 24})
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			_ = backend.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			count, err := backend.Write([]byte("private bytes"))
			if mode == "partial-ack" || mode == "missing-ack" {
				if count != 0 || !errors.Is(err, terminalsession.ErrInputUncertain) {
					t.Fatal("uncertain write acknowledged")
				}
			} else if err != nil || count != len("private bytes") {
				t.Fatalf("write: %d %v", count, err)
			}
			if err := backend.Resize(terminalsession.Size{Columns: 100, Rows: 30}); err != nil {
				t.Fatal(err)
			}
			err = backend.Stop()
			if mode == "crash" {
				if !errors.Is(err, terminalsession.ErrCleanupIncomplete) {
					t.Fatal("crashed provider falsely proved cleanup")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInstalledProviderRejectsRelativeOverrideWithoutFallback(t *testing.T) {
	t.Setenv("DOCKPIPE_TERMINAL_PROVIDER", "./project-helper")
	if _, err := InstalledProvider(context.Background(), testProfile(), terminalsession.Size{Columns: 80, Rows: 24}); err == nil {
		t.Fatal("relative provider accepted")
	}
}

func TestStreamReadyAckThenMetadataOnlyClosed(t *testing.T) {
	t.Setenv("DOCKPIPE_LOCAL_TERMINAL_TEST_PROVIDER", "normal")
	self, _ := os.Executable()
	t.Setenv("DOCKPIPE_TERMINAL_PROVIDER", self)
	input, commands := io.Pipe()
	events, output := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- Stream(ctx, input, output, localAuthority(), InstalledProvider)
		_ = output.Close()
	}()
	encoder, decoder := json.NewEncoder(commands), json.NewDecoder(events)
	if err := encoder.Encode(startFrame()); err != nil {
		t.Fatal(err)
	}
	var event Event
	if err := decoder.Decode(&event); err != nil || event.Type != "ready" || event.SessionID == "" {
		t.Fatalf("ready: %+v %v", event, err)
	}
	if err := encoder.Encode(Command{Schema: Schema, Type: "input", ID: 2, Data: []byte("abc")}); err != nil {
		t.Fatal(err)
	}
	event = Event{}
	if err := decoder.Decode(&event); err != nil || event.Type != "ack" || event.ID != 2 || event.Bytes != 3 {
		t.Fatalf("ack: %+v %v", event, err)
	}
	if err := encoder.Encode(Command{Schema: Schema, Type: "close", ID: 3}); err != nil {
		t.Fatal(err)
	}
	event = Event{}
	if err := decoder.Decode(&event); err != nil || event.Type != "closed" || event.Cleanup != "complete" || len(event.Data) != 0 {
		t.Fatalf("closed: %+v %v", event, err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	_ = commands.Close()
}

func TestProfileBoundsAndExplicitHostEnvironment(t *testing.T) {
	profile := testProfile()
	profile.Env = append(profile.Env, "MULTILINE=line one\nline two", "=C:=C:\\work")
	if err := ValidateProfile(profile); err != nil {
		t.Fatal("valid native multiline/drive environment rejected")
	}
	profile.Env = append(profile.Env, "TERM=duplicate")
	if err := ValidateProfile(profile); err == nil {
		t.Fatal("duplicate environment silently changes frozen snapshot")
	}
	profile = testProfile()
	profile.Route, profile.EnvironmentMode = "host", "inherit-host"
	if err := ValidateProfile(profile); err == nil {
		t.Fatal("host mode missing declared COLORTERM accepted")
	}
	profile.Env = []string{"TERM=xterm", "COLORTERM=truecolor"}
	for index := 0; index < 64; index++ {
		profile.Env = append(profile.Env, "KEY_"+string(rune('a'+index))+"=value")
	}
	if err := ValidateProfile(profile); err != nil {
		t.Fatal("64 user overrides plus terminal env rejected")
	}
	profile.Env = append(profile.Env, "EXCESS=value")
	if err := ValidateProfile(profile); err == nil {
		t.Fatal("host override bound bypassed")
	}
}

func TestProviderCancellationInterruptsBlockedInput(t *testing.T) {
	t.Setenv("DOCKPIPE_LOCAL_TERMINAL_TEST_PROVIDER", "missing-ack")
	self, _ := os.Executable()
	t.Setenv("DOCKPIPE_TERMINAL_PROVIDER", self)
	manager, err := terminalsession.NewWithProvider(testProfile(), localAuthority(), InstalledProvider)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := manager.Open(ctx, terminalsession.Size{Columns: 80, Rows: 24}, 0)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	inputCtx, stopInput := context.WithCancel(context.Background())
	go func() { finished <- session.Input(inputCtx, 1, []byte("uncertain private input")) }()
	time.Sleep(20 * time.Millisecond)
	stopInput()
	select {
	case err := <-finished:
		if !errors.Is(err, terminalsession.ErrInputUncertain) {
			t.Fatalf("cancelled in-flight input: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("input cancellation did not interrupt provider ACK wait")
	}
	wait, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	result, err := session.Wait(wait)
	if err != nil || result.CleanupError != nil {
		t.Fatalf("cleanup: %+v %v", result, err)
	}
}

func TestStreamBlockedConsumerCancelsOwnedProvider(t *testing.T) {
	t.Setenv("DOCKPIPE_LOCAL_TERMINAL_TEST_PROVIDER", "normal")
	self, _ := os.Executable()
	t.Setenv("DOCKPIPE_TERMINAL_PROVIDER", self)
	input, commands := io.Pipe()
	events, output := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	var backend *providerBackend
	factory := func(ctx context.Context, profile terminalsession.Profile, size terminalsession.Size) (terminalsession.Backend, error) {
		started, err := InstalledProvider(ctx, profile, size)
		if err == nil {
			backend = started.(*providerBackend)
		}
		return started, err
	}
	go func() { finished <- Stream(ctx, input, output, localAuthority(), factory) }()
	encoder, decoder := json.NewEncoder(commands), json.NewDecoder(events)
	if err := encoder.Encode(startFrame()); err != nil {
		t.Fatal(err)
	}
	var ready Event
	if err := decoder.Decode(&ready); err != nil || ready.Type != "ready" {
		t.Fatal(err)
	}
	if err := encoder.Encode(Command{Schema: Schema, Type: "input", ID: 2, Data: []byte("private")}); err != nil {
		t.Fatal(err)
	}
	// Stop consuming before the ACK. Cancellation must interrupt stdout and
	// still stop the independently owned native provider, not block in Encode.
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("blocked consumer appeared successful")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumer backpressure prevented owned cancellation")
	}
	select {
	case <-backend.processDone:
	case <-time.After(time.Second):
		t.Fatal("provider process remained after owned cancellation")
	}
	_ = commands.Close()
	_ = events.Close()
}

func TestLocalProviderCannotReplaceRemotePTY(t *testing.T) {
	authority := localAuthority()
	authority.Target.Kind = invocation.Remote
	if _, err := terminalsession.NewWithProvider(testProfile(), authority, InstalledProvider); err == nil {
		t.Fatal("remote authority selected local provider")
	}
	profile := testProfile()
	profile.Route, profile.EnvironmentMode = "host", "inherit-host"
	if _, err := terminalsession.New(profile, localAuthority()); err == nil {
		t.Fatal("built-in PTY interpreted an inherited host environment")
	}
}

func BenchmarkLocalInputFrame(b *testing.B) {
	data, _ := json.Marshal(Command{Schema: Schema, Type: "input", ID: 2, Data: []byte("a")})
	b.Run("decode-only-baseline", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var command Command
			if err := json.Unmarshal(data, &command); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("strict-managed-protocol", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := DecodeCommand(data, false); err != nil {
				b.Fatal(err)
			}
		}
	})
}
