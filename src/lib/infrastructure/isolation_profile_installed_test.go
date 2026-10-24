package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestLoadIsolationProfileUsesSourceHiddenResolverArchive(t *testing.T) {
	project := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	source := t.TempDir()
	profile := "DOCKPIPE_RESOLVER_CMD=installed-tool\nDOCKPIPE_RESOLVER_PACKAGE_ROOT=/forged\n"
	if err := os.WriteFile(filepath.Join(source, "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := PackagesResolversDir(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(store, "dockpipe-resolver-installed-tool-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, "resolvers/installed-tool"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	values, err := LoadIsolationProfile(project, "", "installed-tool")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveResolverFilePath(project, "installed-tool")
	if err != nil {
		t.Fatal(err)
	}
	if values["DOCKPIPE_RESOLVER_CMD"] != "installed-tool" || values["DOCKPIPE_RESOLVER_PACKAGE_ROOT"] != filepath.Dir(resolved) {
		t.Fatalf("installed resolver profile/root not preserved: %#v", values)
	}
}

func TestLoadIsolationProfileUsesSharedResolverPrecedenceAndErrors(t *testing.T) {
	project := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	store, err := PackagesResolversDir(project)
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(store, "example", "profile")
	for path, body := range map[string]string{
		selected: "DOCKPIPE_RESOLVER_CMD=installed\n",
		filepath.Join(project, "templates", "core", "resolvers", "example", "profile"): "DOCKPIPE_RESOLVER_CMD=source\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	values, err := LoadIsolationProfile(project, "", "example")
	if err != nil || values["DOCKPIPE_RESOLVER_CMD"] != "installed" {
		t.Fatalf("shared installed profile precedence changed: %#v, %v", values, err)
	}
	if err := os.WriteFile(selected, []byte(strings.Repeat("x", 128<<10)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIsolationProfile(project, "", "example"); err == nil {
		t.Fatal("invalid selected profile must fail instead of falling back to source")
	}
}
