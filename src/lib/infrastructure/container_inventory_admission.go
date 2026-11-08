package infrastructure

import (
	"context"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/telemetry"
)

// CollectLocalContainerInventory admits the native local snapshot operation
// before any Docker process starts. Remote callers supply their own authority.
func CollectLocalContainerInventory(ctx context.Context) (telemetry.DockerSnapshot, error) {
	target, identity := nativeinvocation.Local()
	id := invocation.ID{Capability: "dockpipe.containers", Action: "snapshot", Version: "1"}
	native, err := invocation.New(invocation.Definition{ID: id, Target: target},
		func(context.Context, struct{}) (invocation.Prepared[telemetry.DockerSnapshot], error) {
			return invocation.Prepared[telemetry.DockerSnapshot]{Execute: func(ctx context.Context) (telemetry.DockerSnapshot, error) {
				result := CollectDockerInventory(ctx)
				return result, ctx.Err()
			}}, nil
		}, nil)
	if err != nil {
		return telemetry.DockerSnapshot{}, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, struct{}{})
}
