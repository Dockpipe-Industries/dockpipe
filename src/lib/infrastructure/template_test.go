package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestBuiltinContainerBuildWithoutCoreTree(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"base-dev", "dev"} {
		image, directory, context, ok, err := ResolveTemplateBuild(root, name)
		if err != nil || !ok || image != "dockpipe-"+name || directory != context {
			t.Fatalf("builtin %q = %q, %q, %q, %v, %v", name, image, directory, context, ok, err)
		}
		content, err := os.ReadFile(filepath.Join(directory, "Dockerfile"))
		if err != nil || !strings.Contains(string(content), "FROM debian:bookworm-slim") || !strings.Contains(string(content), "COPY assets/entrypoint.sh /entrypoint.sh") {
			t.Fatalf("incomplete builtin Dockerfile: %s (%v)", content, err)
		}
		if name == "dev" && (!strings.Contains(string(content), "build-essential") || strings.Contains(string(content), "FROM dockpipe-base-dev")) {
			t.Fatal("development recipe lost its tools or depends on a hidden local base build")
		}
		if _, err := os.Stat(filepath.Join(context, "assets", "entrypoint.sh")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "templates")); !os.IsNotExist(err) {
		t.Fatal("builtin resolution created a template tree")
	}
}

func TestTemplateBuildDiscoversArbitraryPackageImage(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "vendor", "tools", "example-tool")
	directory := filepath.Join(packageRoot, "assets", "images", "example-tool")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte(`{"compile":{"workflows":["vendor/tools"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	image, got, context, ok, err := ResolveTemplateBuild(root, "example-tool")
	if err != nil || !ok || image != "dockpipe-example-tool" || got != directory || context != packageRoot {
		t.Fatalf("package template = %q, %q, %q, %v, %v", image, got, context, ok, err)
	}
	if _, _, ok := TemplateBuild(root, "uninstalled-tool"); ok {
		t.Fatal("unknown tool selected an implicit template")
	}
}

func TestTemplateBuildUsesInstalledResolverImage(t *testing.T) {
	root := t.TempDir()
	store := t.TempDir()
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", store)
	source := t.TempDir()
	directory := filepath.Join(source, "assets", "images", "example-tool")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(directory, "Dockerfile"): "FROM scratch\n",
		filepath.Join(source, "profile"):       "DOCKPIPE_RESOLVER_TEMPLATE=example-tool\n",
		filepath.Join(source, "package.yml"):   "schema: 1\nkind: resolver\nname: example-tool\nversion: 1.2.3\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(store, "resolvers", "dockpipe-resolver-example-tool-1.2.3.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, "resolvers/example-tool"); err != nil {
		t.Fatal(err)
	}
	_, got, context, ok, err := ResolveTemplateBuild(root, "example-tool")
	if err != nil || !ok || got != filepath.Join(context, "assets", "images", "example-tool") {
		t.Fatalf("installed template = %q, %q, %v, %v", got, context, ok, err)
	}
}

func TestTemplateBuildDoesNotDiscoverImplicitCoreImages(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"src/core/assets/images/example-tool", "bundle/workflows/example-tool/assets/images/example-tool"} {
		directory := filepath.Join(root, relative)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, ok := TemplateBuild(root, "example-tool"); ok {
		t.Fatal("implicit core image selected")
	}
}

func TestMaybeVersionTag(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "version"), []byte("0.6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for input, expected := range map[string]string{"dockpipe-example": "dockpipe-example:0.6.0", "ubuntu": "ubuntu", "dockpipe-example:custom": "dockpipe-example:custom", "": ""} {
		if got := MaybeVersionTag(root, input); got != expected {
			t.Fatalf("version tag = %q, want %q", got, expected)
		}
	}
}
