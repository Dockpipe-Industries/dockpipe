package remote

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/process"
)

func TestFeatureReconciliationRequiresIdleWorkerProofAndRevokesAuthority(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	ctx := context.Background()
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(ctx, "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	operation, err := admin.ReconcileFeature(ctx, request.Reference())
	if err != nil || !operation.RecoveryRequested || !operation.CancelRequested || operation.Reconciled || !operation.ReceiptOnly || len(operation.Request.Payload) != 0 {
		t.Fatalf("operator request is not a content-free pending recovery: %+v %v", operation, err)
	}
	broker.mutex.Lock()
	_, admitted := broker.features.admissions[request.ID]
	broker.mutex.Unlock()
	if admitted {
		t.Fatal("recovery retained invocation authority")
	}
	if _, err := admin.FeatureStatus(ctx, request); err != nil {
		t.Fatal(err)
	}
	var pulse contract.FeaturePulse
	if err := worker.Call(ctx, "/v1/features-pulse", request.Reference(), &pulse); err != nil || !pulse.Cancel {
		t.Fatal("status polling renewed the revoked invocation", err)
	}
	var recovery contract.FeatureAssignment
	if err := worker.Call(ctx, "/v1/features-next", catalog, &recovery); err != nil || !recovery.Reconcile {
		t.Fatal("pending operation did not remain blocked awaiting worker proof", err)
	}
	if err := validateFileReconciliation(recovery, catalog.Target); err != nil {
		t.Fatal("broker emitted an invalid recovery assignment", err)
	}
	forgedCompletion := contract.FeatureCompletion{Reference: request.Reference(), Deadline: request.Deadline,
		ReceiptOnly: true, Reconciled: true, Result: contract.FeatureResult{Status: "unknown"}}
	if err := worker.Call(ctx, "/v1/features-result", forgedCompletion, nil); err == nil {
		t.Fatal("completion bypassed the dedicated cleanup protocol")
	}
	proof := contract.FeatureReconciliation{Reference: request.Reference(), Worker: catalog.Target}
	wrong := proof
	wrong.Worker.Generation = strings.Repeat("b", 64)
	if err := worker.Call(ctx, "/v1/features-reconciled", wrong, nil); err == nil {
		t.Fatal("a stale worker lifetime acknowledged cleanup")
	}
	wrong = proof
	wrong.Reference.Target.Generation = strings.Repeat("b", 64)
	if err := worker.Call(ctx, "/v1/features-reconciled", wrong, nil); err == nil {
		t.Fatal("cleanup changed the captured operation lifetime")
	}
	if err := worker.Call(ctx, "/v1/features-reconciled", proof, nil); err != nil {
		t.Fatal(err)
	}
	operation, err = admin.FeatureStatus(ctx, request)
	if err != nil || !operation.Reconciled || operation.Status != "unknown" || operation.Result.Status != "unknown" || len(operation.Request.Payload) != 0 || len(operation.Result.Payload) != 0 {
		t.Fatalf("cleanup changed the unknown mutation outcome: %+v %v", operation, err)
	}
	// Late results and exact resubmissions never reinterpret the acknowledgment.
	late := contract.FeatureResult{Status: "success", Payload: json.RawMessage(`{"private":"late-content"}`)}
	if err := worker.Call(ctx, "/v1/features-result", featureCompletionFor(featureWorkerRecord{Request: request, Result: &late}), nil); err != nil {
		t.Fatal(err)
	}
	operation, err = admin.SubmitFeature(ctx, request)
	if err != nil || !operation.Reconciled || operation.Status != "unknown" || !operation.ReceiptOnly {
		t.Fatal("late publication or exact retry changed recovered outcome", err)
	}
	readback := fileFeatureRequest(t, catalog, "read")
	if _, err := admin.SubmitFeature(ctx, readback); err != nil {
		t.Fatal(err)
	}
	var assignment contract.FeatureAssignment
	if err := worker.Call(ctx, "/v1/features-next", catalog, &assignment); err != nil || assignment.Reconcile || assignment.Request.ID != readback.ID {
		t.Fatal("verified cleanup did not permit a separate readback intent", err)
	}
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	retained := restarted.features.state.Operations[request.ID]
	if !retained.Reconciled || !retained.RecoveryRequested || !retained.CancelRequested || retained.Status != "unknown" || len(retained.Request.Payload) != 0 {
		t.Fatal("broker restart lost the content-free reconciliation proof")
	}
}

func TestFeatureWorkerReconcilesCrashedFileWithoutReplaying(t *testing.T) {
	broker, admin, worker, config, root, permissions, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	// Simulate the crash boundary after durable intent and possible file effects,
	// before completion. The replacement worker must acknowledge, never execute.
	path := filepath.Join(permissions.Roots[0].Path, "fixture.txt")
	if err := os.WriteFile(path, []byte("operator-readback-sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{request.ID: {Request: request}}}
	if err := writeFeatureJournal(root, journal); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ReconcileFeature(context.Background(), request.Reference()); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockFeatureWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := RunFeatureWorker(context.Background(), root, config, worker); err == nil {
		unlock()
		t.Fatal("a second lock holder acknowledged an active worker's cleanup")
	}
	unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	done := make(chan error, 1)
	go func() { done <- RunFeatureWorker(ctx, root, config, worker) }()
	t.Cleanup(func() { cancel(); <-done })
	var recovered contract.FeatureOperation
	for ctx.Err() == nil {
		var err error
		recovered, err = admin.FeatureStatus(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if recovered.Reconciled {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !recovered.Reconciled || recovered.Status != "unknown" || recovered.Request.Target != request.Target {
		t.Fatalf("replacement worker did not acknowledge the exact old intent: %+v", recovered)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "operator-readback-sentinel" {
		t.Fatal("recovery replayed the file mutation", err)
	}
	encoded, err := os.ReadFile(filepath.Join(filepath.Dir(broker.path), "feature-operations.json"))
	if err != nil || strings.Contains(string(encoded), "fixture.txt") || strings.Contains(string(encoded), "operator-readback-sentinel") {
		t.Fatal("recovery receipt retained operation content", err)
	}
}

func TestFeatureReconciliationCancelsUndispatchedProcessBackedOperations(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	catalog.Roots[0].GitWrite = true
	catalog.DockerRead = true
	catalog.DockerControl = true
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		action  string
		root    string
		payload string
	}{
		{action: "git.stage", root: "project", payload: `{"action":"stage","paths":["fixture.txt"]}`},
		{action: "containers.stop", payload: `{"container":"` + strings.Repeat("a", 64) + `"}`},
	} {
		request, err := NewFeatureRequest(catalog.Target, test.root, test.action, json.RawMessage(test.payload), time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.ReconcileFeature(context.Background(), request.Reference()); err != nil {
			t.Fatal("undispatched process-backed intent could not be cancelled", err)
		}
		broker.mutex.Lock()
		operation := broker.features.state.Operations[request.ID]
		broker.mutex.Unlock()
		if operation.RecoveryRequested || operation.Reconciled || operation.Status != "cancelled" {
			t.Fatal("undispatched recovery claimed execution proof or dispatched work")
		}
	}
}

func TestFeatureReconciliationCancelsQueuedIntentBeforeDispatch(t *testing.T) {
	_, admin, worker, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	operation, err := admin.ReconcileFeature(context.Background(), request.Reference())
	if err != nil || operation.Status != "cancelled" || !operation.CancelRequested || operation.Reconciled || !operation.ReceiptOnly {
		t.Fatal("recovery failed to cancel the undispatched intent", err)
	}
	var assignment *contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &assignment); err != nil || assignment != nil {
		t.Fatal("recovery dispatched the captured queued mutation", err)
	}
}

func TestReplacementFeatureWorkerCannotReconcileWithoutOriginalReceipt(t *testing.T) {
	_, admin, worker, config, root, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ReconcileFeature(context.Background(), request.Reference()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = RunFeatureWorker(ctx, root, config, worker)
	operation, err := admin.FeatureStatus(context.Background(), request)
	if err != nil || operation.Reconciled || operation.RecoveryError != "original-receipt-unavailable" {
		t.Fatal("unverified replacement worker cleared the mutation block", err)
	}
}

func TestFeatureReconciliationDoesNotClearAnotherUnknownOperation(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	other := fileFeatureRequest(t, catalog, "create")
	request := fileFeatureRequest(t, catalog, "create")
	for _, pending := range []contract.FeatureRequest{other, request} {
		if _, err := admin.SubmitFeature(context.Background(), pending); err != nil {
			t.Fatal(err)
		}
		broker.mutex.Lock()
		operation := broker.features.state.Operations[pending.ID]
		operation.Status = "unknown"
		operation.Result = &contract.FeatureResult{Status: "unknown"}
		err := broker.storeFeature(operation)
		broker.mutex.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := admin.ReconcileFeature(context.Background(), request.Reference()); err != nil {
		t.Fatal(err)
	}
	var assignment contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &assignment); err != nil || !assignment.Reconcile || assignment.Request.ID != request.ID {
		t.Fatal("another unknown receipt prevented the requested recovery proof", err)
	}
	proof := contract.FeatureReconciliation{Reference: request.Reference(), Worker: catalog.Target}
	if err := worker.Call(context.Background(), "/v1/features-reconciled", proof, nil); err != nil {
		t.Fatal(err)
	}
	operation, err := admin.FeatureStatus(context.Background(), other)
	if err != nil || operation.Reconciled || operation.RecoveryRequested || operation.Status != "unknown" {
		t.Fatal("cleanup of one intent cleared another uncertain operation", err)
	}
	var next *contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &next); err != nil || next != nil {
		t.Fatal("another uncertain mutation failed to keep the worker blocked", err)
	}
}

func TestFileReconciliationAssignmentCannotCarryExecutionAuthority(t *testing.T) {
	_, _, _, _, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	operation := featureMetadataOperation(contract.FeatureOperation{Request: request, Status: "running"})
	valid := contract.FeatureAssignment{Request: operation.Request, Reconcile: true}
	for _, mutate := range []func(*contract.FeatureAssignment){
		func(value *contract.FeatureAssignment) { value.Request.Payload = json.RawMessage(`{}`) },
		func(value *contract.FeatureAssignment) { value.Request.Nonce = request.Nonce },
		func(value *contract.FeatureAssignment) { value.LeaseMillis = 1 },
		func(value *contract.FeatureAssignment) { value.Operator = Hash("operator") },
		func(value *contract.FeatureAssignment) { value.Request.Action = "terminal.open" },
		func(value *contract.FeatureAssignment) { value.Request.Target.Fingerprint = Hash("another-machine") },
	} {
		changed := valid
		mutate(&changed)
		if err := validateFileReconciliation(changed, catalog.Target); err == nil {
			t.Fatal("accepted recovery input with execution authority or changed identity")
		}
	}
}

func priorHostBootTestScope(t *testing.T, scope process.ScopeReference) process.ScopeReference {
	t.Helper()
	switch scope.BootSequence {
	case 0:
		scope.Boot = "00000000-0000-0000-0000-000000000000"
	case 1:
		t.Skip("native kernel has no prior nonzero boot sequence")
	default:
		scope.BootSequence--
	}
	return scope
}

func TestGitRecoveryUsesCapturedBootProofAndPreservesUnknownRepositoryState(t *testing.T) {
	for _, scenario := range []string{"same-boot", "prior-boot", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			_, admin, worker, config, root, permissions, catalog := featureFixture(t)
			permissions.ProcessRecovery = "host-reboot"
			permissions.Roots[0].GitWrite = true
			catalog.Roots[0].GitWrite = true
			if err := WritePrivate(filepath.Join(root, "features.json"), permissions); err != nil {
				t.Fatal(err)
			}
			if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
				t.Fatal(err)
			}
			request, err := NewFeatureRequest(catalog.Target, "project", "git.stage", json.RawMessage(`{"action":"stage","paths":["private-selected-path"]}`), time.Now().Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
				t.Fatal(err)
			}
			scope, err := process.CaptureHostBootScope()
			if err != nil {
				t.Skip("native kernel boot identity unavailable")
			}
			if scenario == "prior-boot" {
				scope = priorHostBootTestScope(t, scope)
			}
			result := recoveredFeatureResult("unknown")
			record := featureWorkerRecord{Request: request, Result: &result, ReceiptOnly: true, Supervised: scenario != "legacy"}
			if record.Supervised {
				record.Scope = &scope
			}
			if err := writeFeatureJournal(root, featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{request.ID: record}}); err != nil {
				t.Fatal(err)
			}
			if err := worker.Call(context.Background(), "/v1/features-result", featureCompletionFor(record), nil); err != nil {
				t.Fatal(err)
			}
			metadata := filepath.Join(permissions.Roots[0].Path, ".git")
			if err := os.Mkdir(metadata, 0700); err != nil {
				t.Fatal(err)
			}
			lock := filepath.Join(metadata, "index.lock")
			if err := os.WriteFile(lock, []byte("unowned-lock-sentinel"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := admin.ReconcileFeature(context.Background(), request.Reference()); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			done := make(chan error, 1)
			go func() { done <- RunFeatureWorker(ctx, root, config, worker) }()
			defer func() { cancel(); <-done }()
			var operation contract.FeatureOperation
			for ctx.Err() == nil {
				operation, err = admin.FeatureStatus(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				if operation.Reconciled || operation.RecoveryError != "" {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if operation.Status != "unknown" || operation.Request.Target != request.Target {
				t.Fatal("Git cleanup changed the original target or claimed mutation success")
			}
			if scenario == "prior-boot" && !operation.Reconciled || scenario == "same-boot" && operation.RecoveryError != "worker-reboot-required" ||
				scenario == "legacy" && operation.RecoveryError != "legacy-process-scope-unavailable" {
				t.Fatalf("boot-fenced recovery used the wrong evidence: %+v", operation)
			}
			content, err := os.ReadFile(lock)
			if err != nil || string(content) != "unowned-lock-sentinel" {
				t.Fatal("Git recovery removed or changed an unowned repository lock", err)
			}
		})
	}
}

func TestContainerRetirementRequiresSeparateGrantExactReviewAndLiveAuthority(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	catalog.DockerRead = true
	catalog.DockerControl = true
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	resource := strings.Repeat("a", 64)
	request, err := NewFeatureRequest(catalog.Target, "", "containers.restart", map[string]string{"container": resource}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ReconcileFeatureRecovery(context.Background(), request.Reference(), "retire-container", resource); err == nil {
		t.Fatal("DockerControl implicitly granted container removal")
	}
	catalog.DockerRecovery = true
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ReconcileFeatureRecovery(context.Background(), request.Reference(), "retire-container", strings.Repeat("b", 64)); err == nil {
		t.Fatal("retirement changed the operator-reviewed immutable container")
	}
	operation, err := admin.ReconcileFeatureRecovery(context.Background(), request.Reference(), "retire-container", resource)
	if err != nil || !operation.RecoveryRequested || operation.Reconciled || operation.RecoveryResource != resource {
		t.Fatal("explicit retirement request lost exact metadata or invented cleanup", err)
	}
	broker.security.Lock()
	var pulse contract.FeaturePulse
	if err := worker.Call(context.Background(), "/v1/features-recovery-pulse", request.Reference(), &pulse); err != nil || !pulse.Cancel {
		t.Fatal("container retirement retained authority after operator lock", err)
	}
	proof := contract.FeatureReconciliation{Reference: request.Reference(), Worker: catalog.Target, Resource: resource}
	if err := worker.Call(context.Background(), "/v1/features-reconciled", proof, nil); err == nil {
		t.Fatal("locked retirement released the original operation block")
	}
}

func TestWorkerProcessRecoveryPolicyPersistsAndChangesGeneration(t *testing.T) {
	_, _, _, _, root, permissions, catalog := featureFixture(t)
	before := permissionFingerprint(permissions)
	permissions.ProcessRecovery = "host-reboot"
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeaturePermissions(root)
	if err != nil || loaded.ProcessRecovery != "host-reboot" || permissionFingerprint(loaded) == before || loaded.catalog(catalog.Target).ProcessRecovery != "host-reboot" {
		t.Fatal("worker recovery policy was not durable, catalog-visible, and generation-bound", err)
	}
	loaded.ProcessRecovery = "assume-clean"
	if err := SaveFeaturePermissions(root, loaded); err == nil {
		t.Fatal("an unrecognized recovery policy reached worker configuration")
	}
}
