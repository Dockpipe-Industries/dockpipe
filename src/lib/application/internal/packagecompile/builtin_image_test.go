package packagecompile

import (
	"dockpipe/src/lib/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestCompiledBuiltinImageContextIsStagedAndPortable(t *testing.T) {
	workdir := t.TempDir()
	staging := t.TempDir()
	image, directory, context, ok, err := compiledTemplateBuild(workdir, staging, "dev")
	if err != nil || !ok || image != "dockpipe-dev" || directory != context {
		t.Fatalf("compiled builtin = %q, %q, %q, %v, %v", image, directory, context, ok, err)
	}
	relative, err := filepath.Rel(staging, context)
	if err != nil || relative == ".." {
		t.Fatalf("builtin context escaped staging: %q, %v", relative, err)
	}
	if _, err := os.Stat(filepath.Join(context, "assets", "entrypoint.sh")); err != nil {
		t.Fatal(err)
	}
}

func TestCompiledBuiltinAptImageRetainsCompleteContext(t *testing.T) {
	root := t.TempDir()
	staging := t.TempDir()
	workflow := &domain.Workflow{Name: "example", Isolate: "base-dev"}
	workflow.Image.Packages.Apt = []string{"file"}
	selection, _, err := selectCompiledImageArtifact(root, staging, "example", workflow, nil, "sha256:test")
	if err != nil {
		t.Fatal(err)
	}
	if selection.Build == nil {
		t.Fatal("derived builtin image has no build context")
	}
	context := selection.Build.Context
	if !filepath.IsAbs(context) {
		context = filepath.Join(staging, context)
	}
	if _, err := os.Stat(filepath.Join(context, "assets", "entrypoint.sh")); err != nil {
		t.Fatalf("derived image lost its entrypoint context: %v", err)
	}
}
