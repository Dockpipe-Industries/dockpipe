package packagecatalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestHiddenDiscoveryPreservesExplicitAndDependencyInstalls(t *testing.T) {
	client, catalog, _ := dependencyFixture(t,
		domain.PackageManifest{Name: "parent", Kind: "workflow", Depends: []string{"helper"}},
		domain.PackageManifest{Name: "helper", Kind: "workflow", Hidden: true},
	)
	visible := catalog.Discovery(false)
	if len(visible.Packages) != 1 || visible.Packages[0].Name != "parent" {
		t.Fatalf("unexpected discovery view: %+v", visible)
	}
	if len(catalog.Packages) != 2 || len(catalog.Discovery(true).Packages) != 2 {
		t.Fatal("discovery lost hidden installation entries")
	}
	for _, index := range []int{0, 1} {
		root := t.TempDir()
		installed, err := client.InstallWithDependencies(context.Background(), catalog, catalog.Packages[index], root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(installed); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, "workflows", catalog.Packages[1].Tarball)); err != nil {
			t.Fatalf("hidden package unavailable for install %d: %v", index, err)
		}
	}
}

func TestHiddenDiscoveryEmptyArrayAndLegacyDefaults(t *testing.T) {
	var catalog Catalog
	if err := json.Unmarshal([]byte(`{"packages":[{"name":"legacy"},{"name":"private-preview","hidden":true}]}`), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Discovery(false).Packages) != 1 || !catalog.Packages[1].Hidden {
		t.Fatal("catalog lost visibility metadata or changed legacy defaults")
	}
	catalog.Packages = catalog.Packages[1:]
	encoded, err := json.Marshal(catalog.Discovery(false))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["packages"]) != "[]" {
		t.Fatalf("empty discovery must remain an array: %s", encoded)
	}
}
