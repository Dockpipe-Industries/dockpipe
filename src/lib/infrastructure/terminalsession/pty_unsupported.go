//go:build !linux && !darwin

package terminalsession

import "context"

// Windows requires a qualified ConPTY backend with a kill-on-close Job Object.
// Ordinary redirected pipes do not implement an interactive terminal.
func startTerminal(context.Context, Profile, Size) (terminalBackend, error) {
	return nil, ErrUnsupported
}
