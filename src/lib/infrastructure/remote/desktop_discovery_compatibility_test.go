package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

// These shapes match strict desktop/v1 decoders before discovery was extended.
type legacyDesktopCatalog struct {
	Schema     string                   `json:"schema"`
	Target     contract.FeatureTarget   `json:"target"`
	Provider   contract.DesktopProvider `json:"provider"`
	View       bool                     `json:"view"`
	Input      bool                     `json:"input"`
	Epoch      string                   `json:"epoch,omitempty"`
	ReceivedAt time.Time                `json:"received_at"`
}

type legacyDesktopExchange struct {
	Schema  string                   `json:"schema"`
	Epoch   string                   `json:"epoch,omitempty"`
	Catalog legacyDesktopCatalog     `json:"catalog"`
	Reports []contract.DesktopReport `json:"reports"`
}

func desktopCompatibilityClients(t *testing.T) (*Broker, *Client, *Client, contract.DesktopCatalog) {
	t.Helper()
	const endpoint = "http://127.0.0.1:47831"
	operatorToken := Hash("operator")
	workerToken := Hash("worker")
	workerIdentity := remoteidentityMustNew(t)
	brokerIdentity := remoteidentityMustNew(t)
	broker := &Broker{
		state: BrokerState{
			AdminHash: Hash(operatorToken),
			Identity:  &brokerIdentity,
			Nodes:     map[string]Enrollment{"worker": {TokenHash: Hash(workerToken), MachineKey: workerIdentity.PublicKey}},
		},
		operatorTokenHash: Hash(operatorToken),
	}
	authority, _ := testSecurityAuthority(t, broker)
	viewer, err := NewClient(endpoint, operatorToken)
	if err != nil {
		t.Fatal(err)
	}
	viewer.OperatorProof = func(ctx context.Context, path string, body []byte) (SecurityProof, error) {
		return authority.Sign(ctx, broker.state.Identity.PublicKey, path, bodyDigest(body))
	}
	worker, err := NewClient(endpoint, workerToken)
	if err != nil {
		t.Fatal(err)
	}
	worker.Identity = &workerIdentity
	worker.BrokerKey = broker.state.Identity.PublicKey
	transport := machineTestTransport(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		broker.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})
	viewer.HTTP.Transport = transport
	worker.HTTP.Transport = transport

	catalog := contract.DesktopCatalog{
		Schema: contract.DesktopVersion,
		Target: contract.FeatureTarget{
			Node: "worker", Fingerprint: remoteidentity.Fingerprint(workerIdentity.PublicKey), Generation: Hash("generation"),
		},
		Provider: contract.DesktopProvider{ID: "fixture.native", Version: "1", Artifact: Hash("artifact")},
		View:     true,
		Input:    true,
		Displays: []contract.DesktopDisplay{{Source: "primary", Width: 1920, Height: 1080, Primary: true}},
	}
	return broker, viewer, worker, catalog
}

func TestDesktopDiscoveryProjectsLegacyAndExtendedCatalogs(t *testing.T) {
	broker, viewer, worker, catalog := desktopCompatibilityClients(t)
	ctx := context.Background()
	catalog.AdministratorInput = true
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Catalog: catalog}
	probe, err := worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(broker.desktops.catalogs) != 0 || len(broker.desktops.records) != 0 {
		t.Fatalf("epoch negotiation published readiness or opened a session: %+v, %v", probe, err)
	}
	exchange.Epoch = probe.Epoch
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}

	catalogs, err := viewer.DesktopCatalog(ctx)
	if err != nil || len(catalogs) != 1 || !slices.Equal(catalogs[0].Displays, catalog.Displays) || catalogs[0].AdministratorInput {
		t.Fatalf("modern catalog lost displays or accepted worker-owned administrator authority: %+v, %v", catalogs, err)
	}
	broker.desktops.administratorKeys = map[string]desktopAdministratorService{
		catalog.Target.Node: {fingerprint: catalog.Target.Fingerprint, tokenHash: Hash("administrator-service")},
	}
	broker.desktops.administratorSeen = map[string]time.Time{catalog.Target.Node: time.Now()}
	catalogs, err = viewer.DesktopCatalog(ctx)
	if err != nil || len(catalogs) != 1 || !catalogs[0].AdministratorInput {
		t.Fatalf("modern catalog lost broker-owned administrator discovery: %+v, %v", catalogs, err)
	}

	var legacy []legacyDesktopCatalog
	err = viewer.Call(ctx, "/v1/desktop-catalog", map[string]string{"schema": contract.DesktopVersion}, &legacy)
	if err != nil || len(legacy) != 1 || legacy[0].Schema != contract.DesktopVersion || legacy[0].Target != catalog.Target ||
		legacy[0].Provider != catalog.Provider || legacy[0].Epoch != exchange.Epoch || !legacy[0].View || !legacy[0].Input {
		t.Fatalf("strict legacy viewer lost desktop discovery: %+v, %v", legacy, err)
	}
}

