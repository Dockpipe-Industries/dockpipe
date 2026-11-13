package remote

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"
)

func trustCertificate(t *testing.T, serial int64) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "public fixture anchor"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func TestNetworkTrustCanonicalPublicDERAndDigest(t *testing.T) {
	target := FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)}
	first, second := trustCertificate(t, 1), trustCertificate(t, 2)
	snapshot, err := NewNetworkTrustSnapshot(target, [][]byte{first, second})
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := NewNetworkTrustSnapshot(target, [][]byte{second, first})
	if err != nil || reordered.Digest != snapshot.Digest {
		t.Fatal("collection order changed canonical digest", err)
	}
	summary := snapshot.Summary()
	if summary.Validate() != nil || summary.Count != 2 || summary.Bytes != len(first)+len(second) {
		t.Fatal(summary)
	}
	first[0] ^= 0xff
	if snapshot.Validate() != nil {
		t.Fatal("caller mutation changed accepted public trust")
	}
	changed := snapshot
	changed.Digest = strings.Repeat("c", 64)
	if changed.Validate() == nil {
		t.Fatal("changed trust digest accepted")
	}
	changed = snapshot
	changed.Certificates = [][]byte{snapshot.Certificates[1], snapshot.Certificates[0]}
	if changed.Validate() == nil {
		t.Fatal("noncanonical trust order accepted")
	}
	if _, err = NewNetworkTrustSnapshot(target, [][]byte{second, second}); err == nil {
		t.Fatal("duplicate anchors accepted")
	}
	empty, err := NewNetworkTrustSnapshot(target, nil)
	if err != nil || empty.Summary().Count != 0 {
		t.Fatal("empty approved anchor set invalid", err)
	}
}

func TestNetworkTrustBoundsAndFailClosedSummary(t *testing.T) {
	target := FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)}
	cases := [][][]byte{
		make([][]byte, MaxNetworkTrustCertificates+1),
		{make([]byte, MaxNetworkTrustCertificateBytes+1)},
		{[]byte("-----BEGIN PRIVATE KEY-----\nprivate\n-----END PRIVATE KEY-----")},
		{nil},
	}
	aggregate := make([][]byte, MaxNetworkTrustBytes/MaxNetworkTrustCertificateBytes+1)
	for index := range aggregate {
		aggregate[index] = make([]byte, MaxNetworkTrustCertificateBytes)
	}
	cases = append(cases, aggregate)
	for index, certificates := range cases {
		if _, err := NewNetworkTrustSnapshot(target, certificates); err == nil {
			t.Errorf("invalid public trust case %d accepted", index)
		}
	}
	for _, summary := range []NetworkTrustSummary{
		{State: "unavailable", Digest: strings.Repeat("a", 64)},
		{State: "disabled", Count: 1},
		{State: "ready", Digest: strings.Repeat("a", 64), Count: MaxNetworkTrustCertificates + 1, Bytes: 1},
		{State: "ready", Digest: strings.Repeat("a", 64), Count: 1, Bytes: MaxNetworkTrustBytes + 1},
		{State: "ready", Digest: strings.Repeat("a", 64), Count: 1},
		{State: "unknown"},
	} {
		if summary.Validate() == nil {
			t.Errorf("invalid sharing summary accepted: %+v", summary)
		}
	}
	if (NetworkPermissions{Schema: NetworkVersion, ShareTLSTrust: true}).Validate() == nil {
		t.Fatal("trust sharing enabled without independent network grant")
	}
}
