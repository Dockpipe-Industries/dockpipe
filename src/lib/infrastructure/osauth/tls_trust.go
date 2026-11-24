package osauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	contract "dockpipe/src/lib/domain/remote"
)

func decodeNetworkTLSAnchors(raw []byte) ([][]byte, error) {
	if len(raw) > contract.MaxNetworkTrustBody {
		return nil, errors.New("native public TLS trust exceeds its 3 MiB JSON bound")
	}
	var response struct {
		Schema       string   `json:"schema"`
		Certificates [][]byte `json:"certificates"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&response) != nil || decoder.Decode(new(any)) != io.EOF || response.Schema != contract.NetworkTrustMaterialVersion {
		return nil, errors.New("native public TLS trust material is invalid")
	}
	if err := contract.ValidateNetworkTrustCertificates(response.Certificates); err != nil {
		return nil, err
	}
	return response.Certificates, nil
}
