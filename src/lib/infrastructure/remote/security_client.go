package remote

import (
	"context"
	"errors"
)

// ProtectLocalOperator routes every operator request through the same OS-user
// authority. It never exposes or persists the authority's signing credential.
func (client *Client) ProtectLocalOperator(brokerKey string) {
	client.OperatorProof = func(ctx context.Context, path string, body []byte) (SecurityProof, error) {
		response, err := CallSecurity(ctx, SecurityRequest{Action: "sign", Audience: brokerKey, Path: path, Digest: bodyDigest(body)})
		if err != nil {
			return SecurityProof{}, err
		}
		if response.Proof == nil || response.Proof.BrokerKey != brokerKey {
			return SecurityProof{}, errors.New("OS security authority is bound to a different broker")
		}
		return *response.Proof, nil
	}
}
