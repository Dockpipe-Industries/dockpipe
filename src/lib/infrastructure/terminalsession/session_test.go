package terminalsession

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

func localAuthority() Authority {
	target, identity := nativeinvocation.Local()
	return Authority{Target: target, Identity: identity}
}

func TestOutputRingCursorAndTruncation(t *testing.T) {
	var ring outputRing
	input := make([]byte, MaxOutput*3+79)
	for index := range input {
		input[index] = byte(index % 251)
	}
	ring.append(input[:MaxOutput-7])
	ring.append(input[MaxOutput-7:])
	output, err := ring.read(0, MaxOutput)
	if err != nil || !output.Truncated || output.Oldest != uint64(len(input)-MaxOutput) || output.Cursor != uint64(len(input)) {
		t.Fatalf("bad cursor/truncation: %+v, %v", output, err)
	}
	if !bytes.Equal(output.Data, input[len(input)-MaxOutput:]) {
		t.Fatal("ring did not retain exact suffix")
	}
	first, _ := ring.read(output.Oldest, 31)
	second, _ := ring.read(first.Cursor, 19)
	if !bytes.Equal(append(first.Data, second.Data...), output.Data[:50]) {
		t.Fatal("partial cursor reads lost bytes")
	}
	output.Data[0] ^= 0xff
	again, _ := ring.read(output.Oldest, 1)
	if again.Data[0] != input[len(input)-MaxOutput] {
		t.Fatal("returned output aliases ring")
	}
	if _, err := ring.read(ring.end+1, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal("future cursor accepted")
	}
}

type fakeTerminal struct {
	mutex  sync.Mutex
	writes [][]byte
	write  func([]byte) (int, error)
}

func (terminal *fakeTerminal) Read([]byte) (int, error) {
	return 0, io.EOF
}
func (terminal *fakeTerminal) Write(data []byte) (int, error) {
	terminal.mutex.Lock()
	defer terminal.mutex.Unlock()
	terminal.writes = append(terminal.writes, append([]byte(nil), data...))
	if terminal.write != nil {
		return terminal.write(data)
	}
	return len(data), nil
}
func (*fakeTerminal) SetWriteDeadline(time.Time) error {
	return nil
}
func (*fakeTerminal) Resize(Size) error {
	return nil
}
func (*fakeTerminal) Wait() (int, error) {
	return 0, nil
}
func (*fakeTerminal) Stop() error {
	return nil
}
func (*fakeTerminal) Close() error {
	return nil
}

func fakeSession(t *testing.T, authority Authority) (*Session, *fakeTerminal) {
	t.Helper()
	ctx, cancel := context.WithCancelCause(context.Background())
	t.Cleanup(func() { cancel(ErrClosed) })
	backend := &fakeTerminal{}
	manager := &Manager{authority: authority, generation: strings.Repeat("a", 64)}
	session := &Session{manager: manager, authority: authority, id: "random-session", backend: backend, ctx: ctx, cancel: cancel,
		done: make(chan struct{}), input: make(chan struct{}, 1), leaseChanged: make(chan struct{}, 1)}
	return session, backend
}

func TestInputOrderedRetryAndUncertainWrite(t *testing.T) {
	t.Run("exact latest retry does not duplicate bytes", func(t *testing.T) {
		session, backend := fakeSession(t, localAuthority())
		if err := session.Input(context.Background(), 1, []byte("secret input")); err != nil {
			t.Fatal(err)
		}
		if err := session.Input(context.Background(), 1, []byte("secret input")); err != nil {
			t.Fatal(err)
		}
		if len(backend.writes) != 1 {
			t.Fatal("retry replayed input")
		}
		if err := session.Input(context.Background(), 3, []byte("next")); !errors.Is(err, ErrSequence) {
			t.Fatal(err)
		}
		if err := session.Input(context.Background(), 2, []byte("next")); err != nil {
			t.Fatal(err)
		}
		if err := session.Input(context.Background(), 1, []byte("secret input")); !errors.Is(err, ErrSequence) {
			t.Fatal("old sequence replayed")
		}
		if err := session.Input(context.Background(), 2, []byte("changed")); !errors.Is(err, ErrInputUncertain) {
			t.Fatal(err)
		}
		if session.ctx.Err() == nil {
			t.Fatal("changed retry did not close session")
		}
	})
	t.Run("partial failed write is never acknowledged or retried", func(t *testing.T) {
		session, backend := fakeSession(t, localAuthority())
		backend.write = func([]byte) (int, error) { return 1, io.ErrUnexpectedEOF }
		if err := session.Input(context.Background(), 1, []byte("unsafe retry")); !errors.Is(err, ErrInputUncertain) {
			t.Fatal(err)
		}
		if session.lastSequence != 0 {
			t.Fatal("partial write was acknowledged")
		}
		_ = session.Input(context.Background(), 1, []byte("unsafe retry"))
		if len(backend.writes) != 1 {
			t.Fatal("uncertain bytes were retried")
		}
	})
}

