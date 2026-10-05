package packageversion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// ForSource returns the nearest explicitly versioned manifest in the source's
// ownership chain. Search stays inside workdir; external sources use their own
// manifest only. Loose sources retain the caller's fallback version.
func ForSource(workdir, source, fallback string) (string, error) {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	directory, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, directory)
	// A different Windows volume is an external source, not a lookup error.
	inside := err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	for {
		manifestPath := filepath.Join(directory, "package.yml")
		if _, err := os.Stat(manifestPath); err == nil {
			manifest, err := domain.ParsePackageManifest(manifestPath)
			if err != nil {
				return "", fmt.Errorf("owning package %s: %w", manifestPath, err)
			}
			if version := strings.TrimSpace(manifest.Version); version != "" {
				return version, nil
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(directory)
		if !inside || directory == root || parent == directory {
			return fallback, nil
		}
		directory = parent
	}
}
