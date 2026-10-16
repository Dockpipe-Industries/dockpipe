package infrastructure

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/engineversion"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func resolverAssetsFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_CORE_ROOT", "")
	writeResolverAssetsFile(t, filepath.Join(repo, "dockpipe.config.json"), `{"schema":1,"compile":{"workflows":["authoring"]}}`)
	return repo
}

func writeResolverAssetsFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveResolverAssetsDirArchiveWithoutProfile(t *testing.T) {
	repo := resolverAssetsFixture(t)
	source := t.TempDir()
	writeResolverAssetsFile(t, filepath.Join(source, "package.yml"), "name: assets-only\nmin_dockpipe_version: 0.6.17\n")
	writeResolverAssetsFile(t, filepath.Join(source, "config.yml"), "name: assets-only\nsteps: []\n")
	writeResolverAssetsFile(t, filepath.Join(source, "assets", "payload.txt"), "archive payload")
	store, err := PackagesResolversDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(store, "dockpipe-resolver-assets-only-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, "resolvers/assets-only"); err != nil {
		t.Fatal(err)
	}
	previous := engineversion.Current()
	t.Cleanup(func() { engineversion.Configure(previous) })
	engineversion.Configure("0.6.16")
	_, err = ResolveResolverAssetsDir(repo, "assets-only")
	var update *domain.DockpipeUpdateRequiredError
	if !errors.As(err, &update) || update.Current != "0.6.16" || update.Minimum != "0.6.17" {
		t.Fatalf("want running engine compatibility error, got %v", err)
	}
	entries, err := os.ReadDir(TarballExtractCacheRoot(repo))
	if !os.IsNotExist(err) && (err != nil || len(entries) != 0) {
		t.Fatalf("incompatible archive was extracted: %v, %v", entries, err)
	}
	// A lower-priority unpacked global package must not shadow the project archive.
	global, err := GlobalPackagesResolversDir()
	if err != nil {
		t.Fatal(err)
	}
	writeResolverAssetsFile(t, filepath.Join(global, "assets-only", "config.yml"), "name: assets-only\nsteps: []\n")
	writeResolverAssetsFile(t, filepath.Join(global, "assets-only", "assets", "payload.txt"), "global shadow")
	engineversion.Configure("0.6.17")
	assets, err := ResolveResolverAssetsDir(repo, "assets-only")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(assets, "payload.txt"))
	if err != nil || string(payload) != "archive payload" {
		t.Fatalf("resolved payload = %q, %v", payload, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(assets), "profile")); !os.IsNotExist(err) {
		t.Fatalf("fixture unexpectedly has execution profile: %v", err)
	}
}

func TestResolveResolverAssetsDirSourceAndMissingAssets(t *testing.T) {
	repo := resolverAssetsFixture(t)
	for _, marker := range []string{"profile", "config.yml"} {
		t.Run(marker, func(t *testing.T) {
			name := "source-" + marker
			base := filepath.Join(repo, "authoring", "nested", name)
			writeResolverAssetsFile(t, filepath.Join(base, marker), "# source fixture\n")
			writeResolverAssetsFile(t, filepath.Join(base, "assets", "payload.txt"), marker)
			assets, err := ResolveResolverAssetsDir(repo, name)
			if err != nil || assets != filepath.Join(base, "assets") {
				t.Fatalf("assets = %q, %v", assets, err)
			}
			if err := os.RemoveAll(filepath.Join(base, "assets")); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveResolverAssetsDir(repo, name); err == nil {
				t.Fatal("missing assets must fail")
			}
		})
	}
	for _, name := range []string{"", ".", "..", "../escape", `..\escape`, "/absolute", "wild*card", "name:stream"} {
		if _, err := ResolveResolverAssetsDir(repo, name); err == nil {
			t.Fatalf("unsafe resolver name %q accepted", name)
		}
	}
}

func TestResolveResolverAssetsDirInstalledPrecedenceAndCompatibility(t *testing.T) {
	repo := resolverAssetsFixture(t)
	configured := filepath.Join(repo, "configured")
	writeResolverAssetsFile(t, filepath.Join(repo, "dockpipe.config.json"), `{"schema":1,"compile":{"workflows":["authoring"]},"packages":{"sources":[{"kind":"store","path":"configured"}]}}`)
	project, err := PackagesResolversDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	global, err := GlobalPackagesResolversDir()
	if err != nil {
		t.Fatal(err)
	}
	roots := []string{project, filepath.Join(configured, "resolvers"), global}
	roots = append(roots, SystemPackagesResolversDirs()...)
	roots = append(roots, filepath.Join(repo, "authoring"))
	for _, root := range roots {
		writeResolverAssetsFile(t, filepath.Join(root, "ordered", "config.yml"), "name: ordered\nsteps: []\n")
		writeResolverAssetsFile(t, filepath.Join(root, "ordered", "assets", "payload.txt"), root)
	}
	for _, root := range roots {
		assets, err := ResolveResolverAssetsDir(repo, "ordered")
		if err != nil || assets != filepath.Join(root, "ordered", "assets") {
			t.Fatalf("priority: got %q, %v; want %q", assets, err, root)
		}
		if err := os.RemoveAll(filepath.Join(root, "ordered")); err != nil {
			t.Fatal(err)
		}
	}
	previous := engineversion.Current()
	engineversion.Configure("0.6.16")
	t.Cleanup(func() { engineversion.Configure(previous) })
	base := filepath.Join(project, "future")
	writeResolverAssetsFile(t, filepath.Join(base, "config.yml"), "name: future\nsteps: []\n")
	writeResolverAssetsFile(t, filepath.Join(base, "package.yml"), "name: future\nmin_dockpipe_version: 0.6.17\n")
	writeResolverAssetsFile(t, filepath.Join(base, "assets", "payload.txt"), "future")
	_, err = ResolveResolverAssetsDir(repo, "future")
	var update *domain.DockpipeUpdateRequiredError
	if !errors.As(err, &update) {
		t.Fatalf("installed compatibility gate bypassed: %v", err)
	}
}
