package terminalsession

import (
	"context"

	"dockpipe/src/lib/domain/invocation"
)

// Backend is the trusted native ownership boundary. Write returns success only
// after bytes reach the terminal, never on queue acceptance. Stop proves cleanup
// or returns ErrCleanupIncomplete. Close must interrupt Read/Write/Wait promptly.
// Implementations are trusted code, not an OS sandbox or caller-supplied plugin.
type Backend = terminalBackend

// ProviderFactory runs only inside the admitted open Execute. The supplied
// profile is an owned snapshot. It must not rediscover defaults or ambient env.
type ProviderFactory func(context.Context, Profile, Size) (Backend, error)

// NewWithProvider is a local native composition seam. Remote managers retain
// the built-in PTY path and may not choose a transport/provider through JSON.
func NewWithProvider(profile Profile, authority Authority, provider ProviderFactory) (*Manager, error) {
	if provider == nil || authority.Target.Kind != invocation.Local || profile.Route == "" || profile.EnvironmentMode == "" {
		return nil, ErrInvalid
	}
	return newManager(profile, authority, provider)
}
