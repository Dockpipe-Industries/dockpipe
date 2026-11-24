package infrastructure

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/process"
)

type ContainerControlRequest struct {
	Action    string `json:"action"`
	Container string `json:"container"`
}

type ContainerControlResult struct {
	Schema    string `json:"schema"`
	Action    string `json:"action"`
	Container string `json:"container"`
}

func ControlContainer(ctx context.Context, request ContainerControlRequest) (ContainerControlResult, error) {
	target, identity := nativeinvocation.Local()
	return ControlContainerWithAuthority(ctx, request, target, identity, nil)
}

// ControlContainerWithAuthority uses the same worker-local Docker executable
// and endpoint configuration as inventory. Requests cannot choose either, supply
// extra arguments, or address an ambiguous short container ID.
func ControlContainerWithAuthority(ctx context.Context, request ContainerControlRequest, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) (ContainerControlResult, error) {
	switch request.Action {
	case "start", "stop", "restart":
	default:
		return ContainerControlResult{}, errors.New("unsupported container control action")
	}
	if !isFullContainerID(request.Container) {
		return ContainerControlResult{}, errors.New("container control requires an exact full container ID")
	}
	id := invocation.ID{Capability: "dockpipe.containers", Action: request.Action, Version: "1"}
	definition := invocation.Definition{ID: id, Target: target, RequireAuthorization: authorize != nil || target.Kind == invocation.Remote}
	native, err := invocation.New(definition, func(_ context.Context, owned ContainerControlRequest) (invocation.Prepared[ContainerControlResult], error) {
		environment := os.Environ()
		prepared := invocation.Prepared[ContainerControlResult]{Execute: func(ctx context.Context) (ContainerControlResult, error) {
			name, err := resolveDockerCommandFromEnvironment(environment)
			if err != nil {
				return ContainerControlResult{}, err
			}
			receipt, err := observeContainerStart(ctx, name, environment, owned.Container)
			if err != nil {
				return ContainerControlResult{}, err
			}
			command := exec.CommandContext(ctx, name, "container", owned.Action, owned.Container)
			command.Env = dockerCommandEnv(environment, name)
			command.Stdout, command.Stderr = io.Discard, io.Discard
			command.WaitDelay = time.Second
			if err := process.RunWithScope(ctx, command); err != nil {
				return ContainerControlResult{}, err
			}
			if err := observeContainerCompletion(ctx, receipt); err != nil {
				return ContainerControlResult{}, err
			}
			return ContainerControlResult{Schema: "dockpipe.containers/v1", Action: owned.Action, Container: owned.Container}, nil
		}}
		if definition.RequireAuthorization {
			prepared.Digest = NativeExecutionDigest([]string{owned.Action, owned.Container}, environment)
		}
		return prepared, nil
	}, authorize)
	if err != nil {
		return ContainerControlResult{}, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, request)
}
