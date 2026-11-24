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
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"dockpipe/src/lib/infrastructure/workspacefiles"
)

func featureFixture(t *testing.T) (*Broker, *Client, *Client, contract.WorkerConfig, string, FeaturePermissions, contract.FeatureCatalog) {
	t.Helper()
	_, broker, admin, server := testBroker(t)
	identity := remoteidentityMustNew(t)
	token, _ := Secret()
	config := contract.WorkerConfig{Schema: contract.Version, Features: true, Node: "feature-worker", Endpoint: server.URL,
		Token: token, MachineKey: identity.PublicKey, MachinePrivateKey: identity.PrivateKey, BrokerKey: broker.state.Identity.PublicKey}
	broker.mutex.Lock()
	broker.state.Nodes[config.Node] = Enrollment{TokenHash: Hash(token), MachineKey: identity.PublicKey}
	if err := broker.save(); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Unlock()
	worker, err := NewWorkerClient(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(worker.HTTP.CloseIdleConnections)
	root := privateTestDirectory(t)
	permissions := FeaturePermissions{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{{
		FeatureRoot: contract.FeatureRoot{ID: "project", Name: "Project", FilesRead: true, FilesWrite: true, GitRead: true}, Path: t.TempDir(),
	}}}
	if err := WritePrivate(filepath.Join(root, "features.json"), permissions); err != nil {
		t.Fatal(err)
	}
	catalog := permissions.catalog(contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(identity.PublicKey), Generation: strings.Repeat("a", 64)})
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	return broker, admin, worker, config, root, permissions, catalog
}