func TestInputOwnershipDenialCancellationAndBackpressure(t *testing.T) {
	input := []byte("owned")
	authority := localAuthority()
	entered, proceed := make(chan struct{}), make(chan struct{})
	authority.Authorize = func(_ context.Context, binding invocation.Binding) error {
		if binding.Contract.Action != "input" || binding.Digest == "" {
			t.Error("missing action/digest")
		}
		copy(input, "other")
		close(entered)
		<-proceed
		return nil
	}
	session, backend := fakeSession(t, authority)
	finished := make(chan error, 1)
	go func() { finished <- session.Input(context.Background(), 1, input) }()
	<-entered
	if err := session.Input(context.Background(), 2, []byte("excess")); !errors.Is(err, ErrInputBusy) {
		t.Fatal(err)
	}
	close(proceed)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if string(backend.writes[0]) != "owned" {
		t.Fatal("authorization changed accepted bytes")
	}

	denied := errors.New("denied")
	authority.Authorize = func(context.Context, invocation.Binding) error { return denied }
	session, backend = fakeSession(t, authority)
	if err := session.Input(context.Background(), 1, []byte("never")); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.Input(ctx, 1, []byte("never")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(backend.writes) != 0 {
		t.Fatal("denied/cancelled input reached backend")
	}
}

func TestRemoteLeaseRequiresAuthorityAndCannotBeResurrected(t *testing.T) {
	authority := Authority{Target: invocation.Target{Kind: invocation.Remote, ID: "worker", Fingerprint: strings.Repeat("a", 64), Generation: "worker-generation"},
		Identity: invocation.Identity{Issuer: "verified-transport", Subject: "operator"}}
	if err := validateAuthority(authority); !errors.Is(err, invocation.ErrAuthorizationUnavailable) {
		t.Fatal(err)
	}
	var actions []string
	authority.Authorize = func(_ context.Context, binding invocation.Binding) error {
		actions = append(actions, binding.Contract.Action)
		return nil
	}
	session, _ := fakeSession(t, authority)
	session.leaseUntil = time.Now().Add(25 * time.Millisecond)
	go session.watchLease()
	select {
	case <-session.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("lease did not expire")
	}
	if !errors.Is(context.Cause(session.ctx), ErrLeaseExpired) {
		t.Fatal(context.Cause(session.ctx))
	}
	if err := session.RenewLease(context.Background(), time.Second); err == nil {
		t.Fatal("expired session resurrected")
	}
	if len(actions) != 1 || actions[0] != "renew" {
		t.Fatal("renewal did not require authorization")
	}
	if err := session.RenewLease(context.Background(), MaxLease+1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestSlowAuthorizationCannotExtendLease(t *testing.T) {
	authority := localAuthority()
	authority.Authorize = func(context.Context, invocation.Binding) error { time.Sleep(30 * time.Millisecond); return nil }
	session, _ := fakeSession(t, authority)
	if err := session.RenewLease(context.Background(), time.Millisecond); !errors.Is(err, ErrLeaseExpired) {
		t.Fatal(err)
	}
}

func TestSessionAndProfileIdentityBindOperations(t *testing.T) {
	authority := localAuthority()
	var bindings []invocation.Binding
	authority.Authorize = func(_ context.Context, binding invocation.Binding) error {
		bindings = append(bindings, binding)
		return nil
	}
	first, _ := fakeSession(t, authority)
	second, _ := fakeSession(t, authority)
	second.id = "other-random-session"
	for _, session := range []*Session{first, second} {
		if err := session.Input(context.Background(), 1, []byte("same bytes")); err != nil {
			t.Fatal(err)
		}
	}
	if bindings[0].Digest == bindings[1].Digest || bindings[0].Revision == "" {
		t.Fatal("input binding omitted session/profile identity")
	}
	if bindings[0].Identity != authority.Identity || bindings[0].Target != authority.Target {
		t.Fatal("input identity/target changed")
	}
}

func TestReleaseNeverEvictsActiveOrUncertainCleanup(t *testing.T) {
	session, _ := fakeSession(t, localAuthority())
	manager := session.manager
	manager.sessions = map[string]*Session{session.id: session}
	if err := manager.Release(context.Background(), session.id); err == nil {
		t.Fatal("active session evicted")
	}
	session.result.CleanupError = ErrCleanupIncomplete
	close(session.done)
	if err := manager.Release(context.Background(), session.id); !errors.Is(err, ErrCleanupIncomplete) {
		t.Fatal(err)
	}
	if len(manager.sessions) != 1 {
		t.Fatal("uncertain cleanup session evicted")
	}
}

func TestDeniedCloseCannotCancelAnotherSession(t *testing.T) {
	denied := errors.New("wrong session authority")
	authority := localAuthority()
	authority.Authorize = func(context.Context, invocation.Binding) error { return denied }
	session, _ := fakeSession(t, authority)
	if err := session.Close(context.Background()); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	if session.ctx.Err() != nil {
		t.Fatal("denied close cancelled a session")
	}
}
