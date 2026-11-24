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

func TestExpiredFeatureWriteStatusReturnsRetainedReceiptWithoutReadmittingWork(t *testing.T) {
	broker, admin, _, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	request.Deadline = time.Now().Add(-time.Second)
	request.ID, _ = request.Digest()
	result := &contract.FeatureResult{Status: "cancelled"}
	broker.mutex.Lock()
	err := broker.storeFeature(contract.FeatureOperation{
		Request: request, Status: "cancelled", Result: result, CancelRequested: true,
	})
	broker.mutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	reference := request.Reference()
	retained, err := admin.FeatureStatusReference(ctx, reference, request.Action)
	if err != nil {
		t.Fatal(err)
	}
	if !retained.ReceiptOnly || retained.Status != "cancelled" || !retained.CancelRequested ||
		retained.Result == nil || retained.Result.Status != "cancelled" || retained.ResultDigest != featureResultDigest(result) ||
		retained.Request.Reference() != reference || retained.Request.Nonce != "" ||
		len(retained.Request.Payload) != 0 || len(retained.Result.Payload) != 0 {
		t.Fatalf("expected the exact redacted cancelled receipt, got %+v", retained)
	}

	if _, err := admin.FeatureStatusReference(ctx, reference, "files.read"); err == nil {
		t.Fatal("the read route disclosed retained write metadata")
	}
	wrongTarget := reference
	wrongTarget.Target.Generation = Hash("different-feature-generation")
	if _, err := admin.FeatureStatusReference(ctx, wrongTarget, request.Action); err == nil {
		t.Fatal("the write status route accepted a different target")
	}
	if _, err := admin.SubmitFeature(ctx, request); err == nil {
		t.Fatal("reading an expired receipt readmitted its original mutation")
	}
	broker.mutex.Lock()
	_, active := broker.features.state.Operations[reference.ID]
	_, admitted := broker.features.admissions[reference.ID]
	broker.mutex.Unlock()
	if active || admitted {
		t.Fatal("reading a retained receipt recreated live operation state")
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

func TestFeatureWorkerStaysAvailableAtJournalCapacity(t *testing.T) {
	for _, durableMutations := range []bool{false, true} {
		name := "acknowledged-reads"
		if durableMutations {
			name = "full-mutation-journal"
		}
		t.Run(name, func(t *testing.T) {
			broker, operator, worker, config, root, permissions, initial := featureFixture(t)
			worktree := permissions.Roots[0].Path
			if err := os.WriteFile(filepath.Join(worktree, "fixture.txt"), []byte("read marker"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			deadline := time.Now().Add(8 * time.Second)
			request := func(target contract.FeatureTarget, action string, expiry time.Time) contract.FeatureRequest {
				t.Helper()
				input := workspacefiles.Request{Schema: workspacefiles.Schema, Action: action, Path: "fixture.txt"}
				if action == "create" {
					input.Path = "unexpected-write.txt"
				}
				prepared, err := NewFeatureRequest(target, "project", "files."+action, input, expiry)
				if err != nil {
					t.Fatal(err)
				}
				return prepared
			}

			if durableMutations {
				journal := featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{}}
				result := &contract.FeatureResult{Status: "success"}
				for range contract.MaxJobs {
					prepared := request(initial.Target, "create", deadline)
					journal.Records[prepared.ID] = featureWorkerRecord{Request: prepared, Result: result}
				}
				broker.mutex.Lock()
				for id, record := range journal.Records {
					broker.features.state.Operations[id] = contract.FeatureOperation{Request: record.Request, Status: "success", Result: result}
				}
				broker.mutex.Unlock()
				if err := writeFeatureJournal(root, journal); err != nil {
					t.Fatal(err)
				}
			}

			done := make(chan struct{})
			var workerErr error
			go func() {
				defer close(done)
				workerErr = RunFeatureWorker(ctx, root, config, worker)
			}()
			t.Cleanup(func() { stop(); <-done })
			wait := func(ready func() bool) {
				t.Helper()
				for !ready() {
					select {
					case <-done:
						t.Fatalf("feature worker stopped: %v", workerErr)
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					case <-time.After(20 * time.Millisecond):
					}
				}
			}
			var target contract.FeatureTarget
			wait(func() bool {
				catalogs, err := operator.FeatureCatalog(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if len(catalogs) != 1 || catalogs[0].Target == initial.Target {
					return false
				}
				target = catalogs[0].Target
				return true
			})

			submit := func(prepared contract.FeatureRequest) {
				t.Helper()
				if _, err := operator.SubmitFeature(ctx, prepared); err != nil {
					t.Fatal(err)
				}
			}
			completed := func(prepared contract.FeatureRequest) contract.FeatureResult {
				t.Helper()
				var result *contract.FeatureResult
				wait(func() bool {
					operation, err := operator.FeatureStatus(ctx, prepared)
					if err != nil {
						t.Fatal(err)
					}
					result = operation.Result
					return result != nil
				})
				return *result
			}
			if !durableMutations {
				var last contract.FeatureRequest
				for range contract.MaxJobs {
					last = request(target, "read", deadline)
					submit(last)
				}
				if result := completed(last); result.Status != "success" {
					t.Fatalf("read batch did not complete: %+v", result)
				}
				wait(func() bool {
					broker.mutex.Lock()
					defer broker.mutex.Unlock()
					completed := 0
					for _, operation := range broker.features.state.Operations {
						if operation.Status == "success" {
							completed++
						}
					}
					return completed == contract.MaxJobs
				})
			}
			// Broker deadlines can elapse before the worker's retention window.
			wait(func() bool { return !time.Now().Before(deadline) })
			if durableMutations {
				mutation := request(target, "create", time.Now().Add(10*time.Second))
				submit(mutation)
				result := completed(mutation)
				if result.Status != "failure" || !strings.Contains(result.Error, "no native operation started") {
					t.Fatalf("full mutation journal did not reject the new write: %+v", result)
				}
				if _, err := os.Stat(filepath.Join(worktree, "unexpected-write.txt")); !os.IsNotExist(err) {
					t.Fatalf("capacity rejection affected the workspace: %v", err)
				}
			}
			read := request(target, "read", time.Now().Add(10*time.Second))
			submit(read)
			if result := completed(read); result.Status != "success" || !strings.Contains(string(result.Payload), base64.StdEncoding.EncodeToString([]byte("read marker"))) {
				t.Fatalf("worker did not serve a new read: %+v", result)
			}
		})
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
