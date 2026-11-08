package remote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/workspacefiles"
)

func TestFeatureReceiptsNeverRetainFileContentsOrPaths(t *testing.T) {
	broker, admin, worker, _, root, _, catalog := featureFixture(t)
	secret := "private-content-sentinel-only-in-live-memory"
	privatePath := "private-path-sentinel.env"
	request, err := NewFeatureRequest(catalog.Target, "project", "files.create", workspacefiles.Request{
		Schema: workspacefiles.Schema, Action: "create", Path: privatePath, Content: []byte(secret),
	}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var assignment contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &assignment); err != nil {
		t.Fatal(err)
	}
	result := contract.FeatureResult{Status: "success", Payload: json.RawMessage(`{"content":"` + secret + `","path":"` + privatePath + `"}`)}
	record := featureWorkerRecord{Request: request, Result: &result}
	if err := writeFeatureJournal(root, featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{request.ID: record}}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/features-result", featureCompletionFor(record), nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(broker.path), "feature-operations.json"), filepath.Join(root, "feature-worker.json")} {
		encoded, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, sentinel := range []string{secret, privatePath, base64.StdEncoding.EncodeToString([]byte(secret))} {
			if strings.Contains(string(encoded), sentinel) {
				t.Fatalf("private content retained in %s", filepath.Base(path))
			}
		}
	}
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	retained := restarted.features.state.Operations[request.ID]
	if !retained.ReceiptOnly || retained.Status != "success" || len(retained.Request.Payload) != 0 || len(retained.Result.Payload) != 0 {
		t.Fatal("broker restart retained plaintext or lost terminal replay protection")
	}
	journal, err := readFeatureJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	if !journal.Records[request.ID].ReceiptOnly || journal.Records[request.ID].Result.Status != "success" {
		t.Fatal("worker receipt cannot reconcile completion")
	}
	if err := worker.Call(context.Background(), "/v1/features-result", featureCompletionFor(journal.Records[request.ID]), nil); err != nil {
		t.Fatalf("metadata-only recovery was not idempotent: %v", err)
	}
}

func TestReadFeaturesDoNotWriteDurableJournals(t *testing.T) {
	broker, admin, worker, _, root, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "read")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var assignment contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &assignment); err != nil {
		t.Fatal(err)
	}
	result := contract.FeatureResult{Status: "success", Payload: json.RawMessage(`{"content":"sensitive-read-result"}`)}
	record := featureWorkerRecord{Request: request, Result: &result}
	if err := writeFeatureJournal(root, featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{request.ID: record}}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/features-result", featureCompletionFor(record), nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(broker.path), "feature-operations.json"), filepath.Join(root, "feature-worker.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("read invocation created a durable journal: %s", path)
		}
	}
}

func TestFeatureBrokerRestartDoesNotDispatchMutationAgain(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	operation := restarted.features.state.Operations[request.ID]
	if operation.Status != "unknown" || !operation.ReceiptOnly || len(operation.Request.Payload) != 0 {
		t.Fatal("broker replayed uncertain mutation after restart")
	}
}

func TestFeaturePermissionConsistencyFailsBeforePublication(t *testing.T) {
	for _, permissions := range []FeaturePermissions{
		{Schema: contract.FeatureVersion, DockerControl: true},
		{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: "root", Name: "Root", FilesWrite: true}, Path: t.TempDir()}}},
		{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: "root", Name: "Root", GitPublish: true}, Path: t.TempDir()}}},
	} {
		root := filepath.Join(t.TempDir(), "not-created")
		if err := SaveFeaturePermissions(root, permissions); err == nil {
			t.Fatal("inconsistent feature grant accepted")
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("invalid grants caused configuration filesystem effects")
		}
	}
}

func TestFeatureExpiredReceiptReconciliationDoesNotRecreateWork(t *testing.T) {
	broker, _, worker, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	request.Deadline = time.Now().Add(-time.Minute)
	request.ID, _ = request.Digest()
	result := recoveredFeatureResult("success")
	record := featureWorkerRecord{Request: request, Result: &result, ReceiptOnly: true, ResultDigest: Hash("prior-result")}
	var acknowledgement contract.FeatureResultAck
	if err := worker.Call(context.Background(), "/v1/features-result", featureCompletionFor(record), &acknowledgement); err != nil {
		t.Fatal(err)
	}
	if !acknowledgement.Expired || acknowledgement.Reference != request.Reference() {
		t.Fatal("expired receipt was not safely acknowledged")
	}
	broker.mutex.Lock()
	_, exists := broker.features.state.Operations[request.ID]
	broker.mutex.Unlock()
	if exists {
		t.Fatal("receipt reconciliation created executable work")
	}
}
