//go:build darwin

package osauth

import (
	"context"
	"time"
)

// NetworkTLSAnchors reads public, unrestricted SSL anchors through the installed
// signed helper. Restricted or denied native trust cannot become accepted data.
func (platform *darwinPlatform) NetworkTLSAnchors(parent context.Context) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	helper, err := platform.helper()
	if err != nil {
		return nil, err
	}
	if err = platform.verifyHelper(ctx, helper); err != nil {
		return nil, err
	}
	runner := platform.runTrust
	if runner == nil {
		runner = platform.run
	}
	raw, err := runner(ctx, helper, "--tls-trust")
	if err != nil {
		return nil, err
	}
	return decodeNetworkTLSAnchors(raw)
}
