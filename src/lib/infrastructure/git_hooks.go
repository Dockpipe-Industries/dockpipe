package infrastructure

import (
	"path/filepath"

	"dockpipe/src/lib/runtimeassets"
)

// LegacyGitHookPath materializes an explicitly selected library-owned runtime
// hook. It never consults project, package or legacy core authoring trees.
func LegacyGitHookPath(workdir, name string) (string, error) {
	data, err := runtimeassets.GitHook(name)
	if err != nil {
		return "", err
	}
	filename := name + ".sh"
	root, err := materializeLibraryResources(workdir, "git-hooks", map[string][]byte{filename: data})
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filename), nil
}
