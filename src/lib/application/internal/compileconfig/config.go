package compileconfig

import (
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// Load reads dockpipe.config.json or returns (nil, nil) if absent. Parse errors propagate.
func Load(repoRoot string) (*domain.DockpipeProjectConfig, error) {
	return domain.LoadDockpipeProjectConfig(repoRoot)
}

func WorkflowRoots(cfg *domain.DockpipeProjectConfig, repoRoot string) []string {
	result := domain.EffectiveWorkflowCompileRootsDetailed(cfg, repoRoot)
	logCompilePathWarnings("workflows", result.MissingPaths)
	return result.Paths
}

func ResolverRoots(cfg *domain.DockpipeProjectConfig, repoRoot string) []string {
	result := domain.EffectiveResolverCompileRootsDetailed(cfg, repoRoot)
	logCompilePathWarnings("resolvers", result.MissingPaths)
	return result.Paths
}

func logCompilePathWarnings(rootKind string, missingPaths []string) {
	for _, path := range missingPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "config.compile_path",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs: map[string]string{
				"path":        path,
				"result":      "skip",
				"root_kind":   strings.TrimSpace(rootKind),
				"skip_reason": "missing_path",
			},
		})
	}
}
