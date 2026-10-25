package domain

import (
	"os"
	"path/filepath"
	"strings"
)

type CompilePathResolution struct {
	Paths        []string
	MissingPaths []string
}

// EffectiveWorkflowCompileRoots resolves dockpipe.config.json compile.workflows or CLI defaults.
func EffectiveWorkflowCompileRoots(cfg *DockpipeProjectConfig, repoRoot string) []string {
	return EffectiveWorkflowCompileRootsDetailed(cfg, repoRoot).Paths
}

// EffectiveWorkflowCompileRootsDetailed returns workflow compile roots plus any configured paths
// that were skipped because they do not currently exist on disk.
func EffectiveWorkflowCompileRootsDetailed(cfg *DockpipeProjectConfig, repoRoot string) CompilePathResolution {
	var result CompilePathResolution
	if cfg != nil && cfg.Compile.Workflows != nil {
		result = ResolveCompilePathList(repoRoot, *cfg.Compile.Workflows)
	} else {
		result.Paths = defaultWorkflowRoots(repoRoot)
	}
	return result
}

// EffectiveResolverCompileRoots discovers resolver packages from the same
// explicitly configured or default roots as workflows.
func EffectiveResolverCompileRoots(cfg *DockpipeProjectConfig, repoRoot string) []string {
	return EffectiveResolverCompileRootsDetailed(cfg, repoRoot).Paths
}

// EffectiveResolverCompileRootsDetailed returns resolver compile roots plus any configured paths
// that were skipped because they do not currently exist on disk.
func EffectiveResolverCompileRootsDetailed(cfg *DockpipeProjectConfig, repoRoot string) CompilePathResolution {
	return EffectiveWorkflowCompileRootsDetailed(cfg, repoRoot)
}

func ResolveCompilePathList(repoRoot string, rels []string) CompilePathResolution {
	var result CompilePathResolution
	for _, r := range rels {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs := r
		if !filepath.IsAbs(r) {
			abs = filepath.Join(repoRoot, filepath.Clean(r))
		} else {
			abs = filepath.Clean(r)
		}
		if _, err := os.Stat(abs); err == nil {
			result.Paths = append(result.Paths, abs)
		} else {
			result.MissingPaths = append(result.MissingPaths, abs)
		}
	}
	return result
}

func defaultWorkflowRoots(repoRoot string) []string {
	var out []string
	rels := []string{"workflows"}
	for _, rel := range rels {
		p := filepath.Join(repoRoot, rel)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