func fileFeatureRequest(t *testing.T, catalog contract.FeatureCatalog, action string) contract.FeatureRequest {
	t.Helper()
	input := workspacefiles.Request{Schema: workspacefiles.Schema, Action: action, Path: "fixture.txt"}
	if action == "create" {
		input.Content = []byte("original")
	}
	request, err := NewFeatureRequest(catalog.Target, "project", "files."+action, input, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestFeatureAuthenticationScopesTargetsAndReplay(t *testing.T) {
	broker, admin, worker, config, _, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if err := admin.Call(context.Background(), "/v1/features-read-submit", request, nil); err == nil {
		t.Fatal("write admitted under read scope")
	}
	operation, err := admin.SubmitFeature(context.Background(), request)
	if err != nil || operation.Status != "queued" {
		t.Fatalf("submit: %+v, %v", operation, err)
	}
	if err := admin.Call(context.Background(), "/v1/features-read-status", request.Reference(), nil); err == nil {
		t.Fatal("read status renewed write authorization")
	}
	wrong := request.Reference()
	wrong.Target.Generation = strings.Repeat("b", 64)
	if err := admin.CancelFeature(context.Background(), wrong); err == nil {
		t.Fatal("cancellation targeted a different worker lifetime")
	}
	changed := request
	changed.Payload = json.RawMessage(`{"schema":"dockpipe.files/v1","action":"create","path":"other","content":"eA=="}`)
	if _, err := admin.SubmitFeature(context.Background(), changed); err == nil {
		t.Fatal("operation ID accepted changed parameters")
	}
	legacy, err := NewClient(config.Endpoint, config.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.HTTP.CloseIdleConnections()
	if err := legacy.Call(context.Background(), "/v1/features-next", catalog, nil); err == nil {
		t.Fatal("bearer-only feature request accepted")
	}
	var assignment *contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &assignment); err != nil || assignment == nil {
		t.Fatalf("assignment: %v", err)
	}
	if assignment.Request.ID != request.ID || assignment.LeaseMillis <= 0 || !validMachineNonce(assignment.Operator) {
		t.Fatal("assignment lacks exact request/authority")
	}
	var duplicate *contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, &duplicate); err != nil || duplicate == nil || duplicate.Request.ID != request.ID {
		t.Fatalf("lost assignment response is not recoverable: %v", err)
	}
	broker.security.Lock()
	var pulse contract.FeaturePulse
	if err := worker.Call(context.Background(), "/v1/features-pulse", request.Reference(), &pulse); err != nil || !pulse.Cancel {
		t.Fatalf("locked authority still leased work: %+v, %v", pulse, err)
	}
}

func TestFeatureCatalogGrantGenerationAndStrictPayload(t *testing.T) {
	_, admin, worker, _, _, permissions, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	permissions.Roots[0].Path = filepath.Join(permissions.Roots[0].Path, "remapped")
	changed := permissions.catalog(catalog.Target)
	changed.Target.Generation = Hash("changed-root-map" + permissionFingerprint(permissions))
	var assignment *contract.FeatureAssignment
	if err := worker.Call(context.Background(), "/v1/features-next", changed, &assignment); err != nil || assignment != nil {
		t.Fatalf("old root ID reinterpreted under new generation: %v", err)
	}
	operation, err := admin.FeatureStatus(context.Background(), request)
	if err != nil || operation.Status != "cancelled" {
		t.Fatalf("stale queued operation not cancelled: %+v %v", operation, err)
	}
	for _, input := range []json.RawMessage{
		json.RawMessage(`{"action":"status","workdir":"/caller-selected"}`),
		json.RawMessage(`{"action":"status","unknown":true}`),
		json.RawMessage(`{"action":"diff","action":"status"}`),
	} {
		if _, err := NewFeatureRequest(changed.Target, "project", "git.status", input, time.Now().Add(time.Minute)); err == nil {
			t.Fatal("Git payload bypassed typed/root boundary")
		}
	}
	if _, err := NewFeatureRequest(changed.Target, "", "terminal.open", json.RawMessage(`{}`), time.Now().Add(time.Minute)); err == nil {
		t.Fatal("unsupported terminal capability exposed")
	}
}

func TestFeatureWorkerFilesEndToEndAndExactRetry(t *testing.T) {
	_, admin, worker, config, root, permissions, _ := featureFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunFeatureWorker(ctx, root, config, worker) }()
	t.Cleanup(func() { cancel(); <-done })
	var catalog contract.FeatureCatalog
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		catalogs, err := admin.FeatureCatalog(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(catalogs) == 1 && catalogs[0].Target.Generation != strings.Repeat("a", 64) {
			catalog = catalogs[0]
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if catalog.Schema == "" {
		t.Fatal("worker never advertised its grants")
	}
	request := fileFeatureRequest(t, catalog, "create")
	if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var operation contract.FeatureOperation
	for time.Now().Before(deadline) {
		var err error
		operation, err = admin.FeatureStatus(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Result != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if operation.Status != "success" {
		t.Fatalf("feature did not complete: %+v", operation)
	}
	path := filepath.Join(permissions.Roots[0].Path, "fixture.txt")
	if err := os.WriteFile(path, []byte("operator changed after completion"), 0600); err != nil {
		t.Fatal(err)
	}
	retried, err := admin.SubmitFeature(context.Background(), request)
	if err != nil || retried.Status != "success" {
		t.Fatalf("exact retry: %+v %v", retried, err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "operator changed after completion" {
		t.Fatal("exact retry reran a completed mutation")
	}

	stale, err := NewFeatureRequest(catalog.Target, "project", "files.write", workspacefiles.Request{
		Schema: workspacefiles.Schema, Action: "write", Path: "fixture.txt",
		Revision: Hash("original"), Content: []byte("stale editor save"),
	}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.SubmitFeature(ctx, stale); err != nil {
		t.Fatal(err)
	}
	var rejected contract.FeatureOperation
	for time.Now().Before(deadline) {
		rejected, err = admin.FeatureStatus(ctx, stale)
		if err != nil {
			t.Fatal(err)
		}
		if rejected.Result != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if rejected.Status != "failure" || rejected.Result == nil || rejected.Result.Error != workspacefiles.ErrChanged.Error() {
		t.Fatalf("stale save did not report the revision conflict: %+v", rejected.Result)
	}
	content, err = os.ReadFile(path)
	if err != nil || string(content) != "operator changed after completion" {
		t.Fatal("stale save changed the worker's latest bytes")
	}

	journal, err := readFeatureJournal(root)
	if err != nil || journal.Records[request.ID].Result == nil {
		t.Fatalf("result not durable before acknowledgement: %v", err)
	}
}

func TestFeatureJournalCrashNeverReexecutesIntent(t *testing.T) {
	_, _, _, _, root, _, catalog := featureFixture(t)
	request := fileFeatureRequest(t, catalog, "create")
	journal := featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{request.ID: {Request: request}}}
	if err := writeFeatureJournal(root, journal); err != nil {
		t.Fatal(err)
	}
	recovered, err := readFeatureJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Records[request.ID].Result == nil || recovered.Records[request.ID].Result.Status != "unknown" {
		t.Fatal("interrupted mutation became executable")
	}
}
