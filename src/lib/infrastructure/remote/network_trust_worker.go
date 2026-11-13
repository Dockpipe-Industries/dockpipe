package remote

import (
	"context"
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

const networkTrustRefresh = 5 * time.Second

var errNetworkTrustChanged = errors.New("public TLS trust changed; replace the network lifetime")

type networkTLSAnchorSource interface {
	NetworkTLSAnchors(context.Context) ([][]byte, error)
}

type networkTrustMaterial struct {
	summary  contract.NetworkTrustSummary
	snapshot *contract.NetworkTrustSnapshot
}

func collectNetworkTrust(parent context.Context, platform DesktopSessionSnapshot, target contract.FeatureTarget, enabled bool) networkTrustMaterial {
	if !enabled {
		return networkTrustMaterial{summary: contract.NetworkTrustSummary{State: "disabled"}}
	}
	unavailable := networkTrustMaterial{summary: contract.NetworkTrustSummary{State: "unavailable"}}
	source, supported := platform.(networkTLSAnchorSource)
	if !supported {
		return unavailable
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	certificates, err := source.NetworkTLSAnchors(ctx)
	if err != nil || ctx.Err() != nil {
		return unavailable
	}
	snapshot, err := contract.NewNetworkTrustSnapshot(target, certificates)
	if err != nil {
		return unavailable
	}
	return networkTrustMaterial{summary: snapshot.Summary(), snapshot: &snapshot}
}

func networkWorkerContextError(ctx context.Context, trustChanged <-chan struct{}) error {
	select {
	case <-trustChanged:
		return errNetworkTrustChanged
	default:
		return ctx.Err()
	}
}
