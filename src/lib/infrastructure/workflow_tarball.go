package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/archivecodec"
	"dockpipe/src/lib/infrastructure/engineversion"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

const tarWorkflowSep = "##"

func loadWorkflowFromTarball(tarPath, entry string) (*domain.Workflow, error) {
	entry = filepath.ToSlash(entry)
	manifestPath := filepath.ToSlash(filepath.Join(filepath.Dir(entry), PackageManifestFilename))
	metadata, err := packagebuild.ReadFileFromTarGzLimit(tarPath, manifestPath, maxCompatibilityMetadataBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := checkPackageEngineManifest(metadata, engineversion.Current()); err != nil {
			return nil, err
		}
	}
	data, err := packagebuild.ReadFileFromTarGz(tarPath, entry)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.ToSlash(filepath.Dir(entry))
	readFile := func(p string) ([]byte, error) {
		return packagebuild.ReadFileFromTarGz(tarPath, filepath.ToSlash(filepath.Clean(p)))
	}
	return domain.ParseWorkflowFromDisk(data, baseDir, readFile)
}

// tryResolveWorkflowTarballURI returns a tar workflow URI when no on-disk config exists but a
// dockpipe-workflow-<name>-*.tar.<codec> exists under the packages store workflows dir (first), then
// packages.tarball_dir or release/artifacts. The archive must contain workflows/<name>/config.yml.
// When dockpipe.config.json sets packages.namespace, the explicit config namespace or
// the same archive's validated compiled manifest namespace must match.
func tryResolveWorkflowTarballURI(repoRoot, workdir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	configRoot := packageSourcesConfigRoot(repoRoot, workdir)
	cfg, err := domain.LoadDockpipeProjectConfig(configRoot)
	if err != nil {
		return "", err
	}
	var searchDirs []string
	if strings.TrimSpace(workdir) != "" {
		if pw, err := PackagesWorkflowsDir(workdir); err == nil {
			searchDirs = append(searchDirs, pw)
		}
	}
	for _, root := range configuredPackageStoreRoots(configRoot) {
		searchDirs = append(searchDirs, filepath.Join(root, "workflows"))
	}
	if gw, err := GlobalPackagesWorkflowsDir(); err == nil {
		searchDirs = append(searchDirs, gw)
	}
	searchDirs = append(searchDirs, SystemPackagesWorkflowsDirs()...)
	searchDirs = append(searchDirs, configuredPackageTarballDirs(configRoot)...)
	if d := workflowTarballSearchDir(configRoot, cfg); d != "" {
		searchDirs = append(searchDirs, d)
	}
	entry := filepath.ToSlash(filepath.Join("workflows", name, "config.yml"))
	for _, dir := range searchDirs {
		pattern := filepath.Join(dir, fmt.Sprintf("dockpipe-workflow-%s-*.tar.*", packagebuild.SafeTarballToken(name)))
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		prefix := "dockpipe-workflow-" + packagebuild.SafeTarballToken(name) + "-"
		sort.Slice(matches, func(i, j int) bool {
			leftStem, _ := archivecodec.StripExtension(filepath.Base(matches[i]))
			rightStem, _ := archivecodec.StripExtension(filepath.Base(matches[j]))
			leftVersion := strings.TrimPrefix(leftStem, prefix)
			rightVersion := strings.TrimPrefix(rightStem, prefix)
			if leftVersion != rightVersion {
				return semver.Compare("v"+strings.TrimPrefix(leftVersion, "v"), "v"+strings.TrimPrefix(rightVersion, "v")) > 0
			}
			return packagebuild.ArchiveCodecPriority(matches[i]) < packagebuild.ArchiveCodecPriority(matches[j])
		})
		for _, tarPath := range matches {
			if !packagebuild.MatchesVersionedTarball(tarPath, "workflow", name) {
				continue
			}
			b, err := packagebuild.ReadFileFromTarGz(tarPath, entry)
			if err != nil {
				continue
			}
			var top struct {
				Namespace string `yaml:"namespace"`
			}
			if err := yaml.Unmarshal(b, &top); err != nil {
				continue
			}
			if cfg != nil && cfg.Packages.Namespace != nil {
				want := strings.TrimSpace(*cfg.Packages.Namespace)
				if want != "" {
					namespace := strings.TrimSpace(top.Namespace)
					if namespace == "" {
						namespace, err = workflowTarballManifestNamespace(tarPath, name)
						if err != nil {
							continue
						}
					}
					if namespace != want {
						continue
					}
				}
			}
			absTar, err := filepath.Abs(tarPath)
			if err != nil {
				return "", err
			}
			return formatTarWorkflowURI(absTar, entry), nil
		}
	}
	return "", nil
}

