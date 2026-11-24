//go:build !linux

package remotecmd

import (
	"context"
	"errors"
)

var errDesktopWorkerUnavailable = errors.New("XDG desktop worker autostart requires Linux")

func desktopWorkerDefinition(string, string, string, string, []string, string, bool) (string, string, []string, error) {
	return "", "", nil, errDesktopWorkerUnavailable
}

func activateDesktopWorker(context.Context, serviceDefinition) error {
	return errDesktopWorkerUnavailable
}

func requireDesktopWorkerSession() error {
	return errDesktopWorkerUnavailable
}

func superviseDesktopWorker(context.Context, string, string, string, bool) error {
	return errDesktopWorkerUnavailable
}
