package remote

import (
	"context"
	"errors"
)

// ErrOperatorLocked is returned only by the trusted local authority. HTTP
// failures must never trigger authentication or replay a possibly admitted action.
var ErrOperatorLocked = errors.New("remote access is locked")

// ProtectLocalOperator routes every operator request through the same OS-user
// authority. It never exposes or persists the authority's signing credential.
func (client *Client) ProtectLocalOperator(brokerKey string) {
	client.protectLocalOperator(brokerKey, CallSecurity)
}

func (client *Client) protectLocalOperator(brokerKey string, call func(context.Context, SecurityRequest) (SecurityResponse, error)) {
	client.OperatorProof = func(ctx context.Context, path string, body []byte) (SecurityProof, error) {
		request := SecurityRequest{Action: "sign", Audience: brokerKey, Path: path, Digest: bodyDigest(body)}
		response, err := call(ctx, request)
		if errors.Is(err, ErrOperatorLocked) && client.ReauthenticateOperator != nil {
			if err := client.ReauthenticateOperator(ctx); err != nil {
				return SecurityProof{}, err
			}
			if err := ctx.Err(); err != nil {
				return SecurityProof{}, err
			}
			response, err = call(ctx, request)
		}
		if err != nil {
			return SecurityProof{}, err
		}
		if response.Proof == nil || response.Proof.BrokerKey != brokerKey {
			return SecurityProof{}, errors.New("OS security authority is bound to a different broker")
		}
		return *response.Proof, nil
	}
}
