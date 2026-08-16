package application

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/application/internal/operationids"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// cmdBuild runs package compile: full `compile all` by default, or `compile for-workflow` when --for-workflow is set.
func cmdBuild(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(buildUsageText)
		return nil
	}
	var (
		wfName          string
		buildImages     = true
		buildSourcePkgs = true
	)
	forward := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--for-workflow":
			if i+1 >= len(args) {
				return fmt.Errorf("--for-workflow requires a workflow name")
			}
			wfName = args[i+1]
			i++
			continue
		case "--images":
			buildImages = true
			continue
		case "--no-images":
			buildImages = false
			continue
		case "--source-builds":
			buildSourcePkgs = true
			continue
		case "--no-source-builds":
			buildSourcePkgs = false
			continue
		}
		forward = append(forward, args[i])
	}
	workdir, err := parseBuildWorkdir(forward)
	if err != nil {
		return err
	}
	compileIDs := buildOperationIDs(workdir, wfName)
	compileIDs["mode"] = "all"
	compileArgs := append([]string{"compile", "all", "--force"}, forward...)
	if wfName != "" {
		compileIDs["mode"] = "for-workflow"
		compileArgs = append([]string{"compile", "for-workflow", wfName, "--force"}, forward...)
	}
	if err := infrastructure.RunOperationWithOptions(os.Stderr, "build.compile", "Compiling DockPipe packages…", compileIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		return cmdPackage(compileArgs)
	}); err != nil {
		return err
	}
	if buildSourcePkgs {
		if _, err := RunPackageBuildSourceFromFlags(workdir, ""); err != nil {
			return err
		}
	}
	if !buildImages {
		return nil
	}
	imageIDs := buildOperationIDs(workdir, wfName)
	err = infrastructure.RunOperationWithOptions(os.Stderr, "build.image.artifacts", "Materializing image artifacts…", imageIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		n, runErr := prebuildCompiledImageArtifacts(workdir)
		if runErr == nil {
			imageIDs["count"] = strconv.Itoa(n)
		}
		return runErr
	})
	if err != nil {
		return err
	}
	return nil
}

func buildOperationIDs(workdir, workflow string) map[string]string {
	return operationids.Build(workdir, workflow)
}

func mergeOperationResultIDs(base map[string]string, extra map[string]string) map[string]string {
	return operationids.Merge(base, extra)
}

func parseBuildWorkdir(args []string) (string, error) {
	var workdir string
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			workdir = args[i+1]
			i++
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root, err := domain.FindProjectRootWithDockpipeConfig(wd)
		if err != nil {
			return "", err
		}
		return filepath.Abs(root)
	}
	return filepath.Abs(workdir)
}

func cmdClean(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(cleanUsageText)
		return nil
	}
	workdir, dryRun, err := parseCleanArgs(args)
	if err != nil {
		return err
	}
	root, err := resolveCheckoutCleanTarget(workdir)
	if err != nil {
		return fmt.Errorf("clean target: %w", err)
	}
	if err := infrastructure.ValidateDisposableRemovalPath(workdir, root); err != nil {
		return fmt.Errorf("clean target: %w", err)
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		if dryRun {
			writeCleanPreview(root, infrastructure.DisposableRemovalTreeSummary{}, "noop")
			return nil
		}
		writeCleanResult(root, infrastructure.DisposableRemovalTreeSummary{}, "noop")
		return nil
	} else if err != nil {
		return err
	}
	summary, err := infrastructure.InspectDisposableRemovalTree(root)
	if err != nil {
		return fmt.Errorf("clean target: %w", err)
	}
	if dryRun {
		writeCleanPreview(root, summary, "remove")
		return nil
	}
	if err := infrastructure.ValidateDisposableRemovalPath(workdir, root); err != nil {
		return fmt.Errorf("clean target changed: %w", err)
	}
	if err := infrastructure.ValidateDisposableRemovalTree(root); err != nil {
		return fmt.Errorf("clean target changed: %w", err)
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	writeCleanResult(root, summary, "removed")
	return nil
}

func writeCleanPreview(root string, summary infrastructure.DisposableRemovalTreeSummary, action string) {
	fmt.Printf("dry_run=true target=%q logical_bytes=%d files=%d action=%s\n", root, summary.LogicalBytes, summary.Files, action)
}

