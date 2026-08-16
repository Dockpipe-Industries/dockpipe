package packagecompile

import (
	"fmt"
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

func cmdPackageCompileWorkflow(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileWorkflowUsageText)
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
		name    string
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
		case args[i] == "--name" && i+1 < len(args):
			name = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile workflow --help)", args[i])
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
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("missing source directory (use --from <path> or a positional path)")
	}
	srcAbs, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return err
	}
	return compileWorkflowOne(workdir, srcAbs, name, force)
}

// compileWorkflowOne validates YAML, materializes a streamable dockpipe-workflow-<name>-<ver>.tar.gz
// under packages/workflows/ (no expanded directory trees in the store).
func compileWorkflowOne(workdir, srcAbs, name string, force bool) error {
	cfgPath := filepath.Join(srcAbs, "config.yml")
	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("workflow source must contain config.yml: %w", err)
	}
	if err := infrastructure.ValidateWorkflowYAML(cfgPath); err != nil {
		return fmt.Errorf("validate workflow: %w", err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	pkgName := strings.TrimSpace(name)
	if pkgName == "" {
		pkgName = strings.TrimSpace(wf.Name)
	}
	if pkgName == "" {
		pkgName = filepath.Base(srcAbs)
	}
	opIDs := packageCompileIDs(workdir, map[string]string{
		"package": pkgName,
		"source":  filepath.ToSlash(srcAbs),
	})
	pw, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return err
	}
	return infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.workflow", "Compiling workflow package…", opIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
		if err := os.MkdirAll(pw, 0o755); err != nil {
			return err
		}
		tarGlob := filepath.Join(pw, fmt.Sprintf("dockpipe-workflow-%s-*.tar.gz", packagebuild.SafeTarballToken(pkgName)))
		legacyDir := filepath.Join(pw, pkgName)
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
						stale, err := infrastructure.SourceDirNewerThanPath(srcAbs, latestTar)
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
				srcMax, err := infrastructure.MaxModTimeFilesUnder(srcAbs)
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
			if n, err := infrastructure.InvalidateTarballExtractCacheForPackage(workdir, "workflow", pkgName); err != nil {
				return err
			} else if n > 0 {
				opIDs["cache_invalidated"] = strconv.Itoa(n)
			}
			_ = infrastructure.RemoveGlob(tarGlob)
			_ = os.RemoveAll(legacyDir)
		}
		staging, err := mkdirCompileStagingDir(workdir, "dockpipe-compile-wf-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		if err := treecopy.Copy(srcAbs, staging); err != nil {
			return fmt.Errorf("copy workflow: %w", err)
		}
		if err := runWorkflowCompileHooks(workdir, srcAbs, staging, wf.CompileHooks); err != nil {
			return err
		}
		b, err = os.ReadFile(filepath.Join(staging, "config.yml"))
		if err != nil {
			return err
		}
		wf, err = domain.ParseWorkflowYAML(b)
		if err != nil {
			return fmt.Errorf("parse workflow: %w", err)
		}
		if err := infrastructure.ValidateWorkflowYAML(filepath.Join(staging, "config.yml")); err != nil {
			return fmt.Errorf("validate workflow after compile_hooks: %w", err)
		}
		if err := ensureWorkflowImageEntrypoint(workdir, staging); err != nil {
			return fmt.Errorf("stage workflow image entrypoint: %w", err)
		}
		if _, err := pipelangmaterialize.MaterializeRoots([]string{staging}, true, ""); err != nil {
			return fmt.Errorf("compile pipelang artifacts: %w", err)
		}
		authoredManifest, err := readAuthoredPackageManifest(srcAbs)
		if err != nil {
			return fmt.Errorf("package manifest: %w", err)
		}
		if err := writeCompiledWorkflowRuntimeArtifacts(workdir, staging, pkgName, wf, authoredManifest); err != nil {
			return fmt.Errorf("write runtime artifacts: %w", err)
		}
		defaultVersion := authoredPackageVersion(workdir)
		manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			pm := map[string]any{
				"schema":       1,
				"name":         pkgName,
				"version":      defaultVersion,
				"title":        pkgName,
				"description":  "Compiled from " + srcAbs,
				"kind":         "workflow",
				"allow_clone":  true,
				"distribution": "source",
			}
			repoRoot, err := filepath.Abs(workdir)
			if err != nil {
				return err
			}
			if ns := strings.TrimSpace(wf.Namespace); ns != "" {
				pm["namespace"] = ns
			} else if pc, err := compileconfig.Load(repoRoot); err == nil && pc != nil && pc.Packages.Namespace != nil {
				if def := strings.TrimSpace(*pc.Packages.Namespace); def != "" {
					pm["namespace"] = def
				}
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
		outPath := filepath.Join(pw, fmt.Sprintf("dockpipe-workflow-%s-%s.tar.gz", packagebuild.SafeTarballToken(pkgName), packagebuild.SafeTarballToken(ver)))
		if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, outPath, "workflows/"+pkgName); err != nil {
			return err
		}
		opIDs["result"] = "compiled"
		opIDs["output"] = filepath.ToSlash(outPath)
		return nil
	})
}

func runWorkflowCompileHooks(workdir, srcAbs, staging string, hooks []string) error {
	return runCompileHooksForStaging(workdir, srcAbs, staging, "workflow", hooks)
}
