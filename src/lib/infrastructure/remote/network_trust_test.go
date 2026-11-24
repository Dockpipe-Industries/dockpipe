package remote

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func networkTrustCertificates(t *testing.T, count, padding int) [][]byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificates := make([][]byte, 0, count)
	for index := 0; index < count; index++ {
		template := &x509.Certificate{SerialNumber: big.NewInt(int64(index + 1)), Subject: pkix.Name{CommonName: "public fixture anchor"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
		if padding > 0 {
			template.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 1}, Value: make([]byte, padding)}}
		}
		certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		certificates = append(certificates, certificate)
	}
	return certificates
}

func networkTrustFixture(t *testing.T) (*Broker, *Client, *Client, contract.NetworkCatalog, contract.NetworkTrustSnapshot) {
	t.Helper()
	broker, operator, worker, catalog := networkFixture(t)
	catalog.Target.Generation = Hash("approved trust lifetime")
	snapshot, err := contract.NewNetworkTrustSnapshot(catalog.Target, networkTrustCertificates(t, 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	summary := snapshot.Summary()
	catalog.TLSTrust = &summary
	if err = worker.Call(context.Background(), "/v1/network-trust-publish", snapshot, nil); err != nil {
		t.Fatal(err)
	}
	if err = worker.Call(context.Background(), "/v1/network-worker", catalog, nil); err != nil {
		t.Fatal(err)
	}
	return broker, operator, worker, catalog, snapshot
}

func TestNetworkTrustAuthenticatedExactRetrievalAndCompactCatalog(t *testing.T) {
	broker, operator, worker, catalog, snapshot := networkTrustFixture(t)
	ctx := context.Background()
	request := contract.NetworkTrustRequest{Schema: contract.NetworkTrustVersion, Target: catalog.Target, Digest: snapshot.Digest}
	received, err := operator.NetworkTrust(ctx, request)
	if err != nil || received.Digest != snapshot.Digest {
		t.Fatal("exact approved trust unavailable", err)
	}
	catalogs, err := operator.NetworkCatalog(ctx)
	if err != nil || len(catalogs) != 1 {
		t.Fatal(catalogs, err)
	}
	encoded, err := json.Marshal(catalogs)
	if err != nil || strings.Contains(string(encoded), "certificates") || len(encoded) > 2048 {
		t.Fatal("ordinary discovery carried certificate arrays", err)
	}
	persisted, err := os.ReadFile(broker.path)
	if err != nil || strings.Contains(string(persisted), snapshot.Digest) {
		t.Fatal("public trust snapshot entered persistent broker state", err)
	}
	for _, scenario := range []string{"digest", "generation", "fingerprint", "node", "stale", "revoked", "locked"} {
		t.Run(scenario, func(t *testing.T) {
			changed := request
			broker.mutex.Lock()
			original := broker.networks.catalogs[catalog.Target.Node]
			enrollment := broker.state.Nodes[catalog.Target.Node]
			switch scenario {
			case "digest":
				changed.Digest = Hash("other")
			case "generation":
				changed.Target.Generation = Hash("other")
			case "fingerprint":
				changed.Target.Fingerprint = Hash("other")
			case "node":
				changed.Target.Node = "other"
			case "stale":
				value := original
				value.ReceivedAt = time.Now().Add(-10 * time.Second)
				broker.networks.catalogs[catalog.Target.Node] = value
			case "revoked":
				value := enrollment
				value.Revoked = true
				broker.state.Nodes[catalog.Target.Node] = value
			case "locked":
				broker.security.Lock()
			}
			broker.mutex.Unlock()
			if _, err := operator.NetworkTrust(ctx, changed); err == nil {
				t.Fatal("foreign or stale trust retrieval accepted")
			}
			broker.mutex.Lock()
			broker.networks.catalogs[catalog.Target.Node] = original
			broker.state.Nodes[catalog.Target.Node] = enrollment
			broker.mutex.Unlock()
		})
	}
	legacy, _ := NewClient(worker.Endpoint, worker.Token)
	defer legacy.HTTP.CloseIdleConnections()
	if legacy.Call(ctx, "/v1/network-trust-publish", snapshot, nil) == nil {
		t.Fatal("bearer-only trust publication accepted")
	}
}

func TestNetworkTrustGenerationImmutabilityAndUnavailableRefusal(t *testing.T) {
	_, operator, worker, catalog, snapshot := networkTrustFixture(t)
	ctx := context.Background()
	changed, err := contract.NewNetworkTrustSnapshot(catalog.Target, networkTrustCertificates(t, 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if worker.Call(ctx, "/v1/network-trust-publish", changed, nil) == nil {
		t.Fatal("trust digest changed within original generation")
	}
	downgraded := catalog
	downgraded.TLSTrust = nil
	if worker.Call(ctx, "/v1/network-worker", downgraded, nil) == nil {
		t.Fatal("same-generation trust silently downgraded")
	}
	catalog.Target.Generation = Hash("new unavailable trust lifetime")
	unavailable := contract.NetworkTrustSummary{State: "unavailable"}
	catalog.TLSTrust = &unavailable
	if err = worker.Call(ctx, "/v1/network-worker", catalog, nil); err != nil {
		t.Fatal(err)
	}
	if operator.Call(ctx, "/v1/network-connect", contract.NetworkConnect{Schema: contract.NetworkVersion, Target: catalog.Target, Address: "localhost:443"}, nil) == nil {
		t.Fatal("unavailable shared trust carried traffic")
	}
	old := contract.NetworkTrustRequest{Schema: contract.NetworkTrustVersion, Target: snapshot.Target, Digest: snapshot.Digest}
	if _, err = operator.NetworkTrust(ctx, old); err == nil {
		t.Fatal("removed trust remained retrievable")
	}
}

func TestNetworkTrustGlobalCacheBoundAndStalePruning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		broker := &Broker{state: BrokerState{Nodes: make(map[string]Enrollment)}}
		enrollment := Enrollment{MachineKey: remoteidentityMustNew(t).PublicKey}
		fingerprint := remoteidentity.Fingerprint(enrollment.MachineKey)
		generation := Hash("trust cache capacity fixture")
		certificates := networkTrustCertificates(t, 240, 8<<10)
		state := broker.networkState()

		for index := range 9 {
			target := contract.FeatureTarget{
				Node: "trust-worker-" + strconv.Itoa(index), Fingerprint: fingerprint, Generation: generation,
			}
			broker.state.Nodes[target.Node] = enrollment
			snapshot, err := contract.NewNetworkTrustSnapshot(target, certificates)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}

			summary := snapshot.Summary()
			_, err = broker.publishNetworkTrust(target.Node, raw)
			if index < 8 {
				if err != nil {
					t.Fatalf("bounded trust record %d: %v", index, err)
				}
				state.catalogs[target.Node] = contract.NetworkCatalog{
					Schema: contract.NetworkVersion, Target: target, Enabled: true,
					ReceivedAt: time.Now(), TLSTrust: &summary,
				}
				continue
			}
			if err == nil {
				t.Fatal("aggregate broker trust cache exceeded 16 MiB DER")
			}
			if len(state.trusts) != index || state.trustBytes != index*summary.Bytes {
				t.Fatal("rejected publication changed the aggregate trust cache")
			}

			time.Sleep(contract.NetworkFreshness + time.Nanosecond)
			if _, err := broker.publishNetworkTrust(target.Node, raw); err != nil {
				t.Fatal("stale trust did not free bounded cache", err)
			}
			if len(state.trusts) != 1 || state.trustBytes != summary.Bytes {
				t.Fatal("stale cache retained unrelated material")
			}
		}
	})
}

