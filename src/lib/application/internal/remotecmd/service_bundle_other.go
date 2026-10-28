//go:build !darwin

package remotecmd

import "context"

func serviceBundleIdentifier(ctx context.Context, executable string) (string, error) {
	return "", nil
}
