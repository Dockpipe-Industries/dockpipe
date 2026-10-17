package packagecompile

import (
	"os"
	"path/filepath"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

// compiledTarballsForPackage filters prefix-glob candidates by their complete
// package identity and semantic version before reuse or removal.
func compiledTarballsForPackage(pattern, kind, name string) ([]string, error) {
	candidates, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, path := range candidates {
		if packagebuild.MatchesVersionedTarball(path, kind, name) {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

func removeCompiledTarballs(paths []string) error {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Remove(path + ".sha256"); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
