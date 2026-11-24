package remote

import (
	"context"
	"errors"
	"os"
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

func collectNetworkTrust(parent context.Context, root string, platform DesktopSessionSnapshot, target contract.FeatureTarget, enabled bool) networkTrustMaterial {
	if !enabled {
		return networkTrustMaterial{summary: contract.NetworkTrustSummary{State: "disabled"}}
	}
	unavailable := networkTrustMaterial{summary: contract.NetworkTrustSummary{State: "unavailable"}}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	certificates, err := loadNetworkTrustAnchors(root)
	if errors.Is(err, os.ErrNotExist) {
		source, supported := platform.(networkTLSAnchorSource)
		if !supported {
			return unavailable
		}
		certificates, err = source.NetworkTLSAnchors(ctx)
	}
	if err != nil || ctx.Err() != nil {
		return unavailable
	}
	snapshot, err := contract.NewNetworkTrustSnapshot(target, certificates)
	if err != nil {
		return unavailable
	}
	return networkTrustMaterial{summary: snapshot.Summary(), snapshot: &snapshot}
}
