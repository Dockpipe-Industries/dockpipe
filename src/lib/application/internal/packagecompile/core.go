package packagecompile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/compileconfig"
	"dockpipe/src/lib/application/internal/pipelangmaterialize"
	"dockpipe/src/lib/application/internal/treecopy"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

func cmdPackageCompileCore(args []string, sourceBuild CoreSourceBuild) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileCoreUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		src     string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			src = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile core --help)", args[i])
		default:
			if src == "" {
				src = args[i]
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
	if strings.TrimSpace(src) == "" {
		cfg, err := compileconfig.Load(repoRoot)
		if err != nil {
			return err
		}
		if p, err := compileconfig.CoreFrom(cfg, repoRoot); err != nil {
			return err
		} else if strings.TrimSpace(p) != "" {
			src = p
		}
	}
	opIDs := packageCompileIDs(workdir, map[string]string{
		"package": "dockpipe.core",
	})
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.core", "Compiling core package…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
		if strings.TrimSpace(src) == "" {
			src, err = defaultCoreSource(repoRoot)
			if err != nil {
				if seeded, seedErr := seedCompiledCoreFromInstalledTarball(workdir, force, opIDs); seedErr == nil && seeded {
					return nil
				}
				return err
			}
		}
		srcAbs, err := filepath.Abs(filepath.Clean(src))
		if err != nil {
			return err
		}
		opIDs["source"] = filepath.ToSlash(srcAbs)
		if st, err := os.Stat(srcAbs); err != nil || !st.IsDir() {
			return fmt.Errorf("core source must be a directory: %s", srcAbs)
		}
		coreDir, err := infrastructure.PackagesCoreDir(workdir)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(coreDir, 0o755); err != nil {
			return err
		}
		coreTarGlob := filepath.Join(coreDir, "dockpipe-core-*.tar.gz")
		if !force {
			if matches, _ := filepath.Glob(coreTarGlob); len(matches) > 0 {
				opIDs["result"] = "skip"
				opIDs["skip_reason"] = "existing_tarball"
				opIDs["output"] = filepath.ToSlash(matches[0])
				return nil
			}
			if st, err := os.Stat(filepath.Join(coreDir, "runtimes")); err == nil && st.IsDir() {
				opIDs["result"] = "skip"
				opIDs["skip_reason"] = "legacy_tree_exists"
				opIDs["output"] = filepath.ToSlash(coreDir)
				return nil
			}
		} else {
			_ = infrastructure.RemoveGlob(coreTarGlob)
			_ = infrastructure.RemoveLegacyPackageSubdirs(coreDir)
		}
		staging, err := mkdirCompileStagingDir(workdir, "dockpipe-compile-core-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		exclude := map[string]bool{"resolvers": true, "bundles": true, "workflows": true}
		if err := treecopy.CopyExcludingTopLevel(srcAbs, staging, exclude); err != nil {
			return fmt.Errorf("copy core: %w", err)
		}
		if sourceBuild == nil {
			return fmt.Errorf("core source build runner is not configured")
		}
		if err := sourceBuild(workdir, staging); err != nil {
			return err
		}
		if _, err := pipelangmaterialize.MaterializeRoots([]string{staging}, true, ""); err != nil {
			return fmt.Errorf("compile pipelang artifacts: %w", err)
		}
		defaultVersion := authoredPackageVersion(workdir)
		manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			pm := map[string]any{
				"schema":       1,
				"name":         "dockpipe.core",
				"version":      defaultVersion,
				"title":        "Compiled core slice",
				"description":  "Compiled from " + srcAbs,
				"kind":         "core",
				"allow_clone":  true,
				"distribution": "source",
				"depends":      []string{},
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
			return fmt.Errorf("package manifest: %w", err)
		}
		ver := strings.TrimSpace(pmParsed.Version)
		if ver == "" {
			ver = defaultVersion
		}
		outPath := filepath.Join(coreDir, fmt.Sprintf("dockpipe-core-%s.tar.gz", packagebuild.SafeTarballToken(ver)))
		if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, outPath, "core"); err != nil {
			return err
		}
		opIDs["result"] = "compiled"
		opIDs["output"] = filepath.ToSlash(outPath)
		return nil
	})
}

func defaultCoreSource(repoRoot string) (string, error) {
	srcCore := filepath.Join(repoRoot, "src", "core")
	if st, err := os.Stat(filepath.Join(srcCore, "runtimes")); err == nil && st.IsDir() {
		return filepath.Abs(srcCore)
	}
	tc := filepath.Join(repoRoot, "templates", "core")
	if st, err := os.Stat(filepath.Join(tc, "runtimes")); err == nil && st.IsDir() {
		return filepath.Abs(tc)
	}
	return "", fmt.Errorf("no default core tree (expected src/core/runtimes or templates/core/runtimes); use --from <dir>")
}

func seedCompiledCoreFromInstalledTarball(workdir string, force bool, opIDs map[string]string) (bool, error) {
	coreDir, err := infrastructure.PackagesCoreDir(workdir)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		return false, err
	}
	coreTarGlob := filepath.Join(coreDir, "dockpipe-core-*.tar.gz")
	if !force {
		if matches, _ := filepath.Glob(coreTarGlob); len(matches) > 0 {
			opIDs["result"] = "skip"
			opIDs["skip_reason"] = "existing_tarball"
			opIDs["output"] = filepath.ToSlash(matches[0])
			return true, nil
		}
	}
	sourceTarball := ""
	if gp, err := infrastructure.GlobalPackagesRoot(); err == nil {
		if p, err := latestGlob(filepath.Join(gp, "core", "dockpipe-core-*.tar.gz")); err != nil {
			return false, err
		} else if p != "" {
			sourceTarball = p
		}
	}
	if sourceTarball == "" {
		for _, dir := range infrastructure.SystemPackagesCoreDirs() {
			if p, err := latestGlob(filepath.Join(dir, "dockpipe-core-*.tar.gz")); err != nil {
				return false, err
			} else if p != "" {
				sourceTarball = p
				break
			}
		}
	}
	if sourceTarball == "" {
		return false, nil
	}
	if force {
		_ = infrastructure.RemoveGlob(coreTarGlob)
		_ = infrastructure.RemoveLegacyPackageSubdirs(coreDir)
	}
	destTarball := filepath.Join(coreDir, filepath.Base(sourceTarball))
	if err := copyFileWithMode(sourceTarball, destTarball, 0o644); err != nil {
		return false, err
	}
	if sum := sourceTarball + ".sha256"; fileExists(sum) {
		if err := copyFileWithMode(sum, destTarball+".sha256", 0o644); err != nil {
			return false, err
		}
	}
	opIDs["source"] = filepath.ToSlash(sourceTarball)
	opIDs["result"] = "seeded"
	opIDs["output"] = filepath.ToSlash(destTarball)
	return true, nil
}

func latestGlob(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	return infrastructure.PickLatestModTimePath(matches), nil
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

const packageCompileCoreUsageText = `dockpipe package compile core

Copies a core authoring tree (default: src/core or templates/core when present) into
<workdir>/bin/.dockpipe/internal/packages/core/ and writes package.yml (kind: core).
Top-level resolvers/, bundles/, and workflows/ in the source are omitted — compile those with
"package compile resolvers|workflows" so they land under packages/resolvers/ or packages/workflows/.

Optional dockpipe.config.json "compile.core_from" overrides the default core path when --from is omitted.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source core root (typically runtimes/, strategies/, assets/, …)
  --force            Replace existing packages/core tree

`
