package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestWorkflowOnlyDiscoveryRetainsOlderBrokerCatalog(t *testing.T) {
	peer := contract.FeatureCatalog{
		Schema: contract.FeatureVersion,
		Target: contract.FeatureTarget{Node: "older-worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Schema string `json:"schema"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || request.Schema != contract.FeatureVersion {
			http.Error(w, "remote operation rejected", http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode([]contract.FeatureCatalog{peer})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	peers, err := client.FeatureCatalog(context.Background())
	if err != nil || len(peers) != 1 || peers[0].Target.Node != "older-worker" || peers[0].WorkflowOnly {
		t.Fatalf("older broker catalog was lost: %v %v", peers, err)
	}
}

func TestWorkflowOnlyTargetPublishesWithoutFeatureGrants(t *testing.T) {
	broker, admin, worker, config, root, _, _ := featureFixture(t)
	config.Features = false
	config.Delivery = &contract.DeliveryPermission{TimeoutSeconds: 60}
	instance := strings.Repeat("b", 64)
	target := workflowOnlyTarget(config, instance)
	catalog := contract.FeatureCatalog{Schema: contract.FeatureVersion, Target: target, WorkflowOnly: true}
	ctx := context.Background()

	mixed := catalog
	mixed.DockerRead = true
	if err := worker.Call(ctx, "/v1/features-next", mixed, nil); err == nil {
		t.Fatal("workflow-only publication accepted Docker authority")
	}
	if err := worker.Call(ctx, "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	var legacy []contract.FeatureCatalog
	if err := admin.Call(ctx, "/v1/features-catalog", map[string]string{"schema": contract.FeatureVersion}, &legacy); err != nil || len(legacy) != 0 {
		t.Fatalf("legacy catalog exposed workflow-only peers: %v %v", legacy, err)
	}
	peers, err := admin.FeatureCatalog(ctx)
	if err != nil || len(peers) != 1 || peers[0].Target != target || !peers[0].WorkflowOnly {
		t.Fatalf("workflow-only peer was not discoverable: %v %v", peers, err)
	}
	if _, err := admin.SubmitFeature(ctx, fileFeatureRequest(t, catalog, "list")); err == nil {
		t.Fatal("workflow-only publication exposed a local file grant")
	}

	bundle := contract.Bundle{WorkflowFile: "config.yml", Files: []contract.BundleFile{{Path: "config.yml", Data: []byte("name: sent\nsteps: []\n")}}}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := contract.SubmitRequest{Submission: contract.Submission{ID: "workflow-only", Node: config.Node, BundleHash: digest, Target: target}, Bundle: &bundle}
	if err := validateWorkerDeliveryTarget(root, config, instance, request.Submission); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
		t.Fatal(err)
	}
	rooted := request
	rooted.ID = "workflow-only-root"
	rooted.Root = "project"
	if err := admin.Call(ctx, "/v1/submit", rooted, nil); err == nil {
		t.Fatal("workflow-only target accepted a root from a pre-existing feature grant")
	}
	if err := validateWorkerDeliveryTarget(root, config, instance, rooted.Submission); err == nil {
		t.Fatal("workflow-only worker accepted a root from a pre-existing feature grant")
	}
	if err := validateWorkerDeliveryTarget(root, config, "new-instance", request.Submission); err == nil {
		t.Fatal("restarted worker accepted a stale captured delivery")
	}

	broker.mutex.Lock()
	stale := broker.features.catalogs[config.Node]
	stale.ReceivedAt = time.Now().Add(-2 * contract.FeatureCatalogFreshness)
	broker.features.catalogs[config.Node] = stale
	broker.mutex.Unlock()
	peers, err = admin.FeatureCatalog(ctx)
	if err != nil || len(peers) != 0 {
		t.Fatalf("expired workflow-only peer remained visible: %v %v", peers, err)
	}
	var job *contract.Job
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": instance}, &job); err != nil || job != nil {
		t.Fatalf("stale workflow-only target acquired execution: %v %v", job, err)
	}
}

func TestCapturedDeliveryTargetAdmissionAssignmentAndPulse(t *testing.T) {
	broker, admin, worker, config, _, _, catalog := featureFixture(t)
	bundle := contract.Bundle{WorkflowFile: "config.yml", Files: []contract.BundleFile{{Path: "config.yml", Data: []byte("name: sent\nsteps: []\n")}}}
	digest, _ := bundle.Digest()
	request := contract.SubmitRequest{Submission: contract.Submission{ID: "captured", Node: config.Node, BundleHash: digest, Target: catalog.Target, Root: "project"}, Bundle: &bundle}
	ctx := context.Background()
	for _, change := range []func(*contract.SubmitRequest){
		func(value *contract.SubmitRequest) { value.Target.Generation = strings.Repeat("b", 64) },
		func(value *contract.SubmitRequest) { value.Target.Fingerprint = strings.Repeat("b", 64) },
		func(value *contract.SubmitRequest) { value.Root = "missing" },
	} {
		changed := request
		change(&changed)
		if err := admin.Call(ctx, "/v1/submit", changed, nil); err == nil {
			t.Fatal("accepted changed captured target")
		}
	}
	if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
		t.Fatal(err)
	}
	session := strings.Repeat("d", 64)
	var job contract.Job
	changed := catalog
	changed.Target.Generation = strings.Repeat("e", 64)
	if err := worker.Call(ctx, "/v1/features-next", changed, nil); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil || job.ID != "" {
		t.Fatal("assigned delivery after generation changed", err)
	}
	broker.mutex.Lock()
	status := broker.state.Jobs[request.ID].Status
	broker.mutex.Unlock()
	if status != "failure" {
		t.Fatal("stale target acquired execution ownership")
	}
	if err := worker.Call(ctx, "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	request.ID = "fresh-captured"
	if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil {
		t.Fatal(err)
	}
	if job.Target != request.Target || job.Root != request.Root {
		t.Fatal("assignment lost captured target")
	}
	if err := worker.Call(ctx, "/v1/features-next", changed, nil); err != nil {
		t.Fatal(err)
	}
	var pulse map[string]bool
	message := map[string]string{"id": request.ID, "session": session}
	if err := worker.Call(ctx, "/v1/pulse", message, &pulse); err != nil || !pulse["cancel"] {
		t.Fatalf("stale target kept execution authority: %v %v", pulse, err)
	}
	if err := worker.Call(ctx, "/v1/bundle", message, nil); err == nil {
		t.Fatal("stale target downloaded source")
	}
}

func TestCapturedDeliveryWorkerRechecksLocalGenerationAndRoot(t *testing.T) {
	_, _, _, config, root, permissions, _ := featureFixture(t)
	instance := strings.Repeat("c", 64)
	target := contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(config.MachineKey), Generation: Hash(instance + permissionFingerprint(permissions))}
	submission := contract.Submission{Node: config.Node, Target: target, Root: "project"}
	if err := validateWorkerDeliveryTarget(root, config, instance, submission); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkerDeliveryTarget(root, config, "another-instance", submission); err == nil {
		t.Fatal("worker restart retained captured execution target")
	}
	permissions.Roots = nil
	if err := WritePrivate(filepath.Join(root, "features.json"), permissions); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkerDeliveryTarget(root, config, instance, submission); err == nil {
		t.Fatal("removed root retained captured execution target")
	}
	if err := validateWorkerDeliveryTarget(root, config, instance, contract.Submission{}); err != nil {
		t.Fatal("legacy delivery compatibility changed", err)
	}
}
