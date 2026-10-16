package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidateTarballExtractCacheForPackageRemovesMarkerAndLegacyEntries(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := TarballExtractCacheRoot(dir)

	markerCache := filepath.Join(cacheRoot, "marker")
	if err := os.MkdirAll(markerCache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(markerCache, ".dockpipe-extracted-package.json"),
		[]byte(`{"tarball":"dockpipe-workflow-mywf-0.6.0.tar.gz"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	legacyCache := filepath.Join(cacheRoot, "legacy", "workflows", "mywf")
	if err := os.MkdirAll(legacyCache, 0o755); err != nil {
		t.Fatal(err)
	}

	otherCache := filepath.Join(cacheRoot, "other", "workflows", "other")
	if err := os.MkdirAll(otherCache, 0o755); err != nil {
		t.Fatal(err)
	}

	n, err := InvalidateTarballExtractCacheForPackage(dir, "workflow", "mywf")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("removed %d cache entries, want 2", n)
	}
	if _, err := os.Stat(markerCache); !os.IsNotExist(err) {
		t.Fatalf("marker cache still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheRoot, "legacy")); !os.IsNotExist(err) {
		t.Fatalf("legacy cache still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(otherCache); err != nil {
		t.Fatalf("unrelated cache removed: %v", err)
	}
}

func TestInvalidateTarballExtractCacheRetainsOnlyActiveSource(t *testing.T) {
	workdir := t.TempDir()
	cacheRoot := TarballExtractCacheRoot(workdir)
	source := filepath.Join(cacheRoot, "active", "workflows", "example")
	stale := filepath.Join(cacheRoot, "stale", "workflows", "example")
	for _, dir := range []string{source, stale} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := InvalidateTarballExtractCacheExceptSource(workdir, "workflow", "example", source)
	if err != nil || removed != 1 {
		t.Fatalf("removed %d entries: %v", removed, err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("active source removed: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale cache retained: %v", err)
	}
}

func TestFindLatestPackageTarballRejectsHyphenatedNameCollisions(t *testing.T) {
	for _, kind := range []string{"resolver", "workflow"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("DOCKPIPE_PACKAGES_ROOT", filepath.Join(root, "store"))
			t.Setenv("DOCKPIPE_GLOBAL_ROOT", filepath.Join(root, "global"))
			t.Setenv("DOCKPIPE_SYSTEM_ROOT", filepath.Join(root, "system"))
			directory := filepath.Join(root, "store", kind+"s")
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			names := []string{
				"agent-self-analysis-stack-proxy-9.9.9",
				"agent-orchestrator-99.0.0",
				"agent-invalid",
				"agent-0.6",
				"agent-0.6.3",
				"agent-0.6.4-staging.123.1.abcdef",
			}
			for _, name := range names {
				path := filepath.Join(directory, "dockpipe-"+kind+"-"+name+".tar.gz")
				if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// A glob-matching directory is not an installable archive.
			if err := os.Mkdir(filepath.Join(directory, "dockpipe-"+kind+"-agent-9.9.9.tar.gz"), 0o755); err != nil {
				t.Fatal(err)
			}
			find := FindLatestResolverTarball
			if kind == "workflow" {
				find = FindLatestWorkflowTarball
			}
			got, err := find(root, "agent")
			want := filepath.Join(directory, "dockpipe-"+kind+"-agent-0.6.4-staging.123.1.abcdef.tar.gz")
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want exact package %q", got, err, want)
			}
			for _, version := range []string{"0.6.3", "0.6.4-staging.123.1.abcdef"} {
				if err := os.Remove(filepath.Join(directory, "dockpipe-"+kind+"-agent-"+version+".tar.gz")); err != nil {
					t.Fatal(err)
				}
			}
			got, err = find(root, "agent")
			if err != nil || got != "" {
				t.Fatalf("colliding packages must not satisfy missing identity: %q, %v", got, err)
			}
		})
	}
}

func TestFindLatestPackageTarballRetainsStorePriority(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project-store")
	global := filepath.Join(root, "global")
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", project)
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", filepath.Join(root, "system"))
	for _, directory := range []string{filepath.Join(project, "resolvers"), filepath.Join(global, "packages", "resolvers")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "dockpipe-resolver-agent-other-99.0.0.tar.gz"), []byte("collision"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(global, "packages", "resolvers", "dockpipe-resolver-agent-0.6.3.tar.gz")
	if err := os.WriteFile(want, []byte("global"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindLatestResolverTarball(root, "agent")
	if err != nil || got != want {
		t.Fatalf("project collision must not shadow global identity: %q, %v", got, err)
	}
	want = filepath.Join(project, "resolvers", "dockpipe-resolver-agent-0.6.2.tar.gz")
	if err := os.WriteFile(want, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = FindLatestResolverTarball(root, "agent")
	if err != nil || got != want {
		t.Fatalf("project identity must keep store priority: %q, %v", got, err)
	}
}
