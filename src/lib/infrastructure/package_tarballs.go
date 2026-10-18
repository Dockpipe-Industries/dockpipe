package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/archivecodec"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"golang.org/x/mod/semver"
)

// TarballExtractCacheRoot is where full tarballs are extracted for tools that need real paths (scripts, resolver profiles).
func TarballExtractCacheRoot(repoRoot string) string {
	return filepath.Join(repoRoot, DockpipeDirRel, "internal", "cache", "tarball")
}

type tarballExtractCacheMarker struct {
	Tarball string `json:"tarball"`
}

// InvalidateTarballExtractCacheForPackage removes extracted tarball caches for a rebuilt package.
// Newer cache entries carry a tarball marker. Older cache entries are matched by the archive root
// they contain, so a rebuild also clears stale pre-marker extractions.
func InvalidateTarballExtractCacheForPackage(workdir, kind, name string) (int, error) {
	return InvalidateTarballExtractCacheExceptSource(workdir, kind, name, "")
}

// InvalidateTarballExtractCacheExceptSource retains the extraction containing an
// active compiler source. Callers may still read that source after compilation.
// Extractions are content-addressed, so a changed archive gets a different cache.
func InvalidateTarballExtractCacheExceptSource(workdir, kind, name, source string) (int, error) {
	if source != "" {
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return 0, err
		}
		source, err = filepath.Abs(resolved)
		if err != nil {
			return 0, err
		}
	}
	var archiveRoot string
	switch kind {
	case "workflow":
		archiveRoot = filepath.Join("workflows", name)
	case "resolver":
		archiveRoot = filepath.Join("resolvers", name)
	default:
		return 0, fmt.Errorf("unknown package cache kind %q", kind)
	}
	cacheRoot := TarballExtractCacheRoot(workdir)
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(cacheRoot, entry.Name())
		if source != "" {
			resolved, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return removed, err
			}
			absolute, err := filepath.Abs(resolved)
			if err != nil {
				return removed, err
			}
			relative, err := filepath.Rel(absolute, source)
			if err == nil && filepath.IsLocal(relative) {
				continue
			}
		}
		if tarballCacheEntryMatchesPackage(dir, kind, name, archiveRoot) {
			if err := os.RemoveAll(dir); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

func tarballCacheEntryMatchesPackage(dir, kind, name, archiveRoot string) bool {
	markerPath := filepath.Join(dir, ".dockpipe-extracted-package.json")
	if b, err := os.ReadFile(markerPath); err == nil {
		var marker tarballExtractCacheMarker
		if json.Unmarshal(b, &marker) == nil && packagebuild.MatchesVersionedTarball(marker.Tarball, kind, name) {
			return true
		}
	}
	if st, err := os.Stat(filepath.Join(dir, archiveRoot)); err == nil && st.IsDir() {
		return true
	}
	return false
}

// FindLatestCoreTarball returns the newest matching dockpipe-core-*.tar.<codec> under packages/core (then global packages/core), or "" if none.
func FindLatestCoreTarball(workdir string) (string, error) {
	projectRoot, _ := domain.FindProjectRootWithDockpipeConfig(workdir)
	d, err := PackagesCoreDir(workdir)
	if err != nil {
		return "", err
	}
	if p, err := findLatestGlob(filepath.Join(d, "dockpipe-core-*.tar.*")); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	for _, root := range configuredPackageStoreRoots(projectRoot) {
		if p, err := findLatestGlob(filepath.Join(root, "core", "dockpipe-core-*.tar.*")); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	for _, dir := range configuredPackageTarballDirs(projectRoot) {
		if p, err := findLatestGlob(filepath.Join(dir, "dockpipe-core-*.tar.*")); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	gp, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	if p, err := findLatestGlob(filepath.Join(gp, "core", "dockpipe-core-*.tar.*")); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	for _, root := range SystemPackagesCoreDirs() {
		if p, err := findLatestGlob(filepath.Join(root, "dockpipe-core-*.tar.*")); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	return "", nil
}

// FindLatestWorkflowTarball returns the newest dockpipe-workflow-<name>-*.tar.<codec> under packages/workflows (then global).
func FindLatestWorkflowTarball(workdir, name string) (string, error) {
	projectRoot, _ := domain.FindProjectRootWithDockpipeConfig(workdir)
	d, err := PackagesWorkflowsDir(workdir)
	if err != nil {
		return "", err
	}
	tok := packagebuild.SafeTarballToken(name)
	pattern := filepath.Join(d, fmt.Sprintf("dockpipe-workflow-%s-*.tar.*", tok))
	if p, err := findLatestGlob(pattern); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	for _, root := range configuredPackageStoreRoots(projectRoot) {
		if p, err := findLatestGlob(filepath.Join(root, "workflows", fmt.Sprintf("dockpipe-workflow-%s-*.tar.*", tok))); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	for _, dir := range configuredPackageTarballDirs(projectRoot) {
		if p, err := findLatestGlob(filepath.Join(dir, fmt.Sprintf("dockpipe-workflow-%s-*.tar.*", tok))); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	gp, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	if p, err := findLatestGlob(filepath.Join(gp, "workflows", fmt.Sprintf("dockpipe-workflow-%s-*.tar.*", tok))); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	for _, root := range SystemPackagesWorkflowsDirs() {
		if p, err := findLatestGlob(filepath.Join(root, fmt.Sprintf("dockpipe-workflow-%s-*.tar.*", tok))); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	return "", nil
}

// FindLatestResolverTarball returns the newest dockpipe-resolver-<name>-*.tar.<codec> under packages/resolvers (then global).
func FindLatestResolverTarball(workdir, name string) (string, error) {
	projectRoot, _ := domain.FindProjectRootWithDockpipeConfig(workdir)
	d, err := PackagesResolversDir(workdir)
	if err != nil {
		return "", err
	}
	tok := packagebuild.SafeTarballToken(name)
	pattern := filepath.Join(d, fmt.Sprintf("dockpipe-resolver-%s-*.tar.*", tok))
	if p, err := findLatestGlob(pattern); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	for _, root := range configuredPackageStoreRoots(projectRoot) {
		if p, err := findLatestGlob(filepath.Join(root, "resolvers", fmt.Sprintf("dockpipe-resolver-%s-*.tar.*", tok))); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	for _, dir := range configuredPackageTarballDirs(projectRoot) {
		if p, err := findLatestGlob(filepath.Join(dir, fmt.Sprintf("dockpipe-resolver-%s-*.tar.*", tok))); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	gp, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	if p, err := findLatestGlob(filepath.Join(gp, "resolvers", fmt.Sprintf("dockpipe-resolver-%s-*.tar.*", tok))); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	for _, root := range SystemPackagesResolversDirs() {
		if p, err := findLatestGlob(filepath.Join(root, fmt.Sprintf("dockpipe-resolver-%s-*.tar.*", tok))); err != nil {
			return "", err
		} else if p != "" {
			return p, nil
		}
	}
	return "", nil
}

func findLatestGlob(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	prefix, suffix, found := strings.Cut(filepath.Base(pattern), "*")
	if !found || suffix != ".tar.*" {
		return "", fmt.Errorf("invalid package archive pattern %q", pattern)
	}
	var candidates []string
	for _, match := range matches {
		stem, ok := archivecodec.StripExtension(filepath.Base(match))
		if !ok || !strings.HasPrefix(stem, prefix) {
			continue
		}
		version := strings.TrimPrefix(stem, prefix)
		normalized := "v" + strings.TrimPrefix(version, "v")
		core := strings.SplitN(strings.SplitN(normalized, "+", 2)[0], "-", 2)[0]
		if !semver.IsValid(normalized) || strings.Count(core, ".") != 2 {
			continue
		}
		info, err := os.Lstat(match)
		if err != nil {
			return "", err
		}
		if info.Mode().IsRegular() {
			candidates = append(candidates, match)
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftStem, _ := archivecodec.StripExtension(filepath.Base(candidates[i]))
		rightStem, _ := archivecodec.StripExtension(filepath.Base(candidates[j]))
		leftVersion := strings.TrimPrefix(leftStem, prefix)
		rightVersion := strings.TrimPrefix(rightStem, prefix)
		if leftVersion != rightVersion {
			return semver.Compare("v"+strings.TrimPrefix(leftVersion, "v"), "v"+strings.TrimPrefix(rightVersion, "v")) > 0
		}
		return packagebuild.ArchiveCodecPriority(candidates[i]) < packagebuild.ArchiveCodecPriority(candidates[j])
	})
	return candidates[0], nil
}

// RemoveGlob removes files matching pattern (no directories).
func RemoveGlob(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, p := range matches {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			_ = os.Remove(p)
			_ = os.Remove(p + ".sha256")
		}
	}
	return nil
}

// RemoveLegacyPackageSubdirs deletes expanded package directories under base (migration from dir layout to tarballs only).
func RemoveLegacyPackageSubdirs(base string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(base, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
