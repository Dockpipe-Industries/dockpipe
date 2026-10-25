package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// EffectiveStrategyName returns CLI --strategy if set, else workflow.strategy.
func EffectiveStrategyName(opts *CliOpts, wf *domain.Workflow) string {
	if opts != nil && strings.TrimSpace(opts.Strategy) != "" {
		return strings.TrimSpace(opts.Strategy)
	}
	if wf != nil {
		return strings.TrimSpace(wf.Strategy)
	}
	return ""
}

// ValidateStrategyAllowlist errors if strategies: is non-empty and name is not listed.
func ValidateStrategyAllowlist(wf *domain.Workflow, name string) error {
	if wf == nil || len(wf.Strategies) == 0 || name == "" {
		return nil
	}
	for _, s := range wf.Strategies {
		if strings.TrimSpace(s) == name {
			return nil
		}
	}
	return fmt.Errorf("strategy %q is not allowed by this workflow (strategies: %v)", name, wf.Strategies)
}

// ResolveStrategyFilePath returns the path to the strategy KEY=value file:
// per-workflow strategies/<name>, then project strategies/<name>, or an explicit path.
func ResolveStrategyFilePath(repoRoot, wfRoot, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("strategy name is empty")
	}
	candidates := []string{}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		explicit := name
		if !filepath.IsAbs(explicit) {
			explicit = filepath.Join(repoRoot, explicit)
		}
		candidates = append(candidates, explicit)
	}
	if wfRoot != "" {
		candidates = append(candidates, filepath.Join(wfRoot, "strategies", name))
	}
	candidates = append(candidates, filepath.Join(repoRoot, "strategies", name))
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("strategy file not found for %q (tried %v); put shared strategies under the project strategies/ directory or select an explicit file", name, candidates)
}

// LoadStrategyAssignments loads and parses a named strategy file.
func LoadStrategyAssignments(repoRoot, wfRoot, name string) (domain.StrategyAssignments, string, error) {
	path, err := ResolveStrategyFilePath(repoRoot, wfRoot, name)
	if err != nil {
		if stringSliceContains(builtinStrategyNames(), name) {
			assignments, hookErr := legacyGitStrategyAssignments(repoRoot, strings.TrimSpace(name))
			return assignments, "", hookErr
		}
		return domain.StrategyAssignments{}, "", err
	}
	m, err := infrastructure.LoadStrategyFile(path)
	if err != nil {
		return domain.StrategyAssignments{}, path, err
	}
	return domain.FromStrategyMap(m), path, nil
}

func builtinStrategyNames() []string {
	return []string{"commit", "worktree"}
}

// legacyGitStrategyAssignments preserves explicitly selected Git strategies
// without a strategy profile tree. Project and workflow files take precedence.
func legacyGitStrategyAssignments(workdir, name string) (domain.StrategyAssignments, error) {
	commit, err := infrastructure.LegacyGitHookPath(workdir, "commit-worktree")
	if err != nil {
		return domain.StrategyAssignments{}, err
	}
	assignments := domain.StrategyAssignments{Kind: "git", After: []string{commit}}
	if name == "worktree" {
		clone, err := infrastructure.LegacyGitHookPath(workdir, "clone-worktree")
		if err != nil {
			return domain.StrategyAssignments{}, err
		}
		assignments.Before = []string{clone}
	}
	return assignments, nil
}

// ResolveStrategyScriptPaths turns repo-relative strategy paths into absolute paths like workflow run:/act:.
func ResolveStrategyScriptPaths(rels []string, wfRoot, repoRoot, projectRoot string) []string {
	out := make([]string, 0, len(rels))
	for _, rel := range rels {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		out = append(out, infrastructure.ResolveWorkflowScript(rel, wfRoot, repoRoot, projectRoot))
	}
	return out
}

// StrategyAfterHandlesBundledCommit reports whether any after script is the bundled commit-worktree.sh.
func StrategyAfterHandlesBundledCommit(afterAbs []string, repoRoot string) bool {
	for _, p := range afterAbs {
		if infrastructure.IsBundledCommitWorktree(p, repoRoot) {
			return true
		}
	}
	return false
}

// ValidateNoDuplicateClone errors when workflow.run lists clone-worktree.sh while the strategy already provides clone.
func ValidateNoDuplicateClone(wf *domain.Workflow, wfRoot, repoRoot, projectRoot string, strategyProvidesClone bool, stratBeforeAbs []string) error {
	if wf == nil {
		return nil
	}
	var stratHasClone bool
	for _, p := range stratBeforeAbs {
		if strings.HasSuffix(p, "clone-worktree.sh") {
			stratHasClone = true
			break
		}
	}
	if !strategyProvidesClone && !stratHasClone {
		return nil
	}
	for _, r := range wf.Run {
		ap := infrastructure.ResolveWorkflowScript(r, wfRoot, repoRoot, projectRoot)
		if strings.HasSuffix(ap, "clone-worktree.sh") {
			return fmt.Errorf("workflow run lists clone-worktree.sh but strategy already runs clone; remove clone from workflow run: (hint: rely on strategy worktree or remove duplicate from run:)")
		}
	}
	return nil
}

// ActWouldBeBundledCommit resolves a workflow/resolver act path and reports bundled commit-worktree.sh.
func ActWouldBeBundledCommit(actRel string, wfRoot, repoRoot, projectRoot string) bool {
	actRel = strings.TrimSpace(actRel)
	if actRel == "" {
		return false
	}
	ap := infrastructure.ResolveWorkflowScript(actRel, wfRoot, repoRoot, projectRoot)
	return infrastructure.IsBundledCommitWorktree(ap, repoRoot)
}

// ResolverActWouldBeBundledCommit resolves resolver-relative act path.
func ResolverActWouldBeBundledCommit(actFromResolver, repoRoot string) bool {
	actFromResolver = strings.TrimSpace(actFromResolver)
	if actFromResolver == "" {
		return false
	}
	p := filepath.Join(repoRoot, actFromResolver)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return false
	}
	return infrastructure.IsBundledCommitWorktree(p, repoRoot)
}

// RunStrategyAfterScripts runs after hooks: bundled commit uses CommitOnHost; other paths use RunHostScript.
func RunStrategyAfterScripts(afterAbs []string, repoRoot string, envMap map[string]string, envSlice []string, opts *CliOpts) error {
	return runStrategyAfterScripts(afterAbs, repoRoot, envMap, envSlice, opts.Workdir, opts.BundleOut, runHostScriptAppFn, runHostCommit)
}

func runStrategyAfterScripts(afterAbs []string, repoRoot string, envMap map[string]string, envSlice []string, workdir, bundleOut string, runScript func(string, []string) error, commit func(string, string, string, bool) error) error {
	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], workdir)
	for _, p := range afterAbs {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("strategy after script not found: %s: %w", p, err)
		}
		if infrastructure.IsBundledCommitWorktree(p, repoRoot) {
			if err := commit(workHost, envMap["DOCKPIPE_COMMIT_MESSAGE"], firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], bundleOut), strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1"); err != nil {
				return err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] Strategy after: %s\n", p)
		if err := runScript(p, envSliceWithScriptContext(envSlice, p)); err != nil {
			return err
		}
	}
	return nil
}
