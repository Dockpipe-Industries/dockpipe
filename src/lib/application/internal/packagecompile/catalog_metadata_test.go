package packagecompile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestCompiledCatalogMetadataRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "store")
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", store)
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", filepath.Join(root, "global"))
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", filepath.Join(root, "system"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "name: editor\nnamespace: exampleteam\ntitle: Friendly editor\ndescription: Opens a development workspace.\ncatalog_type: app\ncategories: [development]\ndocker_preflight: false\nsteps:\n  - id: notice\n    kind: host\n    cmd: echo ready\n"
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := compileWorkflowOne(root, source, "", true); err != nil {
		t.Fatal(err)
	}
	manifest, err := packagebuild.BuildCompiledStore(store, filepath.Join(root, "published"), "1.0.0", "workflows")
	if err != nil {
		t.Fatal(err)
	}
	publishedJSON, err := os.ReadFile(filepath.Join(root, "published", "packages-store-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded packagebuild.StoreBuildManifest
	if err := json.Unmarshal(publishedJSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Packages.Workflows[0].Title != manifest.Packages.Workflows[0].Title || decoded.Packages.Workflows[0].CatalogType != "app" {
		t.Fatal("published JSON lost discovery metadata")
	}
	entry := manifest.Packages.Workflows[0]
	if entry.Name != "editor" || entry.Title != "Friendly editor" || entry.CatalogType != "app" || entry.Description != "Opens a development workspace." || len(entry.Categories) != 1 {
		t.Fatalf("compiled store lost metadata: %+v", entry)
	}
	inventory, err := infrastructure.ListInstalledPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Packages) != 1 || inventory.Packages[0].Kind != "workflow" || inventory.Packages[0].CatalogType != "app" || inventory.Packages[0].Title != entry.Title {
		t.Fatalf("inventory changed identity or lost metadata: %+v", inventory)
	}
	// A package author can override the workflow title without changing either ID.
	manifestPath := filepath.Join(source, "package.yml")
	if err := os.WriteFile(manifestPath, []byte("schema: 1\nname: editor\nkind: workflow\ntitle: Published editor\ncategories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeCompiledCatalogMetadata(manifestPath, filepath.Join(source, "config.yml")); err != nil {
		t.Fatal(err)
	}
	parsed, err := domain.ParsePackageManifest(manifestPath)
	if err != nil || parsed.Title != "Published editor" || len(parsed.Categories) != 0 || parsed.CatalogType != "app" {
		t.Fatalf("explicit package metadata was not preserved: %+v, %v", parsed, err)
	}
}

func TestCompiledCatalogMetadataRejectsInvalidAuthorOverride(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "package.yml")
	configPath := filepath.Join(root, "config.yml")
	if err := os.WriteFile(configPath, []byte("catalog_type: app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []string{"catalog_type: 7\n", "categories: [ai, ai]\n", "categories: invalid\n"} {
		content := []byte("schema: 1\nname: editor\nkind: workflow\n" + metadata)
		if err := os.WriteFile(manifestPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := mergeCompiledCatalogMetadata(manifestPath, configPath); err == nil {
			t.Fatalf("invalid authored metadata was replaced by a default: %s", metadata)
		}
		unchanged, err := os.ReadFile(manifestPath)
		if err != nil || string(unchanged) != string(content) {
			t.Fatalf("failed validation changed the staged manifest: %v", err)
		}
	}
}
