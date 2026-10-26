package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// DockerfileDir is the compatibility directory lookup for a selected image recipe.
// New callers use ResolveTemplateBuild to preserve cache and resolution errors.
func DockerfileDir(repoRoot, name string) string {
	_, directory, _, _, _ := ResolveTemplateBuild(repoRoot, name)
	return directory
}

// TemplateBuild retains the SDK lookup signature. Generic recipes are discovered
// through package/project assets; builtin development recipes need no source tree.
func TemplateBuild(repoRoot, name string) (image, dockerfileDir string, ok bool) {
	image, dockerfileDir, _, ok, err := ResolveTemplateBuild(repoRoot, name)
	return image, dockerfileDir, ok && err == nil
}

// ResolveTemplateBuild resolves a package-owned Dockerfile or a generic builtin
// recipe together with its complete Docker build context.
func ResolveTemplateBuild(repoRoot, name string) (image, directory, context string, ok bool, err error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
		return "", "", "", false, nil
	}
	var packageRoots []string
	if assets, err := ResolveResolverAssetsDir(repoRoot, name); err == nil {
		packageRoots = append(packageRoots, filepath.Dir(assets))
	}
	for _, root := range WorkflowCompileRootsCached(repoRoot) {
		packageRoots = append(packageRoots, filepath.Join(root, name))
	}
	for _, root := range ResolverCompileRootsCached(repoRoot) {
		packageRoots = append(packageRoots, nestedResolverLeafDirs(name, []string{root})...)
	}
	if config, resolveErr := ResolveWorkflowConfigPathWithWorkdir(repoRoot, repoRoot, name); resolveErr == nil {
		config, err = WorkflowConfigOnDiskPath(repoRoot, config)
		if err != nil {
			return "", "", "", false, err
		}
		packageRoots = append(packageRoots, filepath.Dir(config))
	}
	for _, root := range packageRoots {
		directory := filepath.Join(root, "assets", "images", name)
		if info, err := os.Stat(filepath.Join(directory, "Dockerfile")); err == nil && info.Mode().IsRegular() {
			return "dockpipe-" + name, directory, root, true, nil
		}
	}
	return BuiltinContainerBuild(repoRoot, name)
}

// MaybeVersionTag appends :version from repoRoot/version for dockpipe-* images only.
func MaybeVersionTag(repoRoot, image string) string {
	if image == "" {
		return image
	}
	if strings.Contains(image, ":") {
		return image
	}
	b, err := os.ReadFile(filepath.Join(repoRoot, "version"))
	if err != nil {
		return image
	}
	ver := strings.TrimSpace(string(b))
	if ver == "" {
		return image
	}
	if strings.HasPrefix(image, "dockpipe-") {
		return image + ":" + ver
	}
	return image
}
