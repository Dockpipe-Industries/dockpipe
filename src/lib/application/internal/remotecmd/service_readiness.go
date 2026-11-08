package remotecmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func waitForBrokerService(ctx context.Context, root string) error {
	var config OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &config); err != nil {
		return err
	}
	if err := validateListen(config.Listen); err != nil {
		return err
	}
	client, err := remoteio.NewClient("http://"+config.Listen, config.Token)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	return waitForBrokerReadiness(ctx, client, remoteio.CallSecurity)
}

// A service manager can report running before the process has opened either
// listener. Probe only read-only readiness; never unlock or replay an action.
func waitForBrokerReadiness(ctx context.Context, client *remoteio.Client,
	security func(context.Context, remoteio.SecurityRequest) (remoteio.SecurityResponse, error)) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastError error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("broker service did not become ready (%v): %w", lastError, err)
		}
		attempt, cancel := context.WithTimeout(ctx, time.Second)
		response, err := security(attempt, remoteio.SecurityRequest{Action: "status"})
		if err == nil && (response.Status == nil || !response.Status.AuthorityAvailable) {
			err = errors.New("OS remote authority is not ready")
		}
		if err == nil {
			var health struct {
				Schema string `json:"schema"`
			}
			err = client.Call(attempt, "/v1/health", struct{}{}, &health)
			if err == nil && health.Schema != contract.Version {
				err = errors.New("broker returned an unsupported health response")
			}
		}
		cancel()
		if err == nil {
			return nil
		}
		lastError = err
		select {
		case <-ctx.Done():
			return fmt.Errorf("broker service did not become ready (%v): %w", lastError, ctx.Err())
		case <-ticker.C:
		}
	}
}
