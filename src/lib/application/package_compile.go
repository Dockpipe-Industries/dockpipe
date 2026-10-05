package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/application/internal/packagecompile"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func cmdPackageCompile(args []string) error {
	return packagecompile.Command(args, runCoreSourceBuildTarget)
}

func injectCompileWorkdirFromProjectConfig(args []string) ([]string, error) {
	return packagecompile.InjectWorkdirFromProjectConfig(args)
}

func cmdPackageCompileAll(args []string) error {
	return packagecompile.CompileAll(args, runCoreSourceBuildTarget)
}

func cmdPackageCompileWorkflowsBatch(args []string) error {
	return packagecompile.CompileWorkflows(args)
}

func compileSingleResolverDir(workdir, destRoot, source, name, defaultNamespace, defaultVersion string, force bool) error {
	return packagecompile.CompileResolverDir(workdir, destRoot, source, name, defaultNamespace, defaultVersion, force)
}

func compileClosureForWorkflow(projectRoot, workflowName string, force bool) error {
	return packagecompile.CompileClosureForWorkflow(projectRoot, workflowName, force, runCoreSourceBuildTarget)
}

func compileWorkflowOne(workdir, source, name string, force bool) error {
	return packagecompile.CompileWorkflow(workdir, source, name, force)
}

func validateCompileOutputsForMode(workdir string, requireWorkflowNamespace bool) error {
	return packagecompile.ValidateOutputsForMode(workdir, requireWorkflowNamespace)
}

func writeJSONFile(path string, value any) error {
	return packagecompile.WriteJSONFile(path, value)
}

func closureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir string, cfg *domain.DockpipeProjectConfig) ([]string, map[string]bool, error) {
	return packagecompile.ClosureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir, cfg)
}

func runCoreSourceBuildTarget(workdir, staging string) error {
	manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	manifest, err := domain.ParsePackageManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("package manifest: %w", err)
	}
	if manifest.Build.Source == nil || strings.TrimSpace(manifest.Build.Source.Script) == "" {
		return nil
	}
	target := packageScriptTarget{
		Name:       strings.TrimSpace(manifest.Name),
		Manifest:   manifestPath,
		PackageDir: staging,
		ScriptRel:  filepath.Clean(manifest.Build.Source.Script),
		ScriptAbs:  filepath.Join(staging, filepath.FromSlash(manifest.Build.Source.Script)),
	}
	buildIDs := mergeOperationResultIDs(buildOperationIDs(workdir, ""), map[string]string{
		"package": target.Name,
		"script":  filepath.ToSlash(target.ScriptRel),
		"source":  filepath.ToSlash(staging),
	})
	if err := infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.source_build", "Running core source build…", buildIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		return runPackageScriptTarget(workdir, target, packageSourceBuildEnv(workdir, target), "build.source.script")
	}); err != nil {
		return fmt.Errorf("core source build: %w", err)
	}
	return nil
}
