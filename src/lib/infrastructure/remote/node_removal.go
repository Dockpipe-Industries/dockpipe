package remote

import (
	"errors"

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
	if _, exists := b.state.Nodes[request.ID]; !exists {
		return nil, errors.New("node not found")
	}
	for _, job := range b.state.Jobs {
		if job.Node == request.ID && (job.Status == "queued" || job.Status == "running" || job.Status == "unknown") {
			return nil, errors.New("cancel or resolve active jobs before removing this machine")
		}
	}
	delete(b.state.Nodes, request.ID)
	delete(b.monitoring, request.ID)
	for code, pairing := range b.state.Pairings {
		if pairing.Node == request.ID {
			delete(b.state.Pairings, code)
		}
	}
	for id, challenge := range b.pairingChallenges {
		if challenge.Pairing.Node == request.ID {
			delete(b.pairingChallenges, id)
		}
	}
	for nonce, challenge := range b.machineChallenges {
		if challenge.Node == request.ID {
			delete(b.machineChallenges, nonce)
		}
	}
	return map[string]string{"removed": request.ID}, b.save()
}
