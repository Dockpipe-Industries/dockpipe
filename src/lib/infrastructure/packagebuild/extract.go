package packagebuild

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure/archivecodec"
)

// ExtractTarGzToDir extracts all regular files from a supported package archive into destDir.
// Member paths must be local and canonical; links and special files are rejected.
// On failure, files already extracted into destDir remain owned by the caller.
func ExtractTarGzToDir(tarGzPath, destDir string) (err error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, root.Close())
	}()
	decoded, err := archivecodec.OpenFile(context.Background(), tarGzPath, maxPackageArchiveDecodedBytes)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, decoded.Close())
	}()
	tr := tar.NewReader(decoded)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			_, err := io.Copy(io.Discard, decoded)
			return err
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if hdr.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "" || path.Clean(name) != name || strings.ContainsAny(name, "\\:") || !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("unsafe tar path %q", hdr.Name)
		}
		target := filepath.FromSlash(name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := normalizeArchiveMode(name, os.FileMode(hdr.Mode&0o777), false)
			out, err := root.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			if shouldForceExecutable(name) {
				if err := markExecutableOnDisk(filepath.Join(destDir, target)); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported tar member type %d for %q", hdr.Typeflag, hdr.Name)
		}
	}
}

const extractMarkerFile = ".dockpipe-extracted"
const extractPackageMarkerFile = ".dockpipe-extracted-package.json"

type extractedPackageMarker struct {
	SHA256  string `json:"sha256"`
	Tarball string `json:"tarball"`
}

// EnsureTarballExtractedCache extracts tarGzPath into cacheRoot/<sha256(tar file)> if not already present.
// The cache directory is keyed by the tarball file digest so content changes invalidate the cache.
func EnsureTarballExtractedCache(tarGzPath, cacheRoot string) (string, error) {
	sum, err := sha256File(tarGzPath)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(cacheRoot, sum)
	marker := filepath.Join(dest, extractMarkerFile)
	if b, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(b)) == sum {
		if st, err := os.Stat(dest); err == nil && st.IsDir() {
			return dest, nil
		}
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	if err := ExtractTarGzToDir(tarGzPath, dest); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}
	if err := os.WriteFile(marker, []byte(sum+"\n"), 0o644); err != nil {
		return "", err
	}
	meta := extractedPackageMarker{
		SHA256:  sum,
		Tarball: filepath.Base(tarGzPath),
	}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dest, extractPackageMarkerFile), append(b, '\n'), 0o644)
	}
	return dest, nil
}
