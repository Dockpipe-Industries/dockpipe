package remote

import (
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

// removeNode forgets enrollment without deleting historical job records.
// Active work must be resolved first so a new enrollment cannot inherit it.
func (b *Broker) removeNode(raw []byte) (any, error) {
	var request struct {
		ID string `json:"id"`
	}
	if err := Decode(raw, &request); err != nil || !contract.ValidID(request.ID) {
		return nil, errors.New("invalid node identity")
	}
	enrollment, exists := b.state.Nodes[request.ID]
	if !exists {
		return nil, errors.New("node not found")
	}
	if grant, exists := b.state.GuestGrants[enrollment.GuestGrant]; exists && b.guestGrantActive(grant, time.Now()) {
		return nil, errors.New("revoke the guest grant before removing a disposable guest")
	}
	if b.nodeHasUnresolvedJobs(request.ID) {
		return nil, errors.New("cancel or resolve active jobs before removing this machine")
	}
	b.forgetNode(request.ID)
	return map[string]string{"removed": request.ID}, b.save()
}

func (b *Broker) nodeHasUnresolvedJobs(node string) bool {
	for _, job := range b.state.Jobs {
		if job.Node == node && !job.Resolved() {
			return true
		}
	}
	return false
}

func (b *Broker) forgetNode(node string) {
	if b.features != nil && b.features.catalogs[node].WorkflowOnly {
		delete(b.features.catalogs, node)
	}
	delete(b.state.Nodes, node)
	delete(b.monitoring, node)
	for code, pairing := range b.state.Pairings {
		if pairing.Node == node {
			delete(b.state.Pairings, code)
		}
	}
	for id, challenge := range b.pairingChallenges {
		if challenge.Pairing.Node == node {
			delete(b.pairingChallenges, id)
		}
	}
	for nonce, challenge := range b.machineChallenges {
		if challenge.Node == node {
			delete(b.machineChallenges, nonce)
		}
	}
}
