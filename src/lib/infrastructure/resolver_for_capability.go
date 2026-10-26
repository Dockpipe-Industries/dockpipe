package infrastructure

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResolverProfileNameForCapability returns the resolver package declaring a capability.
// Only installed stores and declared resolver roots participate. Multiple providers are ambiguous.
func ResolverProfileNameForCapability(workdir, repoRoot, capability string) (string, error) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return "", nil
	}
	var matches []string
	for _, directory := range resolverPackageDirectories(workdir, repoRoot) {
		declared, err := readResolverCapabilityFromPackageYML(filepath.Join(directory, PackageManifestFilename))
		if err == nil && declared == capability {
			matches = append(matches, filepath.Base(directory))
		}
	}

	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("capability %q is declared by multiple resolver packages: %s — disambiguate in package.yml or use resolver: explicitly", capability, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

// SubstrateRuntimeFromDockpipeCapability maps isolation capabilities to library runtime defaults.
// Host-style ids map to dockerimage.
func SubstrateRuntimeFromDockpipeCapability(capability string) string {
	capability = strings.TrimSpace(strings.ToLower(capability))
	if capability == "" || !strings.HasPrefix(capability, "dockpipe.") {
		return ""
	}
	rest := strings.TrimPrefix(capability, "dockpipe.")
	switch {
	case rest == "cli" || strings.HasPrefix(rest, "cli."):
		return "dockerimage"
	case rest == "powershell" || strings.HasPrefix(rest, "powershell."):
		return "dockerimage"
	case rest == "cmd" || strings.HasPrefix(rest, "cmd."):
		return "dockerimage"
	case rest == "docker" || strings.HasPrefix(rest, "docker."):
		return "dockerimage"
	case rest == "dockerfile" || strings.HasPrefix(rest, "dockerfile."):
		return "dockerfile"
	case rest == "package" || strings.HasPrefix(rest, "package."):
		return "package"
	case rest == "kubepod" || rest == "kube-pod" || strings.HasPrefix(rest, "kubepod.") || strings.HasPrefix(rest, "kube-pod."):
		return "dockerimage"
	case rest == "keystore" || strings.HasPrefix(rest, "keystore."):
		return "dockerimage"
	case rest == "cloud" || strings.HasPrefix(rest, "cloud."):
		return "dockerimage"
	default:
		return ""
	}
}

// resolverPackageDirectories enumerates installed resolver stores and package
// directories inside declared authoring roots, retaining discovery order.
func resolverPackageDirectories(workdir, repoRoot string) []string {
	projectRoot := strings.TrimSpace(workdir)
	if projectRoot == "" {
		projectRoot = repoRoot
	}
	var directories []string
	seen := make(map[string]bool)
	add := func(directory string) {
		directory = filepath.Clean(directory)
		if !seen[directory] {
			seen[directory] = true
			directories = append(directories, directory)
		}
	}
	addStore := func(root string) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				add(filepath.Join(root, entry.Name()))
			}
		}
	}
	if root, err := PackagesResolversDir(projectRoot); err == nil {
		addStore(root)
	}
	for _, root := range configuredPackageStoreRoots(projectRoot) {
		addStore(filepath.Join(root, "resolvers"))
	}
	if root, err := GlobalPackagesResolversDir(); err == nil {
		addStore(root)
	}
	for _, root := range SystemPackagesResolversDirs() {
		addStore(root)
	}
	for _, root := range ResolverCompileRootsCached(projectRoot) {
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && entry.Name() == PackageManifestFilename {
				add(filepath.Dir(path))
			}
			return nil
		})
	}
	return directories
}
