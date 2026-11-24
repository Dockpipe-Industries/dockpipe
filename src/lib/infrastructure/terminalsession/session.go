package terminalsession

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"sync"
	"time"
)

type Result struct {
	ExitCode     int
	Cause        error
	CleanupError error
}

// Session owns its PTY, process lifetime and bounded volatile output. No input
// bytes, output, passwords or environment are logged or persisted. Only the last
// fully written input's sequence and hash are retained for acknowledgement retry.
type Session struct {
	mutex        sync.Mutex
	manager      *Manager
	authority    Authority
	id           string
	backend      terminalBackend
	ctx          context.Context
	cancel       context.CancelCauseFunc
	done         chan struct{}
	outputReady  chan struct{}
	input        chan struct{}
	leaseUntil   time.Time
	leaseChanged chan struct{}
	output       outputRing
	lastSequence uint64
	lastDigest   [32]byte
	result       Result
	released     bool
}

type terminalBackend interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	SetWriteDeadline(time.Time) error
	Resize(Size) error
	Wait() (int, error)
	Stop() error
	Close() error
}

func startSession(ctx context.Context, manager *Manager, authority Authority, id string, size Size, leaseUntil time.Time) (*Session, error) {
	lifetime, cancel := context.WithCancelCause(ctx)
	// Cover startup too: a slow executable/filesystem must not turn an expired
	// launch lease into a fresh shell. Once launched, watchLease owns renewal.
	var startupLease *time.Timer
	if !leaseUntil.IsZero() {
		startupLease = time.AfterFunc(time.Until(leaseUntil), func() { cancel(ErrLeaseExpired) })
	}
	profile := manager.profile
	profile.Args = append([]string(nil), profile.Args...)
	profile.Env = append([]string(nil), profile.Env...)
	backend, err := manager.provider(lifetime, profile, size)
	if startupLease != nil {
		startupLease.Stop()
	}
	if err != nil {
		if cause := context.Cause(lifetime); cause != nil {
			err = cause
		}
		cancel(err)
		return nil, err
	}
	session := &Session{manager: manager, authority: authority, id: id, backend: backend, ctx: lifetime, cancel: cancel,
		done: make(chan struct{}), outputReady: make(chan struct{}, 1), input: make(chan struct{}, 1), leaseChanged: make(chan struct{}, 1)}
	session.leaseUntil = leaseUntil
	go session.run()
	go session.watchLease()
	return session, nil
}

func (session *Session) ID() string {
	return session.id
}
func (session *Session) Done() <-chan struct{} {
	return session.done
}

// OutputReady is a coalesced notification for trusted local composition. Read
// still admits every disclosure and owns cursors/truncation. No polling service
// or unbounded notification queue is needed when a local terminal is idle.
func (session *Session) OutputReady() <-chan struct{} {
	return session.outputReady
}

// Wait observes owned cleanup; it does not authorize use or disclose output.
func (session *Session) Wait(ctx context.Context) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-session.done:
		session.mutex.Lock()
		defer session.mutex.Unlock()
		return session.result, nil
	}
}

func (session *Session) run() {
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buffer := make([]byte, MaxInput)
		defer clear(buffer)
		for {
			count, err := session.backend.Read(buffer)
			if count != 0 {
				session.mutex.Lock()
				session.output.append(buffer[:count])
				session.mutex.Unlock()
				select {
				case session.outputReady <- struct{}{}:
				default:
				}
			}
			if err != nil {
				return
			}
		}
	}()
	waitDone := make(chan Result, 1)
	go func() {
		code, err := session.backend.Wait()
		waitDone <- Result{ExitCode: code, Cause: err}
	}()
	var result Result
	exited := false
	select {
	case result = <-waitDone:
		exited = true
		if cause := context.Cause(session.ctx); cause != nil {
			result.Cause = cause
		}
		session.cancel(ErrClosed)
		// Let already-produced output drain. A background job holding the slave
		// cannot hold teardown indefinitely.
		select {
		case <-readDone:
		case <-time.After(100 * time.Millisecond):
		}
	case <-session.ctx.Done():
		result.Cause = context.Cause(session.ctx)
	}
	// The backend owns process-group cleanup, including interactive job-control
	// groups. Do not close the PTY until those groups have been inspected.
	result.CleanupError = session.backend.Stop()
	_ = session.backend.Close()
	if !exited {
		select {
		case exited := <-waitDone:
			result.ExitCode = exited.ExitCode
		case <-time.After(time.Second):
			result.CleanupError = errors.Join(result.CleanupError, ErrCleanupIncomplete)
		}
	}
	<-readDone // closing the pollable PTY interrupts its read; never a raw blocking fd
	session.mutex.Lock()
	session.result = result
	session.mutex.Unlock()
	close(session.done)
}

