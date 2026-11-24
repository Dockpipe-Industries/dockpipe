//go:build !windows

package remotecmd

import (
	"context"
	"errors"
	"path/filepath"

	remoteio "dockpipe/src/lib/infrastructure/remote"
)

var errWindowsWorkerTaskUnavailable = errors.New("worker task requires Windows")

func prepareWindowsWorkerTask(context.Context, string, []string, string) (string, error) {
	return "", errWindowsWorkerTaskUnavailable
}

func readWindowsWorkerTask(context.Context, string, string) (bool, bool, error) {
	return false, false, errWindowsWorkerTaskUnavailable
}

func activateWindowsWorkerTask(context.Context, string, string, bool) error {
	return errWindowsWorkerTaskUnavailable
}

func lockService(root string) (func(), error) {
	return remoteio.Lock(filepath.Join(root, "service.lock"))
}