func writeCleanResult(root string, summary infrastructure.DisposableRemovalTreeSummary, action string) {
	fmt.Fprintf(os.Stderr, "dry_run=false target=%q logical_bytes=%d files=%d action=%s\n", root, summary.LogicalBytes, summary.Files, action)
}

func parseCleanArgs(args []string) (string, bool, error) {
	var workdir string
	dryRun := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" {
			if i+1 >= len(args) {
				return "", false, fmt.Errorf("--workdir requires a path")
			}
			workdir = args[i+1]
			if cleanPathHasTraversal(workdir) {
				return "", false, fmt.Errorf("--workdir must not contain parent traversal")
			}
			i++
			continue
		}
		if args[i] == "--dry-run" {
			dryRun = true
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			return "", false, fmt.Errorf("unknown option %s (try: dockpipe clean --help)", args[i])
		}
		return "", false, fmt.Errorf("unexpected argument %q (try: dockpipe clean --help)", args[i])
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", false, err
		}
		root, err := domain.FindProjectRootWithDockpipeConfig(wd)
		if err != nil {
			return "", false, err
		}
		wdAbs, err := filepath.Abs(wd)
		if err != nil {
			return "", false, err
		}
		if root != wdAbs {
			fmt.Fprintf(os.Stderr, "[dockpipe] using project root %s (%s)\n", root, domain.DockpipeProjectConfigFileName)
		}
		workdir = root
	}
	workdir, err := filepath.Abs(workdir)
	return workdir, dryRun, err
}

