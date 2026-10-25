package infrastructure

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/archivecodec"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

var errStopScriptWalk = errors.New("stop script walk")

// ResolveWorkflowScript resolves logical scripts through project overrides, installed
// packages and declared workflow roots. Other paths remain relative to workflowRoot.
// An empty projectRoot uses repoRoot. The legacy core.* alias searches package-owned
// workflow assets only; it does not imply a bundled core tree.
func ResolveWorkflowScript(rel, workflowRoot, repoRoot, projectRoot string) string {
	if filepath.IsAbs(rel) {
		return filepath.ToSlash(rel)
	}
	if strings.HasPrefix(rel, "scripts/") {
		if p, ok := projectScriptOverridePath(repoRoot, projectRoot, rel); ok {
			return filepath.ToSlash(p)
		}
		if p, ok := workflowSiblingScriptPath(workflowRoot, rel); ok {
			return filepath.ToSlash(p)
		}
		return filepath.ToSlash(resolveScriptsPrefixedPath(repoRoot, projectRoot, rel))
	}
	return filepath.ToSlash(filepath.Join(workflowRoot, rel))
}

func projectScriptOverridePath(repoRoot, projectRoot, rel string) (string, bool) {
	if projectRoot == "" {
		projectRoot = repoRoot
	}
	rest := strings.TrimPrefix(rel, "scripts/")
	for _, candidate := range []string{
		filepath.Join(projectRoot, "scripts", rest),
	} {
		if scriptFileExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func workflowSiblingScriptPath(workflowRoot, rel string) (string, bool) {
	workflowRoot = strings.TrimSpace(workflowRoot)
	if workflowRoot == "" {
		return "", false
	}
	rest := strings.TrimPrefix(rel, "scripts/")
	if strings.HasPrefix(rest, "core.") || !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || first == "" || after == "" {
		return "", false
	}
	// Package workflow tarballs commonly contain sibling workflow trees:
	// workflows/orchestrate.stack/config.yml can reference scripts/acme/foo.sh,
	// which lives at workflows/acme/assets/scripts/foo.sh in the same package.
	parent := filepath.Dir(workflowRoot)
	candidate := filepath.Join(parent, first, "assets", "scripts", after)
	if scriptFileExists(candidate) {
		return candidate, true
	}
	return "", false
}

// ResolveCoreNamespacedScriptPath resolves scripts/core.<dot.segments> the same way as workflow YAML
// (see ResolveWorkflowScript). Pass dotted without the scripts/ prefix, e.g.
// "assets.scripts.terraform-pipeline.sh" or "core.assets.scripts.terraform-pipeline.sh".
// Returns an absolute path if the file exists, otherwise an error.
func ResolveCoreNamespacedScriptPath(repoRoot, projectRoot, dotted string) (string, error) {
	dotted = strings.TrimSpace(dotted)
	if dotted == "" {
		return "", fmt.Errorf("empty core script path")
	}
	rest := dotted
	if !strings.HasPrefix(rest, "core.") {
		rest = "core." + rest
	}
	rel := "scripts/" + rest
	p := resolveScriptsPrefixedPath(repoRoot, projectRoot, rel)
	if !scriptFileExists(p) {
		return "", fmt.Errorf("core script not found for %q (resolved %s)", dotted, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func scriptFileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// tryBundledAssetsScripts maps rest "domain/tail/path" to <base>/<top>/domain/assets/scripts/tail/path.
func tryBundledAssetsScripts(base, top string, rest string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	p := filepath.Join(base, top, first, "assets", "scripts", after)
	if scriptFileExists(p) {
		return p, true
	}
	return "", false
}

// tryNestedWorkflowScripts maps rest "domain/tail/path" to
// <workflow-compile-root>/**/<domain>/assets/scripts/tail/path (any namespace depth).
func tryNestedWorkflowScripts(rest string, wfRoots []string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	for _, st := range wfRoots {
		var hit string
		err := filepath.WalkDir(st, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !d.IsDir() || filepath.Base(path) != first {
				return nil
			}
			p := filepath.Join(path, "assets", "scripts", after)
			if scriptFileExists(p) {
				hit = p
				return errStopScriptWalk
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScriptWalk) {
			continue
		}
		if hit != "" {
			return hit, true
		}
	}
	return "", false
}

// relFromCoreNamespace parses scripts/ paths after "core." — segments separated by dots become path segments.
// The final filename may contain dots (e.g. terraform-pipeline.sh): the last segment is treated as an extension
// when it matches commonExtSegment, and is joined with the previous segment as the basename.
// Example: "core.assets.scripts.docker-cache-volumes.sh" → "assets/scripts/docker-cache-volumes.sh".
func relFromCoreNamespace(rest string) (string, bool) {
	if !strings.HasPrefix(rest, "core.") {
		return "", false
	}
	inner := strings.TrimPrefix(rest, "core.")
	if inner == "" {
		return "", false
	}
	parts := strings.Split(inner, ".")
	if len(parts) < 2 {
		return "", false
	}
	n := len(parts)
	// Preserve a supported package archive suffix in a dotted asset path.
	if n >= 3 && parts[n-2] == "tar" {
		file := parts[n-3] + ".tar." + parts[n-1]
		if _, err := archivecodec.CodecForPath(file); err == nil {
			dir := parts[:n-3]
			return filepath.Join(append(dir, file)...), true
		}
	}
	if n >= 2 && commonExtSegment(parts[n-1]) {
		file := parts[n-2] + "." + parts[n-1]
		dir := parts[:n-2]
		return filepath.Join(append(dir, file)...), true
	}
	return filepath.Join(parts...), true
}

func commonExtSegment(s string) bool {
	switch strings.ToLower(s) {
	case "sh", "bash", "zsh", "ksh",
		"py", "pl", "rb", "lua", "r", "jl", "sql",
		"ps1", "psm1", "psd1",
		"js", "mjs", "cjs", "ts",
		"json", "yaml", "yml", "toml", "xml",
		"md", "txt", "cfg", "conf", "hcl", "tf", "tfvars":
		return true
	default:
		return false
	}
}

// workflowAssetPathFromRoots finds the first workflow-owned asset whose path ends with rel under the
// provided roots (installed workflows or declared authoring roots).
func workflowAssetPathFromRoots(rel string, roots []string) (string, bool) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return "", false
	}
	wantSuffix := "/" + rel
	for _, root := range roots {
		var hit string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(filepath.ToSlash(path), wantSuffix) && scriptFileExists(path) {
				hit = path
				return errStopScriptWalk
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScriptWalk) {
			continue
		}
		if hit != "" {
			return hit, true
		}
	}
	return "", false
}

func workflowAssetPathFromWorkflowTarballs(projectRoot, pkgDir, rel string) (string, bool) {
	if projectRoot == "" {
		return "", false
	}
	matches, _ := packagebuild.ListPackageArchivePaths(pkgDir, "workflow")
	for _, tgz := range matches {
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			continue
		}
		if p, ok := workflowAssetPathFromRoots(rel, []string{root}); ok {
			return p, true
		}
	}
	return "", false
}

func workflowAssetPathForLogicalScript(rest string, roots []string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	_, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	return workflowAssetPathFromRoots(filepath.Join("assets", "scripts", after), roots)
}

// resolveCoreNamespacedAsset preserves the dotted compatibility alias for assets
// owned by installed workflows or explicitly declared workflow roots.
func resolveCoreNamespacedAsset(repoRoot, projectRoot, rest string) (string, bool) {
	if projectRoot == "" {
		projectRoot = repoRoot
	}
	rel, ok := relFromCoreNamespace(rest)
	if !ok {
		return "", false
	}
	var stores []string
	if root, err := PackagesWorkflowsDir(projectRoot); err == nil {
		stores = append(stores, root)
	}
	for _, root := range configuredPackageStoreRoots(projectRoot) {
		stores = append(stores, filepath.Join(root, "workflows"))
	}
	stores = append(stores, configuredPackageTarballDirs(projectRoot)...)
	if root, err := GlobalPackagesWorkflowsDir(); err == nil {
		stores = append(stores, root)
	}
	for _, root := range SystemPackagesRoots() {
		stores = append(stores, filepath.Join(root, "workflows"))
	}
	for _, root := range stores {
		if path, ok := workflowAssetPathFromRoots(rel, []string{root}); ok {
			return path, true
		}
		if path, ok := workflowAssetPathFromWorkflowTarballs(projectRoot, root, rel); ok {
			return path, true
		}
	}
	return workflowAssetPathFromRoots(rel, WorkflowCompileRootsCached(projectRoot))
}

// scriptPathFromResolverTarballs resolves paths under compiled resolver tarballs (dockpipe-resolver-*.tar.<codec>).
// For rest like "dorkpipe/run.sh" it tries resolvers/dorkpipe/run.sh then
// resolvers/dorkpipe/assets/scripts/run.sh (canonical resolver layout).
func scriptPathFromResolverTarballs(projectRoot, pkgDir, rest string) (string, bool) {
	if projectRoot == "" {
		return "", false
	}
	matches, _ := packagebuild.ListPackageArchivePaths(pkgDir, "resolver")
	for _, tgz := range matches {
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			continue
		}
		base := filepath.Join(root, "resolvers")
		p := filepath.Join(base, rest)
		if scriptFileExists(p) {
			return p, true
		}
		if strings.Contains(rest, "/") {
			first, after, ok := strings.Cut(rest, "/")
			if ok && after != "" {
				p2 := filepath.Join(base, first, "assets", "scripts", after)
				if scriptFileExists(p2) {
					return p2, true
				}
			}
		}
	}
	return "", false
}

// scriptPathFromWorkflowTarballs resolves scripts/<domain>/tail to workflows/<domain>/assets/scripts/tail
// inside dockpipe-workflow-*.tar.<codec> (compiled store).
func scriptPathFromWorkflowTarballs(projectRoot, pkgDir, rest string) (string, bool) {
	if projectRoot == "" {
		return "", false
	}
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	rel := filepath.ToSlash(filepath.Join("workflows", first, "assets", "scripts", after))
	matches, _ := packagebuild.ListPackageArchivePaths(pkgDir, "workflow")
	for _, tgz := range matches {
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		if scriptFileExists(p) {
			return p, true
		}
	}
	return "", false
}

func resolveScriptsPrefixedPath(repoRoot, projectRoot, rel string) string {
	if projectRoot == "" {
		projectRoot = repoRoot
	}
	rest := strings.TrimPrefix(rel, "scripts/")
	user := filepath.Join(projectRoot, "scripts", rest)
	if scriptFileExists(user) {
		return user
	}
	if p, ok := resolveCoreNamespacedAsset(repoRoot, projectRoot, rest); ok {
		return p
	}
	pkgRes, _ := PackagesResolversDir(projectRoot)
	pkgWf, _ := PackagesWorkflowsDir(projectRoot)
	// Compiled local store first (package compile resolvers → tarballs under pkgRes).
	if p, ok := tryBundledAssetsScripts(pkgRes, "", rest); ok {
		return p
	}
	if p, ok := scriptPathFromResolverTarballs(projectRoot, pkgRes, rest); ok {
		return p
	}
	if p, ok := tryBundledAssetsScripts(pkgWf, "", rest); ok {
		return p
	}
	if p, ok := scriptPathFromWorkflowTarballs(projectRoot, pkgWf, rest); ok {
		return p
	}
	if scriptFileExists(filepath.Join(pkgRes, rest)) {
		return filepath.Join(pkgRes, rest)
	}
	if scriptFileExists(filepath.Join(pkgWf, rest)) {
		return filepath.Join(pkgWf, rest)
	}
	for _, root := range configuredPackageStoreRoots(projectRoot) {
		resRoot := filepath.Join(root, "resolvers")
		wfRoot := filepath.Join(root, "workflows")
		if p, ok := tryBundledAssetsScripts(resRoot, "", rest); ok {
			return p
		}
		if p, ok := scriptPathFromResolverTarballs(projectRoot, resRoot, rest); ok {
			return p
		}
		if p, ok := tryBundledAssetsScripts(wfRoot, "", rest); ok {
			return p
		}
		if p, ok := scriptPathFromWorkflowTarballs(projectRoot, wfRoot, rest); ok {
			return p
		}
		if scriptFileExists(filepath.Join(resRoot, rest)) {
			return filepath.Join(resRoot, rest)
		}
		if scriptFileExists(filepath.Join(wfRoot, rest)) {
			return filepath.Join(wfRoot, rest)
		}
	}
	for _, dir := range configuredPackageTarballDirs(projectRoot) {
		if p, ok := scriptPathFromResolverTarballs(projectRoot, dir, rest); ok {
			return p
		}
		if p, ok := scriptPathFromWorkflowTarballs(projectRoot, dir, rest); ok {
			return p
		}
	}
	// Installed dependencies live in the global or system store. Resolve their
	// scripts before consulting declared authoring roots.
	installedRoots := SystemPackagesRoots()
	if globalRoot, err := GlobalPackagesRoot(); err == nil {
		installedRoots = append([]string{globalRoot}, installedRoots...)
	}
	for _, root := range installedRoots {
		if p, ok := scriptPathFromResolverTarballs(projectRoot, filepath.Join(root, "resolvers"), rest); ok {
			return p
		}
		if p, ok := scriptPathFromWorkflowTarballs(projectRoot, filepath.Join(root, "workflows"), rest); ok {
			return p
		}
	}
	wfRoots := WorkflowCompileRootsCached(projectRoot)
	for _, root := range wfRoots {
		if p, ok := tryBundledAssetsScripts(root, "", rest); ok {
			return p
		}
	}
	if p, ok := tryNestedWorkflowScripts(rest, wfRoots); ok {
		return p
	}
	if p, ok := workflowAssetPathForLogicalScript(rest, wfRoots); ok {
		return p
	}
	for _, root := range wfRoots {
		p := filepath.Join(root, rest)
		if scriptFileExists(p) {
			return p
		}
	}
	return user
}

// ResolveActionPath resolves act script like bin/dockpipe.
// projectRoot is the Dockpipe project directory; pass "" to use repoRoot.
func ResolveActionPath(action, repoRoot, cwd, projectRoot string) (string, error) {
	if action == "" {
		return "", nil
	}
	if filepath.IsAbs(action) {
		return action, nil
	}
	candidates := []string{filepath.Join(repoRoot, action)}
	if strings.HasPrefix(action, "scripts/") {
		p := resolveScriptsPrefixedPath(repoRoot, projectRoot, action)
		if scriptFileExists(p) {
			return filepath.Abs(p)
		}
	} else {
		candidates = append(candidates, filepath.Join(repoRoot, "scripts", action))
	}
	candidates = append(candidates, filepath.Join(cwd, action))
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return filepath.Abs(c)
		}
	}
	return filepath.Abs(filepath.Join(cwd, action))
}

