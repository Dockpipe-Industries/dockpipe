package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func desktopFixture(t *testing.T) (*Broker, *Client, *Client, contract.WorkerConfig, string, DesktopPermissions, contract.DesktopCatalog) {
	t.Helper()
	broker, admin, worker, config, root, _, _ := featureFixture(t)
	permissions := DesktopPermissions{Schema: contract.DesktopVersion, Provider: contract.DesktopProvider{ID: "fixture.native", Version: "1", Artifact: strings.Repeat("f", 64)}, View: true, Input: true}
	if err := SaveDesktopPermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	instance, _ := Secret()
	catalog, err := NewDesktopCatalog(config, permissions, instance, true, true)
	if err != nil {
		t.Fatal(err)
	}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Catalog: catalog}
	response, err := worker.ExchangeDesktop(context.Background(), exchange)
	if err != nil {
		t.Fatal(err)
	}
	exchange.Epoch = response.Epoch
	if _, err := worker.ExchangeDesktop(context.Background(), exchange); err != nil {
		t.Fatal(err)
	}
	catalog.Epoch = response.Epoch
	return broker, admin, worker, config, root, permissions, catalog
}

func desktopIntent(t *testing.T, catalog contract.DesktopCatalog) contract.DesktopOpen {
	t.Helper()
	nonce, _ := Secret()
	request := contract.DesktopOpen{Schema: contract.DesktopVersion, Target: catalog.Target, Epoch: catalog.Epoch, Provider: catalog.Provider,
		Mode: "control", Nonce: nonce, Deadline: time.Now().UTC().Add(time.Minute)}
	request.ID, _ = request.Digest()
	return request
}

func TestDesktopAuthenticatedLifecycleSignalingAndExactCleanup(t *testing.T) {
	broker, admin, worker, _, root, _, catalog := desktopFixture(t)
	ctx := context.Background()
	request := desktopIntent(t, catalog)
	view, err := admin.OpenDesktop(ctx, request)
	if err != nil || view.State != "queued" || view.LeaseMillis <= 0 || view.LeaseMillis > contract.DesktopLease.Milliseconds() {
		t.Fatal("desktop open", err)
	}
	if _, err := admin.OpenDesktop(ctx, desktopIntent(t, catalog)); err == nil {
		t.Fatal("concurrent desktop capture/input ownership admitted")
	}
	parameters := contract.DesktopSignal{Reference: request.Reference(), Sequence: 1, Type: "parameters", Parameters: json.RawMessage(`{"temporary":"private-session-parameters"}`)}
	offer := contract.DesktopSignal{Reference: request.Reference(), Sequence: 2, Type: "offer", SDP: "v=0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"}
	for _, signal := range []contract.DesktopSignal{parameters, offer, offer} {
		if _, err := admin.SignalDesktop(ctx, signal); err != nil {
			t.Fatal("same signaling retry must preserve original negotiation", err)
		}
	}
	changed := offer
	changed.SDP += "a=changed\r\n"
	if _, err := admin.SignalDesktop(ctx, changed); err == nil {
		t.Fatal("changed signaling retry accepted")
	}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: catalog.Epoch, Catalog: catalog}
	response, err := worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(response.Directives) != 1 || len(response.Directives[0].Signals) != 2 || response.Directives[0].LeaseMillis <= 0 {
		t.Fatal("native directive lost exact negotiation/lease", err)
	}
	answer := contract.DesktopSignal{Reference: request.Reference(), Sequence: 1, Type: "answer", SDP: offer.SDP}
	exchange.Reports = []contract.DesktopReport{{Reference: request.Reference(), State: "running", Ack: 2, Signals: []contract.DesktopSignal{answer}}}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}
	view, err = admin.PulseDesktop(ctx, contract.DesktopRead{Reference: request.Reference()})
	if err != nil || view.State != "running" || view.LeaseMillis <= 0 || view.Ack != 2 || len(view.Signals) != 1 {
		t.Fatal("viewer did not receive native answer", err)
	}
	wrong := request.Reference()
	wrong.Target.Fingerprint = Hash("other-machine")
	if _, err := admin.CloseDesktop(ctx, wrong); err == nil {
		t.Fatal("foreign session reference closed the desktop")
	}
	view, err = admin.CloseDesktop(ctx, request.Reference())
	if err != nil || view.State != "closing" || view.LeaseMillis != 0 {
		t.Fatal("closure request falsely claimed native cleanup", err)
	}
	response, err = worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(response.Directives) != 1 || !response.Directives[0].Close || response.Directives[0].LeaseMillis != 0 {
		t.Fatal("native endpoint did not receive lease revocation", err)
	}
	exchange.Reports = []contract.DesktopReport{{Reference: request.Reference(), State: "closed", Ack: 2}}
	response, err = worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(response.Forget) != 1 {
		t.Fatal("native closure did not acknowledge original transport", err)
	}
	view, err = admin.DesktopStatus(ctx, contract.DesktopRead{Reference: request.Reference()})
	if err != nil || view.State != "closed" || view.LeaseMillis != 0 || len(view.Signals) != 0 {
		t.Fatal("verified closure lost its metadata receipt", err)
	}
	brokerBytes, err := os.ReadFile(broker.path)
	if err != nil || strings.Contains(string(brokerBytes), "private-session-parameters") {
		t.Fatal("ephemeral connection parameters entered broker persistence", err)
	}
	// No desktop signaling or session journal is written. Only owner policy exists.
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "desktop") && entry.Name() != "desktop.json" {
			t.Fatal("desktop negotiation entered persistent storage", entry.Name())
		}
	}
	broker.mutex.Lock()
	broker.desktops = nil
	broker.mutex.Unlock()
	if _, err := admin.OpenDesktop(ctx, request); err == nil {
		t.Fatal("old desktop opening replayed across broker epoch")
	}
}

