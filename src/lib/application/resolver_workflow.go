package application

import (
	"fmt"
	"os"

	"dockpipe/src/lib/infrastructure"
)

// runPreparedResolverWorkflow enters an already admitted child, preserving the
// established package/state restoration and shared sequential output environment.
func runPreparedResolverWorkflow(
	prepared *preparedWorkflow,
	runStepsFn func(runStepsOpts) error,
	repoRoot string,
	envMap map[string]string,
	opts *CliOpts,
	cliArgs []string,
	locked map[string]bool,
	dataVol, dataDir, resolverName, templateName string,
) error {
	subWf := prepared.workflow
	name, wfRoot, wfPath := prepared.name, prepared.root, prepared.config
	wd, projectRoot := prepared.workdir, prepared.projectRoot
	prevStateDir := envMap[infrastructure.EnvStateDir]
	prevPackageID := envMap[infrastructure.EnvPackageID]
	prevPackageStateDir := envMap[infrastructure.EnvPackageStateDir]
	prevPackageRoot := envMap["DOCKPIPE_PACKAGE_ROOT"]
	prevPackageManifest := envMap["DOCKPIPE_PACKAGE_MANIFEST"]
	applyPackageManifestContext(envMap, wfRoot)
	if err := applyDockpipeStateEnv(envMap, wd, name); err != nil {
		return err
	}
	defer func() {
		envMap[infrastructure.EnvStateDir] = prevStateDir
		envMap[infrastructure.EnvPackageID] = prevPackageID
		envMap[infrastructure.EnvPackageStateDir] = prevPackageStateDir
		envMap["DOCKPIPE_PACKAGE_ROOT"] = prevPackageRoot
		envMap["DOCKPIPE_PACKAGE_MANIFEST"] = prevPackageManifest
	}()
	if err := buildWorkflowEnvInto(envMap, subWf, wfPath, wfRoot, repoRoot, opts); err != nil {
		return err
	}
	if subWf.NeedsDockerReachable() {
		if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Resolver workflow: %s\n", name)
	return runStepsFn(runStepsOpts{
		wf:             subWf,
		wfRoot:         wfRoot,
		wfConfig:       wfPath,
		repoRoot:       repoRoot,
		projectRoot:    projectRoot,
		cliArgs:        cliArgs,
		envMap:         envMap,
		locked:         locked,
		userIsolate:    "",
		userAct:        "",
		firstStepExtra: nil,
		opts:           opts,
		dataVol:        dataVol,
		dataDir:        dataDir,
		resolver:       resolverName,
		templateName:   templateName,
	})
}
