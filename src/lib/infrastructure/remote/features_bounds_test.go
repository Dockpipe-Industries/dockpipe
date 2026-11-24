package remote

import (
	"context"
	"fmt"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestFeatureMutationReceiptsSurviveUntilDeadlineAndExpiredWorkRejects(t *testing.T) {
	broker, admin, _, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	operation := broker.features.state.Operations[request.ID]
	operation.Status = "success"
	operation.Result = &contract.FeatureResult{Status: "success"}
	if err := broker.storeFeature(operation); err != nil {
		t.Fatal(err)
	}
	broker.reclaimFeatures(time.Now())
	_, retained := broker.features.state.Operations[request.ID]
	broker.mutex.Unlock()
	if !retained {
		t.Fatal("valid mutation tombstone reclaimed before its deadline")
	}
	expired := request
	expired.Deadline = time.Now().Add(-time.Second)
	expired.ID, _ = expired.Digest()
	if _, err := admin.SubmitFeature(context.Background(), expired); err == nil {
		t.Fatal("expired operation admitted after reclamation")
	}
	if retried, err := admin.SubmitFeature(context.Background(), request); err != nil || retried.Status != "success" {
		t.Fatalf("terminal retry was not idempotent: %v", err)
	}
}

func TestFeatureQueueEnforcesAggregateOperationBound(t *testing.T) {
	broker, admin, _, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	broker.mutex.Lock()
	for index := 0; index < contract.MaxJobs; index++ {
		retained := request
		retained.Nonce = Hash(fmt.Sprint(index))
		retained.ID, _ = retained.Digest()
		broker.features.state.Operations[retained.ID] = contract.FeatureOperation{Request: retained, Status: "queued"}
	}
	broker.mutex.Unlock()
	if _, err := admin.SubmitFeature(context.Background(), request); err == nil {
		t.Fatal("feature queue exceeded its aggregate operation bound")
	}
}
