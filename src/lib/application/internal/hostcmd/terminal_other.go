//go:build !linux

package hostcmd

import (
	"context"
	"dockpipe/src/lib/infrastructure/nativehost"
)

func relayTerminal(context.Context, *nativehost.Client, string) error {
	return nativehost.ErrUnavailable
}
