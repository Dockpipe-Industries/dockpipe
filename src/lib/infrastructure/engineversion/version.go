// Package engineversion exposes the running CLI's build identity, never a project VERSION file.
package engineversion

import "sync/atomic"

var current atomic.Pointer[string]

// Configure is called by the executable composition root before dispatching commands.
func Configure(version string) {
	current.Store(&version)
}

func Current() string {
	if version := current.Load(); version != nil {
		return *version
	}
	return "dev"
}
