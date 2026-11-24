package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type desktopAdministratorServiceConfig struct {
	Schema    string `json:"schema"`
	Node      string `json:"node"`
	Endpoint  string `json:"endpoint"`
	Token     string `json:"token"`
	BrokerKey string `json:"broker_key"`
}

func desktopAdministratorServiceClient(root string) (*remoteio.Client, string, error) {
	var config desktopAdministratorServiceConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "desktop-administrator-service.json"), &config); err != nil {
		return nil, "", errors.New("administrator input service has no protected broker configuration")
	}
	if config.Schema != "dockpipe.remote.desktop-admin-service/v1" || !contract.ValidID(config.Node) ||
		len(config.Token) != 64 || remoteidentity.Fingerprint(config.BrokerKey) == "" {
		return nil, "", errors.New("invalid protected administrator input service configuration")
	}
	client, err := remoteio.NewClient(config.Endpoint, config.Token)
	if err != nil {
		return nil, "", err
	}
	client.BrokerKey = config.BrokerKey
	return client, config.Node, nil
}

func desktopAdministratorCommand(parent context.Context, root, command, path string) error {
	ctx, cancel := context.WithTimeout(parent, featureCommandTimeout)
	defer cancel()

	data, err := readFeatureInput(ctx, path)
	if err != nil {
		return err
	}
	var result any
	if command == "desktop-administrator-service" {
		client, node, err := desktopAdministratorServiceClient(root)
		if err != nil {
			return err
		}
		defer client.HTTP.CloseIdleConnections()
		var request contract.DesktopAdministratorServiceRequest
		if err := decodeFeatureInput(data, &request); err != nil || request.Node != node {
			return errors.New("administrator service request does not match its protected node")
		}

		result, err = client.PollDesktopAdministratorService(ctx, request)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}

	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	switch command {
	case "desktop-administrator-approval":
		var request contract.DesktopAdministratorApprovalRequest
		if err := decodeFeatureInput(data, &request); err != nil {
			return err
		}
		result, err = client.DesktopAdministratorApproval(ctx, request)
	case "desktop-administrator-grant":
		var consent contract.DesktopAdministratorEnvelope
		if err := decodeFeatureInput(data, &consent); err != nil {
			return err
		}
		result, err = client.GrantDesktopAdministratorInput(ctx, consent)
	default:
		return errors.New("unknown desktop administrator command")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func desktopAdministratorRelayBridge(parent context.Context, root, path string) error {
	if path == "" || path == "-" {
		return errors.New("administrator relay requires an exact private request file")
	}
	data, err := readFeatureInput(parent, path)
	if err != nil {
		return err
	}
	var request contract.DesktopRelayRequest
	if decodeFeatureInput(data, &request) != nil || request.Validate() != nil {
		return errors.New("invalid administrator relay request")
	}

	var client *remoteio.Client
	if request.Role == "worker" {
		var node string
		client, node, err = desktopAdministratorServiceClient(root)
		if err == nil && node != request.Reference.Target.Node {
			return errors.New("administrator relay changed its protected service node")
		}
	} else {
		client, err = operatorClient(root)
	}
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	ticket, err := client.DesktopAdministratorRelayTicket(ctx, request)
	if request.Role == "worker" {
		// The service can launch its relay after approval, just before the
		// viewer's authenticated grant arrives. Retry issuance, never attach.
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for err != nil && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-ticker.C:
				ticket, err = client.DesktopAdministratorRelayTicket(ctx, request)
			}
		}
	}
	if err != nil {
		cancel()
		return err
	}
	relay, err := client.AttachDesktopAdministratorRelay(ctx, ticket)
	cancel()
	if err != nil {
		return err
	}
	defer relay.Close()
	return streamDesktopRelay(parent, relay, os.Stdin, os.Stdout)
}
