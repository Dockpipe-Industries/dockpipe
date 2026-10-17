package infrastructure

import (
	"path"
	"path/filepath"
	"testing"
)

// TestEmbeddedWorkflowConfigExists checks bundled core templates, resolver delegates,
// and one retained maintainer package. Optional external packages have their own
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
	if !EmbeddedWorkflowConfigExists("dockpipe.cloudflare.r2infra") {
		t.Fatal("expected retained nested package workflow")
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
	pfx0 := embeddedPackageRootsPrefixes[0]
	cases := []struct {
		in, want string
	}{
		{"VERSION", "version"},
		{"assets/entrypoint.sh", "assets/entrypoint.sh"},
		{EmbeddedTemplatesPrefix, filepath.Join(BundledLayoutDir, "core")},
		{EmbeddedTemplatesPrefix + "/runtimes/dockerimage/profile", filepath.Join(BundledLayoutDir, "core/runtimes/dockerimage/profile")},
		{EmbeddedTemplatesPrefix + "/workflows/run/config.yml", filepath.Join(BundledLayoutDir, "workflows", "run", "config.yml")},
		{path.Join("workflows", "test", "config.yml"), filepath.Join(BundledLayoutDir, "workflows", "test", "config.yml")},
		{path.Join(pfx0, "cloud", "storage", "resolvers", "r2", "dockpipe.cloudflare.r2infra", "config.yml"), filepath.Join(BundledLayoutDir, "workflows", "dockpipe.cloudflare.r2infra", "config.yml")},
		{path.Join(pfx0, "secrets", "resolvers", "onepassword", "secretstore-onepassword", "config.yml"), filepath.Join(BundledLayoutDir, "workflows", "secretstore-onepassword", "config.yml")},
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
