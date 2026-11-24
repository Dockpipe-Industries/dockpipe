package osauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func publicTLSFixture(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "approved public fixture anchor"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct {
		Schema       string   `json:"schema"`
		Certificates [][]byte `json:"certificates"`
	}{contract.NetworkTrustMaterialVersion, [][]byte{certificate}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNetworkTLSMaterialBoundsAndPublicDER(t *testing.T) {
	if certificates, err := decodeNetworkTLSAnchors(publicTLSFixture(t)); err != nil || len(certificates) != 1 {
		t.Fatal("approved public trust material refused", err)
	}
	for _, raw := range [][]byte{
		[]byte(`{"schema":"dockpipe.remote.network-trust-material/v1","certificates":[],"restrictions":["hostname-only"]}`),
		[]byte(`{"schema":"dockpipe.remote.network-trust-material/v1","certificates":["bm90LWEteDUwOS1jZXJ0aWZpY2F0ZQ=="]}`),
		[]byte(`{"schema":"unapproved","certificates":[]}`),
		append(publicTLSFixture(t), []byte(`{}`)...),
		make([]byte, contract.MaxNetworkTrustBody+1),
	} {
		if _, err := decodeNetworkTLSAnchors(raw); err == nil {
			t.Fatal("invalid or partial native trust accepted")
		}
	}
	output := &boundedOutput{maximum: contract.MaxNetworkTrustBody}
	if _, err := output.Write(make([]byte, 64<<10+1)); err != nil {
		t.Fatal("trust-only command cannot carry more than the ordinary auth command bound", err)
	}
	if _, err := output.Write(make([]byte, contract.MaxNetworkTrustBody)); err == nil {
		t.Fatal("trust command output exceeded its declared JSON bound")
	}
	ordinary := &boundedOutput{maximum: 64 << 10}
	if _, err := ordinary.Write(make([]byte, 64<<10+1)); err == nil {
		t.Fatal("ordinary authentication command output bound expanded")
	}
}
