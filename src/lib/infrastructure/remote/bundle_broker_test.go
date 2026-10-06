package remote

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestBundleBrokerAuthorizationIntegrityAndRestart(t *testing.T) {
	root, broker, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "node")
	otherToken, _ := enroll(t, admin, "other")
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	other, _ := NewClient(server.URL, otherToken)
	defer other.HTTP.CloseIdleConnections()
	bundle := contract.Bundle{WorkflowFile: "config.yml", Files: []contract.BundleFile{{Path: "config.yml", Data: []byte("name: sent\nsteps: []\n")}}}
	digest, _ := bundle.Digest()
	request := contract.SubmitRequest{Submission: contract.Submission{ID: "sent", Node: "node", BundleHash: digest}, Bundle: &bundle}
	ctx := context.Background()
	for range 2 {
		if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
			t.Fatal(err)
		}
	}
	var job contract.Job
	session := strings.Repeat("a", 64)
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil {
		t.Fatal(err)
	}
	message := map[string]string{"id": job.ID, "session": session}
	if err := other.Call(ctx, "/v1/bundle", message, nil); err == nil {
		t.Fatal("another worker downloaded delivery")
	}
	var received contract.Bundle
	if err := worker.Call(ctx, "/v1/bundle", message, &received); err != nil {
		t.Fatal(err)
	}
	if got, _ := received.Digest(); got != digest {
		t.Fatal("delivery changed")
	}
	restarted, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.state.Jobs[job.ID].Status != "unknown" {
		t.Fatal("restart requeued delivery")
	}
	if _, err := restarted.loadBundle(job); err != nil {
		t.Fatal(err)
	}
	bundle.Files[0].Data = []byte("changed")
	request.BundleHash, _ = bundle.Digest()
	if err := admin.Call(ctx, "/v1/submit", request, nil); err == nil {
		t.Fatal("same ID accepted changed source")
	}
	if err := WritePrivate(filepath.Join(root, "bundles", job.ID+".json"), bundle); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.loadBundle(job); err == nil {
		t.Fatal("corrupted storage accepted")
	}
}
