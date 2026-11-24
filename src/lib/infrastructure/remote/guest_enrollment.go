package remote

import (
	"errors"
	"sort"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type guestGrant struct {
	Host            string    `json:"host"`
	HostTokenHash   string    `json:"host_token_hash"`
	HostKey         string    `json:"host_key"`
	Run             string    `json:"run"`
	Limit           int       `json:"limit"`
	LifetimeSeconds int       `json:"lifetime_seconds"`
	Remaining       int       `json:"remaining"`
	ExpiresAt       time.Time `json:"expires_at"`
}

func (b *Broker) validateGuestGrants() error {
	if len(b.state.GuestGrants) > 32 {
		return errors.New("stored guest grant capacity exceeded")
	}
	for id, grant := range b.state.GuestGrants {
		if !contract.ValidEdgeDigest(id) || !contract.ValidID(grant.Host) ||
			!contract.ValidID(grant.Run) || len(grant.Run) > 48 ||
			!contract.ValidEdgeDigest(grant.HostTokenHash) || remoteidentity.Fingerprint(grant.HostKey) == "" ||
			grant.Limit < 1 || grant.Limit > 8 || grant.Remaining < 0 || grant.Remaining > grant.Limit ||
			grant.LifetimeSeconds < 60 || grant.LifetimeSeconds > 3600 || grant.ExpiresAt.IsZero() {
			return errors.New("invalid stored guest grant")
		}
	}
	for _, enrollment := range b.state.Nodes {
		if enrollment.GuestGrant != "" && (!contract.ValidEdgeDigest(enrollment.GuestGrant) ||
			!contract.ValidEdgeDigest(enrollment.TokenHash) || remoteidentity.Fingerprint(enrollment.MachineKey) == "") {
			return errors.New("invalid stored disposable guest identity")
		}
	}
	return nil
}

func (b *Broker) guestGrantHostMatches(grant guestGrant) bool {
	host, exists := b.state.Nodes[grant.Host]
	return exists && !host.Revoked && host.GuestGrant == "" &&
		host.MachineKey == grant.HostKey && host.TokenHash == grant.HostTokenHash
}

func (b *Broker) guestGrantActive(grant guestGrant, now time.Time) bool {
	return now.Before(grant.ExpiresAt) && b.guestGrantHostMatches(grant)
}

func (b *Broker) enrollmentActive(enrollment Enrollment, now time.Time) bool {
	if enrollment.Revoked {
		return false
	}
	if enrollment.GuestGrant == "" {
		return true
	}
	grant, exists := b.state.GuestGrants[enrollment.GuestGrant]
	return exists && b.guestGrantActive(grant, now)
}

func (b *Broker) manageGuestGrant(path string, raw []byte) (any, error) {
	if path == "/v1/guest-grant-revoke" {
		return b.releaseGuestGrant("", raw)
	}

	var request contract.GuestGrantRequest
	if err := Decode(raw, &request); err != nil || !contract.ValidID(request.Run) || len(request.Run) > 48 ||
		request.Guests < 1 || request.Guests > 8 || request.LifetimeSeconds < 60 || request.LifetimeSeconds > 3600 {
		return nil, errors.New("invalid guest grant limits")
	}
	host, exists := b.state.Nodes[request.Host]
	if !exists || host.Revoked || host.GuestGrant != "" || !contract.ValidEdgeDigest(host.TokenHash) ||
		remoteidentity.Fingerprint(host.MachineKey) == "" {
		return nil, errors.New("guest grant requires a paired host identity")
	}

	now := time.Now().UTC()
	reclaimable := make(map[string]struct{})
	for id, grant := range b.state.GuestGrants {
		if !b.guestGrantActive(grant, now) {
			reclaimable[id] = struct{}{}
			continue
		}
		if grant.Run == request.Run {
			if grant.Host != request.Host || grant.Limit != request.Guests || grant.LifetimeSeconds != request.LifetimeSeconds {
				return nil, errors.New("run already binds a different guest grant")
			}
			return b.guestGrantReceipt(id, grant), nil
		}
	}

	for node, enrollment := range b.state.Nodes {
		if _, candidate := reclaimable[enrollment.GuestGrant]; candidate && b.nodeHasUnresolvedJobs(node) {
			delete(reclaimable, enrollment.GuestGrant)
		}
	}
	if len(b.state.GuestGrants)-len(reclaimable) >= 32 {
		return nil, errors.New("guest grant capacity reached")
	}
	id, err := Secret()
	if err != nil {
		return nil, err
	}

	grant := guestGrant{
		Host: request.Host, HostTokenHash: host.TokenHash, HostKey: host.MachineKey,
		Run: request.Run, Remaining: request.Guests, Limit: request.Guests, LifetimeSeconds: request.LifetimeSeconds,
		ExpiresAt: now.Add(time.Duration(request.LifetimeSeconds) * time.Second),
	}
	if b.state.GuestGrants == nil {
		b.state.GuestGrants = make(map[string]guestGrant)
	}
	for node, enrollment := range b.state.Nodes {
		_, grantExists := b.state.GuestGrants[enrollment.GuestGrant]
		_, reclaim := reclaimable[enrollment.GuestGrant]
		// Explicit release deletes the grant immediately, retaining enrollments
		// with live holds. Reclaim them after their last hold is resolved.
		if reclaim || (enrollment.GuestGrant != "" && !grantExists && !b.nodeHasUnresolvedJobs(node)) {
			b.forgetNode(node)
		}
	}
	for id := range reclaimable {
		delete(b.state.GuestGrants, id)
	}
	b.state.GuestGrants[id] = grant
	return b.guestGrantReceipt(id, grant), b.save()
}

func (b *Broker) guestGrantReceipt(id string, grant guestGrant) contract.GuestGrantReceipt {
	return contract.GuestGrantReceipt{
		ID: id, Host: grant.Host, Run: grant.Run, Guests: grant.Limit,
		ExpiresAt: grant.ExpiresAt, BrokerKey: b.state.Identity.PublicKey,
	}
}

func (b *Broker) machineOperation(node string, enrollment Enrollment, path string, raw []byte) (any, error) {
	if !b.enrollmentActive(enrollment, time.Now()) {
		return nil, errors.New("machine enrollment is inactive")
	}
	if path == "/v1/job-log-publish" {
		return nil, b.publishLiveLog(node, raw)
	}
	if enrollment.GuestGrant != "" {
		switch path {
		case "/v1/next", "/v1/pulse", "/v1/bundle", "/v1/result":
			return b.worker(node, path, raw)
		case "/v1/features-next":
			return b.featureWorker(node, path, raw)
		default:
			return nil, errors.New("operation is outside disposable guest workflow authority")
		}
	}
	if path == "/v1/guest-enroll" {
		return b.enrollGuest(node, raw)
	}
	if path == "/v1/guest-release" {
		return b.releaseGuestGrant(node, raw)
	}
	return b.worker(node, path, raw)
}

func (b *Broker) releaseGuestGrant(host string, raw []byte) (any, error) {
	var request struct {
		ID string `json:"id"`
	}
	if err := Decode(raw, &request); err != nil || !contract.ValidEdgeDigest(request.ID) {
		return nil, errors.New("invalid guest grant identity")
	}
	grant, exists := b.state.GuestGrants[request.ID]
	if exists && host != "" && (grant.Host != host || !b.guestGrantHostMatches(grant)) {
		return nil, errors.New("guest grant belongs to another host")
	}
	receipt := contract.GuestReleaseReceipt{Grant: request.ID, Removed: []string{}, Retained: []string{}}
	if !exists {
		return receipt, nil
	}
	delete(b.state.GuestGrants, request.ID)
	for node, enrollment := range b.state.Nodes {
		if enrollment.GuestGrant != request.ID {
			continue
		}
		if b.nodeHasUnresolvedJobs(node) {
			receipt.Retained = append(receipt.Retained, node)
			continue
		}
		b.forgetNode(node)
		receipt.Removed = append(receipt.Removed, node)
	}
	sort.Strings(receipt.Removed)
	sort.Strings(receipt.Retained)
	return receipt, b.save()
}

func (b *Broker) enrollGuest(host string, raw []byte) (any, error) {
	var request contract.GuestEnrollmentRequest
	if err := Decode(raw, &request); err != nil || !contract.ValidID(request.Node) ||
		!contract.ValidEdgeDigest(request.Grant) || !contract.ValidEdgeDigest(request.TokenHash) ||
		!remoteidentity.Verify(request.PublicKey, request.ProofMessage(b.state.Identity.PublicKey), request.Proof) {
		return nil, errors.New("invalid guest identity proof")
	}

	grant, exists := b.state.GuestGrants[request.Grant]
	if !exists || grant.Host != host || !b.guestGrantActive(grant, time.Now()) ||
		!strings.HasPrefix(request.Node, grant.Run+"-") {
		return nil, errors.New("guest enrollment is outside the host grant")
	}
	receipt := contract.GuestEnrollmentReceipt{Node: request.Node, Grant: request.Grant, ExpiresAt: grant.ExpiresAt}
	if existing, exists := b.state.Nodes[request.Node]; exists {
		if !existing.Revoked && existing.GuestGrant == request.Grant &&
			existing.MachineKey == request.PublicKey && existing.TokenHash == request.TokenHash {
			return receipt, nil
		}
		return nil, errors.New("guest node already belongs to another enrollment")
	}
	if grant.Remaining == 0 || len(b.state.Nodes) >= 64 {
		return nil, errors.New("guest enrollment capacity reached")
	}
	for _, existing := range b.state.Nodes {
		if existing.TokenHash == request.TokenHash || existing.MachineKey == request.PublicKey {
			return nil, errors.New("guest requires a distinct installation identity")
		}
	}

	grant.Remaining--
	b.state.GuestGrants[request.Grant] = grant
	b.state.Nodes[request.Node] = Enrollment{
		TokenHash: request.TokenHash, MachineKey: request.PublicKey, GuestGrant: request.Grant,
	}
	return receipt, b.save()
}
