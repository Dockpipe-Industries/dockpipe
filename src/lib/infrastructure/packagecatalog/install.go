package packagecatalog

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/archivecodec"
	"gopkg.in/yaml.v3"
)

const maxArchiveBytes int64 = 512 << 20
const maxExpandedBytes int64 = 2 << 30
const maxArchiveMembers = 100000

// Install publishes a verified tarball into the existing store layout. Nothing is
// extracted or executed during installation; readers use their normal tarball cache.
func (c *Client) Install(ctx context.Context, catalog Catalog, entry Entry, storeRoot string) (string, error) {
	if err := validateEntry(entry); err != nil {
		return "", err
	}
	if err := domain.CheckPackageEngineVersion(entry.Name, entry.MinDockpipeVersion, c.engineVersion); err != nil {
		return "", err
	}
	base, err := parseURL(catalog.Manifest)
	if err != nil {
		return "", err
	}
	target, err := childURL(base, entry.Tarball)
	if err != nil {
		return "", err
	}
	response, err := c.get(ctx, target)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(storeRoot)
	if err != nil {
		return "", err
	}
	defer root.Close()
	subdir := "core"
	if entry.Kind != "core" {
		subdir = entry.Kind + "s"
	}
	if err := root.MkdirAll(subdir, 0o755); err != nil {
		return "", err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	temporary := filepath.Join(subdir, ".download-"+hex.EncodeToString(nonce[:]))
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	defer root.Remove(temporary)
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maxArchiveBytes+1))
	if err != nil {
		return "", err
	}
	if size > maxArchiveBytes {
		return "", fmt.Errorf("package archive exceeds size limit")
	}
	if entry.SizeBytes > 0 && size != entry.SizeBytes {
		return "", fmt.Errorf("package archive size does not match catalog")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), entry.SHA256) {
		return "", fmt.Errorf("package checksum mismatch for %s", entry.Name)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if err := validateArchiveForEngine(ctx, file, entry, c.engineVersion); err != nil {
		return "", fmt.Errorf("invalid package archive: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	destination := filepath.Join(subdir, entry.Tarball)
	if err := root.Rename(temporary, destination); err != nil {
		return "", err
	}
	return filepath.Join(storeRoot, destination), nil
}

func validateArchiveForEngine(ctx context.Context, reader io.Reader, entry Entry, engineVersion string) (resultErr error) {
	codec := "gzip"
	if entry.Tarball != "" {
		var err error
		codec, err = archivecodec.CodecForPath(entry.Tarball)
		if err != nil {
			return err
		}
	}
	decoded, err := archivecodec.Open(ctx, reader, codec, maxExpandedBytes)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, decoded.Close()) }()
	hash := sha256.New()
	bounded := &io.LimitedReader{R: io.TeeReader(decoded, hash), N: maxExpandedBytes + 1}
	archive := tar.NewReader(bounded)
	prefix := "core"
	if entry.Kind != "core" {
		prefix = entry.Kind + "s/" + entry.Name
	}
	seen := make(map[string]bool)
	foundManifest := false
	for count := 0; ; count++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if count >= maxArchiveMembers || bounded.N <= 0 {
			return fmt.Errorf("package archive exceeds expansion limit")
		}
		name := strings.TrimSuffix(header.Name, "/")
		if path.Clean(name) != name || strings.Contains(name, "..") || strings.ContainsAny(name, "\\:") || (name != prefix && !strings.HasPrefix(name, prefix+"/")) {
			return fmt.Errorf("unsafe package archive path %q", header.Name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate package archive path %q", name)
		}
		seen[name] = true
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return fmt.Errorf("package archive links and special files are not supported: %s", name)
		}
		if name == prefix+"/package.yml" && header.Typeflag == tar.TypeReg {
			if header.Size > maxManifestBytes {
				return fmt.Errorf("package metadata exceeds size limit")
			}
			data, err := io.ReadAll(archive)
			if err != nil {
				return err
			}
			var manifest domain.PackageManifest
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				return err
			}
			if manifest.Name != entry.Name || (manifest.Version != "" && manifest.Version != entry.Version) || manifest.Kind != entry.Kind {
				return fmt.Errorf("package metadata does not match catalog identity")
			}
			if err := domain.CheckPackageEngineVersion(manifest.Name, manifest.MinDockpipeVersion, engineVersion); err != nil {
				return err
			}
			if err := domain.ValidatePackageManifest(&manifest); err != nil {
				return err
			}
			foundManifest = true
		}
	}
	// Complete the decoder, checking its trailer/exit status and the entire tar.
	if _, err := io.Copy(io.Discard, bounded); err != nil {
		return err
	}
	if bounded.N <= 0 {
		return fmt.Errorf("package archive exceeds expansion limit")
	}
	if entry.DecodedSHA256 != "" && (!strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), entry.DecodedSHA256) || maxExpandedBytes+1-bounded.N != entry.DecodedSizeBytes) {
		return fmt.Errorf("decoded package tar checksum or size mismatch")
	}
	if !foundManifest {
		return fmt.Errorf("package archive has no matching package.yml")
	}
	return nil
}
