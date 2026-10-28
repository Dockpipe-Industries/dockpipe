package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func activateEdge(ctx context.Context, root, path, approval string, preview bool) error {
	var request contract.EdgeActivationRequest
	if err := remoteio.ReadPrivate(path, &request); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if preview && approval != "" {
		return errors.New("choose dry-run or exact approval")
	}
	if !preview && approval != request.ID() {
		return errors.New("edge activation approval does not match exact request")
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	operation := "/v1/edge-activate"
	if preview {
		operation = "/v1/edge-plan"
	}
	var result contract.EdgeActivationStatus
	if err := client.Call(ctx, operation, request, &result); err != nil {
		return fmt.Errorf("edge request not acknowledged; inspect edge-status --id %s before resubmitting: %w", request.ID(), err)
	}
	if result.ID != request.ID() || result.Schema != contract.EdgeActivationSchema {
		return errors.New("broker returned a different activation identity")
	}
	if preview {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	ctx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	for result.Status == "accepted" || result.Status == "activating" || result.Status == "recovering" {
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("activation observation ended; broker retains bounded recovery authority; inspect edge-status --id %s", request.ID())
		case <-timer.C:
		}
		if err := client.Call(ctx, "/v1/edge-status", map[string]string{"id": request.ID()}, &result); err != nil {
			return fmt.Errorf("activation status is unknown; inspect edge-status --id %s without replay: %w", request.ID(), err)
		}
	}
	if result.ID != request.ID() || result.Schema != contract.EdgeActivationSchema {
		return errors.New("broker returned a different activation identity")
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return err
	}
	if result.Status != "healthy" {
		return fmt.Errorf("activation ended with %s; inspect broker recovery status", result.Status)
	}
	return nil
}

func edgeStatus(ctx context.Context, root, id string) error {
	if !contract.ValidEdgeDigest(id) {
		return errors.New("edge-status requires an exact activation ID")
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	var status contract.EdgeActivationStatus
	if err := client.Call(ctx, "/v1/edge-status", map[string]string{"id": id}, &status); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(status)
}
