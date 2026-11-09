package infrastructure

import (
	"context"
	"os"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/telemetry"
)

// CollectLocalContainerInventory admits the native local snapshot operation
// before any Docker process starts. Remote callers supply their own authority.
func CollectLocalContainerInventory(ctx context.Context) (telemetry.DockerSnapshot, error) {
	target, identity := nativeinvocation.Local()
	return CollectContainerInventoryWithAuthority(ctx, target, identity, nil)
}

func CollectContainerInventoryWithAuthority(ctx context.Context, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) (telemetry.DockerSnapshot, error) {
	id := invocation.ID{Capability: "dockpipe.containers", Action: "snapshot", Version: "1"}
	native, err := invocation.New(invocation.Definition{ID: id, Target: target, RequireAuthorization: authorize != nil || target.Kind == invocation.Remote},
		func(context.Context, struct{}) (invocation.Prepared[telemetry.DockerSnapshot], error) {
			environment := os.Environ()
			prepared := invocation.Prepared[telemetry.DockerSnapshot]{Execute: func(ctx context.Context) (telemetry.DockerSnapshot, error) {
				result := collectDockerInventory(ctx, environment)
				return result, ctx.Err()
			}}
			if authorize != nil || target.Kind == invocation.Remote {
				prepared.Digest = NativeExecutionDigest(environment)
			}
			return prepared, nil
		}, authorize)
	if err != nil {
		return telemetry.DockerSnapshot{}, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, struct{}{})
}
