package infrastructure

import (
	"path"
	"path/filepath"
	"testing"
)

// TestEmbeddedWorkflowConfigExists checks bundled core templates and resolver delegates.
// Optional external packages have their own
// installation contract and are not part of the embedded asset set.
func TestEmbeddedWorkflowConfigExists(t *testing.T) {
	if !EmbeddedWorkflowConfigExists("run") {
		t.Fatal("expected run")
	}
	if !EmbeddedWorkflowConfigExists("run-apply") {
		t.Fatal("expected run-apply")
	}
	if !EmbeddedWorkflowConfigExists("run-apply-validate") {
		t.Fatal("expected run-apply-validate")
	}
	if !EmbeddedWorkflowConfigExists("init") {
		t.Fatal("expected init")
	}
	if !EmbeddedWorkflowConfigExists("example") {
		t.Fatal("expected bundled core resolver delegate")
	}
	if !EmbeddedWorkflowConfigExists("secretstore") {
		t.Fatal("expected secretstore workflow template")
	}
	if EmbeddedWorkflowConfigExists("dockpipe.cloudflare.r2infra") {
		t.Fatal("external package workflow must not be embedded")
	}
	if EmbeddedWorkflowConfigExists("") {
		t.Fatal("empty name should be false")
	}
	if EmbeddedWorkflowConfigExists("../x") {
		t.Fatal("path traversal should be false")
	}
	if EmbeddedWorkflowConfigExists("nope-not-a-real-template-xyz") {
		t.Fatal("unknown template should be false")
	}
}

func TestMapEmbeddedToMaterializedPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"VERSION", "version"},
		{"assets/entrypoint.sh", "assets/entrypoint.sh"},
		{EmbeddedTemplatesPrefix, filepath.Join(BundledLayoutDir, "core")},
		{EmbeddedTemplatesPrefix + "/runtimes/dockerimage/profile", filepath.Join(BundledLayoutDir, "core/runtimes/dockerimage/profile")},
		{EmbeddedTemplatesPrefix + "/workflows/run/config.yml", filepath.Join(BundledLayoutDir, "workflows", "run", "config.yml")},
		{path.Join("workflows", "test", "config.yml"), filepath.Join(BundledLayoutDir, "workflows", "test", "config.yml")},
		{"workflows", filepath.Join(BundledLayoutDir, "workflows")},
		// Already-materialized paths pass through unchanged (bundle cache layout uses BundledLayoutDir).
		{filepath.Join(BundledLayoutDir, "workflows", "init", "config.yml"), filepath.Join(BundledLayoutDir, "workflows", "init", "config.yml")},
	}
	for _, tc := range cases {
		got := mapEmbeddedToMaterializedPath(tc.in)
		if got != tc.want {
			t.Fatalf("mapEmbeddedToMaterializedPath(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestMapEmbeddedWorkflowRelSyntheticRoots(t *testing.T) {
	// The current binary contains no package source. Supply synthetic discovered
	// roots to verify the existing namespace-flattening contract independently.
	initEmbeddedWorkflowRoots()
	previousRoots := embeddedWorkflowRoots
	embeddedWorkflowRoots = []embedWorkflowRoot{
		{prefix: "catalog/resolvers/tool/child", name: "child"},
		{prefix: "catalog/resolvers/tool", name: "tool"},
	}
	t.Cleanup(func() { embeddedWorkflowRoots = previousRoots })

	prefix := embeddedPackageRootsPrefixes[0]
	cases := []struct {
		input string
		want  string
	}{
		{prefix, filepath.Join(BundledLayoutDir, "workflows")},
		{path.Join(prefix, "catalog/resolvers/tool/config.yml"), filepath.Join(BundledLayoutDir, "workflows/tool/config.yml")},
		{path.Join(prefix, "catalog/resolvers/tool/assets/run.sh"), filepath.Join(BundledLayoutDir, "workflows/tool/assets/run.sh")},
		{path.Join(prefix, "catalog/resolvers/tool/child/config.yml"), filepath.Join(BundledLayoutDir, "workflows/child/config.yml")},
		{path.Join(prefix, "unmapped/config.yml"), filepath.Join(BundledLayoutDir, "workflows/unmapped/config.yml")},
	}
	for _, test := range cases {
		got, ok := mapEmbeddedWorkflowRel(test.input)
		if !ok || got != test.want {
			t.Fatalf("mapEmbeddedWorkflowRel(%q) = (%q, %v), want (%q, true)", test.input, got, ok, test.want)
		}
	}
	if got, ok := mapEmbeddedWorkflowRel("outside/config.yml"); ok || got != "" {
		t.Fatalf("unowned prefix mapped to (%q, %v)", got, ok)
	}
}
