package runtimeassets

import (
	"embed"
	"fmt"
)

//go:embed git/clone-worktree.sh git/commit-worktree.sh git/export-patch.sh git/print-summary.sh
var gitHooks embed.FS

// GitHook returns an explicitly selected legacy runtime hook template.
// These hooks are not workflow discovery roots or package dependencies.
func GitHook(name string) ([]byte, error) {
	switch name {
	case "clone-worktree", "commit-worktree", "export-patch", "print-summary":
		return gitHooks.ReadFile("git/" + name + ".sh")
	default:
		return nil, fmt.Errorf("unknown legacy Git hook %q", name)
	}
}
