package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tryResolveRuntime resolves explicitly selected files and package/project runtime profiles.
// Builtin defaults do not require an on-disk profile.
func tryResolveRuntime(repoRoot, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var candidates []string
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(repoRoot, path)
		}
		candidates = append(candidates, path, filepath.Join(path, "profile"))
	} else {
		var roots []string
		if root, err := PackagesRoot(repoRoot); err == nil {
			roots = append(roots, root)
		}
		roots = append(roots, configuredPackageStoreRoots(repoRoot)...)
		for _, root := range roots {
			path := filepath.Join(root, "runtimes", name)
			candidates = append(candidates, path, filepath.Join(path, "profile"))
		}
		path := filepath.Join(repoRoot, "runtimes", name)
		candidates = append(candidates, path, filepath.Join(path, "profile"))
		if root, err := GlobalPackagesRoot(); err == nil {
			path := filepath.Join(root, "runtimes", name)
			candidates = append(candidates, path, filepath.Join(path, "profile"))
		}
		for _, root := range SystemPackagesRoots() {
			path := filepath.Join(root, "runtimes", name)
			candidates = append(candidates, path, filepath.Join(path, "profile"))
		}
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

// ResolveRuntimeFilePath returns the path to a runtime profile file by name.
func ResolveRuntimeFilePath(repoRoot, runtimeName string) (string, error) {
	runtimeName = NormalizeRuntimeProfileName(strings.TrimSpace(runtimeName))
	if runtimeName == "" {
		return "", fmt.Errorf("runtime profile name is empty")
	}
	p := tryResolveRuntime(repoRoot, runtimeName)
	if p == "" {
		return "", fmt.Errorf("runtime profile not found for %q", runtimeName)
	}
	return p, nil
}

// tryResolveResolver uses the shared installed-package and source profile lookup.
func tryResolveResolver(repoRoot, name string) string {
	profile, err := ResolveResolverFilePath(repoRoot, name)
	if err != nil {
		return ""
	}
	return profile
}

// LoadIsolationProfile merges builtin runtime defaults, optional runtime files,
// and resolver files, in that order. Later assignments overlay earlier values.
// A single selected name also checks the other profile category for a paired
// runtime/resolver. Distinct explicit names do not cross-read profiles.
func LoadIsolationProfile(repoRoot, runtimeName, resolverName string) (map[string]string, error) {
	runtimeName = NormalizeRuntimeProfileName(strings.TrimSpace(runtimeName))
	resolverName = strings.TrimSpace(resolverName)
	explicitPair := runtimeName != "" && resolverName != "" && runtimeName != resolverName

	m := make(map[string]string)
	merge := func(path string) error {
		if path == "" {
			return nil
		}
		rm, err := LoadResolverFile(path)
		if err != nil {
			return err
		}
		for k, v := range rm {
			m[k] = v
		}
		return nil
	}
	mergeRuntime := func(name string) error {
		if defaults, ok := builtinRuntimeDefaults(name); ok {
			for key, value := range defaults.assignments() {
				m[key] = value
			}
		}
		return merge(tryResolveRuntime(repoRoot, name))
	}
	mergeResolver := func(path string) error {
		if err := merge(path); err != nil {
			return err
		}
		if path != "" {
			absolute, err := filepath.Abs(filepath.Dir(path))
			if err != nil {
				return err
			}
			m["DOCKPIPE_RESOLVER_PACKAGE_ROOT"] = absolute
		}
		return nil
	}

	if runtimeName != "" {
		if err := mergeRuntime(runtimeName); err != nil {
			return nil, err
		}
		if !explicitPair && resolverName == "" {
			if err := mergeResolver(tryResolveResolver(repoRoot, runtimeName)); err != nil {
				return nil, err
			}
		}
	}
	if resolverName != "" {
		if !explicitPair && runtimeName == "" {
			if err := mergeRuntime(resolverName); err != nil {
				return nil, err
			}
		}
		if err := mergeResolver(tryResolveResolver(repoRoot, resolverName)); err != nil {
			return nil, err
		}
	}

	if len(m) == 0 {
		return nil, fmt.Errorf("no isolation profile found for runtime=%q resolver=%q", runtimeName, resolverName)
	}
	return m, nil
}
