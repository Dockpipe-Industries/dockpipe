package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

// TestRunInvalidResolverErrorsBeforeDocker fails fast when the selected resolver package is unavailable.
func TestRunInvalidResolverErrorsBeforeDocker(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }

	err := Run([]string{"--resolver", "no-such-resolver-xyz", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "isolation profile") {
		t.Fatalf("expected isolation profile error, got %v", err)
	}
}

// TestRunInvalidStrategyErrorsBeforeIsolate fails when the selected strategy is unavailable.
func TestRunInvalidStrategyErrorsBeforeIsolate(t *testing.T) {
	withRunSeams(t)
	repoRoot := testRepoRoot(t)
	repoRootAppFn = func() (string, error) { return repoRoot, nil }

	err := Run([]string{"--strategy", "no-such-strategy-xyz", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "strategy") {
		t.Fatalf("expected strategy resolution error, got %v", err)
	}
}

// TestCrosscutStrategyWorktreeAndCommitResolveFromLibrary exercises explicit library strategies without Docker.
func TestCrosscutStrategyWorktreeAndCommitResolveFromLibrary(t *testing.T) {
	repo := t.TempDir()
	for _, strat := range []string{"commit", "worktree"} {
		_, _, err := LoadStrategyAssignments(repo, "", strat)
		if err != nil {
			t.Fatalf("strategy %q: %v", strat, err)
		}
	}
}

// TestCrosscutRuntimeResolverMergesFixtureProfiles verifies generic profile merging without external package source.
func TestCrosscutRuntimeResolverMergesFixtureProfiles(t *testing.T) {
	repo := t.TempDir()
	store, err := infrastructure.PackagesResolversDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[string]string{
		filepath.Join(repo, "runtimes", "dockerimage", "profile"): "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n",
		filepath.Join(store, "fixture", "profile"):                "DOCKPIPE_RESOLVER_CMD=fixture-tool\n",
	}
	for profile, contents := range profiles {
		if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(profile, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	merged, err := infrastructure.LoadIsolationProfile(repo, "dockerimage", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if merged["DOCKPIPE_RUNTIME_SUBSTRATE"] != "dockerimage" || merged["DOCKPIPE_RESOLVER_CMD"] != "fixture-tool" {
		t.Fatalf("runtime and resolver assignments were not merged: %#v", merged)
	}
}
