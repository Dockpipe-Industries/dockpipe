package packagecatalog

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
	"github.com/klauspost/compress/zstd"
)

func alternateFixture(t *testing.T) (Entry, []byte, []byte) {
	t.Helper()
	gzipBytes := testArchive(t, nil)
	gz, err := gzip.NewReader(bytes.NewReader(gzipBytes))
	if err != nil {
		t.Fatal(err)
	}
	var tarBytes bytes.Buffer
	if _, err := tarBytes.ReadFrom(gz); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	compressed := encoder.EncodeAll(tarBytes.Bytes(), nil)
	entry := Entry{StoreArtifact: packagebuild.StoreArtifact{
		Name: "sample", Version: "1.2.3", Tarball: "dockpipe-workflow-sample-1.2.3.tar.gz",
		SHA256: digestBytes(gzipBytes), SizeBytes: int64(len(gzipBytes)),
		DecodedSHA256: digestBytes(tarBytes.Bytes()), DecodedSizeBytes: int64(tarBytes.Len()),
		Variants: []packagebuild.ArchiveVariant{{
			Codec: "zstd", Tarball: "dockpipe-workflow-sample-1.2.3.tar.zst",
			SHA256: digestBytes(compressed), SizeBytes: int64(len(compressed)),
			DecodedSHA256: digestBytes(tarBytes.Bytes()), DecodedSizeBytes: int64(tarBytes.Len()),
		}},
	}, Kind: "workflow"}
	return entry, gzipBytes, compressed
}

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func TestAlternateArchiveInstallKeepsLegacyAndReadsCache(t *testing.T) {
	entry, gzipBytes, compressed := alternateFixture(t)
	manifest := packagebuild.StoreBuildManifest{Schema: 1, Platform: "test-platform"}
	manifest.Packages.Workflows = []packagebuild.StoreArtifact{entry.StoreArtifact}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_ = json.NewEncoder(w).Encode(manifest)
		case "/" + entry.Tarball:
			_, _ = w.Write(gzipBytes)
		case "/" + entry.Variants[0].Tarball:
			_, _ = w.Write(compressed)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient()
	client.http.Transport = server.Client().Transport
	catalog, err := client.Load(context.Background(), server.URL+"/index.json", "test-platform")
	if err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	for _, codec := range []string{"gzip", "zstd"} {
		installed, err := client.InstallWithDependenciesUsingCodec(context.Background(), catalog, catalog.Packages[0], store, codec)
		if err != nil {
			t.Fatalf("%s install: %v", codec, err)
		}
		selected, err := SelectArchive(entry, codec)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(installed)
		if err != nil || digestBytes(data) != selected.SHA256 {
			t.Fatalf("%s did not store its exact encoded bytes: %v", codec, err)
		}
		cache, err := packagebuild.EnsureTarballExtractedCache(installed, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(cache, "workflows", "sample", "config.yml")); err != nil {
			t.Fatal(err)
		}
	}
	for _, filename := range []string{entry.Tarball, entry.Variants[0].Tarball} {
		if _, err := os.Stat(filepath.Join(store, "workflows", filename)); err != nil {
			t.Fatalf("coexisting archive missing: %v", err)
		}
	}
}

func TestAlternateArchiveValidation(t *testing.T) {
	entry, _, compressed := alternateFixture(t)
	missingCanonicalDigest := entry
	missingCanonicalDigest.DecodedSHA256 = ""
	if err := validateEntry(missingCanonicalDigest); err == nil {
		t.Fatal("variant accepted without canonical decoded identity")
	}
	selected, err := SelectArchive(entry, "zstd")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateArchiveForEngine(context.Background(), bytes.NewReader(compressed), selected, "dev"); err != nil {
		t.Fatal(err)
	}
	selected.DecodedSHA256 = strings.Repeat("a", 64)
	if err := validateArchiveForEngine(context.Background(), bytes.NewReader(compressed), selected, "dev"); err == nil {
		t.Fatal("decoded digest mismatch accepted")
	}
	selected, _ = SelectArchive(entry, "zstd")
	if err := validateArchiveForEngine(context.Background(), bytes.NewReader(compressed[:len(compressed)-1]), selected, "dev"); err == nil {
		t.Fatal("truncated zstd archive accepted")
	}
	if _, err := SelectArchive(entry, "missing"); err == nil {
		t.Fatal("missing requested variant silently downgraded")
	}
	entry.Variants = append(entry.Variants, entry.Variants[0])
	if err := validateEntry(entry); err == nil {
		t.Fatal("duplicate variant accepted")
	}
}
