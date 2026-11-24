//go:build !windows

package remote

import "path/filepath"

func LockWorker(root string) (func(), error) {
	return Lock(filepath.Join(root, "worker.lock"))
}

func LockDesktopWorker(root string) (func(), error) {
	return Lock(filepath.Join(root, "desktop.lock"))
}

func lockNetworkWorker(root string) (func(), error) {
	return Lock(filepath.Join(root, "network-worker.lock"))
}

func lockFeatureWorker(root string) (func(), error) {
	return Lock(filepath.Join(root, "features.lock"))
}

func lockTerminalWorker(root string) (func(), error) {
	return Lock(filepath.Join(root, "terminal.lock"))
}

func LockFeatureConfiguration(root string) (func(), error) {
	return Lock(filepath.Join(root, "feature-config.lock"))
}
