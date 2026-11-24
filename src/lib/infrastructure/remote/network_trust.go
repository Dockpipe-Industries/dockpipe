package remote

import (
	"context"
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type networkTrustRecord struct {
	snapshot  contract.NetworkTrustSnapshot
	summary   contract.NetworkTrustSummary
	published time.Time
}

func (broker *Broker) pruneNetworkTrust() {
	state := broker.networkState()
	for node, record := range state.trusts {
		catalog, exists := state.catalogs[node]
		current := exists && broker.networkCurrent(catalog) && catalog.Target == record.snapshot.Target && catalog.TLSTrust != nil && *catalog.TLSTrust == record.summary
		// A signed publication may precede its first ordinary catalog exchange.
		if !current && time.Since(record.published) > contract.NetworkFreshness {
			state.trustBytes -= record.summary.Bytes
			delete(state.trusts, node)
		}
	}
}

func (broker *Broker) networkTrustUsable(catalog contract.NetworkCatalog) bool {
	if catalog.TLSTrust == nil || catalog.TLSTrust.State == "disabled" {
		return true
	}
	if catalog.TLSTrust.State != "ready" {
		return false
	}
	record, exists := broker.networkState().trusts[catalog.Target.Node]
	return exists && record.snapshot.Target == catalog.Target && record.summary == *catalog.TLSTrust
}

func (broker *Broker) publishNetworkTrust(node string, raw []byte) (any, error) {
	var snapshot contract.NetworkTrustSnapshot
	if len(raw) > contract.MaxNetworkTrustBody || decodeFeature(raw, &snapshot) != nil || snapshot.Validate() != nil || snapshot.Target.Node != node {
		return nil, errors.New("invalid bounded network trust publication")
	}
	enrollment, exists := broker.state.Nodes[node]
	if !exists || enrollment.Revoked || enrollment.MachineKey == "" || remoteidentity.Fingerprint(enrollment.MachineKey) != snapshot.Target.Fingerprint {
		return nil, errors.New("network trust publication changed paired machine identity")
	}
	broker.pruneNetworkTrust()
	state := broker.networkState()
	summary := snapshot.Summary()
	previous := state.trusts[node]
	if previous.snapshot.Target == snapshot.Target && previous.summary.Digest != summary.Digest {
		return nil, errors.New("network trust digest cannot change within a worker generation")
	}
	bytes := state.trustBytes - previous.summary.Bytes + summary.Bytes
	if bytes > contract.MaxNetworkTrustCacheBytes {
		return nil, errors.New("broker public TLS trust cache exceeds its 16 MiB DER budget")
	}
	state.trusts[node] = networkTrustRecord{snapshot: snapshot, summary: summary, published: time.Now()}
	state.trustBytes = bytes
	return summary, nil
}

func (broker *Broker) readNetworkTrust(raw []byte) (any, error) {
	var request contract.NetworkTrustRequest
	if decodeFeature(raw, &request) != nil || request.Validate() != nil {
		return nil, errors.New("invalid exact network trust lookup")
	}
	broker.pruneNetworkTrust()
	state := broker.networkState()
	catalog, exists := state.catalogs[request.Target.Node]
	record, stored := state.trusts[request.Target.Node]
	if !exists || !stored || !broker.networkCurrent(catalog) || !broker.networkTrustUsable(catalog) || catalog.Target != request.Target || record.snapshot.Target != request.Target || catalog.TLSTrust == nil || catalog.TLSTrust.State != "ready" || record.summary.Digest != request.Digest {
		return nil, errors.New("exact fresh network TLS trust snapshot is unavailable")
	}
	return record.snapshot, nil
}

func (client *Client) NetworkTrust(ctx context.Context, request contract.NetworkTrustRequest) (contract.NetworkTrustSnapshot, error) {
	var snapshot contract.NetworkTrustSnapshot
	if err := request.Validate(); err != nil {
		return snapshot, err
	}
	if err := client.Call(ctx, "/v1/network-trust", request, &snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.Validate() != nil || snapshot.Target != request.Target || snapshot.Digest != request.Digest {
		return contract.NetworkTrustSnapshot{}, errors.New("network TLS trust response changed authenticated target or digest")
	}
	return snapshot, nil
}

func networkCatalogTrustSummary(catalog contract.NetworkCatalog) contract.NetworkTrustSummary {
	if catalog.TLSTrust == nil {
		return contract.NetworkTrustSummary{State: "disabled"}
	}
	return *catalog.TLSTrust
}
