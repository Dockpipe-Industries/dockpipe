package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type featureRecoveryClient interface {
	ReconcileFeature(context.Context, contract.FeatureReference) (contract.FeatureOperation, error)
	FeatureStatusReference(context.Context, contract.FeatureReference, string) (contract.FeatureOperation, error)
}

type featureRecoveryChoiceClient struct {
	*remoteio.Client
	choice   string
	resource string
}

func (client featureRecoveryChoiceClient) ReconcileFeature(ctx context.Context, reference contract.FeatureReference) (contract.FeatureOperation, error) {
	return client.Client.ReconcileFeatureRecovery(ctx, reference, client.choice, client.resource)
}

func reconcileFeature(ctx context.Context, root, input, choice, resource string) error {
	ctx, cancel := context.WithTimeout(ctx, featureCommandTimeout)
	defer cancel()
	data, err := readFeatureInput(ctx, input)
	if err != nil {
		return err
	}
	request, err := decodeFeatureReceipt(data)
	if err != nil {
		return err
	}
	if contract.FeatureAccess(request.Action) == "read" || choice == "retire-container" && !strings.HasPrefix(request.Action, "containers.") {
		return errors.New("recovery strategy does not match the captured mutation")
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	operation, recoveryErr := awaitFeatureReconciliation(ctx, featureRecoveryChoiceClient{Client: client, choice: choice, resource: resource}, request)
	if err := json.NewEncoder(os.Stdout).Encode(operation); err != nil {
		return err
	}
	return recoveryErr
}

func awaitFeatureReconciliation(ctx context.Context, client featureRecoveryClient, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	operation, err := client.ReconcileFeature(ctx, request.Reference())
	for err == nil {
		if err = validateFeatureOperation(operation, request); err != nil {
			break
		}
		if operation.RecoveryError != "" {
			operation.Request.Payload = nil
			operation.Request.Nonce = ""
			operation.ReceiptOnly = true
			if operation.Result == nil {
				return operation, errors.New("worker cleanup remains unconfirmed")
			}
			return operation, errors.New(operation.Result.Error)
		}
		if operation.Reconciled || operation.Status == "success" || operation.Status == "failure" || operation.Status == "cancelled" {
			operation.Request.Payload = nil
			operation.Request.Nonce = ""
			operation.ReceiptOnly = true
			if operation.Result != nil {
				operation.Result = &contract.FeatureResult{Status: operation.Result.Status}
			}
			return operation, nil
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			err = ctx.Err()
		case <-timer.C:
			operation, err = client.FeatureStatusReference(ctx, request.Reference(), request.Action)
		}
	}
	// A failed query is not proof of revocation or cleanup. Retain only the
	// captured reference and do not copy unverified response fields to output.
	unknown := unknownFeatureReceipt(request, false)
	if errors.Is(err, remoteio.ErrOperatorLocked) {
		return unknown, err
	}
	return unknown, errors.New("reconciliation could not be confirmed; inspect the captured operation when the worker is reachable, and refresh remote state after cleanup is acknowledged")
}
