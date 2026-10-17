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
	source := filepath.Join(t.TempDir(), "resolvers")
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
			if err := os.WriteFile(filepath.Join(directory, "package.yml"),
				[]byte("kind: resolver\nname: "+filepath.Base(directory)+"\nversion: 1.2.3\n"), 0o644); err != nil {
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
	source := filepath.Join(t.TempDir(), "resolvers")
	if err := os.MkdirAll(filepath.Join(source, "invalid", "config.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "invalid", "package.yml"),
		[]byte("kind: resolver\nname: invalid\nversion: 1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolverSourceDirectory(filepath.Join(source, "invalid")); err == nil {
		t.Fatal("a directory named config.yml must fail instead of being skipped")
	}
}

func TestResolverDiscoveryExcludesWorkflowConfigsWithoutResolverAuthority(t *testing.T) {
	workdir := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	destination, err := infrastructure.PackagesResolversDir(workdir)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		path     string
		manifest string
		profile  bool
	}{
		{path: "workflows/group/ordinary", manifest: "kind: workflow\nname: ordinary\nversion: 1.2.3\n"},
		{path: "workflows/group/config-only"},
		{path: "workflows/group/explicit", manifest: "kind: resolver\nname: explicit\nversion: 1.2.3\n"},
		{path: "workflows/group/assets-only", manifest: "kind: resolver\nname: assets-only\nversion: 1.2.3\n"},
		{path: "workflows/group/profiled", profile: true},
		{path: "resolvers/group/config-resolver"},
		{path: "resolvers/group/typed-resolver", manifest: "kind: resolver\nname: typed-resolver\nversion: 1.2.3\n"},
		{path: "resolvers/group/workflow", manifest: "kind: workflow\nname: workflow\nversion: 1.2.3\n"},
	}
	for _, fixture := range fixtures {
		directory := filepath.Join(source, fixture.path)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if fixture.path != "workflows/group/assets-only" {
			if err := os.WriteFile(filepath.Join(directory, "config.yml"),
				[]byte("name: "+filepath.Base(directory)+"\nsteps: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(directory, "payload.txt"), []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
		if fixture.manifest != "" {
			if err := os.WriteFile(filepath.Join(directory, "package.yml"), []byte(fixture.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if fixture.profile {
			if err := os.WriteFile(filepath.Join(directory, "profile"), []byte("TOOL=fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	count, err := mergeChildPackages(workdir, source, destination, "resolver", "example", "1.2.3", true)
	if err != nil || count != 4 {
		t.Fatalf("compiled %d resolver packages: %v", count, err)
	}
	for _, name := range []string{"explicit", "assets-only", "profiled", "typed-resolver"} {
		archive := filepath.Join(destination, "dockpipe-resolver-"+name+"-1.2.3.tar.gz")
		if _, err := os.Stat(archive); err != nil {
			t.Fatalf("missing authoritative resolver %s: %v", name, err)
		}
	}
	for _, name := range []string{"ordinary", "config-only", "config-resolver", "workflow"} {
		archive := filepath.Join(destination, "dockpipe-resolver-"+name+"-1.2.3.tar.gz")
		if _, err := os.Stat(archive); !os.IsNotExist(err) {
			t.Fatalf("workflow %s became a resolver: %v", name, err)
		}
	}
}

func TestResolverRebuildPreservesNamesSharingPrefix(t *testing.T) {
	workdir := t.TempDir()
	source := filepath.Join(t.TempDir(), "resolvers")
	destination, err := infrastructure.PackagesResolversDir(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"dorkpipe", "dorkpipe-self-analysis"}
	for _, name := range names {
		directory := filepath.Join(source, name)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := []byte("kind: resolver\nname: " + name + "\nversion: 1.2.3\n")
		if err := os.WriteFile(filepath.Join(directory, "package.yml"), manifest, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, order := range [][]string{names, {names[1], names[0]}, names} {
		for _, name := range order {
			if err := compileSingleResolverDir(workdir, destination, filepath.Join(source, name), name, "", "1.2.3", true); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range names {
			archive := filepath.Join(destination, "dockpipe-resolver-"+name+"-1.2.3.tar.gz")
			if _, err := packagebuild.ReadFileFromTarGz(archive, "resolvers/"+name+"/package.yml"); err != nil {
				t.Fatalf("resolver %q lost after order %v: %v", name, order, err)
			}
		}
	}
}
