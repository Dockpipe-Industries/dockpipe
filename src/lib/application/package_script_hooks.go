package application

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/application/internal/compileconfig"
	"dockpipe/src/lib/application/internal/packagescript"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

type packageScriptTarget struct {
	Name       string
	Manifest   string
	PackageDir string
	ScriptRel  string
	ScriptAbs  string
}

func discoverPackageScriptTargets(workdir, only string, selectScript func(*domain.PackageManifest) string) ([]packageScriptTarget, error) {
	only = strings.TrimSpace(only)
	if only != "" && strings.ContainsRune(only, os.PathListSeparator) {
		return nil, fmt.Errorf("package selector accepts one package name")
	}
	projectRoot, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return nil, err
	}
	cfg, err := domain.LoadDockpipeProjectConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	roots := compileconfig.WorkflowRoots(cfg, projectRoot)
	var targets []packageScriptTarget
	seenManifests := map[string]struct{}{}
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Base(path) != "package.yml" {
				return nil
			}
			manifestPath := path
			if abs, err := filepath.Abs(manifestPath); err == nil {
				manifestPath = abs
			}
			if _, ok := seenManifests[manifestPath]; ok {
				return nil
			}
			seenManifests[manifestPath] = struct{}{}
			manifest, err := domain.ParsePackageManifest(manifestPath)
			if err != nil {
				return err
			}
			dirName := filepath.Base(filepath.Dir(manifestPath))
			if only != "" && manifest.Name != only && dirName != only {
				return nil
			}
			rawScript := strings.TrimSpace(selectScript(manifest))
			if rawScript == "" {
				return nil
			}
			scriptRel := filepath.Clean(rawScript)
			packageDir := filepath.Dir(manifestPath)
			targets = append(targets, packageScriptTarget{
				Name:       manifest.Name,
				Manifest:   manifestPath,
				PackageDir: packageDir,
				ScriptRel:  scriptRel,
				ScriptAbs:  filepath.Join(packageDir, filepath.FromSlash(scriptRel)),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets, nil
}

func runPackageScriptTarget(workdir string, target packageScriptTarget, env []string, missingLabel string) error {
	if _, err := os.Stat(target.ScriptAbs); err != nil {
		return fmt.Errorf("%s %q not found (%s)", missingLabel, target.ScriptRel, target.ScriptAbs)
	}
	cmd, bashExe, err := dockpipeScriptCommand(target.ScriptAbs)
	if err != nil {
		return err
	}
	baseEnv := append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_PACKAGE_ROOT="+target.PackageDir,
		"DOCKPIPE_PACKAGE_MANIFEST="+target.Manifest,
	)
	sdkPath := filepath.Join(infrastructure.CoreDir(workdir), "assets", "scripts", "lib", "dockpipe-sdk.sh")
	if stat, err := os.Stat(sdkPath); err == nil && !stat.IsDir() {
		baseEnv = append(baseEnv, "DOCKPIPE_SDK_SH="+sdkPath)
	}
	if stateDir, err := infrastructure.StateRoot(workdir); err == nil {
		baseEnv = append(baseEnv, infrastructure.EnvStateDir+"="+stateDir)
	}
	scope := strings.TrimSpace(target.Name)
	baseEnv = append(baseEnv, infrastructure.EnvPackageID+"="+scope)
	packageState, err := infrastructure.PreparePackageStateDirWithManifests(workdir, scope, target.Manifest)
	if err != nil {
		return fmt.Errorf("prepare package state for %q: %w", target.Name, err)
	}
	baseEnv = append(baseEnv, infrastructure.EnvPackageStateDir+"="+packageState.Dir)
	ciBinding := strings.TrimSpace(os.Getenv("DOCKPIPE_CI_ARTIFACT_SCOPE"))
	packageOwner := strings.TrimSpace(target.Name)
	if ciBinding == "" {
		ciBinding = "package:" + packageOwner
	} else if resolved, err := resolveCIArtifactBinding(ciBinding, os.Getenv("DOCKPIPE_WORKFLOW_NAME"), packageOwner); err == nil {
		ciBinding = resolved
	}
	baseEnv = append(baseEnv, "DOCKPIPE_CI_ARTIFACT_SCOPE="+ciBinding)
	if rawDir, analysisDir, err := ciArtifactDirs(workdir, ciBinding); err == nil {
		if strings.TrimSpace(os.Getenv("DOCKPIPE_CI_RAW_DIR")) == "" {
			baseEnv = append(baseEnv, "DOCKPIPE_CI_RAW_DIR="+rawDir)
		}
		if strings.TrimSpace(os.Getenv("DOCKPIPE_CI_ANALYSIS_DIR")) == "" {
			baseEnv = append(baseEnv, "DOCKPIPE_CI_ANALYSIS_DIR="+analysisDir)
		}
	}
	cmd.Dir = target.PackageDir
	cmd.Env = append(baseEnv, env...)
	if bashExe != "" {
		cmd.Env = upsertEnvLocal(cmd.Env, "DOCKPIPE_HOST_BASH_BIN", bashExe)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func upsertEnvLocal(env []string, key, value string) []string {
	return packagescript.UpsertEnv(env, key, value)
}

func dockpipeScriptCommand(scriptAbs string) (*exec.Cmd, string, error) {
	return packagescript.ScriptCommand(scriptAbs)
}

func dockpipeBashShellCommand(command string) (*exec.Cmd, string, error) {
	return packagescript.BashShellCommand(command)
}

func dockpipePathForBashEnv(bashExe, p string) string {
	return packagescript.PathForBashEnv(bashExe, p)
}
