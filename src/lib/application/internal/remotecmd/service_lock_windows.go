package remotecmd

import "dockpipe/src/lib/infrastructure"

func lockService(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "service.lock")
}
