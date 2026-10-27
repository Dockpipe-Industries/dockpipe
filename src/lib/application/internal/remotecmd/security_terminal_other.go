//go:build !linux

package remotecmd

import "context"

func startTerminalSecurityAgent(ctx context.Context) (func(), error) {
	return func() {}, nil
}
