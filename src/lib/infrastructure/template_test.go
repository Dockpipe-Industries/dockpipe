package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTemplateBuild verifies required generic images remain in the core source.
func TestTemplateBuild(t *testing.T) {
	repoRoot := localModuleRoot(t)
	core := CoreDir(repoRoot)
	cases := []struct {
		name              string
		image             string
		dir               string
		ok                bool
		requireDockerfile bool
	}{
		{"base-dev", "dockpipe-base-dev", filepath.Join(core, "assets", "images", "base-dev"), true, true},
		{"dev", "dockpipe-dev", filepath.Join(core, "assets", "images", "dev"), true, true},
		{"unknown", "", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img, dir, ok := TemplateBuild(repoRoot, tc.name)
			if img != tc.image || dir != tc.dir || ok != tc.ok {
				t.Fatalf("TemplateBuild(%q) = (%q, %q, %v), want (%q, %q, %v)", tc.name, img, dir, ok, tc.image, tc.dir, tc.ok)
			}
			if tc.requireDockerfile {
				if st, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil || st.IsDir() {
					t.Fatalf("expected Dockerfile under %q, stat err=%v", dir, err)
				}
			}
		})
	}
}

func TestTemplateBuildExternalImagesUseConfiguredSource(t *testing.T) {
	repo := t.TempDir()
	vendorRoot := filepath.Join(repo, "vendor", "images")
	for _, name := range []string{"claude", "codex", "vscode", "ollama"} {
		dockerfile := filepath.Join(vendorRoot, name, "assets", "images", name, "Dockerfile")
		if err := os.MkdirAll(filepath.Dir(dockerfile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	config := []byte(`{"compile":{"workflows":["vendor/images"]}}`)
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), config, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		resolved string
	}{
		{"agent-dev", "claude"},
		{"claude", "claude"},
		{"codex", "codex"},
		{"vscode", "vscode"},
		{"ollama", "ollama"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			image, directory, ok := TemplateBuild(repo, test.name)
			expectedImage := "dockpipe-" + test.resolved
			expectedDirectory := filepath.Join(vendorRoot, test.resolved, "assets", "images", test.resolved)
			if !ok || image != expectedImage || directory != expectedDirectory {
				t.Fatalf("TemplateBuild(%q) = (%q, %q, %v), want (%q, %q, true)",
					test.name, image, directory, ok, expectedImage, expectedDirectory)
			}
		})
	}
}

func TestDockerfileDirBundledLayoutUsesBundledWorkflowImage(t *testing.T) {
	repo := t.TempDir()
	dockerfile := filepath.Join(repo, BundledLayoutDir, "workflows", "claude", "assets", "images", "claude", "Dockerfile")
	if err := os.MkdirAll(filepath.Dir(dockerfile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, BundledLayoutDir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := DockerfileDir(repo, "claude")
	if got != filepath.Dir(dockerfile) {
		t.Fatalf("got %q want bundled workflow image dir %q", got, filepath.Dir(dockerfile))
	}
}

func TestDockerfileDirUsesCanonicalWorkflowCompileRoot(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "vendor", "packages")
	dockerfile := filepath.Join(root, "codex", "assets", "images", "codex", "Dockerfile")
	if err := os.MkdirAll(filepath.Dir(dockerfile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(`{"compile":{"workflows":["vendor/packages"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := DockerfileDir(repo, "codex")
	if got != filepath.Dir(dockerfile) {
		t.Fatalf("got %q want configured workflow image dir %q", got, filepath.Dir(dockerfile))
	}
}

// TestMaybeVersionTag appends dockpipe-* image tags from VERSION file when missing.
func TestMaybeVersionTag(t *testing.T) {
	tmp := t.TempDir()
	versionFile := filepath.Join(tmp, "version")
	if err := os.WriteFile(versionFile, []byte("0.6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := MaybeVersionTag(tmp, "dockpipe-claude"); got != "dockpipe-claude:0.6.0" {
		t.Fatalf("dockpipe image should be tagged, got %q", got)
	}
	if got := MaybeVersionTag(tmp, "ubuntu"); got != "ubuntu" {
		t.Fatalf("non-dockpipe image should not be tagged, got %q", got)
	}
	if got := MaybeVersionTag(tmp, "dockpipe-claude:custom"); got != "dockpipe-claude:custom" {
		t.Fatalf("pre-tagged image should not be changed, got %q", got)
	}
	if got := MaybeVersionTag(tmp, ""); got != "" {
		t.Fatalf("empty image should stay empty, got %q", got)
	}
}
