package infrastructure

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// RepoRoot returns the current project context, or an explicitly selected root.
// It never materializes workflow sources from the executable.
func RepoRoot() (string, error) {
	if root := strings.TrimSpace(os.Getenv("DOCKPIPE_REPO_ROOT")); root != "" {
		return filepath.Abs(root)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return domain.FindProjectRootWithDockpipeConfig(cwd)
}
