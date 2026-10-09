package remotecmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// A separate file preserves compatibility with existing operator/edge state.
// Hosted credentials stay private; info uses an explicit public projection.
type connectionConfig struct {
	Schema     string                          `json:"schema"`
	Mode       string                          `json:"mode"`
	Endpoint   string                          `json:"endpoint"`
	Resolver   infrastructure.ResolverMetadata `json:"resolver"`
	Token      string                          `json:"token,omitempty"`
	EdgeDigest string                          `json:"edge_digest,omitempty"`
}

func readConnection(root string) (*connectionConfig, error) {
	var config connectionConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "connection.json"), &config); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if config.Schema != contract.Version || (config.Mode != "local" && config.Mode != "hosted") {
		return nil, errors.New("invalid remote connection metadata")
	}
	if err := contract.Endpoint(config.Endpoint); err != nil {
		return nil, err
	}
	if config.Mode == "hosted" {
		for _, name := range []string{"operator.json", "broker.json", "edge.json"} {
			if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
				return nil, errors.New("hosted and local broker state overlap; preserve state and select separate connection directories")
			}
		}
		if err := validateHostedCredential(config.Endpoint, config.Token); err != nil {
			return nil, err
		}
	}
	return &config, nil
}

func validateHostedCredential(endpoint, token string) error {
	if err := contract.Endpoint(endpoint); err != nil {
		return err
	}
	if !strings.HasPrefix(endpoint, "https://") || len(token) < 32 || len(token) > 8192 {
		return errors.New("hosted broker requires an HTTPS origin and a bounded private credential")
	}
	for _, character := range token {
		if character < 33 || character > 126 {
			return errors.New("hosted broker credential must contain only visible ASCII characters")
		}
	}
	return nil
}

func edgeDigest(edge contract.EdgeConfig) string {
	data, _ := json.Marshal(edge)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func operatorClient(root string) (*remoteio.Client, error) {
	connection, err := readConnection(root)
	if err != nil {
		return nil, err
	}
	if connection != nil && connection.Mode == "hosted" {
		return remoteio.NewClient(connection.Endpoint, connection.Token)
	}
	var config OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &config); err != nil {
		return nil, err
	}
	if err := validateListen(config.Listen); err != nil {
		return nil, err
	}
	// Locally hosted administration stays on loopback, never on its public edge.
	return remoteio.NewClient("http://"+config.Listen, config.Token)
}
