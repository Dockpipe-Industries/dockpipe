package packagebuild

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure/archivecodec"
)

const maxPackageArchiveDecodedBytes int64 = 2 << 30

// ReadFileFromTarGz returns the uncompressed body of the first tar member whose path equals entryName.
// The historical function name remains compatible with callers; archivecodec selects the decoder.
// entryName uses forward slashes (e.g. workflows/demo/config.yml). Does not extract to disk — suitable
// for workflow resolution (dockpipe run) without extracting the archive.
func ReadFileFromTarGz(tarGzPath, entryName string) ([]byte, error) {
	return readFileFromTarGz(context.Background(), tarGzPath, entryName, 0)
}

// ReadFileFromTarGzContext reads one package member while propagating the
// caller's cancellation to its decoder, including an external codec provider.
func ReadFileFromTarGzContext(ctx context.Context, tarPath, entryName string) ([]byte, error) {
	return readFileFromTarGz(ctx, tarPath, entryName, 0)
}

// ReadFileFromTarGzLimit bounds allocation when reading package metadata.
func ReadFileFromTarGzLimit(tarGzPath, entryName string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("tar member size limit must be positive")
	}
	return readFileFromTarGz(context.Background(), tarGzPath, entryName, maxBytes)
}

func readFileFromTarGz(ctx context.Context, tarGzPath, entryName string, maxBytes int64) (data []byte, err error) {
	if ctx == nil {
		return nil, fmt.Errorf("package archive context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entryName = filepath.ToSlash(strings.TrimSpace(entryName))
	if entryName == "" || strings.Contains(entryName, "..") {
		return nil, fmt.Errorf("invalid entry path %q", entryName)
	}
	decoded, err := archivecodec.OpenFile(ctx, tarGzPath, maxPackageArchiveDecodedBytes)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, decoded.Close())
	}()
	tr := tar.NewReader(decoded)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("entry %q not found in %s: %w", entryName, tarGzPath, os.ErrNotExist)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		name = strings.TrimPrefix(name, "./")
		name = filepath.ToSlash(name)
		if name == entryName {
			if maxBytes > 0 && hdr.Size > maxBytes {
				return nil, fmt.Errorf("tar member %q exceeds size limit", entryName)
			}
			limit := maxPackageArchiveDecodedBytes
			if maxBytes > 0 && maxBytes < limit {
				limit = maxBytes
			}
			data, err := io.ReadAll(io.LimitReader(tr, limit+1))
			if err != nil {
				return nil, err
			}
			if int64(len(data)) > limit {
				return nil, fmt.Errorf("tar member %q exceeds size limit", entryName)
			}
			return data, nil
		}
	}
}

// ListTarGzMemberPaths returns regular-file member paths from a supported package archive.
func ListTarGzMemberPaths(tarGzPath string) (out []string, err error) {
	decoded, err := archivecodec.OpenFile(context.Background(), tarGzPath, maxPackageArchiveDecodedBytes)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, decoded.Close())
	}()
	tr := tar.NewReader(decoded)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != 0 {
			continue
		}
		name := strings.TrimSuffix(strings.TrimSpace(hdr.Name), "/")
		name = strings.TrimPrefix(name, "./")
		name = filepath.ToSlash(name)
		if name == "" || strings.Contains(name, "..") {
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, name)
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, err
		}
	}
	if _, err := io.Copy(io.Discard, decoded); err != nil {
		return nil, err
	}
	return out, nil
}

// WorkflowNameFromTarballMembers finds workflows/<name>/config.yml and returns name.
func WorkflowNameFromTarballMembers(members []string) (string, error) {
	for _, m := range members {
		parts := strings.Split(m, "/")
		if len(parts) >= 3 && parts[0] == "workflows" && parts[2] == "config.yml" {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("no workflows/<name>/config.yml in archive")
}
