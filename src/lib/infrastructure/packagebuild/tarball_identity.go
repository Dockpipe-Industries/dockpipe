package packagebuild

import (
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

// MatchesVersionedTarball requires the complete package name and a complete
// semantic version. A prefix glob for "codex" must not match "codex-pav".
func MatchesVersionedTarball(path, kind, packageName string) bool {
	if kind != "workflow" && kind != "resolver" {
		return false
	}
	filename := filepath.Base(path)
	prefix := "dockpipe-" + kind + "-" + SafeTarballToken(packageName) + "-"
	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, ".tar.gz") {
		return false
	}
	version := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), ".tar.gz")
	normalized := "v" + strings.TrimPrefix(version, "v")
	core := strings.SplitN(strings.SplitN(normalized, "+", 2)[0], "-", 2)[0]
	return semver.IsValid(normalized) && strings.Count(core, ".") == 2
}
