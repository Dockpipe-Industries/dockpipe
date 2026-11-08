package terminalsession

import (
	"context"
	"crypto/rand"
	"strconv"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

// Manager retains at most eight sessions, including completed output until
// Release. It never silently evicts an active or incompletely cleaned session.
type Manager struct {
	mutex      sync.Mutex
	profile    Profile
	authority  Authority
	generation string
	sessions   map[string]*Session
	closed     bool
	provider   ProviderFactory
}

// Generation identifies the immutable owner profile, including its full native
// arguments/environment. It is safe catalog metadata; never expose Profile.
func (manager *Manager) Generation() string {
	return manager.generation
}

func New(profile Profile, authority Authority) (*Manager, error) {
	if profile.Route == "host" || profile.EnvironmentMode == "inherit-host" {
		return nil, ErrInvalid
	}
	return newManager(profile, authority, startTerminal)
}

func newManager(profile Profile, authority Authority, provider ProviderFactory) (*Manager, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	profile.Args = append([]string(nil), profile.Args...)
	profile.Env = append([]string(nil), profile.Env...)
	values := []string{profile.ID, profile.Executable, profile.Dir, profile.Route, profile.EnvironmentMode, strconv.Itoa(len(profile.Args))}
	values = append(values, profile.Args...)
	values = append(values, profile.Env...)
	manager := &Manager{profile: profile, authority: authority, generation: digest(values...), sessions: make(map[string]*Session), provider: provider}
	// Validate trusted composition without invoking an authorizer or any effect.
	if err := validateAuthority(authority); err != nil {
		return nil, err
	}
	return manager, nil
}

// Open's context is the session lifetime, not an HTTP request lifetime. Remote
// composition must own that context and renew only after fresh authorization.
// Random IDs cannot select an existing session, and the profile generation is
// bound into every action's digest and revision.
func (manager *Manager) Open(ctx context.Context, size Size, lease time.Duration) (*Session, error) {
	return manager.OpenAs(ctx, size, lease, manager.authority.Identity, manager.authority.Authorize)
}

// OpenAs is for trusted transport composition: identity must already be
// authenticated, never copied from caller JSON. Target/profile/quota stay owned
// by this manager while each session retains its particular operator authority.
func (manager *Manager) OpenAs(ctx context.Context, size Size, lease time.Duration, identity invocation.Identity, authorize invocation.Authorize) (*Session, error) {
	if !size.valid() || !manager.validLease(lease) {
		return nil, ErrInvalid
	}
	id := rand.Text()
	var leaseUntil time.Time
	if lease != 0 {
		leaseUntil = time.Now().Add(lease)
	}
	inputDigest := digest(manager.generation, id, strconv.Itoa(size.Columns), strconv.Itoa(size.Rows), lease.String())
	authority := Authority{Target: manager.authority.Target, Identity: identity, Authorize: authorize}
	return admit(ctx, authority, "open", manager.generation, inputDigest, func(ctx context.Context) (*Session, error) {
		manager.mutex.Lock()
		defer manager.mutex.Unlock()
		if manager.closed {
			return nil, ErrClosed
		}
		if len(manager.sessions) >= MaxSessions {
			return nil, ErrCapacity
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !leaseUntil.IsZero() && !time.Now().Before(leaseUntil) {
			return nil, ErrLeaseExpired
		}
		session, err := startSession(ctx, manager, authority, id, size, leaseUntil)
		if err != nil {
			return nil, err
		}
		manager.sessions[id] = session
		return session, nil
	})
}

func (manager *Manager) validLease(lease time.Duration) bool {
	return lease > 0 && lease <= MaxLease || lease == 0 && !manager.remote()
}

// Release discards completed output only after cleanup succeeded. Keep the
// session handle/receipt for uncertain cleanup; restarting work is not recovery.
func (manager *Manager) Release(ctx context.Context, id string) error {
	_, err := admit(ctx, manager.authority, "release", manager.generation, digest(manager.generation, id), func(context.Context) (struct{}, error) {
		manager.mutex.Lock()
		defer manager.mutex.Unlock()
		session := manager.sessions[id]
		if session == nil {
			return struct{}{}, ErrInvalid
		}
		select {
		case <-session.done:
		default:
			return struct{}{}, ErrActive
		}
		session.mutex.Lock()
		defer session.mutex.Unlock()
		if session.result.CleanupError != nil {
			return struct{}{}, session.result.CleanupError
		}
		clear(session.output.data[:])
		session.released = true
		delete(manager.sessions, id)
		return struct{}{}, nil
	})
	return err
}

// Shutdown is trusted owner teardown, not a caller action. Cancellation cannot
// depend on permission to continue using a revoked session. Cleanup continues
// even when the supplied waiting context expires.
func (manager *Manager) Shutdown(ctx context.Context) error {
	manager.mutex.Lock()
	manager.closed = true
	sessions := make([]*Session, 0, len(manager.sessions))
	for _, session := range manager.sessions {
		sessions = append(sessions, session)
		session.cancel(ErrClosed)
	}
	manager.mutex.Unlock()
	for _, session := range sessions {
		result, err := session.Wait(ctx)
		if err != nil {
			return err
		}
		if result.CleanupError != nil {
			return result.CleanupError
		}
	}
	return nil
}
