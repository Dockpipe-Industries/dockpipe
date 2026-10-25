package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestResolveStrategyFilePathPrefersPerWorkflowStrategiesDir(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "templates", "myflow")
	_ = os.MkdirAll(filepath.Join(wfRoot, "strategies"), 0o755)
	local := filepath.Join(wfRoot, "strategies", "mine")
	if err := os.WriteFile(local, []byte("DOCKPIPE_STRATEGY_BEFORE=a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(repo, "strategies", "mine")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("DOCKPIPE_STRATEGY_BEFORE=core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveStrategyFilePath(repo, wfRoot, "mine")
	if err != nil {
		t.Fatal(err)
	}
	if p != local {
		t.Fatalf("want per-workflow strategy %s got %s", local, p)
	}
}

func TestResolveStrategyFilePathUsesProjectStrategies(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "templates", "myflow")
	_ = os.MkdirAll(wfRoot, 0o755)
	core := filepath.Join(repo, "strategies", "worktree")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("DOCKPIPE_STRATEGY_KIND=git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveStrategyFilePath(repo, wfRoot, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	if p != core {
		t.Fatalf("want core strategy %s got %s", core, p)
	}
}

// TestResolveStrategyFilePathDoesNotSearchAssets verifies strategies are not resolved from
// templates/core/assets (support files only — not lifecycle wrappers).
func TestResolveStrategyFilePathDoesNotSearchAssets(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "templates", "myflow")
	_ = os.MkdirAll(filepath.Join(repo, "templates", "core", "assets", "scripts"), 0o755)
	decoy := filepath.Join(repo, "templates", "core", "assets", "scripts", "worktree")
	if err := os.WriteFile(decoy, []byte("DOCKPIPE_STRATEGY_BEFORE=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveStrategyFilePath(repo, wfRoot, "worktree")
	if err == nil {
		t.Fatal("expected error: strategy must not be loaded from assets/")
	}
}

func TestLoadStrategyAssignmentsLibraryGitHooksAndOverridePrecedence(t *testing.T) {
	repo := t.TempDir()
	workflow := filepath.Join(repo, "workflows", "example")
	for _, name := range []string{"commit", "worktree"} {
		assignments, source, err := LoadStrategyAssignments(repo, workflow, name)
		if err != nil {
			t.Fatal(err)
		}
		if source != "" || assignments.Kind != "git" || len(assignments.After) != 1 {
			t.Fatalf("unexpected builtin strategy %q: %+v, %q", name, assignments, source)
		}
		resolved := ResolveStrategyScriptPaths(assignments.After, workflow, repo, repo)
		if !StrategyAfterHandlesBundledCommit(resolved, repo) {
			t.Fatal("library-owned commit hook lost its host commit identity")
		}
		if name == "worktree" && (len(assignments.Before) != 1 || filepath.Base(assignments.Before[0]) != "clone-worktree.sh") {
			t.Fatalf("worktree strategy lost clone hook: %+v", assignments)
		}
	}
	project := filepath.Join(repo, "strategies", "worktree")
	writeFile(t, project, "DOCKPIPE_STRATEGY_KIND=project\nDOCKPIPE_STRATEGY_BEFORE=scripts/custom.sh\n", 0o644)
	assignments, source, err := LoadStrategyAssignments(repo, workflow, "worktree")
	if err != nil || source != project || assignments.Kind != "project" {
		t.Fatalf("project strategy must override library defaults: %+v, %q, %v", assignments, source, err)
	}
	local := filepath.Join(workflow, "strategies", "worktree")
	writeFile(t, local, "DOCKPIPE_STRATEGY_KIND=workflow\n", 0o644)
	assignments, source, err = LoadStrategyAssignments(repo, workflow, "worktree")
	if err != nil || source != local || assignments.Kind != "workflow" {
		t.Fatalf("workflow strategy must override project defaults: %+v, %q, %v", assignments, source, err)
	}
	hook, err := infrastructure.LegacyGitHookPath(repo, "commit-worktree")
	if err != nil || !infrastructure.IsBundledCommitWorktree(hook, repo) {
		t.Fatalf("owned hook identity: %q, %v", hook, err)
	}
}
