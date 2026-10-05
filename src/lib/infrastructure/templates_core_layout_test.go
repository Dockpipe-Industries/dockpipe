package infrastructure

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// localModuleRoot walks up from the test cwd to the repo root (directory containing go.mod).
func localModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

// TestBundledTemplatesCoreLayoutEnforcesRootEntries fails if the five category directories or the
// intentional package manifest/Python package marker at the src/core root drift.
func TestBundledTemplatesCoreLayoutEnforcesRootEntries(t *testing.T) {
	root := localModuleRoot(t)
	core := filepath.Join(root, "src", "core")
	ents, err := os.ReadDir(core)
	if err != nil {
		t.Fatal(err)
	}
	wantDirs := []string{"assets", "resolvers", "runtimes", "strategies", "workflows"}
	wantFiles := []string{"__init__.py", "package.yml"}
	var dirs, files []string
	for _, e := range ents {
		switch {
		case e.IsDir():
			dirs = append(dirs, e.Name())
		case e.Type().IsRegular():
			files = append(files, e.Name())
		default:
			t.Fatalf("src/core root entry %q must be a regular file or directory", e.Name())
		}
	}
	slices.Sort(dirs)
	slices.Sort(files)
	if !slices.Equal(dirs, wantDirs) || !slices.Equal(files, wantFiles) {
		t.Fatalf("src/core must contain exactly directories %v and files %v (got directories %v and files %v)", wantDirs, wantFiles, dirs, files)
	}
}

// TestBundledTemplatesCoreAssetsSubdirsEnforcesScriptsImagesCompose fails if assets/ gains an
// unexpected top-level sibling (e.g. treating a new primitive as an asset category incorrectly).
// Domain docs live only under bundles/.../assets/docs/ — not under core assets/.
func TestBundledTemplatesCoreAssetsSubdirsEnforcesScriptsImagesCompose(t *testing.T) {
	root := localModuleRoot(t)
	assets := filepath.Join(root, "src", "core", "assets")
	ents, err := os.ReadDir(assets)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "images", "scripts"}
	var names []string
	for _, e := range ents {
		if e.IsDir() && e.Name()[0] != '.' {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("templates/core/assets must contain exactly %v (got %v)", want, names)
	}
}

// TestBundledAssetsIncludePowerShellExample ensures the reusable script asset remains present.
func TestBundledAssetsIncludePowerShellExample(t *testing.T) {
	root := localModuleRoot(t)
	ps := filepath.Join(root, "src", "core", "assets", "scripts", "helloworld.ps1")
	st, err := os.Stat(ps)
	if err != nil || st.IsDir() {
		t.Fatalf("expected src/core/assets/scripts/helloworld.ps1: %v", err)
	}
}
