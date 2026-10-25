package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowStatePathsPreserveLayoutWithoutCreatingDirectories(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "custom-checkout")
	for _, test := range []struct {
		name  string
		scope string
	}{
		{name: "", scope: "default"},
		{name: "  CI / Build:_v2.1  ", scope: "CI-Build-_v2.1"},
		{name: "--", scope: "default"},
	} {
		t.Run(test.scope+test.name, func(t *testing.T) {
			state, err := WorkflowStateDir(workdir, test.name)
			if err != nil {
				t.Fatal(err)
			}
			wantState := filepath.Join(workdir, "bin", ".dockpipe", "workflows", test.scope)
			if state != wantState {
				t.Fatalf("state = %q, want %q", state, wantState)
			}
			artifacts, err := WorkflowArtifactRoot(workdir, test.name)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(wantState, "artifacts"); artifacts != want {
				t.Fatalf("artifacts = %q, want %q", artifacts, want)
			}
		})
	}
	if _, err := os.Stat(workdir); !os.IsNotExist(err) {
		t.Fatalf("path lookup must not create workdir: %v", err)
	}
}

func TestCIArtifactDirsBindingCompatibility(t *testing.T) {
	workdir := t.TempDir()
	wantRoot := filepath.Join(workdir, "bin", ".dockpipe", "workflows", "CI-Build", "artifacts")
	for _, binding := range []string{"CI/Build", " workflow: CI/Build "} {
		raw, analysis, err := CIArtifactDirs(workdir, binding)
		if err != nil {
			t.Fatal(err)
		}
		if raw != filepath.Join(wantRoot, "ci-raw") || analysis != filepath.Join(wantRoot, "ci-analysis") {
			t.Fatalf("binding %q: raw=%q analysis=%q", binding, raw, analysis)
		}
	}
	for _, binding := range []string{"", "workflow: ", "package: "} {
		if _, _, err := CIArtifactDirs(workdir, binding); err == nil || !strings.Contains(err.Error(), "requires a") {
			t.Fatalf("binding %q should reject absent owner, got %v", binding, err)
		}
	}
}