func (session *Session) watchLease() {
	for {
		session.mutex.Lock()
		until := session.leaseUntil
		session.mutex.Unlock()
		var timer *time.Timer
		var expired <-chan time.Time
		if !until.IsZero() {
			timer = time.NewTimer(time.Until(until))
			expired = timer.C
		}
		select {
		case <-session.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-session.leaseChanged:
		case <-expired:
			session.mutex.Lock()
			if !session.leaseUntil.IsZero() && !time.Now().Before(session.leaseUntil) {
				session.cancel(ErrLeaseExpired)
			}
			session.mutex.Unlock()
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

func (session *Session) check() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if !session.leaseUntil.IsZero() && !time.Now().Before(session.leaseUntil) {
		session.cancel(ErrLeaseExpired)
		return ErrLeaseExpired
	}
	if session.released {
		return ErrClosed
	}
	return session.ctx.Err()
}

func (session *Session) Input(ctx context.Context, sequence uint64, data []byte) error {
	if sequence == 0 || len(data) == 0 || len(data) > MaxInput {
		return ErrInvalid
	}
	// At most one owned pending-input buffer, including slow authorization.
	// Backpressure rejects before copying instead of building an unbounded queue.
	select {
	case session.input <- struct{}{}:
		defer func() { <-session.input }()
	default:
		return ErrInputBusy
	}
	owned := append([]byte(nil), data...)
	defer clear(owned)
	hash := sha256.Sum256(owned)
	_, err := session.admit(ctx, "input", digest(strconv.FormatUint(sequence, 10), string(owned)), func(ctx context.Context) (struct{}, error) {
		if err := session.check(); err != nil {
			return struct{}{}, err
		}
		if sequence == session.lastSequence {
			if hash == session.lastDigest {
				return struct{}{}, nil
			}
			session.cancel(ErrInputUncertain)
			return struct{}{}, ErrInputUncertain
		}
		if sequence != session.lastSequence+1 {
			return struct{}{}, ErrSequence
		}
		deadline := time.Now().Add(inputTimeout)
		if requested, ok := ctx.Deadline(); ok && requested.Before(deadline) {
			deadline = requested
		}
		if err := session.backend.SetWriteDeadline(deadline); err != nil {
			return struct{}{}, err
		}
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			_ = session.backend.SetWriteDeadline(time.Now())
			close(interrupted)
		})
		defer func() {
			if !stop() {
				<-interrupted
			}
		}()
		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		remaining := owned
		for len(remaining) != 0 {
			count, err := session.backend.Write(remaining)
			if err != nil || count <= 0 {
				session.cancel(ErrInputUncertain)
				return struct{}{}, ErrInputUncertain
			}
			remaining = remaining[count:]
		}
		// Acknowledgement means all bytes reached the PTY, not that a command
		// completed or that the program consumed them. Never retry shell effects.
		session.lastSequence, session.lastDigest = sequence, hash
		return struct{}{}, nil
	})
	return err
}

func (session *Session) Resize(ctx context.Context, size Size) error {
	if !size.valid() {
		return ErrInvalid
	}
	_, err := session.admit(ctx, "resize", digest(strconv.Itoa(size.Columns), strconv.Itoa(size.Rows)), func(context.Context) (struct{}, error) {
		if err := session.check(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, session.backend.Resize(size)
	})
	return err
}

func (session *Session) Read(ctx context.Context, cursor uint64, limit int) (Output, error) {
	if limit < 1 || limit > MaxOutput {
		return Output{}, ErrInvalid
	}
	var output Output
	_, err := session.admit(ctx, "read", digest(strconv.FormatUint(cursor, 10), strconv.Itoa(limit)), func(context.Context) (struct{}, error) {
		session.mutex.Lock()
		defer session.mutex.Unlock()
		if session.released {
			return struct{}{}, ErrClosed
		}
		if session.manager.remote() && !time.Now().Before(session.leaseUntil) {
			return struct{}{}, ErrLeaseExpired
		}
		var err error
		output, err = session.output.read(cursor, limit)
		select {
		case <-session.done:
			output.Closed = true
		default:
		}
		return struct{}{}, err
	})
	return output, err
}

func (session *Session) RenewLease(ctx context.Context, lease time.Duration) error {
	if lease <= 0 || lease > MaxLease {
		return ErrInvalid
	}
	leaseUntil := time.Now().Add(lease)
	_, err := session.admit(ctx, "renew", digest(lease.String()), func(context.Context) (struct{}, error) {
		session.mutex.Lock()
		defer session.mutex.Unlock()
		if session.ctx.Err() != nil || session.released {
			return struct{}{}, ErrClosed
		}
		if !time.Now().Before(leaseUntil) || !session.leaseUntil.IsZero() && !time.Now().Before(session.leaseUntil) {
			session.cancel(ErrLeaseExpired)
			return struct{}{}, ErrLeaseExpired
		}
		session.leaseUntil = leaseUntil
		select {
		case session.leaseChanged <- struct{}{}:
		default:
		}
		return struct{}{}, nil
	})
	return err
}

// Close authorizes a shutdown request. Wait/Done report completion and whether
// native cleanup was verified; a successful Close is not that completion proof.
func (session *Session) Close(ctx context.Context) error {
	// Caller-directed cancellation is authorized like every other session action.
	// Trusted owner teardown uses the lifetime context or Manager.Shutdown, so an
	// expired/revoked caller cannot stop a session belonging to someone else.
	_, err := session.admit(ctx, "close", "", func(context.Context) (struct{}, error) {
		session.cancel(ErrClosed)
		return struct{}{}, nil
	})
	return err
}

func (session *Session) admit(ctx context.Context, action, parameters string, execute func(context.Context) (struct{}, error)) (struct{}, error) {
	return admit(ctx, session.authority, action, session.manager.generation,
		digest(session.manager.generation, session.id, parameters), execute)
}
