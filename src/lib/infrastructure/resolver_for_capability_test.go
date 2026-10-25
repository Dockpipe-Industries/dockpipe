package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubstrateRuntimeFromDockpipeCapability(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"dockpipe.cli", "dockerimage"},
		{"dockpipe.cli.foo", "dockerimage"},
		{"dockpipe.docker", "dockerimage"},
		{"dockpipe.docker.alpine", "dockerimage"},
		{"dockpipe.dockerfile", "dockerfile"},
		{"dockpipe.package", "package"},
		{"dockpipe.kubepod", "dockerimage"},
		{"dockpipe.keystore", "dockerimage"},
		{"dockpipe.cloud.aws", "dockerimage"},
		{"dockpipe.powershell", "dockerimage"},
		{"cli.codex", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := SubstrateRuntimeFromDockpipeCapability(tc.in); got != tc.want {
			t.Errorf("SubstrateRuntimeFromDockpipeCapability(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolverProfileNameForCapability_fromPackageYML(t *testing.T) {
	repo := t.TempDir()
	declarePathTestRoots(t, repo)
	rsDir := filepath.Join(repo, "vendor", "resolvers", "mytool")
	if err := os.MkdirAll(rsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, PackageManifestFilename), []byte("kind: resolver\nname: mytool\ncapability: app.mytool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolverProfileNameForCapability(repo, repo, "app.mytool")
	if err != nil {
		t.Fatal(err)
	}
	if got != "mytool" {
		t.Fatalf("got %q want mytool", got)
	}
}

func TestResolverProfileNameForCapability_unknownId(t *testing.T) {
	repo := t.TempDir()
	declarePathTestRoots(t, repo)
	rsDir := filepath.Join(repo, "vendor", "resolvers", "x")
	if err := os.MkdirAll(rsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, PackageManifestFilename), []byte("kind: resolver\nname: x\ncapability: cli.x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolverProfileNameForCapability(repo, repo, "workflow.does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestResolverProfileNameForCapability_ambiguous(t *testing.T) {
	repo := t.TempDir()
	declarePathTestRoots(t, repo)
	base := filepath.Join(repo, "vendor", "resolvers")
	for _, name := range []string{"a", "b"} {
		d := filepath.Join(base, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, PackageManifestFilename), []byte("kind: resolver\nname: "+name+"\ncapability: dup.same\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := ResolverProfileNameForCapability(repo, repo, "dup.same")
	if err == nil {
		t.Fatal("expected error for duplicate capability")
	}
}

func TestResolverCapabilityDiscoveryUsesDeclaredPackagesOnly(t *testing.T) {
	repo := t.TempDir()
	declarePathTestRoots(t, repo)
	for _, fixture := range []struct{ directory, name, capability string }{
		{"vendor/toolset/resolvers", "declared-tool", "app.declared"},
		{"templates/core/resolvers", "retired-tool", "app.retired"},
	} {
		directory := filepath.Join(repo, fixture.directory, fixture.name)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := "kind: resolver\nname: " + fixture.name + "\ncapability: " + fixture.capability + "\n"
		if err := os.WriteFile(filepath.Join(directory, PackageManifestFilename), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	capabilities := SatisfiedCapabilityIDs(repo, repo)
	if _, ok := capabilities["app.declared"]; !ok {
		t.Fatal("declared package capability is missing")
	}
	if _, ok := capabilities["app.retired"]; ok {
		t.Fatal("undeclared core capability leaked into discovery")
	}
	if got, err := ResolverProfileNameForCapability(repo, repo, "app.retired"); err != nil || got != "" {
		t.Fatalf("undeclared core provider: %q, %v", got, err)
	}
	if resolverProfileExists(repo, repo, "retired-tool") {
		t.Fatal("undeclared core resolver appeared installed")
	}
}
