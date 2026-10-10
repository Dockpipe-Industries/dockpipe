package packagecatalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func testArchive(t *testing.T, extra *tar.Header) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gz)
	metadata := "schema: 1\nname: sample\nversion: 1.2.3\nkind: workflow\n"
	files := map[string]string{
		"workflows/sample/package.yml": metadata,
		"workflows/sample/config.yml":  "name: sample\nnamespace: sample\nsteps: []\n",
	}
	for name, body := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		if err := tarWriter.WriteHeader(extra); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func fixture(t *testing.T, archive []byte) (*Client, *httptest.Server, Entry) {
	t.Helper()
	digest := sha256.Sum256(archive)
	entry := Entry{packagebuild.StoreArtifact{Name: "sample", Version: "1.2.3", Tarball: "dockpipe-workflow-sample-1.2.3.tar.gz", SHA256: hex.EncodeToString(digest[:])}, "workflow"}
	mux := http.NewServeMux()
	mux.HandleFunc("/packages/latest.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"manifest":"releases/1/release.json"}`)
	})
	mux.HandleFunc("/releases/1/release.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"schema":1,"stores":{"test-platform":{"manifest":"store/index.json"}}}`)
	})
	mux.HandleFunc("/releases/1/store/index.json", func(w http.ResponseWriter, r *http.Request) {
		manifest := packagebuild.StoreBuildManifest{Schema: 1}
		manifest.Packages.Workflows = []packagebuild.StoreArtifact{entry.StoreArtifact}
		json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/releases/1/store/"+entry.Tarball, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	client := NewClient()
	client.http.Transport = server.Client().Transport
	return client, server, entry
}

func TestCatalogInstallAndReplacement(t *testing.T) {
	archive := testArchive(t, nil)
	client, server, entry := fixture(t, archive)
	catalog, err := client.Load(context.Background(), server.URL+"/packages/latest.json", "test-platform")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Packages) != 1 || catalog.Packages[0].Name != "sample" || !strings.HasSuffix(catalog.Manifest, "/store/index.json") {
		t.Fatalf("wrong catalog: %+v", catalog)
	}
	root := t.TempDir()
	installed, err := client.Install(context.Background(), catalog, entry, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Install(context.Background(), catalog, entry, root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(installed)
	if err != nil || !bytes.Equal(data, archive) {
		t.Fatalf("installed bytes: %v", err)
	}
	files, _ := os.ReadDir(filepath.Dir(installed))
	if len(files) != 1 {
		t.Fatalf("temporary files leaked: %v", files)
	}
}

func TestInstallFailurePreservesExisting(t *testing.T) {
	for _, scenario := range []string{"checksum", "cancelled", "traversal", "link", "identity", "duplicate", "absolute"} {
		t.Run(scenario, func(t *testing.T) {
			var extra *tar.Header
			switch scenario {
			case "traversal":
				extra = &tar.Header{Name: "workflows/sample/../../outside", Typeflag: tar.TypeReg}
			case "link":
				extra = &tar.Header{Name: "workflows/sample/link", Linkname: "/tmp/outside", Typeflag: tar.TypeSymlink}
			case "duplicate":
				extra = &tar.Header{Name: "workflows/sample/package.yml", Typeflag: tar.TypeReg}
			case "absolute":
				extra = &tar.Header{Name: "/workflows/sample/outside", Typeflag: tar.TypeReg}
			}
			client, server, entry := fixture(t, testArchive(t, extra))
			catalog, err := client.Load(context.Background(), server.URL+"/packages/latest.json", "test-platform")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "checksum" {
				entry.SHA256 = strings.Repeat("0", 64)
			}
			if scenario == "cancelled" {
				cancel()
			}
			if scenario == "identity" {
				entry.Name = "other"
				entry.Tarball = "dockpipe-workflow-other-1.2.3.tar.gz"
			}
			root := t.TempDir()
			oldPath := filepath.Join(root, "workflows", entry.Tarball)
			if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(oldPath, []byte("previous"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Install(ctx, catalog, entry, root); err == nil {
				t.Fatal("unsafe install accepted")
			}
			data, _ := os.ReadFile(oldPath)
			if string(data) != "previous" {
				t.Fatal("existing package changed on failure")
			}
			files, _ := os.ReadDir(filepath.Dir(oldPath))
			if len(files) != 1 {
				t.Fatal("temporary file leaked")
			}
		})
	}
}

func TestCatalogRejectsUnsafeMetadata(t *testing.T) {
	for _, payload := range []string{
		`{"manifest":"../other.json"}`,
		`{"manifest":"https://example.com/other.json"}`,
		`{"manifest":"%2e%2e/other.json"}`,
		`{"schema":2,"packages":{}}`,
		`{"schema":1,"packages":{}}`,
		`{"schema":1,"stores":{"different-platform":{"manifest":"store.json"}}}`,
		`{"schema":1,"packages":{"workflows":[{"name":"sample","version":"1","tarball":"../a.tar.gz","sha256":"bad"}]}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, payload) }))
			defer server.Close()
			client := NewClient()
			client.http.Transport = server.Client().Transport
			if _, err := client.Load(context.Background(), server.URL+"/index.json", "test-platform"); err == nil {
				t.Fatal("unsafe catalog accepted")
			}
		})
	}
	for _, remote := range []string{"http://example.com/index.json", "file:///tmp/index.json", "https://user:pass@example.com/index.json", "https://example.com/index.json?secret=value"} {
		if _, err := NewClient().Load(context.Background(), remote, "test-platform"); err == nil {
			t.Fatalf("unsafe remote accepted: %s", remote)
		}
	}
}

func TestRedirectPolicy(t *testing.T) {
	client := NewClient()
	for _, destination := range []string{"http://example.com/index.json", "https://other.example.com/index.json"} {
		initial, _ := http.NewRequest(http.MethodGet, "https://example.com/index.json", nil)
		next, _ := http.NewRequest(http.MethodGet, destination, nil)
		if err := client.http.CheckRedirect(next, []*http.Request{initial}); err == nil {
			t.Fatalf("redirect accepted: %s", destination)
		}
	}
}

func TestArchiveIdentityMismatch(t *testing.T) {
	archive := testArchive(t, nil)
	entry := Entry{packagebuild.StoreArtifact{Name: "sample", Version: "9.0.0"}, "workflow"}
	if err := validateArchive(context.Background(), bytes.NewReader(archive), entry); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("identity mismatch: %v", err)
	}
}

func TestOversizedManifest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat(" ", maxManifestBytes+1))
	}))
	defer server.Close()
	client := NewClient()
	client.http.Transport = server.Client().Transport
	if _, err := client.Load(context.Background(), server.URL+"/index.json", "test-platform"); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized manifest: %v", err)
	}
}

func TestInstallRejectsStoreSymlinkEscape(t *testing.T) {
	client, server, entry := fixture(t, testArchive(t, nil))
	catalog, err := client.Load(context.Background(), server.URL+"/packages/latest.json", "test-platform")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "workflows")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := client.Install(context.Background(), catalog, entry, root); err == nil {
		t.Fatal("store symlink escape accepted")
	}
	files, _ := os.ReadDir(outside)
	if len(files) != 0 {
		t.Fatal("wrote outside store")
	}
}

func TestEntryValidation(t *testing.T) {
	valid := Entry{packagebuild.StoreArtifact{Name: "sample", Version: "1.0.0", Tarball: "dockpipe-workflow-sample-1.0.0.tar.gz", SHA256: strings.Repeat("a", 64)}, "workflow"}
	for _, change := range []func(*Entry){
		func(entry *Entry) { entry.Name = "../sample" },
		func(entry *Entry) { entry.Name = "a..b" },
		func(entry *Entry) { entry.Kind = "unknown" },
		func(entry *Entry) { entry.Tarball = "dockpipe-workflow-other-1.0.0.tar.gz" },
		func(entry *Entry) { entry.SHA256 = "" },
		func(entry *Entry) { entry.SHA256 = strings.Repeat("z", 64) },
	} {
		entry := valid
		change(&entry)
		if err := validateEntry(entry); err == nil {
			t.Fatalf("invalid entry accepted: %+v", entry)
		}
	}
}
