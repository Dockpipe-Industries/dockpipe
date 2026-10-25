package packagebuild

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IsTarballExtractionSource identifies a derived source within the extraction of
// this exact archive. Extraction timestamps are not authored source changes.
func IsTarballExtractionSource(archive, cacheRoot, source string) (bool, error) {
	absoluteSource, err := filepath.Abs(source)
	if err != nil {
		return false, err
	}
	absoluteCache, err := filepath.Abs(cacheRoot)
	if err != nil {
		return false, err
	}
	lexicalRelative, lexicalError := filepath.Rel(absoluteCache, absoluteSource)
	lexicallyCached := lexicalError == nil && filepath.IsLocal(lexicalRelative)
	resolvedSource, err := filepath.EvalSymlinks(absoluteSource)
	if err != nil {
		return false, err
	}
	resolvedCache, err := filepath.EvalSymlinks(cacheRoot)
	if os.IsNotExist(err) && !lexicallyCached {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(resolvedCache, resolvedSource)
	if err != nil || !filepath.IsLocal(relative) {
		if lexicallyCached {
			return false, fmt.Errorf("materialized package source provenance escapes the extraction cache")
		}
		return false, nil
	}
	if strings.TrimSpace(archive) == "" {
		return false, fmt.Errorf("materialized package source provenance has no installed archive")
	}
	sum, err := sha256File(archive)
	if err != nil {
		return false, err
	}
	entry := filepath.Join(resolvedCache, sum)
	relative, err = filepath.Rel(entry, resolvedSource)
	if err != nil || !filepath.IsLocal(relative) {
		return false, fmt.Errorf("materialized package source provenance does not match the installed archive digest")
	}
	markerPath := filepath.Join(entry, extractMarkerFile)
	info, err := os.Lstat(markerPath)
	if err != nil {
		return false, fmt.Errorf("read package extraction provenance: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 66 {
		return false, fmt.Errorf("package extraction provenance marker must be a regular file of at most 66 bytes")
	}
	marker, err := os.Open(markerPath)
	if err != nil {
		return false, err
	}
	defer marker.Close()
	contents, err := io.ReadAll(io.LimitReader(marker, 67))
	if err != nil {
		return false, err
	}
	if len(contents) > 66 || strings.TrimSpace(string(contents)) != sum {
		return false, fmt.Errorf("package extraction provenance marker does not match the installed archive digest")
	}
	return true, nil
}
