package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

var (
	errNetworkGrantUnavailable   = errors.New("independent worker TCP network permission is unavailable or changed")
	errNetworkSessionUnavailable = errors.New("worker OS lock or sleep evidence is unavailable or changed")
	errNetworkLockUnavailable    = errors.New("worker network lifetime lock is unavailable")
)

// RunNetworkWorker owns one independent TCP worker lifetime. The caller owns
// cancellation and any private readiness signaling; normal services pass nil.
func RunNetworkWorker(parent context.Context, root string, config contract.WorkerConfig, client *Client, ready func() error) error {
	platform, err := osauth.New()
	if err != nil {
		return err
	}
	return runNetworkWorkerWithPlatform(parent, root, config, client, ready, platform)
}

func runNetworkWorkerWithPlatform(parent context.Context, root string, config contract.WorkerConfig, client *Client, ready func() error, platform DesktopSessionSnapshot) error {
	if client == nil || client.Identity == nil {
		return errors.New("network worker requires paired machine identity")
	}
	unlock, err := lockNetworkWorker(root)
	if err != nil {
		return fmt.Errorf("%w: %w", errNetworkLockUnavailable, err)
	}
	defer unlock()
	frozen, err := LoadNetworkPermissions(root)
	if err != nil {
		return fmt.Errorf("%w: %w", errNetworkGrantUnavailable, err)
	}
	if frozen.Validate() != nil || !frozen.Enabled {
		return errNetworkGrantUnavailable
	}
	instance, err := Secret()
	if err != nil {
		return err
	}
	fence := NewDesktopSessionFence(platform)
	if err = fence.Check(parent); err != nil {
		return fmt.Errorf("%w: %w", errNetworkSessionUnavailable, err)
	}
	target := contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(config.MachineKey), Generation: Hash(instance)}
	material := collectNetworkTrust(parent, root, platform, target, frozen.ShareTLSTrust)
	if err := parent.Err(); err != nil {
		return err
	}
	permissionBytes, _ := json.Marshal(frozen)
	target.Generation = Hash(instance + string(permissionBytes) + material.summary.State + material.summary.Digest)
	catalog := contract.NetworkCatalog{Schema: contract.NetworkVersion, Enabled: true, Target: target}
	if frozen.ShareTLSTrust {
		catalog.TLSTrust = &material.summary
		if material.snapshot != nil {
			material.snapshot.Target = target
			publication, stop := context.WithTimeout(parent, 5*time.Second)
			var accepted contract.NetworkTrustSummary
			err = client.Call(publication, "/v1/network-trust-publish", material.snapshot, &accepted)
			stop()
			if err != nil {
				return err
			}
			if accepted != material.summary {
				return errors.New("network trust publication changed its authenticated summary")
			}
		}
	}

	ctx, cancel := context.WithCancelCause(parent)
	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel(nil)
	// Separate fence checks cancel a stalled exchange/dial rather than waiting
	// through its network timeout after a worker lock or grant change.
	if frozen.ShareTLSTrust {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ticker := time.NewTicker(networkTrustRefresh)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					current := collectNetworkTrust(ctx, root, platform, target, true)
					if ctx.Err() != nil {
						return
					}
					if current.summary != material.summary {
						cancel(errNetworkTrustChanged)
						return
					}
				}
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, permissionErr := LoadNetworkPermissions(root)
				if permissionErr != nil || current != frozen {
					cancel(errNetworkGrantUnavailable)
					return
				}
				if fence.Check(ctx) != nil {
					cancel(errNetworkSessionUnavailable)
					return
				}
			}
		}
	}()
	slots := make(chan struct{}, contract.MaxNetworkNodeStreams)
	announced := false
	for {
		var tickets []contract.NetworkTicket
		exchange, stop := context.WithTimeout(ctx, 3*time.Second)
		err = client.Call(exchange, "/v1/network-worker", catalog, &tickets)
		stop()
		if err != nil {
			if ctx.Err() != nil {
				return context.Cause(ctx)
			}
			return err
		}
		if len(tickets) > contract.MaxNetworkNodeStreams {
			return errors.New("network directive capacity exceeded")
		}
		for _, ticket := range tickets {
			if ticket.Request.Validate() != nil || ticket.Request.Target != catalog.Target {
				return errors.New("network directive changed exact worker target")
			}
			select {
			case slots <- struct{}{}:
			default:
				return errors.New("network worker stream capacity exceeded")
			}
			workers.Add(1)
			go func(ticket contract.NetworkTicket) {
				defer workers.Done()
				defer func() { <-slots }()
				_ = client.ServeNetworkTCP(ctx, ticket, frozen)
			}(ticket)
		}
		if !announced {
			if ready != nil {
				if err = ready(); err != nil {
					return err
				}
			}
			announced = true
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// A normal worker supervises the explicit local grant, not network retries.
// After an uncertain/locked lifetime ends, a new observed enable transition or
// service restart is required. No destination open is replayed automatically.
func runNetworkService(ctx context.Context, root string, start func(context.Context) error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	previouslyEnabled := false
	var previous contract.NetworkPermissions
	for {
		if ctx.Err() != nil {
			return
		}
		permission, err := LoadNetworkPermissions(root)
		enabled := err == nil && permission.Enabled
		if enabled && (!previouslyEnabled || permission != previous) {
			previouslyEnabled = true
			previous = permission
			startErr := start(ctx)
			if errors.Is(startErr, errNetworkTrustChanged) {
				previouslyEnabled = false
			}
			if startErr != nil && !errors.Is(startErr, errNetworkTrustChanged) && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "Remote TCP network service stopped (%s); verify the worker before re-enabling Network.\n",
					networkServiceStopReason(startErr))
			}
		} else if !enabled {
			previouslyEnabled = false
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func networkServiceStopReason(err error) string {
	var status *HTTPError
	switch {
	case errors.Is(err, errNetworkGrantUnavailable):
		return "grant-unavailable"
	case errors.Is(err, errNetworkSessionUnavailable):
		return "os-session-unavailable"
	case errors.Is(err, errNetworkLockUnavailable):
		return "worker-lock-unavailable"
	case errors.As(err, &status):
		return fmt.Sprintf("broker-http-%d", status.StatusCode)
	case retryable(err):
		return "broker-transport"
	default:
		return "worker-error"
	}
}
