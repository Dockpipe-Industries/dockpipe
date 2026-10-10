package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
)

func ValidateWorkerAuthority(config contract.WorkerConfig) error {
	if config.Delivery == nil || len(config.Profiles) > 0 {
		if err := ValidateProfiles(config.Profiles); err != nil {
			return err
		}
	}
	if config.Delivery != nil && (config.Delivery.TimeoutSeconds < 1 || config.Delivery.TimeoutSeconds > 86400) {
		return errors.New("delivery timeout must be between 1 and 86400 seconds")
	}
	return nil
}

func deliveryProfile(ctx context.Context, client *Client, root string, job contract.Job, permission *contract.DeliveryPermission) (contract.Profile, error) {
	if permission == nil {
		return contract.Profile{}, errors.New("workflow delivery is disabled on this worker; pair with --allow-delivery to approve operator-supplied code")
	}
	var bundle contract.Bundle
	request := map[string]string{"id": job.ID, "session": job.Session}
	if err := client.callWithRetry(ctx, "/v1/bundle", request, &bundle); err != nil {
		return contract.Profile{}, err
	}
	return StageBundle(ctx, root, job.Submission, bundle, permission.TimeoutSeconds)
}

// StageBundle creates a fresh per-job source directory after integrity checks.
// No existing checkout or installed/global package store is modified. Failed
// staging is retained for inspection; the journal prevents automatic execution.
func StageBundle(ctx context.Context, root string, submission contract.Submission, bundle contract.Bundle, timeout int) (contract.Profile, error) {
	if err := submission.Validate(); err != nil {
		return contract.Profile{}, err
	}
	digest, err := bundle.Digest()
	if err != nil || digest != submission.BundleHash {
		return contract.Profile{}, errors.New("delivery integrity check failed")
	}
	if err := validateBundlePackages(bundle); err != nil {
		return contract.Profile{}, err
	}
	parent := filepath.Join(root, "deliveries")
	if err := PrivateDirectory(parent); err != nil {
		return contract.Profile{}, err
	}
	directory := filepath.Join(parent, submission.ID)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return contract.Profile{}, fmt.Errorf("delivery staging must be new: %w", err)
	}
	stage, err := os.OpenRoot(directory)
	if err != nil {
		return contract.Profile{}, err
	}
	defer stage.Close()
	for _, file := range bundle.Files {
		if err := ctx.Err(); err != nil {
			return contract.Profile{}, err
		}
		if err := stage.MkdirAll(filepath.Dir(file.Path), 0o700); err != nil {
			return contract.Profile{}, err
		}
		mode := os.FileMode(0o600)
		if file.Executable {
			mode = 0o700
		}
		output, err := stage.OpenFile(file.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return contract.Profile{}, err
		}
		_, writeErr := output.Write(file.Data)
		closeErr := output.Close()
		if writeErr != nil {
			return contract.Profile{}, writeErr
		}
		if closeErr != nil {
			return contract.Profile{}, closeErr
		}
	}
	// A generated, minimal project configuration selects only the delivered
	// package store. Never copy the sender's project config or secret settings.
	config := map[string]any{"schema": 1, "compile": map[string]any{"workflows": []string{"store/workflows"}}, "packages": map[string]any{"sources": []map[string]string{{"kind": "store", "path": "store"}}}, "secrets": map[string]string{"vault": "none"}}
	data, err := json.Marshal(config)
	if err != nil {
		return contract.Profile{}, err
	}
	if err := stage.WriteFile("dockpipe.config.json", data, 0o600); err != nil {
		return contract.Profile{}, err
	}
	for _, file := range bundle.Files {
		if file.Path == bundle.WorkflowFile || strings.HasSuffix(file.Path, "/config.yml") && strings.HasPrefix(file.Path, "store/workflows/") {
			if err := infrastructure.ValidateWorkflowYAML(filepath.Join(directory, file.Path)); err != nil {
				return contract.Profile{}, fmt.Errorf("delivered workflow validation failed: %w", err)
			}
		}
	}
	return contract.Profile{Workdir: directory, WorkflowFile: filepath.Join(directory, bundle.WorkflowFile), TimeoutSeconds: timeout, Artifacts: bundle.Artifacts}, nil
}
