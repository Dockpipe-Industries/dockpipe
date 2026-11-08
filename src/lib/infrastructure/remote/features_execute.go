package remote

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/workspacefiles"
	"dockpipe/src/lib/infrastructure/workspacegit"
)

type featureContainerRequest struct {
	Container string `json:"container,omitempty"`
}

// ValidateFeaturePayload is shared by CLI and broker admission. Domain inputs
// retain their own semantic validation at execution; unknown wire fields and
// caller-selected host roots are rejected before submission.
func ValidateFeaturePayload(request contract.FeatureRequest) error {
	return validateFeaturePayload(request)
}

// Decoding is strict and action-specific. Payloads never select a local root,
// executable, Docker context, arbitrary argv, or another action.
func validateFeaturePayload(request contract.FeatureRequest) error {
	if contract.FeatureAccess(request.Action) == "" {
		return errors.New("unsupported remote feature action")
	}
	switch {
	case strings.HasPrefix(request.Action, "files."):
		var input workspacefiles.Request
		if decodeFeature(request.Payload, &input) != nil || input.Schema != workspacefiles.Schema || "files."+input.Action != request.Action {
			return errors.New("invalid file feature payload")
		}
	case strings.HasPrefix(request.Action, "git."):
		var input workspacegit.WorkspaceGitRequest
		if decodeFeature(request.Payload, &input) != nil || input.Workdir != "" || "git."+input.Action != request.Action {
			return errors.New("Git feature payload cannot select a host directory or different action")
		}
	case strings.HasPrefix(request.Action, "containers."):
		var input featureContainerRequest
		if decodeFeature(request.Payload, &input) != nil {
			return errors.New("invalid container feature payload")
		}
		if request.Action == "containers.snapshot" {
			if input.Container != "" {
				return errors.New("container snapshot does not select a container")
			}
		} else if len(input.Container) != 64 || strings.Trim(input.Container, "0123456789abcdef") != "" {
			return errors.New("container control requires an exact full container ID")
		}
	default:
		return errors.New("unsupported feature executor")
	}
	return nil
}

func executeFeature(ctx context.Context, permissions FeaturePermissions, request contract.FeatureRequest, identity invocation.Identity, authorize invocation.Authorize) contract.FeatureResult {
	if err := request.Validate(); err != nil || validateFeaturePayload(request) != nil || !featureCatalogAllows(permissions.catalog(request.Target), request) {
		return contract.FeatureResult{Status: "failure", Error: "Feature request or worker-local grant rejected."}
	}
	target := invocation.Target{Kind: invocation.Remote, ID: request.Target.Node, Fingerprint: request.Target.Fingerprint, Generation: request.Target.Generation}
	var value any
	var err error
	var root string
	for _, permission := range permissions.Roots {
		if permission.ID == request.Root {
			root = permission.Path
			break
		}
	}
	switch {
	case strings.HasPrefix(request.Action, "files."):
		var input workspacefiles.Request
		err = decodeFeature(request.Payload, &input)
		if err == nil {
			var service *workspacefiles.Service
			service, err = workspacefiles.New(root, target, identity, authorize)
			if err == nil {
				value, err = service.Run(ctx, input)
				closeErr := service.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
	case strings.HasPrefix(request.Action, "git."):
		var input workspacegit.WorkspaceGitRequest
		err = decodeFeature(request.Payload, &input)
		if err == nil {
			input.Workdir = root
			value, err = workspacegit.RunWorkspaceGitWithAuthority(ctx, input, target, identity, authorize)
		}
	case strings.HasPrefix(request.Action, "containers."):
		value, err = executeFeatureContainer(ctx, request, target, identity, authorize)
	}
	if err != nil {
		if ctx.Err() != nil {
			return contract.FeatureResult{Status: "cancelled", Error: "Feature authority expired or execution was cancelled; inspect state before a new mutation."}
		}
		return contract.FeatureResult{Status: "failure", Error: "Native feature operation rejected or failed; refresh the selected resource."}
	}
	payload, err := json.Marshal(value)
	if err != nil || len(payload) > contract.MaxBody-4096 {
		// Execution may already have changed state. Never relabel this a safe retry.
		return contract.FeatureResult{Status: "unknown", Error: "Execution completed but its result exceeded the response contract; inspect state before new work."}
	}
	result := contract.FeatureResult{Status: "success", Payload: payload}
	response, err := json.Marshal(contract.FeatureOperation{Request: request, Status: result.Status, Result: &result})
	if err != nil || len(response) > contract.MaxBody {
		return contract.FeatureResult{Status: "unknown", Error: "Execution completed but its operation response exceeded the transport contract; inspect state before new work."}
	}
	return result
}

func executeFeatureContainer(ctx context.Context, request contract.FeatureRequest, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) (any, error) {
	var input featureContainerRequest
	if err := decodeFeature(request.Payload, &input); err != nil {
		return nil, err
	}
	id := invocation.ID{Capability: "dockpipe.containers", Action: strings.TrimPrefix(request.Action, "containers."), Version: "1"}
	if request.Action != "containers.snapshot" {
		return infrastructure.ControlContainerWithAuthority(ctx, infrastructure.ContainerControlRequest{Action: id.Action, Container: input.Container}, target, identity, authorize)
	}
	definition := invocation.Definition{ID: id, Target: target, RequireAuthorization: true}
	native, err := invocation.New(definition, func(context.Context, featureContainerRequest) (invocation.Prepared[any], error) {
		return invocation.Prepared[any]{Digest: request.ID, Execute: func(ctx context.Context) (any, error) {
			if request.Action == "containers.snapshot" {
				return infrastructure.CollectDockerInventory(ctx), nil
			}
			return nil, errors.New("unsupported container snapshot")
		}}, nil
	}, authorize)
	if err != nil {
		return nil, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, input)
}
