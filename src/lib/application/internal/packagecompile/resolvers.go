package packagecompile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dockpipe/src/lib/application/internal/compileconfig"
	"dockpipe/src/lib/application/internal/pipelangmaterialize"
	"dockpipe/src/lib/application/internal/treecopy"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

func cmdPackageCompileResolvers(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileResolversUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		from    []string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			from = append(from, args[i+1])
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile resolvers --help)", args[i])
		default:
			if len(from) == 0 {
				from = append(from, args[i])
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if len(from) == 0 {
		cfg, err := compileconfig.Load(repoRoot)
		if err != nil {
			return err
		}
		from = compileconfig.ResolverRoots(cfg, repoRoot)
	}
	if len(from) == 0 {
		return fmt.Errorf("no resolver source directories (set compile.workflows in %s or pass --from)", domain.DockpipeProjectConfigFileName)
	}
	destRes, err := infrastructure.PackagesResolversDir(workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destRes, 0o755); err != nil {
		return err
	}
	var defResolverNamespace string
	if cfg, err := compileconfig.Load(repoRoot); err == nil && cfg != nil && cfg.Packages.Namespace != nil {
		defResolverNamespace = strings.TrimSpace(*cfg.Packages.Namespace)
	}
	opIDs := packageCompileIDs(workdir, map[string]string{
		"root_count": strconv.Itoa(len(from)),
	})
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.resolvers", "Compiling resolver packages…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
		total := 0
		skippedRoots := 0
		for _, root := range from {
			srcAbs, err := filepath.Abs(filepath.Clean(root))
			if err != nil {
				return err
			}
			if st, err := os.Stat(srcAbs); err != nil || !st.IsDir() {
				skippedRoots++
				continue
			}
			n, err := mergeChildPackages(workdir, srcAbs, destRes, "resolver", defResolverNamespace, authoredPackageVersion(repoRoot), force)
			if err != nil {
				return err
			}
			total += n
		}
		opIDs["count"] = strconv.Itoa(total)
		opIDs["output"] = filepath.ToSlash(destRes)
		if skippedRoots > 0 {
			opIDs["skipped_roots"] = strconv.Itoa(skippedRoots)
		}
		if total == 0 {
			opIDs["result"] = "noop"
		} else {
			opIDs["result"] = "compiled"
		}
		return nil
	})
}

const resolverMetaFilename = "resolver.yaml"

// readResolverNamespaceYAML returns the optional namespace from <dir>/resolver.yaml (empty if absent).
func readResolverNamespaceYAML(dir string) (string, error) {
	p := filepath.Join(dir, resolverMetaFilename)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var aux struct {
		Namespace string `yaml:"namespace"`
	}
	if err := yaml.Unmarshal(b, &aux); err != nil {
		return "", fmt.Errorf("parse %s: %w", p, err)
	}
	if err := domain.ValidateNamespace(aux.Namespace); err != nil {
		return "", err
	}
	return strings.TrimSpace(aux.Namespace), nil
}

// collectResolverPackRoots returns every directory named resolvers under srcRoot, plus the flat
// srcRoot/resolvers tree when present. The caller's compile roots define the search space; no
// repository-specific layout assumptions are baked into src/lib.
func collectResolverPackRoots(srcRoot string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(filepath.Join(srcRoot, "resolvers"))
	_ = filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != srcRoot {
			switch d.Name() {
			case ".git", ".dockpipe", ".dorkpipe", "node_modules", "target":
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
		}
		if path != srcRoot && d.Name() == "resolvers" {
			add(path)
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

// hasNestedResolverPackLayout reports whether dir looks like a grouped resolver tree (at least one
// immediate child directory has no profile/ — recurse into group folders until profile/ is found).
func hasNestedResolverPackLayout(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "profile")); err != nil {
			return true
		}
	}
	return false
}