func TestDesktopIndependentGrantsIdentityRevocationAndUnknownQuota(t *testing.T) {
	for _, scenario := range []string{"view-only", "fingerprint", "provider", "lock", "generation", "revoked", "expiry"} {
		t.Run(scenario, func(t *testing.T) {
			broker, admin, worker, config, _, _, catalog := desktopFixture(t)
			ctx := context.Background()
			request := desktopIntent(t, catalog)
			if scenario == "view-only" {
				catalog.Input = false
			}
			if scenario == "fingerprint" {
				request.Target.Fingerprint = Hash("another-machine")
			}
			if scenario == "provider" {
				request.Provider.Artifact = Hash("another-artifact")
			}
			request.ID, _ = request.Digest()
			exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: catalog.Epoch, Catalog: catalog}
			if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
				t.Fatal(err)
			}
			_, err := admin.OpenDesktop(ctx, request)
			if scenario == "view-only" || scenario == "fingerprint" || scenario == "provider" {
				if err == nil {
					t.Fatal("desktop opening exceeded independent grant/identity")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
				t.Fatal(err)
			}
			if scenario == "lock" {
				broker.security.Lock()
			}
			broker.mutex.Lock()
			record := broker.desktops.records[request.ID]
			switch scenario {
			case "generation":
				exchange.Catalog.Target.Generation = Hash("new-desktop-policy")
			case "revoked":
				enrollment := broker.state.Nodes[config.Node]
				enrollment.Revoked = true
				broker.state.Nodes[config.Node] = enrollment
			case "expiry":
				record.admission.deadline = time.Now().Add(-time.Second)
			}
			if scenario == "revoked" {
				if broker.desktopLease(record) != 0 {
					t.Fatal("revoked paired machine retained desktop lease")
				}
			}
			broker.mutex.Unlock()
			response, err := worker.ExchangeDesktop(ctx, exchange)
			if scenario == "revoked" {
				if err == nil {
					t.Fatal("revoked machine published desktop state")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "generation" && (len(response.Directives) != 1 || !response.Directives[0].Close || response.Directives[0].LeaseMillis != 0) {
				t.Fatal("lost authority did not stop native media/input admission")
			}
			broker.mutex.Lock()
			unknown := record.state == "unknown" && record.close
			broker.mutex.Unlock()
			if !unknown {
				t.Fatal("lost native cleanup proof released desktop ownership")
			}
		})
	}
}

func TestDesktopOwnerPolicyIsIndependentAndFrozen(t *testing.T) {
	_, _, _, config, root, permissions, catalog := desktopFixture(t)
	changed := permissions
	changed.Input = false
	if err := SaveDesktopPermissions(root, changed); err != nil {
		t.Fatal(err)
	}
	if CheckDesktopPolicy(root, permissions) == nil {
		t.Fatal("policy change kept the previous native admission")
	}
	instance, _ := Secret()
	if _, err := NewDesktopCatalog(config, changed, instance, true, true); err == nil {
		t.Fatal("provider readiness elevated owner desktop permissions")
	}
	next, err := NewDesktopCatalog(config, changed, instance, true, false)
	if err != nil || next.Target.Generation == catalog.Target.Generation {
		t.Fatal("desktop policy did not produce an independent generation", err)
	}
	if _, err := os.Stat(filepath.Join(root, "desktop.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopSignalBudgetsAndNoCrossEndpointReplay(t *testing.T) {
	_, admin, worker, _, _, _, catalog := desktopFixture(t)
	ctx := context.Background()
	request := desktopIntent(t, catalog)
	if _, err := admin.OpenDesktop(ctx, request); err != nil {
		t.Fatal(err)
	}
	signal := contract.DesktopSignal{Reference: request.Reference(), Sequence: 1, Type: "answer", SDP: "v=0\r\n"}
	if _, err := admin.SignalDesktop(ctx, signal); err == nil {
		t.Fatal("viewer impersonated worker's SDP answer")
	}
	signal.Type = "ice-complete"
	if _, err := admin.SignalDesktop(ctx, signal); err == nil {
		t.Fatal("ICE preceded authenticated peer description")
	}
	signal.Type, signal.SDP = "offer", "v=0\r\n"
	if _, err := admin.SignalDesktop(ctx, signal); err != nil {
		t.Fatal(err)
	}
	for sequence := uint64(2); sequence <= contract.MaxDesktopSignals; sequence++ {
		signal.Sequence, signal.Type, signal.SDP, signal.Candidate = sequence, "ice", "", "candidate:fixture"
		if _, err := admin.SignalDesktop(ctx, signal); err != nil {
			t.Fatal(err)
		}
	}
	signal.Sequence++
	if _, err := admin.SignalDesktop(ctx, signal); err == nil {
		t.Fatal("per-session trickle signaling budget was unbounded")
	}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: catalog.Epoch, Catalog: catalog,
		Reports: []contract.DesktopReport{{Reference: request.Reference(), State: "running", Ack: contract.MaxDesktopSignals + 1}}}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err == nil {
		t.Fatal("worker acknowledged nonexistent viewer signaling")
	}
	foreign := request.Reference()
	foreign.Target.Generation = Hash("foreign-lifetime")
	exchange.Reports[0] = contract.DesktopReport{Reference: foreign, State: "closed"}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err == nil {
		t.Fatal("foreign lifetime report released exact desktop quota")
	}
	legacy, err := NewClient(worker.Endpoint, worker.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.HTTP.CloseIdleConnections()
	if err := legacy.Call(ctx, "/v1/desktop-worker", exchange, nil); err == nil {
		t.Fatal("bearer-only worker published desktop negotiation")
	}
}

func TestDesktopFeaturePairingAndOwnerFileDoNotInventProviderReadiness(t *testing.T) {
	_, admin, _, _, root, _, _ := featureFixture(t)
	permissions := DesktopPermissions{Schema: contract.DesktopVersion, Provider: contract.DesktopProvider{ID: "fixture.native", Version: "1", Artifact: Hash("artifact")}, View: true, Input: true}
	for _, configured := range []bool{false, true} {
		if configured {
			if err := SaveDesktopPermissions(root, permissions); err != nil {
				t.Fatal(err)
			}
		}
		catalogs, err := admin.DesktopCatalog(context.Background())
		if err != nil || len(catalogs) != 0 {
			t.Fatal("pairing/features/owner configuration invented native desktop readiness", err)
		}
	}
}

func TestDesktopAggregateSignalingBudgetAcrossEndpoints(t *testing.T) {
	catalog := contract.DesktopCatalog{Schema: contract.DesktopVersion, Target: contract.FeatureTarget{Node: "worker", Fingerprint: Hash("worker"), Generation: Hash("generation")},
		Epoch: Hash("epoch"), Provider: contract.DesktopProvider{ID: "fixture.native", Version: "1", Artifact: Hash("artifact")}}
	request := desktopIntent(t, catalog)
	record := &desktopRecord{open: request, state: "running"}
	for _, viewer := range []bool{true, false} {
		queue := &record.viewer
		descriptionType := "offer"
		if !viewer {
			queue, descriptionType = &record.worker, "answer"
		}
		for sequence := uint64(1); sequence <= contract.MaxDesktopSignals; sequence++ {
			signal := contract.DesktopSignal{Reference: request.Reference(), Sequence: sequence, Type: "ice", Candidate: "candidate:" + strings.Repeat("a", contract.MaxDesktopICE-10)}
			if sequence == 1 {
				signal.Type, signal.Candidate = descriptionType, ""
				signal.SDP = "v=0\r\na=" + strings.Repeat("a", contract.MaxDesktopSDP-9) + "\r\n"
			}
			err := record.addSignal(queue, signal, viewer)
			if err != nil {
				if viewer || record.bytes > contract.MaxDesktopSignalBytes || record.bytes < contract.MaxDesktopSignalBytes-contract.MaxDesktopICE-1024 {
					t.Fatal("aggregate signal budget failed outside its documented bound", err)
				}
				return
			}
		}
	}
	t.Fatal("two bounded endpoint queues exceeded their shared signaling memory budget")
}

func TestDesktopClientDeductsTransitFromAuthoritativeLease(t *testing.T) {
	catalog := contract.DesktopCatalog{Target: contract.FeatureTarget{Node: "worker", Fingerprint: Hash("worker"), Generation: Hash("generation")},
		Epoch: Hash("epoch"), Provider: contract.DesktopProvider{ID: "fixture.native", Version: "1", Artifact: Hash("artifact")}}
	request := desktopIntent(t, catalog)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(contract.DesktopView{Reference: request.Reference(), State: "running", LeaseMillis: 100})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, Hash("token"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	view, err := client.OpenDesktop(context.Background(), request)
	if err != nil || view.LeaseMillis <= 0 || view.LeaseMillis > 90 {
		t.Fatal("viewer transit time became additional media/input authority", err)
	}
}

func TestDesktopDiscoveryUsesPairedFingerprintWithoutFeatureGrant(t *testing.T) {
	broker, admin, _, config, _, _, _ := featureFixture(t)
	broker.mutex.Lock()
	broker.features.catalogs = map[string]contract.FeatureCatalog{}
	broker.mutex.Unlock()
	var nodes []contract.NodeStatus
	if err := admin.Call(context.Background(), "/v1/nodes", map[string]string{}, &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Node != config.Node || nodes[0].Fingerprint != remoteidentity.Fingerprint(config.MachineKey) {
		t.Fatal("paired identity projection depends on feature permission")
	}
	encoded, _ := json.Marshal(nodes)
	if bytes.Contains(encoded, []byte(config.Token)) || bytes.Contains(encoded, []byte(config.MachineKey)) || bytes.Contains(encoded, []byte("private")) {
		t.Fatal("paired fingerprint projection exposed credentials")
	}
}

func TestDesktopMetadataRecoveryAfterReauthenticationNeverRenews(t *testing.T) {
	fixture := newDesktopRelayFixture(t)
	ctx := context.Background()
	fixture.broker.mutex.Lock()
	original := fixture.broker.desktops.records[fixture.open.ID].admission
	fixture.broker.mutex.Unlock()
	fixture.broker.security.Lock()
	if err := fixture.broker.security.Unlock(ctx, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.viewer.DesktopStatus(ctx, contract.DesktopRead{Reference: fixture.open.Reference()}); err == nil {
		t.Fatal("new OS admission read old signaling")
	}
	read := contract.DesktopRead{Reference: fixture.open.Reference(), MetadataOnly: true}
	view, err := fixture.viewer.DesktopStatus(ctx, read)
	if err != nil || view.Reference != fixture.open.Reference() || view.LeaseMillis != 0 || len(view.Signals) != 0 {
		t.Fatal("metadata recovery leaked signaling or authority", err)
	}
	if _, err := fixture.viewer.PulseDesktop(ctx, read); err == nil {
		t.Fatal("metadata recovery renewed old media authority")
	}
	view, err = fixture.viewer.CloseDesktop(ctx, fixture.open.Reference())
	if err != nil || view.LeaseMillis != 0 || len(view.Signals) != 0 || view.State == "closed" {
		t.Fatal("revocation became native cleanup proof", err)
	}
	fixture.broker.mutex.Lock()
	unchanged := fixture.broker.desktops.records[fixture.open.ID].admission == original
	fixture.broker.mutex.Unlock()
	if !unchanged {
		t.Fatal("recovery replaced original OS admission")
	}
	_, err = fixture.worker.ExchangeDesktop(ctx, contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: fixture.catalog.Epoch, Catalog: fixture.catalog,
		Reports: []contract.DesktopReport{{Reference: fixture.open.Reference(), State: "closed", Ack: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	view, err = fixture.viewer.DesktopStatus(ctx, read)
	if err != nil || view.State != "closed" || view.LeaseMillis != 0 || len(view.Signals) != 0 {
		t.Fatal("exact native closure proof was not recoverable", err)
	}
	fixture.broker.mutex.Lock()
	enrollment := fixture.broker.state.Nodes[fixture.open.Target.Node]
	enrollment.MachineKey = remoteidentityMustNew(t).PublicKey
	fixture.broker.state.Nodes[fixture.open.Target.Node] = enrollment
	fixture.broker.mutex.Unlock()
	if _, err := fixture.viewer.DesktopStatus(ctx, read); err == nil {
		t.Fatal("replacement paired identity recovered old reference")
	}
}
