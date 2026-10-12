package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func pairOnline(ctx context.Context, root, endpoint, node string, allowDelivery bool, timeout int) error {
	if err := contract.Endpoint(endpoint); err != nil {
		return err
	}
	if !contract.ValidID(node) || !allowDelivery {
		return errors.New("online pairing requires --node and explicit --allow-delivery approval")
	}
	permission := &contract.DeliveryPermission{TimeoutSeconds: timeout}
	config := contract.WorkerConfig{Schema: contract.Version, Endpoint: endpoint, Node: node,
		Profiles: map[string]contract.Profile{}, Delivery: permission}
	if err := remoteio.ValidateWorkerAuthority(config); err != nil {
		return err
	}
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "worker.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(root, "worker.json")
	var saved contract.WorkerConfig
	if err := remoteio.ReadPrivate(path, &saved); err == nil {
		if saved.Schema != config.Schema || saved.Endpoint != endpoint || saved.Node != node ||
			!reflect.DeepEqual(saved.Delivery, permission) || len(saved.Profiles) != 0 {
			return errors.New("saved worker identity or authority differs; inspect it before changing pairing")
		}
		config = saved
	} else if !os.IsNotExist(err) {
		return err
	} else {
		config.Token, err = remoteio.Secret()
		if err != nil {
			return err
		}
		// Persist before sending. Cancellation and lost responses retain the same
		// identity on retry, and never issue a second worker credential.
		if err := remoteio.WritePrivate(path, config); err != nil {
			return err
		}
	}
	client, err := remoteio.NewClient(endpoint, "")
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	lastCode := ""
	rotated := false
	for {
		var status contract.PairingStatus
		request := contract.PairingRequest{Node: node, Token: config.Token}
		if err := client.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
			return fmt.Errorf("pairing request failed; open pairing on the broker and retry with the same options: %w", err)
		}
		if status.Node != node {
			return errors.New("pairing response changed node identity")
		}
		if status.Code != lastCode || status.Status != "pending" {
			if err := json.NewEncoder(os.Stdout).Encode(status); err != nil {
				return err
			}
			lastCode = status.Code
			if status.Status == "pending" {
				fmt.Fprintf(os.Stderr, "Verification code: %s\nOn your broker, compare this code and approve it in Machines or run: dockpipe remote approve --code %s\n", status.Code, status.Code)
			}
		}
		switch status.Status {
		case "revoked":
			if rotated {
				return errors.New("replacement worker credential was revoked; pairing stopped")
			}
			// Only an explicit pairing operation rotates a revoked identity. The
			// worker service never re-enrolls itself or restores access automatically.
			config.Token, err = remoteio.Secret()
			if err != nil {
				return err
			}
			if err := remoteio.WritePrivate(path, config); err != nil {
				return err
			}
			rotated = true
			lastCode = ""
			fmt.Fprintln(os.Stderr, "Previous access was removed. Requesting fresh approval with a new credential.")
			continue
		case "approved":
			fmt.Fprintln(os.Stderr, "Paired. Start this worker with: dockpipe remote service --role worker")
			return nil
		case "denied":
			return errors.New("pairing denied by broker")
		case "pending":
			if !time.Now().Before(status.ExpiresAt) {
				return errors.New("pairing expired")
			}
		default:
			return errors.New("invalid pairing status")
		}
		if lastCode != "" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func pairingOperator(ctx context.Context, root, command, code string) error {
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	var result json.RawMessage
	if err := client.Call(ctx, "/v1/"+command, map[string]string{"code": code}, &result); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
