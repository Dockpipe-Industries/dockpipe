//go:build windows

package remote

import "dockpipe/src/lib/infrastructure"

func LockWorker(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "worker.lock")
}

func LockDesktopWorker(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "desktop.lock")
}

func lockNetworkWorker(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "network-worker.lock")
}

func lockFeatureWorker(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "features.lock")
}

func lockTerminalWorker(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "terminal.lock")
}

func LockFeatureConfiguration(root string) (func(), error) {
	return infrastructure.TryLockPrivateState(root, "feature-config.lock")
}
