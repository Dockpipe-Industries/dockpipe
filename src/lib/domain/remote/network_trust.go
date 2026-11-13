package remote

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"
)

const NetworkTrustVersion = "dockpipe.remote.network-trust/v1"
const NetworkTrustMaterialVersion = "dockpipe.remote.network-trust-material/v1"
const MaxNetworkTrustCertificates = 512
const MaxNetworkTrustCertificateBytes = 16 << 10
const MaxNetworkTrustBytes = 2 << 20
const MaxNetworkTrustBody = 3 << 20
const MaxNetworkTrustCacheBytes = 16 << 20

// NetworkTrustSummary carries no certificate material in ordinary discovery.
// Ready means additive unrestricted SSL anchors, not full native policy parity.
type NetworkTrustSummary struct {
	State  string `json:"state"`
	Digest string `json:"digest,omitempty"`
	Count  int    `json:"count"`
	Bytes  int    `json:"bytes"`
}

func (summary NetworkTrustSummary) Validate() error {
	switch summary.State {
	case "disabled", "unavailable":
		if summary.Digest != "" || summary.Count != 0 || summary.Bytes != 0 {
			return errors.New("unavailable network trust cannot carry accepted material")
		}
	case "ready":
		if !featureDigest(summary.Digest) || summary.Count < 0 || summary.Count > MaxNetworkTrustCertificates || summary.Bytes < 0 || summary.Bytes > MaxNetworkTrustBytes || summary.Count == 0 && summary.Bytes != 0 || summary.Count > 0 && summary.Bytes == 0 {
			return errors.New("invalid bounded network trust summary")
		}
	default:
		return errors.New("unknown network trust sharing state")
	}
	return nil
}

type NetworkTrustRequest struct {
	Schema string        `json:"schema"`
	Target FeatureTarget `json:"target"`
	Digest string        `json:"digest"`
}

func (request NetworkTrustRequest) Validate() error {
	if request.Schema != NetworkTrustVersion || request.Target.Validate() != nil || !featureDigest(request.Digest) {
		return errors.New("network trust requires an exact target and digest")
	}
	return nil
}

// Certificates are canonical DER, encoded as base64 by JSON. Only public X.509
// material is accepted. The digest covers length-delimited, SHA256-sorted DER.
type NetworkTrustSnapshot struct {
	Schema       string        `json:"schema"`
	Target       FeatureTarget `json:"target"`
	Digest       string        `json:"digest"`
	Certificates [][]byte      `json:"certificates"`
}

func NetworkTrustDigest(certificates [][]byte) (string, int, error) {
	return networkTrustDigest(certificates, true)
}

// Native collectors may enumerate anchors in any order. Wire snapshots enforce
// canonical order separately, while every boundary shares DER and byte checks.
func ValidateNetworkTrustCertificates(certificates [][]byte) error {
	_, _, err := networkTrustDigest(certificates, false)
	return err
}

func networkTrustDigest(certificates [][]byte, canonical bool) (string, int, error) {
	if len(certificates) > MaxNetworkTrustCertificates {
		return "", 0, errors.New("network trust certificate count exceeds 512")
	}
	total := 0
	previous := ""
	seen := make(map[string]bool, len(certificates))
	digest := sha256.New()
	_, _ = digest.Write([]byte(NetworkTrustVersion + "\n"))
	for _, certificate := range certificates {
		if len(certificate) == 0 || len(certificate) > MaxNetworkTrustCertificateBytes || len(certificate) > MaxNetworkTrustBytes-total {
			return "", 0, errors.New("network trust DER byte budget exceeded")
		}
		total += len(certificate)
		parsed, err := x509.ParseCertificate(certificate)
		if err != nil || len(parsed.Raw) != len(certificate) {
			return "", 0, errors.New("network trust requires public X.509 DER certificates")
		}
		fingerprint := sha256.Sum256(certificate)
		key := hex.EncodeToString(fingerprint[:])
		if seen[key] || canonical && key <= previous {
			return "", 0, errors.New("network trust certificates must be unique and SHA256-sorted")
		}
		seen[key] = true
		previous = key
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(certificate)))
		_, _ = digest.Write(length[:])
		_, _ = digest.Write(certificate)
	}
	return hex.EncodeToString(digest.Sum(nil)), total, nil
}

func NewNetworkTrustSnapshot(target FeatureTarget, certificates [][]byte) (NetworkTrustSnapshot, error) {
	snapshot := NetworkTrustSnapshot{Schema: NetworkTrustVersion, Target: target}
	if target.Validate() != nil || len(certificates) > MaxNetworkTrustCertificates {
		return snapshot, errors.New("invalid network trust snapshot identity or count")
	}
	// Copy only after enforcing aggregate byte bounds. Callers cannot mutate an
	// accepted snapshot through a shared collector buffer after publication.
	total := 0
	for _, certificate := range certificates {
		if len(certificate) == 0 || len(certificate) > MaxNetworkTrustCertificateBytes || len(certificate) > MaxNetworkTrustBytes-total {
			return snapshot, errors.New("network trust DER byte budget exceeded")
		}
		total += len(certificate)
	}
	snapshot.Certificates = make([][]byte, 0, len(certificates))
	for _, certificate := range certificates {
		snapshot.Certificates = append(snapshot.Certificates, append([]byte(nil), certificate...))
	}
	sort.Slice(snapshot.Certificates, func(i, j int) bool {
		left := sha256.Sum256(snapshot.Certificates[i])
		right := sha256.Sum256(snapshot.Certificates[j])
		return string(left[:]) < string(right[:])
	})
	var err error
	snapshot.Digest, _, err = NetworkTrustDigest(snapshot.Certificates)
	if err != nil {
		return NetworkTrustSnapshot{}, err
	}
	return snapshot, snapshot.Validate()
}

func (snapshot NetworkTrustSnapshot) Validate() error {
	if snapshot.Schema != NetworkTrustVersion || snapshot.Target.Validate() != nil || snapshot.Certificates == nil {
		return errors.New("invalid network trust snapshot identity")
	}
	digest, _, err := NetworkTrustDigest(snapshot.Certificates)
	if err != nil {
		return err
	}
	if digest != snapshot.Digest {
		return errors.New("network trust snapshot digest changed")
	}
	return nil
}

func (snapshot NetworkTrustSnapshot) Summary() NetworkTrustSummary {
	_, bytes, err := NetworkTrustDigest(snapshot.Certificates)
	if err != nil || snapshot.Validate() != nil {
		return NetworkTrustSummary{State: "unavailable"}
	}
	return NetworkTrustSummary{State: "ready", Digest: snapshot.Digest, Count: len(snapshot.Certificates), Bytes: bytes}
}
