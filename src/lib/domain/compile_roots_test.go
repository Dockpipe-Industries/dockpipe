package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveWorkflowCompileRootsUsesCanonicalConfiguredRoots(t *testing.T) {
	repo := t.TempDir()
	for _, rel := range []string{"workflows", "vendor/packages"} {
		if err := os.MkdirAll(filepath.Join(repo, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	roots := []string{"vendor/packages", "workflows", "missing"}
	cfg := &DockpipeProjectConfig{Compile: DockpipeCompileConfig{Workflows: &roots}}
	got := EffectiveWorkflowCompileRootsDetailed(cfg, repo)
	want := []string{filepath.Join(repo, "vendor/packages"), filepath.Join(repo, "workflows")}
	if len(got.Paths) != len(want) {
		t.Fatalf("paths = %v, want %v", got.Paths, want)
	}
	for i := range want {
		if got.Paths[i] != want[i] {
			t.Fatalf("path %d = %q, want %q", i, got.Paths[i], want[i])
		}
	}
	if len(got.MissingPaths) != 1 || got.MissingPaths[0] != filepath.Join(repo, "missing") {
		t.Fatalf("missing paths = %v", got.MissingPaths)
	}
}

func TestEffectiveResolverCompileRootsUsesWorkflowsBeforeCore(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "workflows", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "packages", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src", "core", "resolvers", "r1"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := []string{"workflows", "packages", filepath.Join("src", "core", "resolvers")}
	cfg := &DockpipeProjectConfig{Compile: DockpipeCompileConfig{Workflows: &wf}}
	got := EffectiveResolverCompileRoots(cfg, repo)
	want := []string{
		filepath.Join(repo, "workflows"),
		filepath.Join(repo, "packages"),
		filepath.Join(repo, "src", "core", "resolvers"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("root %d = %q want %q", i, got[i], want[i])
		}
	}
}

func TestResolveCompilePathListReportsMissingPaths(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := ResolveCompilePathList(repo, []string{"workflows", "missing"})
	if len(result.Paths) != 1 {
		t.Fatalf("want 1 existing path, got %d: %v", len(result.Paths), result.Paths)
	}
	if len(result.MissingPaths) != 1 {
		t.Fatalf("want 1 missing path, got %d: %v", len(result.MissingPaths), result.MissingPaths)
	}
	if got, want := filepath.Clean(result.MissingPaths[0]), filepath.Join(repo, "missing"); got != filepath.Clean(want) {
		t.Fatalf("missing path = %q want %q", got, want)
	}
}
