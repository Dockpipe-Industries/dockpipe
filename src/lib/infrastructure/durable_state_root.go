package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
)

// canonicalDurableStateRoot resolves aliases in the OS-selected storage base,
// such as /home -> /var/home. The Dockpipe-owned suffix is deliberately excluded:
// links within that boundary must still fail the private-state checks.
func canonicalDurableStateRoot(root, goos string) (string, error) {
	suffix := "dockpipe"
	if goos == "windows" || goos == "darwin" {
		suffix = filepath.Join("dockpipe", "state")
	}
	base := filepath.Dir(root)
	if goos == "windows" || goos == "darwin" {
		base = filepath.Dir(base)
	}

	// A first run may not have an OS state directory yet. Resolve its nearest
	// existing ancestor without creating directories or following dangling links.
	missing := []string{}
	for {
		_, err := os.Lstat(base)
		if err == nil {
			canonical, resolveErr := filepath.EvalSymlinks(base)
			if resolveErr != nil {
				return "", fmt.Errorf("resolve durable state base: %w", resolveErr)
			}
			for index := len(missing) - 1; index >= 0; index-- {
				canonical = filepath.Join(canonical, missing[index])
			}
			return filepath.Join(canonical, suffix), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect durable state base: %w", err)
		}
		parent := filepath.Dir(base)
		if parent == base {
			return "", fmt.Errorf("durable state base does not exist: %s", base)
		}
		missing = append(missing, filepath.Base(base))
		base = parent
	}
}
