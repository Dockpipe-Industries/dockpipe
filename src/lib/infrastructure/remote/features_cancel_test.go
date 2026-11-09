package remote

import (
	"context"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestFeatureCancellationReturnsExpiredQueuedOutcomeBeforeReclamation(t *testing.T) {
	broker, admin, _, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	request.Deadline = time.Now().Add(-time.Minute)
	request.ID, _ = request.Digest()
	broker.mutex.Lock()
	err := broker.storeFeature(contract.FeatureOperation{Request: request, Status: "queued"})
	broker.mutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	operation, err := admin.CancelFeatureResult(context.Background(), request.Reference())
	if err != nil || operation.Status != "cancelled" || operation.Result == nil || operation.Result.Status != "cancelled" || !operation.CancelRequested ||
		operation.Request.Reference() != request.Reference() {
		t.Fatalf("authoritative queued cancellation was discarded: %+v %v", operation, err)
	}
	if _, err := admin.FeatureStatusReference(context.Background(), request.Reference(), request.Action); err == nil {
		t.Fatal("fixture did not exercise immediate known-terminal reclamation")
	}
	if operation.Status != "cancelled" || operation.Request.Reference() != request.Reference() {
		t.Fatal("reclamation erased the already returned authoritative cancellation receipt")
	}
}
