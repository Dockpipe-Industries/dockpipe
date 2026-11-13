package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	"dockpipe/src/lib/infrastructure/remoteidentity"
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
	unlock, err := Lock(filepath.Join(root, "network-worker.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	frozen, err := LoadNetworkPermissions(root)
	if err != nil {
		return err
	}
	if frozen.Validate() != nil || !frozen.Enabled {
		return errors.New("independent worker TCP network permission is disabled")
	}
	instance, err := Secret()
	if err != nil {
		return err
	}
	fence := NewDesktopSessionFence(platform)
	if err = fence.Check(parent); err != nil {
		return err
	}
	target := contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(config.MachineKey), Generation: Hash(instance)}
	material := collectNetworkTrust(parent, platform, target, frozen.ShareTLSTrust)
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

	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel()
	// Separate fence checks cancel a stalled exchange/dial rather than waiting
	// through its network timeout after a worker lock or grant change.
	trustChanged := make(chan struct{}, 1)
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
					current := collectNetworkTrust(ctx, platform, target, true)
					if ctx.Err() != nil {
						return
					}
					if current.summary != material.summary {
						trustChanged <- struct{}{}
						cancel()
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
				if permissionErr != nil || current != frozen || fence.Check(ctx) != nil {
					cancel()
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
				return networkWorkerContextError(ctx, trustChanged)
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
				_ = client.ServeNetworkTCP(ctx, ticket)
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
			return networkWorkerContextError(ctx, trustChanged)
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
				fmt.Fprintln(os.Stderr, "Remote TCP network service stopped; inspect the independent grant and OS session before re-enabling it.")
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
