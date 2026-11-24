package remote

import (
	"bytes"
	"crypto/x509"
	"errors"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

// loadNetworkTrustAnchors reads only the roots explicitly selected in the paired
// worker's private state. The platform trust store is never scanned here.
func loadNetworkTrustAnchors(root string) ([][]byte, error) {
	var material struct {
		Schema       string   `json:"schema"`
		Certificates [][]byte `json:"certificates"`
	}
	path := filepath.Join(root, "network-trust.json")
	if err := readPrivateBounded(path, &material, contract.MaxNetworkTrustBody); err != nil {
		return nil, err
	}
	if material.Schema != contract.NetworkTrustMaterialVersion || material.Certificates == nil {
		return nil, errors.New("worker TLS trust file requires a material schema and certificate array")
	}
	if err := contract.ValidateNetworkTrustCertificates(material.Certificates); err != nil {
		return nil, err
	}

	now := time.Now()
	for _, certificate := range material.Certificates {
		anchor, err := x509.ParseCertificate(certificate)
		if err != nil || !anchor.BasicConstraintsValid || !anchor.IsCA ||
			!bytes.Equal(anchor.RawIssuer, anchor.RawSubject) || now.Before(anchor.NotBefore) || now.After(anchor.NotAfter) ||
			anchor.CheckSignatureFrom(anchor) != nil {
			return nil, errors.New("worker TLS trust file requires valid self-signed public CA certificates")
		}
	}
	return material.Certificates, nil
}
