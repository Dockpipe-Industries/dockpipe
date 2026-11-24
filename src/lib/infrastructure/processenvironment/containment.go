package processenvironment

import (
	"os"
	"runtime"
)

// IsContained identifies supported application and container process boundaries.
// It does not infer containment from the host distribution or available tools.
// This is a capability restriction, not a security boundary against an operator
// who can change the process environment or mount namespace.
func IsContained() bool {
	return isContained(runtime.GOOS, os.Getenv, os.Stat)
}

func isContained(goos string, getenv func(string) string, stat func(string) (os.FileInfo, error)) bool {
	if goos != "linux" {
		return false
	}
	for _, name := range []string{"FLATPAK_ID", "SNAP", "container"} {
		if getenv(name) != "" {
			return true
		}
	}
	for _, path := range []string{"/.flatpak-info", "/.dockerenv", "/run/.containerenv", "/run/systemd/container"} {
		_, err := stat(path)
		if !os.IsNotExist(err) {
			// An unreadable boundary marker must not admit host-only effects.
			return true
		}
	}
	return false
}
