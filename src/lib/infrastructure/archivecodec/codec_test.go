package archivecodec

import (
	"bytes"
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

func encodedFixture(t *testing.T, codec string, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	var writer io.WriteCloser
	if codec == "gzip" {
		writer = gzip.NewWriter(&output)
	} else {
		var err error
		writer, err = zstd.NewWriter(&output, zstd.WithEncoderConcurrency(1))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestCanonicalExtensions(t *testing.T) {
	for codec, suffix := range map[string]string{"gzip": ".tar.gz", "zstd": ".tar.zst", "private9": ".tar.private9"} {
		extension, err := Extension(codec)
		if err != nil || extension != suffix {
			t.Fatalf("extension %s: %q, %v", codec, extension, err)
		}
		name := "dockpipe-example-1.0" + suffix
		identified, err := CodecForPath(name)
		if err != nil || identified != codec {
			t.Fatalf("identity %s: %q, %v", name, identified, err)
		}
		if stripped, ok := StripExtension(name); !ok || stripped != "dockpipe-example-1.0" {
			t.Fatalf("strip %q: %q, %v", name, stripped, ok)
		}
	}
	for _, codec := range []string{"", "gz", "zst", "UPPER", "a/b", "a-b", strings.Repeat("a", 33)} {
		if _, err := Extension(codec); err == nil {
			t.Fatalf("accepted invalid codec %q", codec)
		}
	}
	for _, name := range []string{"x.tar", "x.tar.gzip", "x.tar.zstd", "x.tar.UPPER", "x.tar.gz/child"} {
		if _, err := CodecForPath(name); err == nil {
			t.Fatalf("accepted invalid archive %q", name)
		}
	}
}

func TestBuiltinIntegrityAndLimits(t *testing.T) {
	data := bytes.Repeat([]byte("archive payload\n"), 1000)
	for _, codec := range []string{"gzip", "zstd"} {
		t.Run(codec, func(t *testing.T) {
			encoded := encodedFixture(t, codec, data)
			for _, limit := range []int64{int64(len(data)), int64(len(data)) + 1} {
				reader, err := Open(context.Background(), bytes.NewReader(encoded), codec, limit)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := io.ReadAll(reader)
				closeErr := reader.Close()
				if err != nil || closeErr != nil || !bytes.Equal(actual, data) {
					t.Fatalf("roundtrip: read=%v close=%v", err, closeErr)
				}
			}
			for _, limit := range []int64{0, int64(len(data)) - 1} {
				reader, err := Open(context.Background(), bytes.NewReader(encoded), codec, limit)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := io.ReadAll(reader)
				reader.Close()
				if !errors.Is(err, ErrDecodedLimit) || int64(len(actual)) > limit {
					t.Fatalf("limit: %d bytes, %v", len(actual), err)
				}
			}
			corrupt := bytes.Clone(encoded)
			corrupt[len(corrupt)-1] ^= 0xff
			for _, invalid := range [][]byte{encoded[:len(encoded)-1], corrupt} {
				reader, err := Open(context.Background(), bytes.NewReader(invalid), codec, int64(len(data)))
				if err == nil {
					_, err = io.ReadAll(reader)
					reader.Close()
				}
				if err == nil {
					t.Fatal("accepted truncated/corrupt complete frame")
				}
			}
		})
	}
}

func TestCancellationAndFileOwnership(t *testing.T) {
	for _, codec := range []string{"gzip", "zstd"} {
		t.Run(codec, func(t *testing.T) {
			data := []byte("payload")
			extension, _ := Extension(codec)
			path := filepath.Join(t.TempDir(), "example"+extension)
			if err := os.WriteFile(path, encodedFixture(t, codec, data), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			reader, err := OpenFile(ctx, path, 100)
			if err != nil {
				t.Fatal(err)
			}
			cancel()
			if _, err := reader.Read(make([]byte, 1)); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatalf("source still open: %v", err)
			}
		})
	}
}
