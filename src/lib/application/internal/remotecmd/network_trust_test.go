package remotecmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestNetworkProxyPrivateReadinessTrustSelection(t *testing.T) {
	target := contract.FeatureTarget{Node: "worker", Fingerprint: remoteio.Hash("paired worker"), Generation: remoteio.Hash("trust generation")}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "approved public fixture anchor"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := contract.NewNetworkTrustSnapshot(target, [][]byte{der})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"ready", "legacy", "disabled", "unavailable", "catalog-generation", "response-generation", "response-digest"} {
		t.Run(scenario, func(t *testing.T) {
			summary := snapshot.Summary()
			catalog := contract.NetworkCatalog{Schema: contract.NetworkVersion, Target: target, Enabled: true, TLSTrust: &summary}
			response := snapshot
			switch scenario {
			case "legacy":
				catalog.TLSTrust = nil
			case "disabled":
				summary = contract.NetworkTrustSummary{State: "disabled"}
			case "unavailable":
				summary = contract.NetworkTrustSummary{State: "unavailable"}
			case "catalog-generation":
				catalog.Target.Generation = remoteio.Hash("new generation")
			case "response-generation":
				response.Target.Generation = remoteio.Hash("wrong generation")
			case "response-digest":
				response.Digest = remoteio.Hash("wrong digest")
			}
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/v1/network-catalog":
					_ = json.NewEncoder(writer).Encode([]contract.NetworkCatalog{catalog})
				case "/v1/network-trust":
					var lookup contract.NetworkTrustRequest
					if json.NewDecoder(request.Body).Decode(&lookup) != nil || lookup.Target != target || lookup.Digest != snapshot.Digest || lookup.Validate() != nil {
						t.Error("private trust retrieval changed exact reviewed target or digest")
					}
					_ = json.NewEncoder(writer).Encode(response)
				default:
					t.Error("unexpected remote request path")
					http.Error(writer, "unexpected", 400)
				}
			}))
			defer server.Close()
			client, err := remoteio.NewClient(server.URL, remoteio.Hash("fixture bearer"))
			if err != nil {
				t.Fatal(err)
			}
			defer client.HTTP.CloseIdleConnections()
			received, err := proxyNetworkTrust(context.Background(), client, target)
			switch scenario {
			case "ready":
				if err != nil || received == nil || received.Target != target || received.Digest != snapshot.Digest {
					t.Fatal("verified private readiness material missing", err)
				}
			case "legacy", "disabled":
				if err != nil || received != nil {
					t.Fatal("disabled sharing changed backward compatible readiness", err)
				}
			default:
				if err == nil || received != nil {
					t.Fatal("unavailable or drifting trust entered private readiness")
				}
			}
		})
	}
}
