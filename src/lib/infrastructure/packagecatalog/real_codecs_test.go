package packagecatalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
	"gopkg.in/yaml.v3"
)

type realCodecFixture struct {
	Schema               int    `json:"schema"`
	Manifest             string `json:"manifest"`
	ManifestSHA256       string `json:"manifest_sha256"`
	ProviderConfig       string `json:"provider_config"`
	ProviderConfigSHA256 string `json:"provider_config_sha256"`
	Leaves               []struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Version   string `json:"version"`
		RawSHA256 string `json:"raw_sha256"`
		Variants  []struct {
			Codec            string `json:"codec"`
			Tarball          string `json:"tarball"`
			SHA256           string `json:"sha256"`
			SizeBytes        int64  `json:"size_bytes"`
			DecodedSHA256    string `json:"decoded_sha256"`
			DecodedSizeBytes int64  `json:"decoded_size_bytes"`
		} `json:"variants"`
	} `json:"leaves"`
}

const maxRealCodecFixtureFile = 64 << 20

func fixtureFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > maxRealCodecFixtureFile {
		return "", fmt.Errorf("fixture file is not regular or exceeds 64 MiB: %s", path)
	}
	hash := sha256.New()
	read, err := io.Copy(hash, io.LimitReader(file, maxRealCodecFixtureFile+1))
	if err != nil {
		return "", err
	}
	if read > maxRealCodecFixtureFile {
		return "", fmt.Errorf("fixture file grew beyond 64 MiB: %s", path)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func loadRealCodecFixture(t *testing.T) (string, realCodecFixture) {
	t.Helper()
	root := os.Getenv("DOCKPIPE_CODEC_FIXTURES")
	if root == "" {
		t.Skip("set DOCKPIPE_CODEC_FIXTURES to the verified private eight-leaf fixture")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("DOCKPIPE_CODEC_FIXTURES must be absolute")
	}
	indexFile, err := os.Open(filepath.Join(root, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer indexFile.Close()
	data, err := io.ReadAll(io.LimitReader(indexFile, (1<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 1<<20 {
		t.Fatal("real codec fixture index exceeds 1 MiB")
	}
	var fixture realCodecFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != 1 || len(fixture.Leaves) != 8 {
		t.Fatalf("expected schema 1 and eight real package leaves: %+v", fixture)
	}
	for _, item := range []struct {
		path string
		sha  string
	}{
		{fixture.Manifest, fixture.ManifestSHA256},
		{fixture.ProviderConfig, fixture.ProviderConfigSHA256},
	} {
		if item.path == "" || filepath.IsAbs(item.path) || strings.Contains(item.path, "..") {
			t.Fatalf("unsafe fixture path %q", item.path)
		}
		actual, err := fixtureFileSHA256(filepath.Join(root, item.path))
		if err != nil || actual != item.sha {
			t.Fatalf("fixture file changed: %s: %v", item.path, err)
		}
	}
	return root, fixture
}

func TestRealPackageCodecIntegration(t *testing.T) {
	root, fixture := loadRealCodecFixture(t)
	t.Setenv("DOCKPIPE_ARCHIVE_DECODERS", filepath.Join(root, fixture.ProviderConfig))
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")

	httpRoot := filepath.Join(root, "http-root")
	server := httptest.NewTLSServer(http.FileServer(http.Dir(httpRoot)))
	defer server.Close()
	client := NewClient()
	client.http.Transport = server.Client().Transport
	client.engineVersion = "0.6.18"
	manifestPath := "/" + strings.TrimPrefix(fixture.Manifest, "http-root/")
	catalog, err := client.Load(context.Background(), server.URL+manifestPath, "linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Packages) != len(fixture.Leaves) {
		t.Fatalf("loaded %d package leaves, expected %d", len(catalog.Packages), len(fixture.Leaves))
	}
	entries := make(map[string]Entry)
	for _, entry := range catalog.Packages {
		entries[entry.Kind+"/"+entry.Name] = entry
	}

	store := filepath.Join(global, "packages")
	cache := t.TempDir()
	installedPaths := make(map[string]string)
	for _, codec := range []string{"gzip", "zstd", "nucleon"} {
		for _, leaf := range fixture.Leaves {
			identity := leaf.Kind + "/" + leaf.Name
			entry, found := entries[identity]
			if !found || entry.Version != leaf.Version {
				t.Fatalf("catalog lost real package %s@%s", identity, leaf.Version)
			}
			selected, err := SelectArchive(entry, codec)
			if err != nil {
				t.Fatalf("%s %s: %v", codec, identity, err)
			}
			var expectedSHA string
			for _, variant := range leaf.Variants {
				if variant.Codec == codec {
					if variant.Tarball != selected.Tarball || variant.DecodedSHA256 != leaf.RawSHA256 {
						t.Fatalf("%s %s selected the wrong complete tar", codec, identity)
					}
					expectedSHA = variant.SHA256
				}
			}
			if expectedSHA == "" || selected.SHA256 != expectedSHA {
				t.Fatalf("%s %s has no matching frozen archive", codec, identity)
			}
			installed, err := client.InstallWithDependenciesUsingCodec(context.Background(), catalog, entry, store, codec)
			if err != nil {
				t.Fatalf("%s %s install: %v", codec, identity, err)
			}
			actualSHA, err := fixtureFileSHA256(installed)
			if err != nil || actualSHA != expectedSHA {
				t.Fatalf("%s %s installed different archive bytes: %v", codec, identity, err)
			}
			installedPaths[codec+"/"+identity] = installed

			prefix := "core"
			if entry.Kind != "core" {
				prefix = entry.Kind + "s/" + entry.Name
			}
			metadata, err := packagebuild.ReadFileFromTarGz(installed, prefix+"/package.yml")
			if err != nil {
				t.Fatalf("%s %s package manifest read: %v", codec, identity, err)
			}
			var packageManifest domain.PackageManifest
			if err := yaml.Unmarshal(metadata, &packageManifest); err != nil {
				t.Fatal(err)
			}
			if packageManifest.Name != entry.Name || packageManifest.Kind != entry.Kind ||
				(entry.Kind != "core" && packageManifest.Version != entry.Version) {
				t.Fatalf("%s %s archive identity changed: %+v", codec, identity, packageManifest)
			}
			extracted, err := packagebuild.EnsureTarballExtractedCache(installed, cache)
			if err != nil {
				t.Fatalf("%s %s extraction: %v", codec, identity, err)
			}
			unpacked, err := os.ReadFile(filepath.Join(extracted, filepath.FromSlash(prefix), "package.yml"))
			if err != nil || !bytes.Equal(unpacked, metadata) {
				t.Fatalf("%s %s extracted metadata differs: %v", codec, identity, err)
			}
		}
	}
	if len(installedPaths) != 24 {
		t.Fatalf("expected 24 installed codec/package identities, got %d", len(installedPaths))
	}

	project := t.TempDir()
	inventory, err := infrastructure.ListInstalledPackages(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Packages) != 24 || len(inventory.Warnings) != 0 {
		t.Fatalf("mixed-codec inventory missed packages: %d entries, warnings %v", len(inventory.Packages), inventory.Warnings)
	}
	remove := installedPaths["zstd/workflow/codex"]
	if err := infrastructure.UninstallUserPackage(remove); err != nil {
		t.Fatalf("mixed-codec uninstall: %v", err)
	}
	if _, err := os.Stat(remove); !os.IsNotExist(err) {
		t.Fatalf("uninstalled archive remains: %v", err)
	}
	for _, codec := range []string{"gzip", "nucleon"} {
		if _, err := os.Stat(installedPaths[fmt.Sprintf("%s/workflow/codex", codec)]); err != nil {
			t.Fatalf("uninstall removed a different encoding: %v", err)
		}
	}
	if err := infrastructure.UninstallUserPackage(installedPaths["gzip/core/dockpipe.core"]); err == nil {
		t.Fatal("core archive was removable")
	}
}
