package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func decodeDesktopOpen(data []byte, now time.Time) (contract.DesktopOpen, error) {
	var request contract.DesktopOpen
	if err := decodeFeatureInput(data, &request); err != nil {
		return request, err
	}
	if request.ID == "" {
		request.ID, _ = request.Digest()
	}
	if request.Validate() != nil || !request.Deadline.After(now) || request.Deadline.After(now.Add(contract.DesktopLifetime)) {
		return request, errors.New("desktop intent requires an exact identity and deadline within one hour")
	}
	return request, nil
}

type desktopPrepared struct {
	Type      string                    `json:"type"`
	Reference contract.DesktopReference `json:"reference"`
	Provider  contract.DesktopProvider  `json:"provider"`
	Mode      string                    `json:"mode"`
	Deadline  time.Time                 `json:"deadline"`
}

func preparedDesktop(request contract.DesktopOpen) desktopPrepared {
	return desktopPrepared{Type: "prepared", Reference: request.Reference(), Provider: request.Provider, Mode: request.Mode, Deadline: request.Deadline}
}

func prepareDesktop(ctx context.Context, input string, output io.Writer) error {
	data, err := readFeatureInput(ctx, input)
	if err != nil {
		return err
	}
	request, err := decodeDesktopOpen(data, time.Now())
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(preparedDesktop(request))
}

func configureDesktop(ctx context.Context, root, input string) error {
	var config contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
		return errors.New("desktop permissions require an already paired worker")
	}
	client, err := remoteio.NewWorkerClient(config)
	if err != nil || client.Identity == nil {
		return errors.New("desktop permissions require valid paired machine keys")
	}
	defer client.HTTP.CloseIdleConnections()
	var permissions remoteio.DesktopPermissions
	if input == "" {
		permissions, err = remoteio.LoadDesktopPermissions(root)
	} else {
		var data []byte
		data, err = readFeatureInput(ctx, input)
		if err == nil {
			err = decodeFeatureInput(data, &permissions)
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			if permissions.View || permissions.Input {
				err = requireDesktopHosting()
			}
		}
		if err == nil {
			err = remoteio.SaveDesktopPermissions(root, permissions)
		}
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(permissions)
}

func desktopCommand(ctx context.Context, root, command, input string) error {
	if command == "desktop-config" {
		return configureDesktop(ctx, root, input)
	}
	ctx, cancel := context.WithTimeout(ctx, featureCommandTimeout)
	defer cancel()
	var data []byte
	var err error
	if command != "desktop-catalog" {
		data, err = readFeatureInput(ctx, input)
		if err != nil {
			return err
		}
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	var result any
	switch command {
	case "desktop-catalog":
		result, err = client.DesktopCatalog(ctx)
	case "desktop-open":
		var request contract.DesktopOpen
		request, err = decodeDesktopOpen(data, time.Now())
		if err == nil {
			result, err = client.OpenDesktop(ctx, request)
		}
	case "desktop-signal":
		var signal contract.DesktopSignal
		err = decodeFeatureInput(data, &signal)
		if err == nil {
			result, err = client.SignalDesktop(ctx, signal)
		}
	case "desktop-close":
		var reference contract.DesktopReference
		err = decodeFeatureInput(data, &reference)
		if err == nil {
			result, err = client.CloseDesktop(ctx, reference)
		}
	case "desktop-status", "desktop-pulse":
		var read contract.DesktopRead
		err = decodeFeatureInput(data, &read)
		if err == nil && command == "desktop-status" {
			result, err = client.DesktopStatus(ctx, read)
		} else if err == nil {
			result, err = client.PulseDesktop(ctx, read)
		}
	default:
		return errors.New("unsupported desktop control operation")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func desktopRequestCommand(command string) bool {
	switch command {
	case "desktop-administrator-approval", "desktop-administrator-grant", "desktop-administrator-service", "desktop-administrator-relay",
		"desktop-cleanup", "desktop-relay", "desktop", "desktop-config", "desktop-prepare", "desktop-open", "desktop-status", "desktop-pulse", "desktop-signal", "desktop-close":
		return true
	}
	return false
}
