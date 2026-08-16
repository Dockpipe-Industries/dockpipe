package packagecompile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/application/internal/operationids"
	"dockpipe/src/lib/application/internal/packagescript"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

const packageCompileProgressEvery = 5 * time.Second

func packageCompileIDs(workdir string, extra map[string]string) map[string]string {
	return operationids.Merge(operationids.Build(workdir, ""), extra)
}

// injectCompileWorkdirFromProjectConfig prepends --workdir <dir> when args does not already
// set it, where dir is the directory containing dockpipe.config.json found by walking up
// from the current working directory (or cwd if the file is absent).
func injectCompileWorkdirFromProjectConfig(args []string) ([]string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			return args, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := domain.FindProjectRootWithDockpipeConfig(cwd)
	if err != nil {
		return nil, err
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	if root != cwdAbs {
		fmt.Fprintf(os.Stderr, "[dockpipe] using project root %s (%s)\n", root, domain.DockpipeProjectConfigFileName)
	}
	return append([]string{"--workdir", root}, args...), nil
}

func cmdPackageCompile(args []string, sourceBuild CoreSourceBuild) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(packageCompileUsageText)
		return nil
	}
	switch args[0] {
	case "for-workflow":
		return cmdPackageCompileForWorkflow(args[1:], sourceBuild)
	case "workflow":
		return cmdPackageCompileWorkflow(args[1:])
	case "core":
		return cmdPackageCompileCore(args[1:], sourceBuild)
	case "resolvers":
		return cmdPackageCompileResolvers(args[1:])
	case "bundles":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			fmt.Print(packageCompileBundlesUsageText)
			return nil
		}
		// Compatibility command alias; it uses the canonical workflow compile path.
		return cmdPackageCompileWorkflowsBatch(args[1:])
	case "workflows":
		return cmdPackageCompileWorkflowsBatch(args[1:])
	case "all":
		return cmdPackageCompileAll(args[1:], sourceBuild)
	default:
		return fmt.Errorf("unknown package compile target %q (try: dockpipe package compile --help; use for-workflow for dependency-scoped compile)", args[0])
	}
}

func mkdirCompileStagingDir(workdir, pattern string) (string, error) {
	stateRoot, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return "", err
	}
	tmpRoot := filepath.Join(stateRoot, "tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(tmpRoot, pattern)
}

func runCompileHooksForStaging(workdir, srcAbs, staging, kind string, hooks []string) error {
	for i, hook := range hooks {
		hook = strings.TrimSpace(hook)
		if hook == "" {
			continue
		}
		cmd, bashExe, err := packagescript.BashShellCommand(hook)
		if err != nil {
			return fmt.Errorf("compile_hooks[%d]: %w", i, err)
		}
		cmd.Dir = staging
		compileWorkdir := workdir
		compileSourceDir := srcAbs
		compileStagingDir := staging
		if bashExe != "" {
			compileWorkdir = packagescript.PathForBashEnv(bashExe, workdir)
			compileSourceDir = packagescript.PathForBashEnv(bashExe, srcAbs)
			compileStagingDir = packagescript.PathForBashEnv(bashExe, staging)
		}
		cmd.Env = append(os.Environ(),
			"DOCKPIPE_COMPILE_KIND="+strings.TrimSpace(kind),
			"DOCKPIPE_COMPILE_WORKDIR="+compileWorkdir,
			"DOCKPIPE_COMPILE_SOURCE_DIR="+compileSourceDir,
			"DOCKPIPE_COMPILE_STAGING_DIR="+compileStagingDir,
		)
		if bashExe != "" {
			cmd.Env = packagescript.UpsertEnv(cmd.Env, "DOCKPIPE_HOST_BASH_BIN", bashExe)
		}
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		hookIDs := packageCompileIDs(workdir, map[string]string{
			"compile_kind": strings.TrimSpace(kind),
			"hook_index":   strconv.Itoa(i),
			"source":       filepath.ToSlash(srcAbs),
			"staging":      filepath.ToSlash(staging),
		})
		if err := infrastructure.RunOperationWithOptions(os.Stderr, "package.compile.hook", "Running compile hook…", hookIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: packageCompileProgressEvery}, func() error {
			return cmd.Run()
		}); err != nil {
			return fmt.Errorf("compile_hooks[%d]: %w", i, err)
		}
	}
	return nil
}

func readAuthoredPackageManifest(root string) (*domain.PackageManifest, error) {
	manifestPath := filepath.Join(root, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return domain.ParsePackageManifest(manifestPath)
}

func validateWorkflowConfigsUnderDir(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".dockpipe", ".dorkpipe", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "config.yml" {
			return nil
		}
		if err := infrastructure.ValidateResolvedWorkflowYAML(path); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	})
}

func compiledPackageWorkflowConfigsValid(tgz string) (bool, string) {
	members, err := packagebuild.ListTarGzMemberPaths(tgz)
	if err != nil {
		return false, err.Error()
	}
	for _, entry := range members {
		if !compiledPackageWorkflowConfigEntry(entry) {
			continue
		}
		if err := validateWorkflowConfigInTarball(tgz, entry); err != nil {
			return false, fmt.Sprintf("%s: %v", entry, err)
		}
	}
	return true, ""
}

func compiledPackageWorkflowConfigEntry(entry string) bool {
	parts := strings.Split(filepath.ToSlash(entry), "/")
	if len(parts) < 3 || parts[len(parts)-1] != "config.yml" {
		return false
	}
	switch parts[0] {
	case "workflows":
		return len(parts) == 3
	case "resolvers":
		// Resolver packages may include a resolver-shaped workflow at
		// resolvers/<name>/config.yml or embedded child workflows one level down.
		return len(parts) == 3 || len(parts) == 4
	default:
		return false
	}
}

func validateWorkflowConfigInTarball(tgz, entry string) error {
	entry = filepath.ToSlash(entry)
	b, err := packagebuild.ReadFileFromTarGz(tgz, entry)
	if err != nil {
		return err
	}
	baseDir := filepath.ToSlash(filepath.Dir(entry))
	readFile := func(p string) ([]byte, error) {
		return packagebuild.ReadFileFromTarGz(tgz, filepath.ToSlash(filepath.Clean(p)))
	}
	wf, err := domain.ParseWorkflowFromDisk(b, baseDir, readFile)
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	if err := domain.ValidateLoadedWorkflow(wf); err != nil {
		return err
	}
	return nil
}

func ensureWorkflowImageEntrypoint(workdir, staging string) error {
	imagesDir := filepath.Join(staging, "assets", "images")
	if st, err := os.Stat(imagesDir); err != nil || !st.IsDir() {
		return nil
	}
	needsEntrypoint := false
	if err := filepath.WalkDir(imagesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "Dockerfile" {
			needsEntrypoint = true
			return filepath.SkipAll
		}
		return nil
	}); err != nil {
		return err
	}
	if !needsEntrypoint {
		return nil
	}
	dst := filepath.Join(staging, "assets", "entrypoint.sh")
	if st, err := os.Stat(dst); err == nil && !st.IsDir() {
		return nil
	}
	src := filepath.Join(workdir, "assets", "entrypoint.sh")
	b, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
