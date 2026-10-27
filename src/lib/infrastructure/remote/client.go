package remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type Client struct {
	Identity  *remoteidentity.Identity
	BrokerKey string
	Endpoint  string
	Token     string
	HTTP      *http.Client
}

func NewClient(endpoint, token string) (*Client, error) {
	if err := contract.Endpoint(endpoint); err != nil {
		return nil, err
	}
	return &Client{
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		Token:    token,
		HTTP: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("remote redirects are forbidden") },
			Transport: &http.Transport{
				TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS13},
				MaxResponseHeaderBytes: 16 << 10,
				IdleConnTimeout:        30 * time.Second,
			},
		},
	}, nil
}

func (c *Client) Call(ctx context.Context, path string, input, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if len(data) > contract.MaxBody {
		return errors.New("remote request exceeds size limit")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("Content-Type", "application/json")
	nonce := ""
	if c.Identity != nil {
		if remoteidentity.Fingerprint(c.BrokerKey) == "" {
			return errors.New("paired broker machine identity is required")
		}
		nonce, err = c.machineNonce(request)
		if err != nil {
			return err
		}
		signature, signErr := c.Identity.Sign(machineMessage("request", nonce, path, 0, data))
		if signErr != nil {
			return signErr
		}
		request.Header.Set(machineNonceHeader, nonce)
		request.Header.Set(machineSignatureHeader, signature)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return fmt.Errorf("remote connection failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, contract.MaxBody+1))
	if err != nil {
		return err
	}
	if len(body) > contract.MaxBody {
		return errors.New("remote response exceeds size limit")
	}
	if c.Identity != nil && !remoteidentity.Verify(c.BrokerKey, machineMessage("response", nonce, path, response.StatusCode, body), response.Header.Get(machineSignatureHeader)) {
		return errors.New("broker machine response verification failed")
	}
	if response.StatusCode != http.StatusOK {
		// Provider error pages and reflected content are not safe diagnostics.
		return &HTTPError{StatusCode: response.StatusCode}
	}
	if output == nil {
		return nil
	}
	return Decode(body, output)
}

// NewWorkerClient retains legacy paired workers, while new pairings require
// proof from both persistent machine identities on every worker request.
func NewWorkerClient(config contract.WorkerConfig) (*Client, error) {
	client, err := NewClient(config.Endpoint, config.Token)
	if err != nil {
		return nil, err
	}
	if config.MachineKey == "" && len(config.MachinePrivateKey) == 0 && config.BrokerKey == "" {
		return client, nil
	}
	identity := remoteidentity.Identity{PublicKey: config.MachineKey, PrivateKey: config.MachinePrivateKey}
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	if remoteidentity.Fingerprint(config.BrokerKey) == "" {
		return nil, errors.New("paired broker machine identity is required")
	}
	client.Identity = &identity
	client.BrokerKey = config.BrokerKey
	return client, nil
}
