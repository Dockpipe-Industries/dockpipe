package remote

import (
	"context"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestRemoveNodeDeletesEnrollmentAndPreservesHistory(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "paired", true: "revoked"}[revoked], func(t *testing.T) {
			root, broker, admin, server := testBroker(t)
			token, _ := enroll(t, admin, "old-machine")
			enrollment := broker.state.Nodes["old-machine"]
			enrollment.Revoked = revoked
			broker.state.Nodes["old-machine"] = enrollment
			broker.state.Pairings["old-code"] = PairingSession{PairingStatus: contract.PairingStatus{Node: "old-machine", Status: "approved"}}
			broker.pairingChallenges = map[string]hardwarePairingChallenge{"old": {Pairing: broker.state.Pairings["old-code"]}}
			broker.machineChallenges = map[string]machineChallenge{"old": {Node: "old-machine"}}
			broker.state.Jobs["history"] = contract.Job{Submission: contract.Submission{ID: "history", Node: "old-machine", Profile: "bench"}, Status: "cancelled"}
			worker, err := NewClient(server.URL, token)
			if err != nil {
				t.Fatal(err)
			}
			defer worker.HTTP.CloseIdleConnections()
			request := map[string]string{"id": "old-machine"}
			if err := worker.Call(context.Background(), "/v1/remove", request, nil); err == nil {
				t.Fatal("worker could delete enrollment")
			}
			if err := admin.Call(context.Background(), "/v1/remove", request, nil); err != nil {
				t.Fatal(err)
			}
			if len(broker.state.Nodes) != 0 || len(broker.state.Pairings) != 0 || len(broker.pairingChallenges) != 0 || len(broker.machineChallenges) != 0 {
				t.Fatal("machine enrollment or pairing state remained")
			}
			if err := worker.Call(context.Background(), "/v1/next", struct{}{}, nil); err == nil {
				t.Fatal("removed credential retained access")
			}
			reopened, err := NewBroker(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(reopened.state.Nodes) != 0 || len(reopened.state.Pairings) != 0 || reopened.state.Jobs["history"].Node != "old-machine" {
				t.Fatal("removal was not persisted or history was lost")
			}
		})
	}
}

func TestRemoveNodeRejectsActiveWork(t *testing.T) {
	for _, state := range []string{"queued", "running", "unknown"} {
		t.Run(state, func(t *testing.T) {
			_, broker, admin, _ := testBroker(t)
			enroll(t, admin, "busy-machine")
			broker.state.Jobs["active"] = contract.Job{Submission: contract.Submission{ID: "active", Node: "busy-machine", Profile: "bench"}, Status: state}
			if err := admin.Call(context.Background(), "/v1/remove", map[string]string{"id": "busy-machine"}, nil); err == nil {
				t.Fatal("active work did not block removal")
			}
			if _, exists := broker.state.Nodes["busy-machine"]; !exists {
				t.Fatal("failed removal changed enrollment")
			}
		})
	}
}