// ResolvePreScriptPath resolves --run path.
func ResolvePreScriptPath(p, repoRoot, projectRoot string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if strings.HasPrefix(p, "scripts/") {
		return resolveScriptsPrefixedPath(repoRoot, projectRoot, p)
	}
	c := filepath.Join(repoRoot, p)
	if _, err := os.Stat(c); err == nil {
		return c
	}
	return p
}

// ResolveResolverFilePath returns the path to a specific resolver profile (KEY=value) by name.
// Search order: project/configured stores, declared resolver roots, global/system stores, then archives.
func ResolveResolverFilePath(repoRoot, resolverName string) (string, error) {
	resolverName = strings.TrimSpace(resolverName)
	if resolverName == "" {
		return "", fmt.Errorf("resolver profile name is empty")
	}
	var candidates []string
	if pr, err := PackagesResolversDir(repoRoot); err == nil {
		candidates = append(candidates,
			filepath.Join(pr, resolverName),
			filepath.Join(pr, resolverName, "profile"),
		)
	}
	for _, root := range configuredPackageStoreRoots(repoRoot) {
		candidates = append(candidates,
			filepath.Join(root, "resolvers", resolverName),
			filepath.Join(root, "resolvers", resolverName, "profile"),
		)
	}
	candidates = append(candidates, nestedResolverProfileCandidates(repoRoot, resolverName, ResolverCompileRootsCached(repoRoot))...)
	if gr, err := GlobalPackagesResolversDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(gr, resolverName),
			filepath.Join(gr, resolverName, "profile"),
		)
	}
	for _, sr := range SystemPackagesResolversDirs() {
		candidates = append(candidates,
			filepath.Join(sr, resolverName),
			filepath.Join(sr, resolverName, "profile"),
		)
	}
	if tgz, err := FindLatestResolverTarball(repoRoot, resolverName); err == nil && tgz != "" {
		if root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(repoRoot)); err == nil {
			candidates = append(candidates,
				filepath.Join(root, "resolvers", resolverName),
				filepath.Join(root, "resolvers", resolverName, "profile"),
			)
		}
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("resolver profile not found for %q (tried %v); list maintainer trees under compile.workflows in %s or install the resolver package", resolverName, candidates, domain.DockpipeProjectConfigFileName)
}

// IsBundledCommitWorktree reports whether action is the bundled commit-worktree.sh.
func IsBundledCommitWorktree(actionPath, repoRoot string) bool {
	a, err := filepath.Abs(actionPath)
	if err != nil {
		return false
	}
	for _, b := range []string{
		filepath.Join(repoRoot, "scripts", "commit-worktree.sh"),
	} {
		bp, err := filepath.Abs(b)
		if err != nil {
			continue
		}
		if a == bp {
			return true
		}
	}
	if filepath.Base(a) != "commit-worktree.sh" {
		return false
	}
	libraryHook, err := LegacyGitHookPath(repoRoot, "commit-worktree")
	if err != nil {
		return false
	}
	return a == libraryHook
}
