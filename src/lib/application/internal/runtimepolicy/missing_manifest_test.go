package runtimepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestAuthoredContainerPolicyCannotRunWithoutCompiledManifest(t *testing.T) {
	for _, owner := range []string{"workflow", "step"} {
		t.Run(owner, func(t *testing.T) {
			workflow := &domain.Workflow{}
			step := domain.Step{ID: "bounded-fixture", Cmd: "echo fixture"}
			if owner == "workflow" {
				workflow.Security.Network.Mode = "offline"
			} else {
				step.Security.Network.Mode = "offline"
			}
			options := infrastructure.RunOpts{NetworkMode: "unchanged-before-admission"}
			manifest, err := ApplyCompiledRuntimePolicyForStep(&options, workflow, "", t.TempDir(), step, step.ID)
			if err == nil || !strings.Contains(err.Error(), "no compiled runtime manifest") {
				t.Fatalf("authored policy was silently ignored: manifest=%+v error=%v", manifest, err)
			}
			if manifest != nil || options.NetworkMode != "unchanged-before-admission" {
				t.Fatalf("failed policy admission changed container options: %+v", options)
			}
		})
	}
}

func TestLegacyUncompiledContainerWithoutDeclaredPolicyRemainsUnchanged(t *testing.T) {
	options := infrastructure.RunOpts{NetworkMode: "legacy-selection"}
	manifest, err := ApplyCompiledRuntimePolicyForStep(&options, &domain.Workflow{}, "", t.TempDir(), domain.Step{}, "legacy")
	if err != nil || manifest != nil || options.NetworkMode != "legacy-selection" {
		t.Fatalf("uncompiled compatibility changed: manifest=%+v options=%+v error=%v", manifest, options, err)
	}
}

func TestAuthoredSingleContainerWorkflowCannotRunWithoutCompiledManifest(t *testing.T) {
	workflow := &domain.Workflow{}
	workflow.Security.Network.Mode = "offline"
	options := infrastructure.RunOpts{NetworkMode: "unchanged-before-admission"}
	manifest, err := ApplyCompiledContainerWorkflowPolicy(&options, workflow, "", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no compiled runtime manifest") || manifest != nil {
		t.Fatalf("single-container declared policy bypassed admission: manifest=%+v error=%v", manifest, err)
	}
	if options.NetworkMode != "unchanged-before-admission" {
		t.Fatalf("failed policy admission changed container options: %+v", options)
	}
}

func TestDeclaredPolicyArchiveMissingOrCorruptManifestCannotFallBack(t *testing.T) {
	for _, state := range []string{"missing", "corrupt"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			staging := filepath.Join(root, "staging")
			if err := os.MkdirAll(staging, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(staging, "config.yml"), []byte("name: fixture\nsteps: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if state == "corrupt" {
				metadata := filepath.Join(staging, domain.RuntimeManifestDirName)
				if err := os.MkdirAll(metadata, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(metadata, domain.RuntimeManifestFileName), []byte("{"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			archive := filepath.Join(root, "fixture.tar.gz")
			if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, archive, "workflows/fixture"); err != nil {
				t.Fatal(err)
			}
			locator := infrastructure.FormatTarWorkflowURI(archive, "workflows/fixture/config.yml")
			workflow := &domain.Workflow{}
			workflow.Security.Network.Mode = "offline"
			options := infrastructure.RunOpts{NetworkMode: "unchanged-before-admission"}
			manifest, err := ApplyCompiledRuntimePolicyForStep(&options, workflow, locator, "", domain.Step{}, "fixture")
			if err == nil || manifest != nil || options.NetworkMode != "unchanged-before-admission" {
				t.Fatalf("invalid archived policy fell back: manifest=%+v options=%+v error=%v", manifest, options, err)
			}
			if state == "corrupt" && strings.Contains(err.Error(), "no compiled runtime manifest") {
				t.Fatalf("corrupt metadata was silently treated as absent: %v", err)
			}
		})
	}
}
