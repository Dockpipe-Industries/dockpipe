package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestApplyDockpipeStateEnv(t *testing.T) {
	base := t.TempDir()
	wd := filepath.Join(base, "checkout")
	if err := os.Mkdir(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	envMap := map[string]string{}
	if err := applyDockpipeStateEnv(envMap, wd, "Pipeon Dev/Stack"); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap[infrastructure.EnvStateDir], filepath.Join(wd, "bin", ".dockpipe"); got != want {
		t.Fatalf("state dir = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvPackageID], "Pipeon Dev/Stack"; got != want {
		t.Fatalf("package id = %q want %q", got, want)
	}
	wantPackageState, err := infrastructure.ProjectPackageStateDir(wd, "Pipeon Dev/Stack")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := envMap[infrastructure.EnvPackageStateDir], wantPackageState; got != want {
		t.Fatalf("package state dir = %q want %q", got, want)
	}
}

func TestApplyPackageManifestContextClearsInheritedAuthority(t *testing.T) {
	env := map[string]string{"DOCKPIPE_PACKAGE_MANIFEST": "/stale/package.yml"}
	applyPackageManifestContext(env, t.TempDir())
	if _, ok := env["DOCKPIPE_PACKAGE_MANIFEST"]; ok {
		t.Fatalf("inherited package manifest authority was retained: %#v", env)
	}
}

func TestApplyCIArtifactEnvWorkflow(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{"DOCKPIPE_WORKFLOW_NAME": "docs.orchestrate"}
	if err := applyCIArtifactEnv(envMap, wd); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_CI_RAW_DIR"], filepath.Join(wd, "bin", ".dockpipe", "workflows", "docs.orchestrate", "artifacts", "ci-raw"); got != want {
		t.Fatalf("raw dir = %q want %q", got, want)
	}
	if got, want := envMap["DOCKPIPE_CI_ANALYSIS_DIR"], filepath.Join(wd, "bin", ".dockpipe", "workflows", "docs.orchestrate", "artifacts", "ci-analysis"); got != want {
		t.Fatalf("analysis dir = %q want %q", got, want)
	}
}

func TestApplyCIArtifactEnvRequiresOwnerBindingAndPreservesExplicit(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{"DOCKPIPE_CI_ANALYSIS_DIR": "/custom/analysis"}
	if err := applyCIArtifactEnv(envMap, wd); err != nil {
		t.Fatal(err)
	}
	if got := envMap["DOCKPIPE_CI_RAW_DIR"]; got != "" {
		t.Fatalf("unbound raw dir should remain unset, got %q", got)
	}
	if got := envMap["DOCKPIPE_CI_ANALYSIS_DIR"]; got != "/custom/analysis" {
		t.Fatalf("analysis dir should preserve explicit value, got %q", got)
	}
}

func TestApplyCIArtifactEnvExplicitPackageBinding(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{"DOCKPIPE_CI_ARTIFACT_SCOPE": "package:acme-ci"}
	if err := applyCIArtifactEnv(envMap, wd); err != nil {
		t.Fatal(err)
	}
	runtimeRoot, err := infrastructure.PackageRuntimeDir(wd, "acme-ci")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_CI_RAW_DIR"], filepath.Join(runtimeRoot, "ci", "raw"); got != want {
		t.Fatalf("raw dir = %q want %q", got, want)
	}
	if got, want := envMap["DOCKPIPE_CI_ANALYSIS_DIR"], filepath.Join(runtimeRoot, "ci", "analysis"); got != want {
		t.Fatalf("analysis dir = %q want %q", got, want)
	}
}

func TestApplyCIArtifactEnvCurrentPackageBinding(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{
		"DOCKPIPE_CI_ARTIFACT_SCOPE": "package",
		infrastructure.EnvPackageID:  "acme-ci",
	}
	if err := applyCIArtifactEnv(envMap, wd); err != nil {
		t.Fatal(err)
	}
	runtimeRoot, err := infrastructure.PackageRuntimeDir(wd, "acme-ci")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_CI_ANALYSIS_DIR"], filepath.Join(runtimeRoot, "ci", "analysis"); got != want {
		t.Fatalf("analysis dir = %q want %q", got, want)
	}
}

func TestCIArtifactPackageBindingsRemainCollisionSafe(t *testing.T) {
	wd := t.TempDir()
	firstRaw, _, err := ciArtifactDirs(wd, "package:Package.One/component/Worker")
	if err != nil {
		t.Fatal(err)
	}
	secondRaw, _, err := ciArtifactDirs(wd, "package:package-one-component-worker")
	if err != nil {
		t.Fatal(err)
	}
	if firstRaw == secondRaw {
		t.Fatalf("collision-prone package owners shared CI runtime path %q", firstRaw)
	}
	for _, path := range []string{firstRaw, secondRaw} {
		if !strings.Contains(filepath.ToSlash(path), "/bin/.dockpipe/packages-runtime/") {
			t.Fatalf("CI artifact path did not use package runtime: %q", path)
		}
	}
}

func TestApplyWorkflowArtifactEnv(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{}
	if err := applyWorkflowArtifactEnv(envMap, wd, "CI/Test"); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_SOURCE_ROOT"], wd; got != want {
		t.Fatalf("source root = %q want %q", got, want)
	}
	if got, want := envMap["DOCKPIPE_ARTIFACT_ROOT"], filepath.Join(wd, "bin", ".dockpipe", "workflows", "CI-Test", "artifacts"); got != want {
		t.Fatalf("artifact root = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvDockpipeEventLog], filepath.Join(wd, "bin", ".dockpipe", "workflows", "CI-Test", "artifacts", "events.jsonl"); got != want {
		t.Fatalf("event log = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvDockpipeEventIndex], filepath.Join(wd, "bin", ".dockpipe", "workflows", "CI-Test", "artifacts", "events-index.json"); got != want {
		t.Fatalf("event index = %q want %q", got, want)
	}
}

func TestApplyWorkflowArtifactEnvPreservesExplicitEventPaths(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{
		infrastructure.EnvDockpipeEventLog:   filepath.Join(wd, "custom-events.jsonl"),
		infrastructure.EnvDockpipeEventIndex: filepath.Join(wd, "custom-events-index.json"),
	}
	if err := applyWorkflowArtifactEnv(envMap, wd, "CI/Test"); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap[infrastructure.EnvDockpipeEventLog], filepath.Join(wd, "custom-events.jsonl"); got != want {
		t.Fatalf("event log should preserve explicit value, got %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvDockpipeEventIndex], filepath.Join(wd, "custom-events-index.json"); got != want {
		t.Fatalf("event index should preserve explicit value, got %q want %q", got, want)
	}
}
