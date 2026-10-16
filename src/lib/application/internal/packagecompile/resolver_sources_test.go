package packagecompile

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestResolverBatchIncludesAssetOnlyLeavesAndGroupedProfiles(t *testing.T) {
	workdir := t.TempDir()
	source := filepath.Join(t.TempDir(), "vendor")
	destination, err := infrastructure.PackagesResolversDir(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"assets-only", "group/tool-profile", "group/config-only"} {
		directory := filepath.Join(source, relative)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if relative == "group/tool-profile" {
			if err := os.WriteFile(filepath.Join(directory, "profile"), []byte("TOOL=fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else {
			config := "name: " + filepath.Base(directory) + "\nsteps: []\n"
			if err := os.WriteFile(filepath.Join(directory, "config.yml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "payload.txt"), []byte(relative), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	count, err := mergeChildPackagesWalk(workdir, source, destination, "resolver", "example", "1.2.3", true)
	if err != nil || count != 3 {
		t.Fatalf("compiled %d resolver packages: %v", count, err)
	}
	for _, name := range []string{"assets-only", "tool-profile", "config-only"} {
		archive := filepath.Join(destination, "dockpipe-resolver-"+name+"-1.2.3.tar.gz")
		manifest, err := packagebuild.ReadFileFromTarGz(archive, "resolvers/"+name+"/package.yml")
		if err != nil || len(manifest) == 0 {
			t.Fatalf("missing actual %s resolver archive: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "dockpipe-resolver-group-1.2.3.tar.gz")); !os.IsNotExist(err) {
		t.Fatalf("group folder must not become an installable resolver: %v", err)
	}
}

func TestResolverBatchRejectsMalformedConfigMarker(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "invalid", "config.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := hasNestedResolverPackLayout(source); err == nil {
		t.Fatal("a directory named config.yml must fail instead of being skipped")
	}
}