func TestDesktopDiscoveryFallsBackForStrictLegacyPeers(t *testing.T) {
	broker, viewer, worker, catalog := desktopCompatibilityClients(t)
	ctx := context.Background()
	forward := viewer.HTTP.Transport
	var workerSchemas, viewerSchemas []string
	legacy := machineTestTransport(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/desktop-catalog", "/v1/desktop-worker":
		default:
			return forward.RoundTrip(request)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(body))

		var version struct {
			Schema string `json:"schema"`
		}
		if err := json.Unmarshal(body, &version); err != nil {
			return nil, err
		}
		if request.URL.Path == "/v1/desktop-catalog" {
			viewerSchemas = append(viewerSchemas, version.Schema)
		} else {
			workerSchemas = append(workerSchemas, version.Schema)
			var exchange legacyDesktopExchange
			if err := Decode(body, &exchange); err != nil {
				t.Fatalf("worker sent discovery fields to a strict legacy broker: %v", err)
			}
			if version.Schema == contract.DesktopDiscoveryVersion && (exchange.Epoch != "" || len(exchange.Reports) != 0) {
				t.Fatal("worker sent an effect-bearing exchange before negotiating legacy compatibility")
			}
		}
		if version.Schema != contract.DesktopVersion {
			return desktopCompatibilityConflict(broker, request, body)
		}
		return forward.RoundTrip(request)
	})
	viewer.HTTP.Transport = legacy
	worker.HTTP.Transport = legacy

	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Catalog: catalog}
	probe, err := worker.ExchangeDesktop(ctx, exchange)
	if err != nil {
		t.Fatal(err)
	}
	exchange.Epoch = probe.Epoch
	exchange.Catalog.AdministratorInput = true
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}
	catalogs, err := viewer.DesktopCatalog(ctx)
	if err != nil || len(catalogs) != 1 || catalogs[0].Target != catalog.Target || catalogs[0].Epoch != probe.Epoch ||
		len(catalogs[0].Displays) != 0 || catalogs[0].AdministratorInput || !catalogs[0].View || !catalogs[0].Input {
		t.Fatalf("legacy broker desktop discovery failed: %+v, %v", catalogs, err)
	}

	if !slices.Equal(workerSchemas, []string{contract.DesktopDiscoveryVersion, contract.DesktopVersion, contract.DesktopVersion}) ||
		!slices.Equal(viewerSchemas, []string{contract.DesktopDiscoveryVersion, contract.DesktopVersion}) {
		t.Fatalf("unexpected worker or viewer negotiation: %v, %v", workerSchemas, viewerSchemas)
	}
}

func desktopCompatibilityConflict(broker *Broker, request *http.Request, body []byte) (*http.Response, error) {
	const payload = "remote operation rejected\n"
	kind, nonce, header := "response", request.Header.Get(machineNonceHeader), machineSignatureHeader
	if request.URL.Path == "/v1/desktop-catalog" {
		admission, err := broker.security.Verify(request.Context(), request.Header.Get(operatorProofHeader), request.URL.Path, body)
		if err != nil {
			return nil, err
		}
		kind, nonce, header = "operator-response", admission.nonce, operatorResponseHeader
	}
	signature, err := broker.state.Identity.Sign(machineMessage(kind, nonce, request.URL.Path, http.StatusConflict, []byte(payload)))
	if err != nil {
		return nil, err
	}
	recorder := httptest.NewRecorder()
	recorder.Header().Set(header, signature)
	recorder.WriteHeader(http.StatusConflict)
	_, _ = io.WriteString(recorder, payload)
	return recorder.Result(), nil
}

func TestDesktopDiscoveryDoesNotReplayWorkerReportsOrTrustUnsignedRejections(t *testing.T) {
	broker, viewer, worker, catalog := desktopCompatibilityClients(t)
	ctx := context.Background()
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Catalog: catalog}
	probe, err := worker.ExchangeDesktop(ctx, exchange)
	if err != nil {
		t.Fatal(err)
	}
	exchange.Epoch = probe.Epoch
	catalog.Epoch = probe.Epoch
	exchange.Reports = []contract.DesktopReport{{Reference: desktopIntent(t, catalog).Reference(), State: "closed"}}

	forward := worker.HTTP.Transport
	publications := 0
	worker.HTTP.Transport = machineTestTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/v1/desktop-worker" {
			publications++
			return desktopCompatibilityConflict(broker, request, nil)
		}
		return forward.RoundTrip(request)
	})
	var rejection *HTTPError
	if _, err := worker.ExchangeDesktop(ctx, exchange); !errors.As(err, &rejection) || rejection.StatusCode != http.StatusConflict || publications != 1 {
		t.Fatalf("report was replayed or its original outcome swallowed: count %d, %v", publications, err)
	}

	viewerRequests := 0
	viewer.HTTP.Transport = machineTestTransport(func(request *http.Request) (*http.Response, error) {
		viewerRequests++
		recorder := httptest.NewRecorder()
		recorder.WriteHeader(http.StatusConflict)
		return recorder.Result(), nil
	})
	if _, err := viewer.DesktopCatalog(ctx); err == nil || viewerRequests != 1 {
		t.Fatalf("unsigned conflict triggered fallback: count %d, %v", viewerRequests, err)
	}
	viewer.OperatorProof = nil
	viewerRequests = 0
	if _, err := viewer.DesktopCatalog(ctx); !errors.As(err, &rejection) || viewerRequests != 1 {
		t.Fatalf("an unauthenticated client retried after receiving HTTP conflict: count %d, %v", viewerRequests, err)
	}
}
