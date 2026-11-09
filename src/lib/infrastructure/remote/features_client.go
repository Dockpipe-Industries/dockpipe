package remote

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

// NewFeatureRequest creates a new intent. Transport retries must reuse this
// exact object; rebuilding it with a fresh nonce authorizes distinct work.
func NewFeatureRequest(target contract.FeatureTarget, root, action string, payload any, deadline time.Time) (contract.FeatureRequest, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return contract.FeatureRequest{}, err
	}
	nonce, err := Secret()
	if err != nil {
		return contract.FeatureRequest{}, err
	}
	request := contract.FeatureRequest{Schema: contract.FeatureVersion, Target: target, Root: root, Action: action, Payload: encoded, Nonce: nonce, Deadline: deadline.UTC()}
	request.ID, err = request.Digest()
	if err != nil {
		return contract.FeatureRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return contract.FeatureRequest{}, err
	}
	if err := validateFeaturePayload(request); err != nil {
		return contract.FeatureRequest{}, err
	}
	return request, nil
}

func (client *Client) FeatureCatalog(ctx context.Context) ([]contract.FeatureCatalog, error) {
	var catalogs []contract.FeatureCatalog
	err := client.Call(ctx, "/v1/features-catalog", map[string]string{"schema": contract.FeatureVersion}, &catalogs)
	if err != nil {
		return nil, err
	}
	if len(catalogs) > 64 {
		return nil, errors.New("feature catalog node limit exceeded")
	}
	for _, catalog := range catalogs {
		if err := catalog.ValidateOperatorCatalog(); err != nil {
			return nil, err
		}
	}
	return catalogs, nil
}

func (client *Client) SubmitFeature(ctx context.Context, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	var operation contract.FeatureOperation
	if err := request.Validate(); err != nil {
		return operation, err
	}
	path := "/v1/features-write-submit"
	if contract.FeatureAccess(request.Action) == "read" {
		path = "/v1/features-read-submit"
	}
	err := client.Call(ctx, path, request, &operation)
	if err == nil && operation.Request.ID != request.ID {
		return operation, errors.New("feature response identity changed")
	}
	return operation, err
}

// FeatureStatus renews only the same operation's authenticated read/write lease.
// The caller must continue polling while it owns an active operation; worker
// heartbeats alone never renew OS-session authority.
func (client *Client) FeatureStatus(ctx context.Context, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	if err := request.Validate(); err != nil {
		return contract.FeatureOperation{}, err
	}
	return client.FeatureStatusReference(ctx, request.Reference(), request.Action)
}

// FeatureStatusReference recovers status from a non-sensitive operation receipt.
// The broker checks the stored operation's access class independently of action.
func (client *Client) FeatureStatusReference(ctx context.Context, reference contract.FeatureReference, action string) (contract.FeatureOperation, error) {
	var operation contract.FeatureOperation
	if err := reference.Validate(); err != nil {
		return operation, err
	}
	if contract.FeatureAccess(action) == "" {
		return operation, errors.New("unsupported feature status action")
	}
	path := "/v1/features-write-status"
	if contract.FeatureAccess(action) == "read" {
		path = "/v1/features-read-status"
	}
	err := client.Call(ctx, path, reference, &operation)
	if err == nil && operation.Request.ID != reference.ID {
		return operation, errors.New("feature response identity changed")
	}
	return operation, err
}

func (client *Client) CancelFeature(ctx context.Context, reference contract.FeatureReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	return client.Call(ctx, "/v1/features-cancel", reference, nil)
}

// CancelFeatureResult retains the broker's synchronous operation receipt. Its
// terminal result can outlive broker reclamation; cancellation alone is never
// proof that an executed native mutation had no effects.
func (client *Client) CancelFeatureResult(ctx context.Context, reference contract.FeatureReference) (contract.FeatureOperation, error) {
	var operation contract.FeatureOperation
	if err := reference.Validate(); err != nil {
		return operation, err
	}
	err := client.Call(ctx, "/v1/features-cancel", reference, &operation)
	if err == nil && operation.Request.Reference() != reference {
		return contract.FeatureOperation{}, errors.New("feature cancellation response identity changed")
	}
	return operation, err
}

// ReconcileFeature revokes the captured invocation and asks the idle worker for
// cleanup proof. It never resubmits a payload or infers a successful mutation.
func (client *Client) ReconcileFeature(ctx context.Context, reference contract.FeatureReference) (contract.FeatureOperation, error) {
	return client.ReconcileFeatureChoice(ctx, reference, "verify")
}

func (client *Client) ReconcileFeatureChoice(ctx context.Context, reference contract.FeatureReference, choice string) (contract.FeatureOperation, error) {
	return client.ReconcileFeatureRecovery(ctx, reference, choice, "")
}

func (client *Client) ReconcileFeatureRecovery(ctx context.Context, reference contract.FeatureReference, choice, resource string) (contract.FeatureOperation, error) {
	var operation contract.FeatureOperation
	if err := reference.Validate(); err != nil {
		return operation, err
	}
	if choice != "verify" && choice != "retire-container" {
		return operation, errors.New("unsupported feature recovery choice")
	}
	err := client.Call(ctx, "/v1/features-reconcile", contract.FeatureRecoveryRequest{FeatureReference: reference, Choice: choice, Resource: resource}, &operation)
	if err == nil && operation.Request.Reference() != reference {
		return operation, errors.New("feature reconciliation response identity changed")
	}
	return operation, err
}
