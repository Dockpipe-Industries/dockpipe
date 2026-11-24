package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path"
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
	if !validTarWorkflowMember(entry) {
		return nil, fmt.Errorf("invalid contained workflow archive member")
	}
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
	return resolveWorkflowTarballMatching(repoRoot, workdir, name, nil)
}

// resolveWorkflowTarballMatching keeps archive precedence and project namespace
// admission identical for ordinary, package-selected and nested workflow calls.
func resolveWorkflowTarballMatching(repoRoot, workdir, name string, accept func(string, []byte) bool) (string, error) {
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
			if accept != nil && !accept(tarPath, b) {
				continue
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
	manifest, err := workflowTarballIdentityManifest(tarPath, workflowName)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(manifest.Namespace), nil
}

func workflowTarballIdentityManifest(tarPath, workflowName string) (*domain.PackageManifest, error) {
	manifestPath := filepath.ToSlash(filepath.Join("workflows", workflowName, PackageManifestFilename))
	data, err := packagebuild.ReadFileFromTarGzLimit(tarPath, manifestPath, maxCompatibilityMetadataBytes)
	if err != nil {
		return nil, err
	}
	var manifest domain.PackageManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	domain.NormalizePackageManifestYAMLAliases(&manifest)
	if err := domain.ValidatePackageManifest(&manifest); err != nil {
		return nil, err
	}
	archiveName, archiveVersion, valid := packagebuild.PackageArchiveIdentity(tarPath, "workflow")
	if !valid || manifest.Schema != 1 || manifest.Kind != "workflow" || manifest.Name != workflowName ||
		archiveName != packagebuild.SafeTarballToken(workflowName) || manifest.Version != archiveVersion {
		return nil, fmt.Errorf("compiled workflow manifest identity differs from its archive")
	}
	return &manifest, nil
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

// FormatTarWorkflowURI names an exact workflow entry in a selected package archive.
func FormatTarWorkflowURI(absTarPath, entryInTar string) string {
	return formatTarWorkflowURI(absTarPath, entryInTar)
}

// SplitTarWorkflowURI splits a catalog locator and admits only canonical,
// contained POSIX archive members, independently of the current host OS.
func SplitTarWorkflowURI(locator string) (tarPath, entry string, ok bool) {
	if !strings.HasPrefix(locator, "tar://") {
		return "", "", false
	}
	rest := strings.TrimPrefix(locator, "tar://")
	i := strings.Index(rest, tarWorkflowSep)
	if i < 0 {
		return "", "", false
	}
	tarPath, entry = rest[:i], rest[i+len(tarWorkflowSep):]
	if tarPath == "" || !validTarWorkflowMember(entry) {
		return "", "", false
	}
	return tarPath, entry, true
}

func validTarWorkflowMember(member string) bool {
	if member == "" || member == "." || member == ".." || path.IsAbs(member) || path.Clean(member) != member ||
		strings.HasPrefix(member, "../") || strings.ContainsAny(member, "\\\x00") {
		return false
	}
	// Archive members use POSIX separators on every host. Reject Windows drive
	// prefixes even when the caller is running on a Unix host.
	if len(member) >= 2 && member[1] == ':' {
		return false
	}
	return true
}

// WorkflowCompileStartDir returns an absolute directory containing config.yml for workflowName for
// package compile closure (filesystem walk). On-disk configs under projectRoot (including nested
// compile.workflows) win before bundle cache + tarball resolution — RepoRoot() is often the
// materialized embed bundle, not the user's git tree.
// When resolution is tar://, the workflow tarball is extracted under TarballExtractCacheRoot(projectRoot)
// and the path to workflows/<name>/ inside that tree is returned.
func WorkflowCompileStartDir(repoRoot, projectRoot, workflowName string) (string, error) {
	source, err := ResolveWorkflowCompileSource(repoRoot, projectRoot, workflowName)
	if err != nil {
		return "", err
	}
	return source.Directory, nil
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
	relative, err := filepath.Rel(root, cfg)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("workflow archive config escapes its extracted root")
	}
	if st, err := os.Stat(cfg); err != nil || st.IsDir() {
		return "", fmt.Errorf("extracted workflow config missing %s", cfg)
	}
	return filepath.Clean(cfg), nil
}
