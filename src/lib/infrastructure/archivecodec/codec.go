// Package archivecodec decodes package transport formats without choosing package
// identities or executable providers from catalog data.
package archivecodec

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/klauspost/compress/zstd"
)

var codecID = regexp.MustCompile(`^[a-z][a-z0-9]{0,31}$`)

// Extension returns a canonical archive suffix, independent of provider availability.
func Extension(codec string) (string, error) {
	switch codec {
	case "gzip":
		return ".tar.gz", nil
	case "zstd":
		return ".tar.zst", nil
	case "gz", "zst":
		return "", fmt.Errorf("reserved archive codec alias %q", codec)
	}
	if !codecID.MatchString(codec) {
		return "", fmt.Errorf("invalid archive codec %q", codec)
	}
	return ".tar." + codec, nil
}

func CodecForPath(name string) (string, error) {
	position := strings.LastIndex(name, ".tar.")
	if position <= 0 || name[position-1] == '/' || name[position-1] == '\\' {
		return "", fmt.Errorf("unrecognized archive suffix %q", name)
	}
	suffix := name[position+5:]
	switch suffix {
	case "gz":
		return "gzip", nil
	case "zst":
		return "zstd", nil
	case "gzip", "zstd":
		return "", fmt.Errorf("noncanonical archive suffix %q", name)
	}
	if _, err := Extension(suffix); err != nil {
		return "", err
	}
	return suffix, nil
}

func StripExtension(name string) (string, bool) {
	codec, err := CodecForPath(name)
	if err != nil {
		return name, false
	}
	extension, _ := Extension(codec)
	return strings.TrimSuffix(name, extension), true
}

// Open borrows source. Callers must read through EOF to observe complete builtin
// integrity checks, and Close every successful result. Cancellation is checked
// between reads; the source owner must interrupt any blocking underlying I/O.
// External decoders finish successfully before their private output is exposed.
func Open(ctx context.Context, source io.Reader, codec string, maxDecodedBytes int64) (io.ReadCloser, error) {
	if ctx == nil || source == nil || maxDecodedBytes < 0 {
		return nil, errors.New("invalid archive decoder arguments")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := Extension(codec); err != nil {
		return nil, err
	}
	input := contextReader{ctx: ctx, reader: source}
	var decoded io.ReadCloser
	switch codec {
	case "gzip":
		reader, err := gzip.NewReader(input)
		if err != nil {
			return nil, err
		}
		decoded = reader
	case "zstd":
		// Bound decoder workspace independently of the caller's output limit.
		reader, err := zstd.NewReader(input, zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderLowmem(true), zstd.WithDecoderMaxMemory(64<<20),
			zstd.WithDecoderMaxWindow(64<<20))
		if err != nil {
			return nil, err
		}
		decoded = reader.IOReadCloser()
	default:
		return openExternal(ctx, input, codec, maxDecodedBytes)
	}
	return &boundedReader{ctx: ctx, reader: decoded, remaining: maxDecodedBytes}, nil
}

func OpenFile(ctx context.Context, path string, maxDecodedBytes int64) (io.ReadCloser, error) {
	codec, err := CodecForPath(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	reader, err := Open(ctx, file, codec, maxDecodedBytes)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return &fileReader{ReadCloser: reader, file: file}, nil
}

type fileReader struct {
	io.ReadCloser
	file *os.File
}

func (reader *fileReader) Close() error {
	return errors.Join(reader.ReadCloser.Close(), reader.file.Close())
}
