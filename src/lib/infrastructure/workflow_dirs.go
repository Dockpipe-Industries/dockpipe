package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

// ResolveWorkflowConfigPath resolves project workflows and installed packages.
func ResolveWorkflowConfigPath(repoRoot, name string) (string, error) {
	return ResolveWorkflowConfigPathWithWorkdir(repoRoot, "", name)
}

// ResolveWorkflowConfigPathWithWorkdir prefers the active project's authored
// workflows, then another explicitly selected root, then installed packages.
func ResolveWorkflowConfigPathWithWorkdir(repoRoot, workdir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("workflow name is empty")
	}
	if workdir != "" {
		if path := projectAuthoredWorkflowConfigPath(workdir, name); path != "" {
			return path, nil
		}
	}
	if path := OnDiskWorkflowConfigPath(repoRoot, name); path != "" {
		return path, nil
	}
	if uri, err := tryResolveWorkflowTarballURI(repoRoot, workdir, name); err != nil {
		return "", err
	} else if uri != "" {
		return uri, nil
	}
	for _, root := range []string{workdir, repoRoot} {
		if strings.TrimSpace(root) != "" {
			if path := resolverWorkflowConfigPath(root, name); path != "" {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("workflow config not found for %q", name)
}

// ProjectWorkflowConfigPath returns the selected workflow directory's config.
func ProjectWorkflowConfigPath(projectRoot, name string) string {
	if strings.TrimSpace(projectRoot) == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	path := filepath.Join(WorkflowsRootDir(projectRoot), name, "config.yml")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path
	}
	return ""
}

func projectAuthoredWorkflowConfigPath(workdir, name string) string {
	if strings.TrimSpace(workdir) == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	root, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return ""
	}
	return OnDiskWorkflowConfigPath(root, name)
}

// OnDiskWorkflowConfigPath searches the project's selected and declared roots.
// It does not consult package archives or infer legacy source directories.
func OnDiskWorkflowConfigPath(projectRoot, name string) string {
	if strings.TrimSpace(projectRoot) == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	if path := ProjectWorkflowConfigPath(projectRoot, name); path != "" {
		return path
	}
	for _, path := range nestedWorkflowConfigCandidates(projectRoot, name, WorkflowCompileRootsCached(projectRoot)) {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

func resolverWorkflowConfigPath(projectRoot, name string) string {
	profile, err := ResolveResolverFilePath(projectRoot, name)
	if err != nil {
		return ""
	}
	path := filepath.Join(filepath.Dir(profile), "config.yml")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path
	}
	return ""
}

// ResolveEmbeddedResolverWorkflowConfigPath is retained for API compatibility;
// delegate workflows come from installed resolver packages and project roots.
func ResolveEmbeddedResolverWorkflowConfigPath(repoRoot, name string) (string, error) {
	return ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir(repoRoot, "", name)
}

// ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir prefers the selected
// resolver package's delegate, then normal project/package workflow resolution.
func ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir(repoRoot, workdir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("resolver workflow name is empty")
	}
	for _, root := range []string{workdir, repoRoot} {
		if strings.TrimSpace(root) != "" {
			if path := resolverWorkflowConfigPath(root, name); path != "" {
				return path, nil
			}
		}
	}
	return ResolveWorkflowConfigPathWithWorkdir(repoRoot, workdir, name)
}

// ListWorkflowNamesInRepoRoot lists workflows in the selected and declared roots.
func ListWorkflowNamesInRepoRoot(repoRoot string) ([]string, error) {
	seen := make(map[string]bool)
	roots := append([]string{WorkflowsRootDir(repoRoot)}, WorkflowCompileRootsCached(repoRoot)...)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !entry.IsDir() && entry.Name() == "config.yml" {
				seen[filepath.Base(filepath.Dir(path))] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ListWorkflowNamesInPackagesStore returns workflow names from dockpipe-workflow-*.tar.<codec> under packages/workflows (streamable blobs only).
func ListWorkflowNamesInPackagesStore(workdir string) ([]string, error) {
	root, err := PackagesWorkflowsDir(workdir)
	if err != nil {
		return nil, err
	}
	return listWorkflowNamesFromTarballDir(root)
}

func listWorkflowNamesFromTarballDir(root string) ([]string, error) {
	matches, err := packagebuild.ListPackageArchivePaths(root, "workflow")
	if err != nil {
		return nil, err
	}
	var out []string
	seen := make(map[string]bool)
	for _, tgz := range matches {
		members, err := packagebuild.ListTarGzMemberPaths(tgz)
		if err != nil {
			continue
		}
		name, err := packagebuild.WorkflowNameFromTarballMembers(members)
		if err != nil {
			continue
		}
		if !packagebuild.MatchesVersionedTarball(tgz, "workflow", name) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// ListWorkflowNamesInGlobalPackagesStore lists workflows from dockpipe-workflow-*.tar.<codec> under the global packages/workflows dir.
func ListWorkflowNamesInGlobalPackagesStore() ([]string, error) {
	root, err := GlobalPackagesWorkflowsDir()
	if err != nil {
		return nil, err
	}
	return listWorkflowNamesFromTarballDir(root)
}

// ListWorkflowNamesInSystemPackagesStores lists workflows from dockpipe-workflow-*.tar.<codec> under system-shared packages/workflows dirs.
func ListWorkflowNamesInSystemPackagesStores() ([]string, error) {
	var out []string
	for _, root := range SystemPackagesWorkflowsDirs() {
		names, err := listWorkflowNamesFromTarballDir(root)
		if err != nil {
			return nil, err
		}
		out = append(out, names...)
	}
	sort.Strings(out)
	return out, nil
}

// ListWorkflowNamesInRepoRootAndPackages merges ListWorkflowNamesInRepoRoot with ListWorkflowNamesInPackagesStore (deduped).
func ListWorkflowNamesInRepoRootAndPackages(repoRoot, workdir string) ([]string, error) {
	a, err := ListWorkflowNamesInRepoRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	var b []string
	if strings.TrimSpace(workdir) != "" {
		b, err = ListWorkflowNamesInPackagesStore(workdir)
		if err != nil {
			return nil, err
		}
	}
	configRoot := packageSourcesConfigRoot(repoRoot, workdir)
	var c []string
	for _, root := range configuredPackageStoreRoots(configRoot) {
		names, err := listWorkflowNamesFromTarballDir(filepath.Join(root, "workflows"))
		if err != nil {
			return nil, err
		}
		c = append(c, names...)
	}
	for _, dir := range configuredPackageTarballDirs(configRoot) {
		names, err := listWorkflowNamesFromTarballDir(dir)
		if err != nil {
			return nil, err
		}
		c = append(c, names...)
	}
	g, err := ListWorkflowNamesInGlobalPackagesStore()
	if err != nil {
		return nil, err
	}
	s, err := ListWorkflowNamesInSystemPackagesStores()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, x := range append(append(append(append(append([]string{}, a...), b...), c...), g...), s...) {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	sort.Strings(out)
	return out, nil
}

// WorkflowsDirHasDockpipeWorkflow reports whether dir contains at least one immediate child directory with config.yml.
func WorkflowsDirHasDockpipeWorkflow(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfg := filepath.Join(dir, e.Name(), "config.yml")
		if st, err := os.Stat(cfg); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// WorkflowsDirHasFiles reports whether dir contains any regular file anywhere under it.
// Empty directories or placeholder directory trees return false.
func WorkflowsDirHasFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d == nil || d.IsDir() {
			return nil
		}
		found = true
		return filepath.SkipAll
	})
	return found
}