// mergeChildPackages packs each immediate child directory from srcRoot into
// dockpipe-resolver-<name>-<ver>.tar.gz under destRoot (no expanded trees).
// For resolvers, when the tree is grouped (child dirs without profile/), we descend until profile/ is found.
//
// Resolver authoring can be flat (srcRoot/resolvers) or nested under any compile root-provided tree.
// Each resolver child still becomes its own dockpipe-resolver-<name>-*.tar.gz for the store.
func mergeChildPackages(workdir, srcRoot, destRoot string, kind string, defaultNamespace string, defaultVersion string, force bool) (int, error) {
	if kind == "resolver" {
		roots := collectResolverPackRoots(srcRoot)
		// Drop top-level resolvers/ if it does not exist (collectResolverPackRoots still added it — fix)
		roots = filterExistingResolverRoots(roots)
		if len(roots) > 0 {
			total := 0
			for _, root := range roots {
				n, err := mergeChildPackagesWalk(workdir, root, destRoot, kind, defaultNamespace, defaultVersion, force)
				total += n
				if err != nil {
					return total, err
				}
			}
			return total, nil
		}
	}
	return mergeChildPackagesWalk(workdir, srcRoot, destRoot, kind, defaultNamespace, defaultVersion, force)
}

func filterExistingResolverRoots(roots []string) []string {
	var out []string
	for _, p := range roots {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// compileSingleResolverDir packs one resolver profile directory (contains profile) into
// dockpipe-resolver-<name>-<ver>.tar.gz under destRoot.
func compileSingleResolverDir(workdir, destRoot, from, name string, defaultNamespace string, defaultVersion string, force bool) error {
	kind := "resolver"
	opIDs := packageCompileIDs(workdir, map[string]string{
		"package": name,
		"source":  filepath.ToSlash(from),
	})
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.resolver", "Compiling resolver package…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
		if err := validateWorkflowConfigsUnderDir(from); err != nil {
			return fmt.Errorf("validate resolver %s: %w", name, err)
		}
		tarGlob := filepath.Join(destRoot, fmt.Sprintf("dockpipe-resolver-%s-*.tar.gz", packagebuild.SafeTarballToken(name)))
		legacyDir := filepath.Join(destRoot, name)
		rebuild := force
		if !force {
			matches, _ := filepath.Glob(tarGlob)
			if len(matches) > 0 {
				latestTar := infrastructure.PickLatestModTimePath(matches)
				if latestTar == "" {
					rebuild = true
				} else {
					if ok, reason := compiledPackageWorkflowConfigsValid(latestTar); !ok {
						opIDs["rebuild_reason"] = "invalid_store_tarball"
						opIDs["validation_error"] = reason
						rebuild = true
					}
					if !rebuild {
						stale, err := infrastructure.SourceDirNewerThanPath(from, latestTar)
						if err != nil {
							return err
						}
						if !stale {
							opIDs["result"] = "skip"
							opIDs["skip_reason"] = "up_to_date_tarball"
							opIDs["output"] = filepath.ToSlash(latestTar)
							return nil
						}
						opIDs["rebuild_reason"] = "source_newer_than_tarball"
						rebuild = true
					}
				}
			} else if _, err := os.Stat(legacyDir); err == nil {
				refMax, err := infrastructure.MaxModTimeFilesUnder(legacyDir)
				if err != nil {
					return err
				}
				srcMax, err := infrastructure.MaxModTimeFilesUnder(from)
				if err != nil {
					return err
				}
				switch {
				case srcMax.IsZero():
					opIDs["rebuild_reason"] = "untimed_sources"
					rebuild = true
				case refMax.IsZero():
					opIDs["rebuild_reason"] = "empty_legacy_store"
					rebuild = true
				case !srcMax.After(refMax):
					opIDs["result"] = "skip"
					opIDs["skip_reason"] = "up_to_date_legacy_store"
					opIDs["output"] = filepath.ToSlash(legacyDir)
					return nil
				default:
					opIDs["rebuild_reason"] = "source_newer_than_legacy_store"
					rebuild = true
				}
			}
		}
		if rebuild {
			if n, err := infrastructure.InvalidateTarballExtractCacheForPackage(workdir, "resolver", name); err != nil {
				return err
			} else if n > 0 {
				opIDs["cache_invalidated"] = strconv.Itoa(n)
			}
			_ = infrastructure.RemoveGlob(tarGlob)
			_ = os.RemoveAll(legacyDir)
		}
		staging, err := mkdirCompileStagingDir(workdir, "dockpipe-compile-"+kind+"-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		if err := treecopy.Copy(from, staging); err != nil {
			return fmt.Errorf("copy %s %s: %w", kind, name, err)
		}
		if b, err := os.ReadFile(filepath.Join(staging, "config.yml")); err == nil {
			wf, err := domain.ParseWorkflowYAML(b)
			if err != nil {
				return fmt.Errorf("%s %s: parse workflow: %w", kind, name, err)
			}
			if err := runCompileHooksForStaging(workdir, from, staging, kind, wf.CompileHooks); err != nil {
				return fmt.Errorf("%s %s: %w", kind, name, err)
			}
			if err := infrastructure.ValidateWorkflowYAML(filepath.Join(staging, "config.yml")); err != nil {
				return fmt.Errorf("validate %s %s after compile_hooks: %w", kind, name, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("%s %s: read config.yml: %w", kind, name, err)
		}
		if err := ensureWorkflowImageEntrypoint(workdir, staging); err != nil {
			return fmt.Errorf("stage resolver image entrypoint: %w", err)
		}
		if _, err := pipelangmaterialize.MaterializeRoots([]string{staging}, true, ""); err != nil {
			return fmt.Errorf("compile pipelang artifacts for %s %s: %w", kind, name, err)
		}
		manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			pm := map[string]any{
				"schema":       1,
				"name":         name,
				"version":      defaultVersion,
				"title":        name,
				"description":  "Compiled from " + from,
				"kind":         kind,
				"allow_clone":  true,
				"distribution": "source",
			}
			ns, err := readResolverNamespaceYAML(staging)
			if err != nil {
				return fmt.Errorf("resolver %s: %w", name, err)
			}
			if ns != "" {
				pm["namespace"] = ns
			} else if strings.TrimSpace(defaultNamespace) != "" {
				pm["namespace"] = strings.TrimSpace(defaultNamespace)
			}
			out, err := yaml.Marshal(pm)
			if err != nil {
				return err
			}
			if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
				return err
			}
		}
		pmParsed, err := domain.ParsePackageManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("%s %s: %w", kind, name, err)
		}
		cfgPath := filepath.Join(staging, "config.yml")
		if b, err := os.ReadFile(cfgPath); err == nil {
			wf, err := domain.ParseWorkflowYAML(b)
			if err != nil {
				return fmt.Errorf("%s %s: parse workflow: %w", kind, name, err)
			}
			if err := writeCompiledWorkflowRuntimeArtifacts(workdir, staging, name, wf, pmParsed); err != nil {
				return fmt.Errorf("%s %s: write runtime artifacts: %w", kind, name, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("%s %s: read config.yml: %w", kind, name, err)
		}
		ver := strings.TrimSpace(pmParsed.Version)
		if ver == "" {
			ver = defaultVersion
		}
		prefix := "resolvers/" + name
		base := fmt.Sprintf("dockpipe-resolver-%s-%s.tar.gz", packagebuild.SafeTarballToken(name), packagebuild.SafeTarballToken(ver))
		outPath := filepath.Join(destRoot, base)
		if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, outPath, prefix); err != nil {
			return fmt.Errorf("%s %s: %w", kind, name, err)
		}
		opIDs["result"] = "compiled"
		opIDs["output"] = filepath.ToSlash(outPath)
		return nil
	})
}

func mergeChildPackagesWalk(workdir, srcRoot, destRoot string, kind string, defaultNamespace string, defaultVersion string, force bool) (int, error) {
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return 0, err
	}
	nestedPack := kind == "resolver" && hasNestedResolverPackLayout(srcRoot)
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		from := filepath.Join(srcRoot, name)
		if nestedPack {
			if _, err := os.Stat(filepath.Join(from, "profile")); err != nil {
				sub, err := mergeChildPackagesWalk(workdir, from, destRoot, kind, defaultNamespace, defaultVersion, force)
				if err != nil {
					return n, err
				}
				n += sub
				continue
			}
		}
		if kind != "resolver" {
			return n, fmt.Errorf("mergeChildPackages: unknown kind %q", kind)
		}
		if err := compileSingleResolverDir(workdir, destRoot, from, name, defaultNamespace, defaultVersion, force); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
