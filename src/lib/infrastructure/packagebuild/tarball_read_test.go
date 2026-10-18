package packagebuild

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestReadFileFromTarGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte("name: x\nrun: y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, "out.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(src, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	b, err := ReadFileFromTarGz(tgz, "workflows/demo/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "name: x\nrun: y\n" {
		t.Fatalf("got %q", b)
	}
}

func TestReadFileFromTarGzContextCancelledBeforeOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ReadFileFromTarGzContext(ctx, "missing.tar.zst", "workflows/demo/config.yml")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read returned %v", err)
	}
}

func TestReadAndExtractZstdPackageArchive(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gzipPath := filepath.Join(directory, "dockpipe-workflow-demo-1.2.3.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(source, gzipPath, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(gzipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	decoded, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	tarBytes, err := io.ReadAll(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := decoded.Close(); err != nil {
		t.Fatal(err)
	}
	zstdPath := strings.TrimSuffix(gzipPath, ".gz") + ".zst"
	output, err := os.Create(zstdPath)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.Write(tarBytes); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	entry := "workflows/demo/config.yml"
	data, err := ReadFileFromTarGz(zstdPath, entry)
	if err != nil || string(data) != "name: demo\n" {
		t.Fatalf("zstd member: %q, %v", data, err)
	}
	members, err := ListTarGzMemberPaths(zstdPath)
	if err != nil || len(members) != 1 || members[0] != entry {
		t.Fatalf("zstd members: %v, %v", members, err)
	}
	destination := filepath.Join(directory, "extracted")
	if err := ExtractTarGzToDir(zstdPath, destination); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(destination, filepath.FromSlash(entry)))
	if err != nil || string(data) != "name: demo\n" {
		t.Fatalf("zstd extraction: %q, %v", data, err)
	}
}
