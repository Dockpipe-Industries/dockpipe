package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure/engineversion"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

// ResolveResolverAssetsDir resolves package-owned assets without requiring an execution profile.
// Installed stores take precedence over authoring trees, and archives are checked before extraction.
func ResolveResolverAssetsDir(repoRoot, resolverName string) (string, error) {
	resolverName = strings.TrimSpace(resolverName)
	if resolverName == "" || resolverName == "." || resolverName == ".." {
		return "", fmt.Errorf("invalid resolver name %q", resolverName)
	}
	for _, character := range resolverName {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-') {
			return "", fmt.Errorf("invalid resolver name %q", resolverName)
		}
	}

	archive, err := FindLatestResolverTarball(repoRoot, resolverName)
	if err != nil {
		return "", err
	}
	var stores []string
	if root, err := PackagesResolversDir(repoRoot); err == nil {
		stores = append(stores, root)
	}
	for _, root := range configuredPackageStoreRoots(repoRoot) {
		stores = append(stores, filepath.Join(root, "resolvers"))
	}
	stores = append(stores, configuredPackageTarballDirs(repoRoot)...)
	if root, err := GlobalPackagesResolversDir(); err == nil {
		stores = append(stores, root)
	}
	stores = append(stores, SystemPackagesResolversDirs()...)
	for _, store := range stores {
		directory := filepath.Join(store, resolverName)
		if resolverAssetsPackageExists(directory) {
			return checkedResolverAssetsDir(directory, resolverName)
		}
		if archive != "" && filepath.Clean(filepath.Dir(archive)) == filepath.Clean(store) {
			return resolverArchiveAssetsDir(repoRoot, resolverName, archive)
		}
	}

	var sources []string
	if UsesBundledAssetLayout(repoRoot) {
		sources = append(sources, filepath.Join(WorkflowsRootDir(repoRoot), resolverName))
	} else {
		roots := ResolverCompileRootsCached(repoRoot)
		sources = append(sources, nestedResolverLeafDirs(resolverName, roots)...)
		for _, config := range nestedWorkflowConfigCandidates(repoRoot, resolverName, roots) {
			sources = append(sources, filepath.Dir(config))
		}
	}
	sources = append(sources, filepath.Join(CoreDir(repoRoot), "resolvers", resolverName))
	for _, directory := range sources {
		if resolverAssetsPackageExists(directory) {
			return checkedResolverAssetsDir(directory, resolverName)
		}
	}
	return "", fmt.Errorf("resolver %q assets directory is unavailable", resolverName)
}

func resolverArchiveAssetsDir(repoRoot, resolverName, archive string) (string, error) {
	entry := filepath.ToSlash(filepath.Join("resolvers", resolverName, PackageManifestFilename))
	metadata, err := packagebuild.ReadFileFromTarGzLimit(archive, entry, maxCompatibilityMetadataBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err == nil {
		if err := checkPackageEngineManifest(metadata, engineversion.Current()); err != nil {
			return "", err
		}
	}
	root, err := packagebuild.EnsureTarballExtractedCache(archive, TarballExtractCacheRoot(repoRoot))
	if err != nil {
		return "", err
	}
	return checkedResolverAssetsDir(filepath.Join(root, "resolvers", resolverName), resolverName)
}

func resolverAssetsPackageExists(directory string) bool {
	for _, marker := range []string{"profile", "config.yml"} {
		if info, err := os.Stat(filepath.Join(directory, marker)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func checkedResolverAssetsDir(directory, resolverName string) (string, error) {
	if err := checkPackageEngineFile(filepath.Join(directory, "config.yml")); err != nil {
		return "", err
	}
	assets := filepath.Join(directory, "assets")
	info, err := os.Stat(assets)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("resolver %q assets directory is unavailable", resolverName)
	}
	return filepath.Abs(assets)
}
