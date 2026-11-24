//go:build windows

package remote

import "dockpipe/src/lib/infrastructure"

// Host Terminal and feature discovery share the receipt lock. Preserve the
// same lock name on every platform.
func lockContainerTerminalStore(root string) (func(), error) {
	return infrastructure.LockPrivateState(root, "container-terminals.lock")
}
