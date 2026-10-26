package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/filepublication"
)

// WriteDirTarGzWithPrefix packs absSrcDir into a gzip tar at outPath. Archive paths are
// prefixInArchive/rel (forward slashes). prefixInArchive must be non-empty and must not contain "..".
func WriteDirTarGzWithPrefix(absSrcDir, outPath, prefixInArchive string) (sha256Hex string, err error) {
	absSrcDir = filepath.Clean(absSrcDir)
	prefixInArchive = strings.Trim(prefixInArchive, "/\\")
	prefixInArchive = filepath.ToSlash(filepath.Clean(prefixInArchive))
	if prefixInArchive == "" || prefixInArchive == "." || strings.Contains(prefixInArchive, "..") {
		return "", fmt.Errorf("invalid archive prefix %q", prefixInArchive)
	}
	st, err := os.Stat(absSrcDir)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", absSrcDir)
	}
	canonicalSource, err := filepath.EvalSymlinks(absSrcDir)
	if err != nil {
		return "", err
	}
	canonicalSource, err = filepath.Abs(canonicalSource)
	if err != nil {
		return "", err
	}
	for _, destination := range []string{outPath, outPath + ".sha256"} {
		canonicalDestination, err := canonicalArchiveDestination(destination)
		if err != nil {
			return "", err
		}
		insideSource, err := archiveDestinationHasSourceAncestor(canonicalDestination, st)
		if err != nil {
			return "", err
		}
		if insideSource {
			return "", fmt.Errorf("archive output must be outside its source directory: %s", destination)
		}
		if !strings.EqualFold(filepath.VolumeName(canonicalSource), filepath.VolumeName(canonicalDestination)) {
			continue
		}
		relative, err := filepath.Rel(canonicalSource, canonicalDestination)
		if err != nil {
			return "", err
		}
		if filepath.IsLocal(relative) {
			return "", fmt.Errorf("archive output must be outside its source directory: %s", destination)
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}
	w, err := os.CreateTemp(filepath.Dir(outPath), ".dockpipe-archive-*")
	if err != nil {
		return "", err
	}
	temporaryArchive := w.Name()
	defer os.Remove(temporaryArchive)
	if err := w.Chmod(0o644); err != nil {
		_ = w.Close()
		return "", err
	}
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	walkErr := filepath.WalkDir(absSrcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(absSrcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		if strings.Contains(rel, "..") {
			return fmt.Errorf("unsafe path %q", rel)
		}
		nameInTar := prefixInArchive + "/"
		if rel != "" {
			nameInTar = prefixInArchive + "/" + filepath.ToSlash(rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		// Package identity depends on payloads and modes, not source-build timestamps.
		hdr.ModTime = time.Unix(0, 0).UTC()
		hdr.AccessTime = time.Time{}
		hdr.ChangeTime = time.Time{}
		hdr.Name = nameInTar
		hdr.Mode = int64(normalizeArchiveMode(hdr.Name, info.Mode(), info.IsDir()))
		if hdr.Typeflag == tar.TypeDir && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		_ = f.Close()
		return err
	})
	if walkErr != nil {
		_ = tw.Close()
		_ = gz.Close()
		_ = w.Close()
		_ = os.Remove(temporaryArchive)
		return "", walkErr
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		_ = w.Close()
		_ = os.Remove(temporaryArchive)
		return "", err
	}
	if err := gz.Close(); err != nil {
		_ = w.Close()
		_ = os.Remove(temporaryArchive)
		return "", err
	}
	if err := w.Sync(); err != nil {
		_ = w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		_ = os.Remove(temporaryArchive)
		return "", err
	}
	sum, err := sha256File(temporaryArchive)
	if err != nil {
		return "", err
	}
	shaPath := outPath + ".sha256"
	temporaryChecksum, err := filepublication.Stage(filepath.Dir(outPath), ".dockpipe-checksum-*", []byte(sum+"\n"))
	if err != nil {
		return "", err
	}
	defer os.Remove(temporaryChecksum)
	if err := os.Chmod(temporaryChecksum, 0o644); err != nil {
		return "", err
	}
	for _, destination := range []string{outPath, shaPath} {
		info, err := os.Lstat(destination)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && info.IsDir() {
			return "", fmt.Errorf("artifact destination is a directory: %s", destination)
		}
	}
	// Both outputs are complete before replacing either previous artifact.
	// Two paths cannot be published atomically; report a checksum publication
	// failure explicitly if the archive rename has already succeeded.
	if err := os.Rename(temporaryArchive, outPath); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryChecksum, shaPath); err != nil {
		return "", fmt.Errorf("archive published but checksum publication failed: %w", err)
	}
	return sum, nil
}

// canonicalArchiveDestination resolves existing parent aliases without requiring
// the output directory to exist. The final filename is replaced by rename, so
// an existing final symlink is not followed when choosing the publication path.
func canonicalArchiveDestination(destination string) (string, error) {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	ancestor := filepath.Dir(absolute)
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Join(resolved, filepath.Base(absolute)), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		// A present but dangling symlink must not be treated as a missing folder.
		if _, statErr := os.Lstat(ancestor); statErr == nil || !os.IsNotExist(statErr) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", err
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
}

// Compare existing ancestor identities as well as lexical paths: case aliases on
// case-insensitive filesystems need not be normalized by EvalSymlinks. Missing
// output-directory suffixes are skipped until an existing ancestor is reached.
func archiveDestinationHasSourceAncestor(destination string, source os.FileInfo) (bool, error) {
	ancestor := filepath.Dir(destination)
	for {
		info, err := os.Stat(ancestor)
		if err == nil {
			if os.SameFile(source, info) {
				return true, nil
			}
		} else if !os.IsNotExist(err) {
			return false, err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return false, nil
		}
		ancestor = parent
	}
}
