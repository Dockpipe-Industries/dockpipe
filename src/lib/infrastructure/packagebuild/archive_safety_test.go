package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArchiveSafetyFixture(t *testing.T, header *tar.Header, body string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "fixture.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	for _, closer := range []interface{ Close() error }{archive, compressed, file} {
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return archivePath
}

func TestExtractRejectsUnsafeNamesAndSpecialMembers(t *testing.T) {
	for _, header := range []*tar.Header{
		{Name: "../outside", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "/absolute", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "a/../outside", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: `a\outside`, Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target"},
		{Name: "link", Typeflag: tar.TypeLink, Linkname: "target"},
		{Name: "fifo", Typeflag: tar.TypeFifo},
	} {
		t.Run(strings.ReplaceAll(header.Name, "/", "_"), func(t *testing.T) {
			archivePath := writeArchiveSafetyFixture(t, header, "")
			if err := ExtractTarGzToDir(archivePath, t.TempDir()); err == nil {
				t.Fatalf("accepted invalid archive member %#v", header)
			}
		})
	}
}

func TestExtractCannotWriteThroughExistingDestinationSymlink(t *testing.T) {
	destination := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(destination, "redirect")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	archivePath := writeArchiveSafetyFixture(t, &tar.Header{
		Name: "redirect/payload", Typeflag: tar.TypeReg, Mode: 0o644, Size: 7,
	}, "payload")
	if err := ExtractTarGzToDir(archivePath, destination); err == nil {
		t.Fatal("extraction through an escaping destination symlink succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "payload")); !os.IsNotExist(err) {
		t.Fatalf("outside path was modified: %v", err)
	}
}

func TestArchiveBuildFailurePreservesPreviousArchiveAndChecksum(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "unsafe..source"), []byte("invalid archive path"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "package.tar.gz")
	for name, contents := range map[string]string{archivePath: "previous archive", archivePath + ".sha256": "previous checksum"} {
		if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := WriteDirTarGzWithPrefix(source, archivePath, "workflows/demo"); err == nil {
		t.Fatal("invalid source unexpectedly built")
	}
	for name, contents := range map[string]string{archivePath: "previous archive", archivePath + ".sha256": "previous checksum"} {
		actual, err := os.ReadFile(name)
		if err != nil || string(actual) != contents {
			t.Fatalf("failed build changed previous artifact %s: %q, %v", name, actual, err)
		}
	}
}

func TestSelectedArchiveMemberRequiresValidDecoderTrailer(t *testing.T) {
	archivePath := writeArchiveSafetyFixture(t, &tar.Header{
		Name: "workflows/demo/config.yml", Typeflag: tar.TypeReg, Mode: 0o644, Size: 4,
	}, "demo")
	contents, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	contents[len(contents)-8] ^= 0xff
	if err := os.WriteFile(archivePath, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if body, err := ReadFileFromTarGz(archivePath, "workflows/demo/config.yml"); err == nil {
		t.Fatalf("read accepted a corrupt decoder trailer: %q", body)
	}
}

func TestArchiveChecksumDestinationFailurePreservesPreviousArchive(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "source.txt"), []byte("new archive payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "package.tar.gz")
	if err := os.WriteFile(archivePath, []byte("previous archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(archivePath+".sha256", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(source, archivePath, "workflows/demo"); err == nil {
		t.Fatal("checksum destination directory unexpectedly published")
	}
	actual, err := os.ReadFile(archivePath)
	if err != nil || string(actual) != "previous archive" {
		t.Fatalf("failed checksum preflight changed old archive: %q, %v", actual, err)
	}
	entries, err := os.ReadDir(filepath.Dir(archivePath))
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary publication files remain: %v, %v", entries, err)
	}
}

func TestArchiveBuildRejectsOutputInsideSourceBeforeCreatingFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		nested bool
	}{
		{name: "existing-output"},
		{name: "missing-output-directory", nested: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			payload := filepath.Join(source, "source.txt")
			if err := os.WriteFile(payload, []byte("source payload"), 0o644); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(source, "package.tar.gz")
			if test.nested {
				archivePath = filepath.Join(source, "new", "nested", "package.tar.gz")
			} else {
				if err := os.WriteFile(archivePath, []byte("previous archive"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := WriteDirTarGzWithPrefix(source, archivePath, "workflows/demo"); err == nil {
				t.Fatal("accepted archive output inside source")
			}
			actual, err := os.ReadFile(payload)
			if err != nil || string(actual) != "source payload" {
				t.Fatalf("source changed: %q, %v", actual, err)
			}
			if test.nested {
				if _, err := os.Stat(filepath.Join(source, "new")); !os.IsNotExist(err) {
					t.Fatalf("preflight created output directories: %v", err)
				}
			} else {
				actual, err := os.ReadFile(archivePath)
				if err != nil || string(actual) != "previous archive" {
					t.Fatalf("previous output changed: %q, %v", actual, err)
				}
			}
			if _, err := os.Stat(archivePath + ".sha256"); !os.IsNotExist(err) {
				t.Fatalf("preflight created checksum: %v", err)
			}
		})
	}
}

func TestArchiveBuildRejectsAliasedOutputInsideSource(t *testing.T) {
	source := t.TempDir()
	alias := filepath.Join(t.TempDir(), "source-alias")
	if err := os.Symlink(source, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, paths := range [][2]string{
		{alias, filepath.Join(source, "package.tar.gz")},
		{source, filepath.Join(alias, "new", "package.tar.gz")},
	} {
		if _, err := WriteDirTarGzWithPrefix(paths[0], paths[1], "workflows/demo"); err == nil {
			t.Fatalf("accepted aliased source/output pair %v", paths)
		}
	}
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preflight created output inside source: %v, %v", entries, err)
	}
}

func TestArchiveDestinationSourceAncestry(t *testing.T) {
	source := filepath.Join(t.TempDir(), "Source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		destination string
		inside      bool
	}{
		{name: "direct", destination: filepath.Join(source, "package.tar.gz"), inside: true},
		{name: "missing-suffix", destination: filepath.Join(source, "new", "nested", "package.tar.gz"), inside: true},
		{name: "outside", destination: filepath.Join(t.TempDir(), "package.tar.gz")},
	} {
		t.Run(test.name, func(t *testing.T) {
			inside, err := archiveDestinationHasSourceAncestor(test.destination, sourceInfo)
			if err != nil || inside != test.inside {
				t.Fatalf("source ancestry = %v, %v; want %v", inside, err, test.inside)
			}
		})
	}
}

func TestArchiveBuildRejectsFilesystemCaseAliasInsideSource(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "Source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "source")
	sourceInfo, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("filesystem distinguishes path case")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(sourceInfo, aliasInfo) {
		t.Skip("case variant is a different filesystem object")
	}
	archivePath := filepath.Join(alias, "new", "package.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(source, archivePath, "workflows/demo"); err == nil {
		t.Fatal("accepted output through a filesystem case alias of source")
	}
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preflight created output inside source: %v, %v", entries, err)
	}
}
