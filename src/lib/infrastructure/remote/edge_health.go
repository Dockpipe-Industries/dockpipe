package remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func validateEdgeSource(path string) error {
	if err := infrastructure.ValidateUnlinkedPath(path); err != nil {
		return err
	}
	return infrastructure.ValidatePrivatePath(filepath.Dir(path), true)
}

func edgeHTTPClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("edge health redirects are forbidden") },
		Transport: &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13}, MaxResponseHeaderBytes: 16 << 10, DisableKeepAlives: true}}
}

func checkEdgeReadiness(ctx context.Context, child managedEdge, address string) error {
	if !child.alive() || !child.ownsReadiness(address) {
		return errors.New("readiness listener is not owned by the supervised process")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	response, err := edgeHTTPClient().Do(request)
	if err != nil {
		return errors.New("edge readiness probe failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("edge is not ready")
	}
	return nil
}

type edgeHealthProof struct {
	Schema    string `json:"schema"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

func (broker *Broker) proveEdgeHealth(raw []byte) (any, error) {
	var request struct {
		Nonce string `json:"nonce"`
	}
	if err := Decode(raw, &request); err != nil || !contract.ValidEdgeDigest(request.Nonce) {
		return nil, errors.New("invalid health challenge")
	}
	signature, err := broker.state.Identity.Sign([]byte("dockpipe.edge.health/v1\n" + request.Nonce))
	if err != nil {
		return nil, err
	}
	return edgeHealthProof{Schema: contract.EdgeActivationSchema, Nonce: request.Nonce, Signature: signature}, nil
}

func probeEdgePublic(ctx context.Context, endpoint, key string, client *http.Client) error {
	nonce, err := Secret()
	if err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"nonce": nonce})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v1/edge-health", bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return errors.New("published edge TLS/round-trip probe failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 || response.StatusCode != http.StatusOK {
		return errors.New("published edge health response failed")
	}
	var proof edgeHealthProof
	if err := Decode(body, &proof); err != nil || proof.Schema != contract.EdgeActivationSchema || proof.Nonce != nonce ||
		!remoteidentity.Verify(key, []byte("dockpipe.edge.health/v1\n"+nonce), proof.Signature) {
		return errors.New("published edge returned a different broker identity or stale challenge")
	}
	return nil
}

func verifyEdgeSnapshot(path, expected string) error {
	digest, err := edgePrivateDigest(path)
	if err != nil || digest != expected {
		return errors.New("edge snapshot checksum mismatch")
	}
	return nil
}
