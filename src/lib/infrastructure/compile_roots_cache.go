package infrastructure

import (
	"path/filepath"
	"sync"

	"dockpipe/src/lib/domain"
)

var (
	wfRootsCache  sync.Map // string (abs repo root) -> []string
	resRootsCache sync.Map
)

func absRepoKey(repoRoot string) string {
	a, err := filepath.Abs(repoRoot)
	if err != nil {
		return filepath.Clean(repoRoot)
	}
	return filepath.Clean(a)
}

// WorkflowCompileRootsCached returns absolute workflow compile roots from dockpipe.config.json
// (same list as dockpipe package compile workflows). Cached per process.
func WorkflowCompileRootsCached(repoRoot string) []string {
	k := absRepoKey(repoRoot)
	if v, ok := wfRootsCache.Load(k); ok {
		return v.([]string)
	}
	cfg, _ := domain.LoadDockpipeProjectConfig(repoRoot)
	out := domain.EffectiveWorkflowCompileRoots(cfg, repoRoot)
	wfRootsCache.Store(k, out)
	return out
}

// ResolverCompileRootsCached returns absolute resolver compile roots from dockpipe.config.json.
func ResolverCompileRootsCached(repoRoot string) []string {
	k := absRepoKey(repoRoot)
	if v, ok := resRootsCache.Load(k); ok {
		return v.([]string)
	}
	cfg, _ := domain.LoadDockpipeProjectConfig(repoRoot)
	out := domain.EffectiveResolverCompileRoots(cfg, repoRoot)
	resRootsCache.Store(k, out)
	return out
}
