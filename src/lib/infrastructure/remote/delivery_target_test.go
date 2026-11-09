package remote

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

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