func cleanPathHasTraversal(path string) bool {
	path = strings.ReplaceAll(path, `\`, "/")
	for _, component := range strings.Split(path, "/") {
		if component == ".." {
			return true
		}
	}
	return false
}

func resolveCheckoutCleanTarget(workdir string) (string, error) {
	if err := validateCleanStateRelativePath(infrastructure.DockpipeDirRel); err != nil {
		return "", err
	}
	root, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return validateCheckoutCleanTarget(workdir, root)
}

func validateCleanStateRelativePath(stateRel string) error {
	if filepath.IsAbs(stateRel) || filepath.VolumeName(stateRel) != "" || cleanPathHasTraversal(stateRel) {
		return fmt.Errorf("refusing non-project state path %q", stateRel)
	}
	stateRel = filepath.Clean(stateRel)
	if stateRel == "." || stateRel == "" {
		return fmt.Errorf("refusing workdir target")
	}
	return nil
}

func validateCheckoutCleanTarget(workdir, root string) (string, error) {
	workdir, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return "", err
	}
	volumeRoot := filepath.VolumeName(workdir) + string(filepath.Separator)
	if filepath.VolumeName(workdir) == "" {
		volumeRoot = string(filepath.Separator)
	}
	if sameBuildPath(workdir, volumeRoot) {
		return "", fmt.Errorf("refusing filesystem-root workdir %q", workdir)
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	expected, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return "", err
	}
	if !sameBuildPath(root, expected) {
		return "", fmt.Errorf("refusing nonstandard checkout state target %q", root)
	}
	if !buildPathContains(workdir, root) || sameBuildPath(workdir, root) || buildPathContains(root, workdir) {
		return "", fmt.Errorf("refusing workdir or ancestor target %q", root)
	}
	durableRoot, err := infrastructure.DurableStateRoot()
	if err != nil {
		return "", err
	}
	durableRoot, err = filepath.Abs(filepath.Clean(durableRoot))
	if err != nil {
		return "", err
	}
	if buildPathContains(root, durableRoot) || buildPathContains(durableRoot, root) {
		return "", fmt.Errorf("refusing durable state target %q", root)
	}
	return root, nil
}

func cmdRebuild(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(rebuildUsageText)
		return nil
	}
	workdir, err := parseBuildWorkdir(args)
	if err != nil {
		return err
	}
	if err := resetCompiledPackagesRoot(workdir); err != nil {
		return err
	}
	return cmdBuild(args)
}

func resetCompiledPackagesRoot(workdir string) error {
	root, err := infrastructure.PackagesRoot(workdir)
	if err != nil {
		return err
	}
	if err := validateCompiledPackagesResetTarget(workdir, root); err != nil {
		return fmt.Errorf("rebuild package-store reset: %w", err)
	}
	ids := mergeOperationResultIDs(buildOperationIDs(workdir, ""), map[string]string{"root": root})
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "build.package_store_reset",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs:        mergeOperationResultIDs(ids, map[string]string{"result": "noop"}),
		})
		return nil
	} else if err != nil {
		return err
	}
	if err := infrastructure.ValidateDisposableRemovalTree(root); err != nil {
		return fmt.Errorf("rebuild package-store reset target: %w", err)
	}
	return infrastructure.RunOperation(os.Stderr, "build.package_store_reset", "Resetting compiled package store…", ids, func() error {
		if err := os.RemoveAll(root); err != nil {
			return fmt.Errorf("reset compiled package store: %w", err)
		}
		return nil
	})
}

func validateCompiledPackagesResetTarget(workdir, root string) error {
	workdir, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return err
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return err
	}
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	if filepath.VolumeName(root) == "" {
		volumeRoot = string(filepath.Separator)
	}
	if sameBuildPath(root, volumeRoot) {
		return fmt.Errorf("refusing filesystem root %q", root)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		home, err = filepath.Abs(filepath.Clean(home))
		if err == nil && sameBuildPath(root, home) {
			return fmt.Errorf("refusing user home %q", root)
		}
	}
	if buildPathContains(root, workdir) {
		return fmt.Errorf("refusing workdir or its ancestor %q", root)
	}
	durableRoot, err := infrastructure.DurableStateRoot()
	if err != nil {
		return err
	}
	durableRoot, err = filepath.Abs(filepath.Clean(durableRoot))
	if err != nil {
		return err
	}
	if buildPathContains(root, durableRoot) || buildPathContains(durableRoot, root) {
		return fmt.Errorf("refusing durable state target %q", root)
	}
	return nil
}

func buildPathContains(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func sameBuildPath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

const buildUsageText = `dockpipe build [--for-workflow <name>] [options]

Without --for-workflow: same as dockpipe package compile all --force (full store).
PipeLang sources are compiled during package staging and included in package tarballs.
Dockerfile-backed image artifacts are prebuilt by default after compile.

With --for-workflow <name>: same as dockpipe package compile for-workflow <name> --force
(transitive core + resolver + workflow closure only).

Options:
  --for-workflow <name>   Dependency-scoped compile instead of compile all
  --source-builds         Run package.yml build.source.script hooks after compile (default)
  --no-source-builds      Skip package-owned source-checkout build hooks
  --images                Prebuild Dockerfile-backed image artifacts after compile (default)
  --no-images             Only compile package/runtime/image manifests; do not run docker build
  Otherwise same as package compile all / for-workflow: --workdir
  (see: dockpipe package compile all --help)

`

const cleanUsageText = `dockpipe clean

Remove the complete checkout-local disposable tree at <workdir>/bin/.dockpipe.
Use --dry-run to report the exact target, logical bytes, file count, and action without mutation.

When --workdir is omitted, the project directory is the folder containing
dockpipe.config.json (walking up from the current directory), or the current
directory if that file is not found.

Usage:
  dockpipe clean [--workdir <path>] [--dry-run]

DOCKPIPE_PACKAGES_ROOT does not widen clean outside the checkout. Rebuild owns a separate,
guarded reset of the resolved compiled package store for compatibility with that override.

`

const rebuildUsageText = `dockpipe rebuild

Safely resets the resolved compiled package store, then runs dockpipe build (compile all with
--force, or compile for-workflow if you pass --for-workflow). The reset honors
DOCKPIPE_PACKAGES_ROOT but rejects filesystem roots, home/workdir authority, durable state,
links/reparse points, filesystem substitutions, and other unsafe targets.

Default project directory (when --workdir omitted) is the same as compile: the directory
with dockpipe.config.json, found by walking up from the current directory.

Usage:
  dockpipe rebuild [options]

Options:
  Same as dockpipe build / package compile all (--workdir).
  build implies --force for compile outputs. See: dockpipe package compile all --help

`