// workflowTarballManifestNamespace reads only the identity-bound manifest beside
// this workflow's config. The consumer's filter must never supply package identity.
func workflowTarballManifestNamespace(tarPath, workflowName string) (string, error) {
	manifestPath := filepath.ToSlash(filepath.Join("workflows", workflowName, PackageManifestFilename))
	data, err := packagebuild.ReadFileFromTarGzLimit(tarPath, manifestPath, maxCompatibilityMetadataBytes)
	if err != nil {
		return "", err
	}
	var manifest domain.PackageManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	domain.NormalizePackageManifestYAMLAliases(&manifest)
	if err := domain.ValidatePackageManifest(&manifest); err != nil {
		return "", err
	}
	archiveName, archiveVersion, valid := packagebuild.PackageArchiveIdentity(tarPath, "workflow")
	if !valid || manifest.Schema != 1 || manifest.Kind != "workflow" || manifest.Name != workflowName ||
		archiveName != packagebuild.SafeTarballToken(workflowName) || manifest.Version != archiveVersion {
		return "", fmt.Errorf("compiled workflow manifest identity differs from its archive")
	}
	return strings.TrimSpace(manifest.Namespace), nil
}

func workflowTarballSearchDir(repoRoot string, cfg *domain.DockpipeProjectConfig) string {
	if cfg != nil && cfg.Packages.TarballDir != nil {
		rel := strings.TrimSpace(*cfg.Packages.TarballDir)
		if rel != "" {
			p := filepath.Join(repoRoot, filepath.Clean(rel))
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
		}
	}
	def := filepath.Join(repoRoot, "release", "artifacts")
	if st, err := os.Stat(def); err == nil && st.IsDir() {
		return def
	}
	return ""
}

func formatTarWorkflowURI(absTarPath, entryInTar string) string {
	entryInTar = filepath.ToSlash(strings.TrimPrefix(entryInTar, "/"))
	return "tar://" + absTarPath + tarWorkflowSep + entryInTar
}

// SplitTarWorkflowURI splits a LoadWorkflow path produced by tryResolveWorkflowTarballURI.
func SplitTarWorkflowURI(path string) (tarPath, entry string, ok bool) {
	if !strings.HasPrefix(path, "tar://") {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, "tar://")
	i := strings.Index(rest, tarWorkflowSep)
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+len(tarWorkflowSep):], true
}

// WorkflowCompileStartDir returns an absolute directory containing config.yml for workflowName for
// package compile closure (filesystem walk). On-disk configs under projectRoot (including nested
// compile.workflows) win before bundle cache + tarball resolution — RepoRoot() is often the
// materialized embed bundle, not the user's git tree.
// When resolution is tar://, the workflow tarball is extracted under TarballExtractCacheRoot(projectRoot)
// and the path to workflows/<name>/ inside that tree is returned.
func WorkflowCompileStartDir(repoRoot, projectRoot, workflowName string) (string, error) {
	workflowName = strings.TrimSpace(workflowName)
	if workflowName == "" {
		return "", fmt.Errorf("workflow name is empty")
	}
	if p := OnDiskWorkflowConfigPath(projectRoot, workflowName); p != "" {
		return filepath.Dir(p), nil
	}
	wfPath, err := ResolveWorkflowConfigPathWithWorkdir(repoRoot, projectRoot, workflowName)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(wfPath, "tar://") {
		tarPath, entry, ok := SplitTarWorkflowURI(wfPath)
		if !ok {
			return "", fmt.Errorf("invalid tar workflow path %q", wfPath)
		}
		root, err := packagebuild.EnsureTarballExtractedCache(tarPath, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			return "", fmt.Errorf("extract workflow tarball for compile: %w", err)
		}
		entry = filepath.ToSlash(entry)
		relDir := filepath.ToSlash(filepath.Dir(entry))
		start := filepath.Join(root, filepath.FromSlash(relDir))
		cfg := filepath.Join(start, "config.yml")
		if st, err := os.Stat(cfg); err != nil || st.IsDir() {
			return "", fmt.Errorf("extracted workflow tree missing %s", cfg)
		}
		return filepath.Clean(start), nil
	}
	return filepath.Dir(wfPath), nil
}

// WorkflowConfigOnDiskPath returns a filesystem path for a workflow config resolved by the CLI.
// Plain paths are returned unchanged. Tarball-backed workflows are extracted into the standard
// tarball cache under projectRoot and the extracted config.yml path is returned.
func WorkflowConfigOnDiskPath(projectRoot, wfConfig string) (string, error) {
	wfConfig = strings.TrimSpace(wfConfig)
	if wfConfig == "" {
		return "", fmt.Errorf("workflow config path is empty")
	}
	if !strings.HasPrefix(wfConfig, "tar://") {
		return wfConfig, nil
	}
	tarPath, entry, ok := SplitTarWorkflowURI(wfConfig)
	if !ok {
		return "", fmt.Errorf("invalid tar workflow path %q", wfConfig)
	}
	root, err := packagebuild.EnsureTarballExtractedCache(tarPath, TarballExtractCacheRoot(projectRoot))
	if err != nil {
		return "", fmt.Errorf("extract workflow tarball: %w", err)
	}
	cfg := filepath.Join(root, filepath.FromSlash(filepath.ToSlash(entry)))
	if st, err := os.Stat(cfg); err != nil || st.IsDir() {
		return "", fmt.Errorf("extracted workflow config missing %s", cfg)
	}
	return filepath.Clean(cfg), nil
}