// Identity proof payload limits apply to the domain-separated body hash, not the
// raw HTTP body. Exercise both authenticated transports with a snapshot larger
// than that proof-message limit and near the advertised DER maximum.
func TestNetworkTrustNearMaximumSignedPublicationAndRetrieval(t *testing.T) {
	broker, operator, worker, catalog := networkFixture(t)
	catalog.Target.Generation = Hash("near-maximum signed trust lifetime")
	snapshot, err := contract.NewNetworkTrustSnapshot(catalog.Target, networkTrustCertificates(t, 240, 8<<10))
	if err != nil {
		t.Fatal(err)
	}
	summary := snapshot.Summary()
	if summary.Bytes < contract.MaxNetworkTrustBytes*95/100 || summary.Bytes > contract.MaxNetworkTrustBytes {
		t.Fatalf("fixture must exercise the near-maximum DER contract: %d bytes", summary.Bytes)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) <= remoteidentity.MaxSignedPayload || len(encoded) > contract.MaxNetworkTrustBody || len(encoded) > contract.MaxBody {
		t.Fatalf("fixture must exceed the identity proof limit while fitting both HTTP bounds: %d bytes", len(encoded))
	}
	proof := machineMessage("request", Hash("fixture nonce"), "/v1/network-trust-publish", 0, encoded)
	if len(proof) >= remoteidentity.MaxSignedPayload {
		t.Fatal("machine proof unexpectedly contains the raw snapshot")
	}
	ctx := context.Background()
	var accepted contract.NetworkTrustSummary
	if err = worker.Call(ctx, "/v1/network-trust-publish", snapshot, &accepted); err != nil {
		t.Fatal("near-maximum signed machine publication failed", err)
	}
	if accepted != summary {
		t.Fatal("signed publication changed the near-maximum snapshot summary")
	}
	catalog.TLSTrust = &summary
	if err = worker.Call(ctx, "/v1/network-worker", catalog, nil); err != nil {
		t.Fatal(err)
	}
	// The fixture uses the production authority freshness gate. Refresh its OS
	// sample after constructing/parsing megabytes, as the running broker does.
	broker.security.Sample(ctx)
	retrieved, err := operator.NetworkTrust(ctx, contract.NetworkTrustRequest{Schema: contract.NetworkTrustVersion, Target: catalog.Target, Digest: snapshot.Digest})
	if err != nil {
		t.Fatal("near-maximum OS-authenticated signed retrieval failed", err)
	}
	if retrieved.Target != snapshot.Target || retrieved.Digest != snapshot.Digest || retrieved.Summary() != summary {
		t.Fatal("near-maximum authenticated retrieval changed exact identity or material")
	}
	if len(retrieved.Certificates) != len(snapshot.Certificates) {
		t.Fatal("near-maximum signed response was truncated")
	}
	t.Logf("signed publication/retrieval preserved %d DER bytes in %d JSON bytes; identity proof is %d bytes", summary.Bytes, len(encoded), len(proof))
}
