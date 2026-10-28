package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestEdgePublicHealthRequiresFreshSignatureAndVerifiedTLS(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	server := httptest.NewTLSServer(broker)
	defer server.Close()
	if err := probeEdgePublic(context.Background(), server.URL, broker.state.Identity.PublicKey, server.Client()); err != nil {
		t.Fatal(err)
	}
	other, _ := remoteidentity.New()
	if err := probeEdgePublic(context.Background(), server.URL, other.PublicKey, server.Client()); err == nil {
		t.Fatal("another broker's response accepted")
	}
	if err := probeEdgePublic(context.Background(), server.URL, broker.state.Identity.PublicKey, edgeHTTPClient()); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	fake := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"schema":"dockpipe.remote.edge-activation/v1","nonce":"stale","signature":"invalid"}`))
	}))
	defer fake.Close()
	if err := probeEdgePublic(context.Background(), fake.URL, broker.state.Identity.PublicKey, fake.Client()); err == nil {
		t.Fatal("stale health page accepted")
	}
}

func TestEdgeActivationRoutesRequireOSAuthorityAndPauseJobs(t *testing.T) {
	root, broker, admin, server := testBroker(t)
	base := contract.EdgeConfig{Schema: contract.Version, Endpoint: "https://edge.example.test", Executable: filepath.Join(root, "old-edge"), Arguments: []string{"serve"}}
	broker.edge = newEdgeSupervisor(context.Background(), root, base, func(context.Context, string) error { return nil })
	broker.edge.baseline = edgeBaseline{Config: base, Digest: Hash("recovery"), BaseDigest: contract.EdgeConfigDigest(base)}
	raw, _ := NewClient(server.URL, admin.Token)
	for _, path := range []string{"/v1/edge-plan", "/v1/edge-activate", "/v1/edge-status"} {
		if err := raw.Call(context.Background(), path, nil, nil); err == nil {
			t.Fatal("bearer bypassed OS auth", path)
		}
	}
	request := contract.EdgeActivationRequest{Schema: contract.EdgeActivationSchema, ExpectedConfigDigest: contract.EdgeConfigDigest(base), Candidate: base, Recovery: base,
		CandidateSHA256: Hash("candidate"), RecoverySHA256: Hash("recovery"), ReadinessURL: "http://127.0.0.1:45000/fixture-health", StartupSeconds: 2, StableSeconds: 2, RecoverySeconds: 30, Maintenance: "allow-connector-replacement"}
	var result contract.EdgeActivationStatus
	if err := admin.Call(context.Background(), "/v1/edge-plan", request, &result); err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	broker.mutex.Lock()
	broker.state.Jobs["active"] = contract.Job{Status: "unknown"}
	broker.mutex.Unlock()
	if err := admin.Call(context.Background(), "/v1/edge-activate", request, &result); err == nil {
		t.Fatal("unknown job allowed replacement")
	}
	broker.mutex.Lock()
	delete(broker.state.Jobs, "active")
	broker.mutex.Unlock()
	if err := admin.Call(context.Background(), "/v1/edge-activate", request, &result); err != nil || result.Status != "accepted" {
		t.Fatal(result, err)
	}
	if _, err := broker.operator("/v1/submit", []byte(`{}`)); err == nil {
		t.Fatal("submission accepted during maintenance")
	}
}
