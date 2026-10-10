package mcpbridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"dorkpipe.orchestrator/providersession"
)

const (
	privateCancellationFramePrefix = "DORKPIPE_PRIVATE_CANCELLATION_V1 "
	privateCancellationFrameLimit  = 64 * 1024
)

type providerPoolCancellationScope struct {
	Session     providersession.SessionRef  `json:"session"`
	Correlation providersession.Correlation `json:"correlation"`
}

type privateCancellationScopeFrame struct {
	Type  string                        `json:"type"`
	Scope providerPoolCancellationScope `json:"scope"`
}

type privateCancellationIntentFrame struct {
	Type   string                             `json:"type"`
	Intent providersession.CancellationIntent `json:"intent"`
}

type pendingCancellation struct {
	scope       providerPoolCancellationScope
	intents     chan *cancellationDelivery
	submitted   bool
	failure     chan struct{}
	failureErr  error
	failureOnce sync.Once
}

func (p *pendingCancellation) fail(err error) {
	if err == nil {
		err = errors.New("pending cancellation scope is no longer available")
	}
	p.failureOnce.Do(func() {
		p.failureErr = err
		close(p.failure)
	})
}

type cancellationDelivery struct {
	intent     providersession.CancellationIntent
	controller *transientCancellationController
	pending    *pendingCancellation
	ack        chan error
	once       sync.Once
}

func (d *cancellationDelivery) complete(err error) {
	d.once.Do(func() {
		if err != nil {
			d.controller.failPending(d.pending, err)
		} else {
			d.controller.completePending(d.pending)
		}
		d.ack <- err
	})
}

type transientCancellationController struct {
	mu       sync.Mutex
	pending  *pendingCancellation
	closed   bool
	closedCh chan struct{}
}

func newTransientCancellationController() *transientCancellationController {
	return &transientCancellationController{closedCh: make(chan struct{})}
}

func cloneCancellationScope(scope providerPoolCancellationScope) providerPoolCancellationScope {
	return providerPoolCancellationScope{Session: scope.Session, Correlation: scope.Correlation}
}

func cloneCancellationIntent(intent providersession.CancellationIntent) providersession.CancellationIntent {
	return providersession.CancellationIntent{Session: intent.Session, Correlation: intent.Correlation, Reason: intent.Reason}
}

func validateCancellationScope(scope providerPoolCancellationScope) error {
	if err := scope.Session.Validate(); err != nil {
		return errors.New("invalid neutral cancellation scope")
	}
	if scope.Session.Provider != "codex" || strings.TrimSpace(scope.Session.SessionID) != scope.Session.SessionID {
		return errors.New("invalid neutral cancellation scope")
	}
	correlation := scope.Correlation
	for _, value := range []string{correlation.ProcessIncarnationID, correlation.ConnectionID, correlation.SessionID, correlation.InteractionID} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return errors.New("invalid neutral cancellation scope")
		}
	}
	if correlation.SessionID != scope.Session.SessionID || correlation.ActivityID != "" || correlation.RequestID != "" || correlation.DecisionID != "" {
		return errors.New("invalid neutral cancellation scope")
	}
	return nil
}

func validateCancellationIntent(intent providersession.CancellationIntent, scope providerPoolCancellationScope) error {
	if err := intent.Validate(); err != nil {
		return errors.New("invalid neutral cancellation intent")
	}
	if intent.Session != scope.Session || intent.Correlation != scope.Correlation {
		return errors.New("cancellation intent must match the exact pending scope")
	}
	switch intent.Reason {
	case providersession.CancellationReasonUserRequested, providersession.CancellationReasonSafetyStop, providersession.CancellationReasonDeadline:
		return nil
	default:
		return errors.New("invalid neutral cancellation intent")
	}
}

func (c *transientCancellationController) pendingScope() (providerPoolCancellationScope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pending == nil || c.pending.submitted {
		return providerPoolCancellationScope{}, errors.New("no exact provider-pool cancellation scope is pending")
	}
	return cloneCancellationScope(c.pending.scope), nil
}

func (c *transientCancellationController) registerScope(scope providerPoolCancellationScope) (*pendingCancellation, error) {
	scope = cloneCancellationScope(scope)
	if err := validateCancellationScope(scope); err != nil {
		return nil, err
	}
	pending := &pendingCancellation{
		scope:   scope,
		intents: make(chan *cancellationDelivery),
		failure: make(chan struct{}),
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("provider-pool cancellation transport is closed")
	}
	if c.pending != nil {
		return nil, errors.New("a provider-pool cancellation scope is already pending")
	}
	c.pending = pending
	return pending, nil
}

func (c *transientCancellationController) awaitIntent(ctx context.Context, scope providerPoolCancellationScope) (*cancellationDelivery, error) {
	pending, err := c.registerScope(scope)
	if err != nil {
		return nil, err
	}
	return c.awaitRegisteredIntent(ctx, pending)
}

func (c *transientCancellationController) awaitRegisteredIntent(ctx context.Context, pending *pendingCancellation) (*cancellationDelivery, error) {
	select {
	case delivery := <-pending.intents:
		return delivery, nil
	case <-pending.failure:
		return nil, pending.failureErr
	case <-c.closedCh:
		return nil, errors.New("provider-pool cancellation transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return nil, ctx.Err()
	}
}

func (c *transientCancellationController) submit(ctx context.Context, intent providersession.CancellationIntent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	intent = cloneCancellationIntent(intent)
	c.mu.Lock()
	pending := c.pending
	if c.closed || pending == nil || pending.submitted {
		c.mu.Unlock()
		return errors.New("no exact provider-pool cancellation scope is pending")
	}
	scope := cloneCancellationScope(pending.scope)
	if err := validateCancellationIntent(intent, scope); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("cancellation intent rejected: %w", err)
	}
	pending.submitted = true
	c.mu.Unlock()

	delivery := &cancellationDelivery{intent: intent, controller: c, pending: pending, ack: make(chan error, 1)}
	select {
	case pending.intents <- delivery:
	case <-pending.failure:
		return pending.failureErr
	case <-c.closedCh:
		return errors.New("provider-pool cancellation transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return ctx.Err()
	}
	select {
	case err := <-delivery.ack:
		return err
	case <-pending.failure:
		return pending.failureErr
	case <-c.closedCh:
		return errors.New("provider-pool cancellation transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return ctx.Err()
	}
}

func (c *transientCancellationController) completePending(pending *pendingCancellation) {
	c.mu.Lock()
	if c.pending == pending {
		c.pending = nil
	}
	c.mu.Unlock()
}

func (c *transientCancellationController) failPending(pending *pendingCancellation, err error) {
	c.mu.Lock()
	if c.pending == pending {
		c.pending = nil
		pending.fail(err)
	}
	c.mu.Unlock()
}

func (c *transientCancellationController) invalidate(err error) {
	c.mu.Lock()
	if c.pending != nil {
		pending := c.pending
		c.pending = nil
		pending.fail(err)
	}
	c.mu.Unlock()
}

func (c *transientCancellationController) close(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if c.pending != nil {
		pending := c.pending
		c.pending = nil
		pending.fail(err)
	}
	close(c.closedCh)
	c.mu.Unlock()
}
