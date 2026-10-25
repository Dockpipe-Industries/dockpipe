package packagebuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteDirTarGzWithPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "p.tar.gz")
	sum, err := WriteDirTarGzWithPrefix(dir, out, "workflows/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) != 64 {
		t.Fatalf("sha256 len: %s", sum)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		if h.Name == "workflows/demo/a/b/f.txt" {
			b, _ := io.ReadAll(tr)
			if string(b) != "x" {
				t.Fatalf("content %q", b)
			}
		}
	}
	if !strings.Contains(strings.Join(names, ","), "workflows/demo/a/b/f.txt") {
		t.Fatalf("names: %v", names)
	}
}

func TestWriteDirTarGzWithPrefix_ForcesExecutableModesForPackagedPayloads(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "assets", "scripts", "run.sh")
	bin := filepath.Join(dir, "assets", "tooling", "bin", "linux", "dockpipe")
	doc := filepath.Join(dir, "assets", "docs", "readme.txt")
	for _, p := range []string{script, bin, doc} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(t.TempDir(), "p.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(dir, out, "workflows/demo"); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	modes := map[string]int64{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		modes[h.Name] = h.Mode
	}

	if got := modes["workflows/demo/assets/scripts/run.sh"]; got != 0o755 {
		t.Fatalf("script mode = %o, want 755", got)
	}
	if got := modes["workflows/demo/assets/tooling/bin/linux/dockpipe"]; got != 0o755 {
		t.Fatalf("binary mode = %o, want 755", got)
	}
	if got := modes["workflows/demo/assets/docs/readme.txt"]; got != 0o644 {
		t.Fatalf("doc mode = %o, want 644", got)
	}
}

func TestWriteDirTarGzWithPrefixIgnoresSourceTimestamps(t *testing.T) {
	var archives [][]byte
	var checksums []string
	prefix := "packages/widget"
	payloads := map[string]string{
		"assets/scripts/run.sh": "#!/bin/sh\nprintf widget\n",
		"docs/readme.txt":       "widget documentation\n",
	}

	for index, modified := range []time.Time{
		time.Unix(1_600_000_000, 123_456_789),
		time.Unix(1_800_000_000, 987_654_321),
	} {
		source := t.TempDir()
		for name, contents := range payloads {
			path := filepath.Join(source, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			return os.Chtimes(path, modified.Add(time.Hour), modified)
		}); err != nil {
			t.Fatal(err)
		}

		output := filepath.Join(t.TempDir(), "widget.tar.gz")
		checksum, err := WriteDirTarGzWithPrefix(source, output, prefix)
		if err != nil {
			t.Fatal(err)
		}
		archive, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		archives = append(archives, archive)
		checksums = append(checksums, checksum)

		compressed, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			t.Fatal(err)
		}
		reader := tar.NewReader(compressed)
		found := map[string]bool{}
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(header.Name, prefix+"/") {
				t.Fatalf("archive escaped prefix: %q", header.Name)
			}
			if !header.ModTime.Equal(time.Unix(0, 0)) || !header.AccessTime.IsZero() || !header.ChangeTime.IsZero() {
				t.Fatalf("tree %d retained source timestamp metadata: %+v", index, header)
			}
			relative := strings.TrimPrefix(header.Name, prefix+"/")
			info, err := os.Stat(filepath.Join(source, filepath.FromSlash(relative)))
			if err != nil {
				t.Fatal(err)
			}
			original, err := tar.FileInfoHeader(info, "")
			if err != nil {
				t.Fatal(err)
			}
			if header.Uid != original.Uid || header.Gid != original.Gid || header.Uname != original.Uname || header.Gname != original.Gname {
				t.Fatal("archive ownership contract changed")
			}
			if contents, exists := payloads[relative]; exists {
				actual, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if string(actual) != contents {
					t.Fatalf("payload changed: %q", relative)
				}
				mode := int64(0o644)
				if relative == "assets/scripts/run.sh" {
					mode = 0o755
				}
				if header.Mode != mode {
					t.Fatalf("mode for %q = %o, want %o", relative, header.Mode, mode)
				}
				found[relative] = true
			}
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
		if len(found) != len(payloads) {
			t.Fatalf("missing payloads: %v", found)
		}
	}

	if !bytes.Equal(archives[0], archives[1]) || checksums[0] != checksums[1] {
		t.Fatal("identical payloads produced different package bytes across source timestamps")
	}
}
