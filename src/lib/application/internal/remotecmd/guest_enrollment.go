package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func guestRequestCommand(command string) bool {
	return command == "guest-grant" || command == "guest-prepare" || command == "guest-enroll"
}

func guestCommand(ctx context.Context, root, command, requestPath, id string) error {
	if guestRequestCommand(command) && requestPath == "" {
		return errors.New("guest enrollment requires --request with an owner-private JSON file")
	}

	switch command {
	case "guest-prepare":
		return prepareGuest(root, requestPath)
	case "guest-enroll", "guest-release":
		var host contract.WorkerConfig
		if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &host); err != nil {
			return err
		}
		if host.GuestGrant != "" || host.MachineKey == "" {
			return errors.New("guest enrollment requires the paired host identity")
		}
		client, err := remoteio.NewWorkerClient(host)
		if err != nil {
			return err
		}
		defer client.HTTP.CloseIdleConnections()

		if command == "guest-release" {
			if !contract.ValidEdgeDigest(id) {
				return errors.New("guest grant release requires an exact --id")
			}
			return endGuestGrant(ctx, client, "/v1/guest-release", id)
		}
		var request contract.GuestEnrollmentRequest
		if err := remoteio.ReadPrivate(requestPath, &request); err != nil {
			return err
		}
		var receipt contract.GuestEnrollmentReceipt
		if err := client.Call(ctx, "/v1/guest-enroll", request, &receipt); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(receipt)
	}

	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()

	if command == "guest-grant-revoke" {
		if !contract.ValidEdgeDigest(id) {
			return errors.New("guest grant revocation requires an exact --id")
		}
		return endGuestGrant(ctx, client, "/v1/guest-grant-revoke", id)
	}

	var request contract.GuestGrantRequest
	if err := remoteio.ReadPrivate(requestPath, &request); err != nil {
		return err
	}
	var receipt contract.GuestGrantReceipt
	if err := client.Call(ctx, "/v1/guest-grant", request, &receipt); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(receipt)
}

func endGuestGrant(ctx context.Context, client *remoteio.Client, path, id string) error {
	var receipt contract.GuestReleaseReceipt
	if err := client.Call(ctx, path, map[string]string{"id": id}, &receipt); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(receipt)
}

func prepareGuest(root, requestPath string) error {
	var input struct {
		Endpoint string                     `json:"endpoint"`
		Node     string                     `json:"node"`
		Grant    contract.GuestGrantReceipt `json:"grant"`
	}
	if err := remoteio.ReadPrivate(requestPath, &input); err != nil {
		return err
	}
	grant := input.Grant
	if contract.Endpoint(input.Endpoint) != nil || !contract.ValidID(input.Node) ||
		!contract.ValidID(grant.Run) || !strings.HasPrefix(input.Node, grant.Run+"-") ||
		!contract.ValidEdgeDigest(grant.ID) || remoteidentity.Fingerprint(grant.BrokerKey) == "" ||
		!time.Now().Before(grant.ExpiresAt) {
		return errors.New("invalid or expired guest preparation request")
	}
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := remoteio.LockWorker(root)
	if err != nil {
		return err
	}
	defer unlock()

	path := filepath.Join(root, "worker.json")
	var config contract.WorkerConfig
	err = remoteio.ReadPrivate(path, &config)
	switch {
	case err == nil:
		if config.Schema != contract.Version || config.GuestGrant != grant.ID || config.Node != input.Node || config.Endpoint != input.Endpoint ||
			config.BrokerKey != grant.BrokerKey || config.Features || config.Delivery == nil || len(config.Profiles) != 0 ||
			!contract.ValidEdgeDigest(config.Token) {
			return errors.New("existing worker state does not match this guest enrollment")
		}
	case os.IsNotExist(err):
		identity, err := remoteidentity.New()
		if err != nil {
			return err
		}
		token, err := remoteio.Secret()
		if err != nil {
			return err
		}
		config = contract.WorkerConfig{
			Schema: contract.Version, Node: input.Node, Endpoint: input.Endpoint,
			GuestGrant: grant.ID, BrokerKey: grant.BrokerKey, Token: token,
			MachineKey: identity.PublicKey, MachinePrivateKey: identity.PrivateKey,
			Profiles: map[string]contract.Profile{}, Delivery: &contract.DeliveryPermission{TimeoutSeconds: 3600},
		}
		if err := remoteio.WritePrivate(path, config); err != nil {
			return err
		}
	default:
		return err
	}

	claim := contract.GuestEnrollment{
		Grant: grant.ID, Node: config.Node, PublicKey: config.MachineKey, TokenHash: remoteio.Hash(config.Token),
	}
	identity := remoteidentity.Identity{PublicKey: config.MachineKey, PrivateKey: config.MachinePrivateKey}
	proof, err := identity.Sign(claim.ProofMessage(grant.BrokerKey))
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(contract.GuestEnrollmentRequest{GuestEnrollment: claim, Proof: proof})
}
