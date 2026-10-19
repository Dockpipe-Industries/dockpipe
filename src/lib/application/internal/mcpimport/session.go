package mcpimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/mcpsession"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Lease is owned runtime state, not merely the docker CLI process. Prepare must
// preserve the selected runtime policy and verify the installed receipt/payload.
// Close must stop/remove only the exact owned runtime object, even if its attach
// client crashed. This seam supplies no production Docker lease implementation.
type Lease struct {
	Prepare mcpsession.CommandFactory
	Close   func(context.Context) error
}

// Executor must never pull/build images, inject ambient credentials, or fall
// back to host execution. Existing RunContainer needs an owned create/attach/
// cleanup extraction before it can implement this protocol-safe lease.
type Executor func(context.Context, Selection, infrastructure.RunOpts) (*Lease, error)

type Session struct {
	plan      *Plan
	authorize Authorizer
	protocol  *mcpsession.Session
	lease     *Lease
	cancel    context.CancelFunc
	once      sync.Once
	closeErr  error
}

func (p *Plan) Start(ctx context.Context, authorize Authorizer, execute Executor) (*Session, error) {
	if execute == nil {
		return nil, fmt.Errorf("MCP import runtime executor is unavailable")
	}
	if err := p.authorize(ctx, authorize, "start", "", "", nil); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	// Pass a detached snapshot to the runtime owner; mutable caller/executor
	// configuration cannot change the authorization fingerprint held by the plan.
	data, err := json.Marshal(p.selection)
	if err != nil {
		cancel()
		return nil, err
	}
	var selection Selection
	if err = json.Unmarshal(data, &selection); err != nil {
		cancel()
		return nil, err
	}
	options := infrastructure.RunOpts{Image: p.options.Image, WorkdirHost: p.options.WorkdirHost}
	if err = runtimepolicy.ApplyCompiledRuntimeManifest(&options, &selection.Runtime); err != nil {
		cancel()
		return nil, err
	}
	lease, err := execute(lifetime, selection, options)
	if err != nil {
		cancel()
		if lease != nil && lease.Close != nil {
			err = errors.Join(err, closeLease(lease))
		}
		return nil, err
	}
	if lease == nil || lease.Prepare == nil || lease.Close == nil {
		cancel()
		if lease != nil && lease.Close != nil {
			return nil, errors.Join(fmt.Errorf("MCP executor returned an incomplete ownership lease"), closeLease(lease))
		}
		return nil, fmt.Errorf("MCP executor returned an incomplete ownership lease")
	}
	protocol, err := mcpsession.Start(lifetime, func(commandContext context.Context) (*exec.Cmd, error) {
		if err := p.authorize(commandContext, authorize, "start", "", "", nil); err != nil {
			return nil, err
		}
		return lease.Prepare(commandContext)
	})
	if err != nil {
		cancel()
		return nil, errors.Join(err, closeLease(lease))
	}
	session := &Session{plan: p, authorize: authorize, protocol: protocol, lease: lease, cancel: cancel}
	go func() {
		select {
		case <-lifetime.Done():
		case <-protocol.Exited():
		}
		_ = session.Stop() // retained in closeErr and returned by every subsequent Stop
	}()
	return session, nil
}

func (s *Session) Discover(ctx context.Context) (mcpsession.Discovery, error) {
	if err := s.plan.authorize(ctx, s.authorize, "discover", "", "", nil); err != nil {
		return mcpsession.Discovery{}, err
	}
	return s.protocol.Discover(ctx)
}

func (s *Session) Call(ctx context.Context, metadata, tool string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	return s.protocol.CallChecked(ctx, metadata, tool, arguments, func(callContext context.Context) error {
		return s.plan.authorize(callContext, s.authorize, "call", tool, metadata, arguments)
	})
}

// Stop is always available for owned cleanup; revocation must never strand a
// runtime object. The same idempotent owner handles cancellation and child exit.
func (s *Session) Stop() error {
	s.once.Do(func() {
		s.cancel()
		s.closeErr = errors.Join(s.protocol.Stop(), closeLease(s.lease))
	})
	return s.closeErr
}

func closeLease(lease *Lease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return lease.Close(ctx)
}
