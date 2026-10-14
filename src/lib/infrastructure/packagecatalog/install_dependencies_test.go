package packagecatalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
	"gopkg.in/yaml.v3"
)

func dependencyFixture(t *testing.T, manifests ...domain.PackageManifest) (*Client, Catalog, map[string][]byte) {
	t.Helper()
	catalog := Catalog{Platform: "test"}
	archives := make(map[string][]byte)
	for _, manifest := range manifests {
		manifest.Version = "1.0.0"
		data, err := yaml.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		var buffer bytes.Buffer
		gz := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(gz)
		prefix := manifest.Kind + "s/" + manifest.Name
		if err := writer.WriteHeader(&tar.Header{Name: prefix + "/package.yml", Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		archive := buffer.Bytes()
		digest := sha256.Sum256(archive)
		entry := Entry{StoreArtifact: packagebuild.StoreArtifact{
			Name: manifest.Name, Version: manifest.Version,
			Tarball: "dockpipe-" + manifest.Kind + "-" + manifest.Name + "-1.0.0.tar.gz",
			SHA256:  hex.EncodeToString(digest[:]), SizeBytes: int64(len(archive)),
		}, Kind: manifest.Kind}
		catalog.Packages = append(catalog.Packages, entry)
		archives[entry.Tarball] = archive
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := archives[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	client := NewClient()
	client.http.Transport = server.Client().Transport
	client.engineVersion = "0.6.14"
	catalog.Manifest = server.URL + "/catalog.json"
	return client, catalog, archives
}

func TestInstallDependencyLimits(t *testing.T) {
	for _, scenario := range []string{"package count boundary", "too many packages", "depth boundary", "too deep", "size"} {
		t.Run(scenario, func(t *testing.T) {
			manifests := []domain.PackageManifest{{Name: "parent", Kind: "workflow"}}
			wantSuccess := strings.HasSuffix(scenario, "boundary")
			switch scenario {
			case "package count boundary", "too many packages":
				count := maxInstallPackages
				if !wantSuccess {
					count++
				}
				for index := 1; index < count; index++ {
					name := fmt.Sprintf("dependency-%d", index)
					manifests[0].Depends = append(manifests[0].Depends, name)
					manifests = append(manifests, domain.PackageManifest{Name: name, Kind: "workflow"})
				}
			case "depth boundary", "too deep":
				depth := maxInstallDepth
				if !wantSuccess {
					depth++
				}
				for index := 1; index <= depth; index++ {
					name := fmt.Sprintf("dependency-%d", index)
					manifests[index-1].Depends = []string{name}
					manifests = append(manifests, domain.PackageManifest{Name: name, Kind: "workflow"})
				}
			}
			client, catalog, _ := dependencyFixture(t, manifests...)
			if scenario == "size" {
				catalog.Packages[0].SizeBytes = maxInstallBytes + 1
			}
			root := t.TempDir()
			_, err := client.InstallWithDependencies(context.Background(), catalog, catalog.Packages[0], root)
			if wantSuccess {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "limit") {
					t.Fatalf("expected limit failure, got %v", err)
				}
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatalf("limit failure left package state: %v, %v", entries, err)
				}
			}
		})
	}
}

func TestInstallReportsPartialPublication(t *testing.T) {
	client, catalog, _ := dependencyFixture(t,
		domain.PackageManifest{Name: "parent", Kind: "workflow", Depends: []string{"helper"}},
		domain.PackageManifest{Name: "helper", Kind: "workflow"},
	)
	root := t.TempDir()
	// A directory occupying the requested archive path makes publication fail
	// after its dependency succeeds, without relying on OS permission behavior.
	if err := os.MkdirAll(filepath.Join(root, "workflows", catalog.Packages[0].Tarball), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := client.InstallWithDependencies(context.Background(), catalog, catalog.Packages[0], root)
	if err == nil || !strings.Contains(err.Error(), "after publishing 1 dependency packages") {
		t.Fatalf("missing partial publication outcome: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "workflows", catalog.Packages[1].Tarball)); err != nil {
		t.Fatalf("published dependency missing: %v", err)
	}
	staged, err := filepath.Glob(filepath.Join(root, ".install-*"))
	if err != nil || len(staged) != 0 {
		t.Fatalf("staging leaked after partial publication: %v, %v", staged, err)
	}
}

func TestInstallDependencyClosureFromVerifiedLegacyArchives(t *testing.T) {
	client, catalog, archives := dependencyFixture(t,
		domain.PackageManifest{Name: "parent", Kind: "workflow", Depends: []string{"helper"}},
		domain.PackageManifest{Name: "helper", Kind: "workflow", RequiresResolvers: []string{"tool"}},
		domain.PackageManifest{Name: "tool", Kind: "resolver"},
	)
	root := t.TempDir()
	installed, err := client.InstallWithDependencies(context.Background(), catalog, catalog.Packages[0], root)
	if err != nil {
		t.Fatal(err)
	}
	if installed != filepath.Join(root, "workflows", catalog.Packages[0].Tarball) {
		t.Fatalf("wrong requested artifact: %s", installed)
	}
	for _, entry := range catalog.Packages {
		data, err := os.ReadFile(filepath.Join(root, entry.Kind+"s", entry.Tarball))
		if err != nil || !bytes.Equal(data, archives[entry.Tarball]) {
			t.Fatalf("missing or changed dependency %s: %v", entry.Name, err)
		}
	}
	staged, _ := filepath.Glob(filepath.Join(root, ".install-*"))
	if len(staged) != 0 {
		t.Fatalf("staging leaked: %v", staged)
	}
}

func TestInstallDependencyFailuresPreserveExistingStore(t *testing.T) {
	for _, scenario := range []string{"missing", "cycle", "ambiguous", "checksum", "engine", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			parent := domain.PackageManifest{Name: "parent", Kind: "workflow", Depends: []string{"helper"}}
			helper := domain.PackageManifest{Name: "helper", Kind: "resolver"}
			manifests := []domain.PackageManifest{parent, helper}
			switch scenario {
			case "missing":
				manifests = manifests[:1]
			case "cycle":
				manifests[1].Depends = []string{"parent"}
			case "ambiguous":
				manifests = append(manifests, domain.PackageManifest{Name: "helper", Kind: "workflow"})
			case "engine":
				manifests[1].MinDockpipeVersion = "0.7.0"
			}
			client, catalog, _ := dependencyFixture(t, manifests...)
			if scenario == "checksum" {
				catalog.Packages[1].SHA256 = strings.Repeat("a", 64)
			}
			root := t.TempDir()
			existing := filepath.Join(root, "workflows", catalog.Packages[0].Tarball)
			if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(existing, []byte("previous version"), 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			if _, err := client.InstallWithDependencies(ctx, catalog, catalog.Packages[0], root); err == nil {
				t.Fatal("expected failed install")
			} else if scenario == "engine" && !strings.Contains(err.Error(), "Update Dockpipe") {
				t.Fatalf("missing update guidance: %v", err)
			}
			data, err := os.ReadFile(existing)
			if err != nil || string(data) != "previous version" {
				t.Fatal("failed install changed the existing package")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 || entries[0].Name() != "workflows" {
				t.Fatalf("failed install published dependencies or left staging: %v", entries)
			}
		})
	}
}

func TestRequiredResolverDisambiguatesDependency(t *testing.T) {
	client, catalog, _ := dependencyFixture(t,
		domain.PackageManifest{Name: "parent", Kind: "workflow", Depends: []string{"helper"}, RequiresResolvers: []string{"helper"}},
		domain.PackageManifest{Name: "helper", Kind: "resolver"},
		domain.PackageManifest{Name: "helper", Kind: "workflow"},
	)
	root := t.TempDir()
	if _, err := client.InstallWithDependencies(context.Background(), catalog, catalog.Packages[0], root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "workflows", catalog.Packages[2].Tarball)); !os.IsNotExist(err) {
		t.Fatalf("installed an unrelated same-name workflow: %v", err)
	}
}

func TestCatalogMinimumRejectsBeforeDownload(t *testing.T) {
	client := NewClient()
	client.engineVersion = "0.6.13"
	entry := Entry{StoreArtifact: packagebuild.StoreArtifact{
		Name: "example", Version: "1.0.0", Tarball: "dockpipe-workflow-example-1.0.0.tar.gz",
		SHA256: strings.Repeat("a", 64), MinDockpipeVersion: "0.6.14",
	}, Kind: "workflow"}
	_, err := client.Install(context.Background(), Catalog{Manifest: "invalid URL"}, entry, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "requires Dockpipe 0.6.14") {
		t.Fatalf("did not reject incompatibility before network access: %v", err)
	}
}
