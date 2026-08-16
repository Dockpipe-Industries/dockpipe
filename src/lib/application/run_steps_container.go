package application

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// buildStepContainer returns argv, docker run options, and Dockerfile build dir/context (if any).
// ra is optional assignments from a shared core runtime profile; must not describe host isolate (handled before this).
func buildStepContainer(o *runStepsOpts, i, n int, step domain.Step, envMap, dockerEnv map[string]string, ra *domain.ResolverAssignments) (
	argv []string, runOpts infrastructure.RunOpts, buildDir, buildCtx string, rm *domain.CompiledRuntimeManifest, err error,
) {
	argv, err = parseStepArgv(step.CmdLine())
	if err != nil {
		return nil, runOpts, "", "", nil, err
	}
	if i == n-1 && len(argv) == 0 && len(o.cliArgs) > 0 {
		argv = append(argv, o.cliArgs...)
	}
	if len(argv) == 0 {
		return nil, runOpts, "", "", nil, fmt.Errorf("step %d has no cmd/command and no command after --", i+1)
	}

	effIso := step.Isolate
	if effIso == "" && ra != nil && ra.Template != "" {
		effIso = ra.Template
	}
	if effIso == "" {
		effIso = o.userIsolate
	}
	if effIso == "" {
		effIso = o.wf.Isolate
	}
	if effIso == "" {
		effIso = o.resolver
	}

	effAct := step.ActPath()
	if effAct == "" && ra != nil && ra.Action != "" {
		effAct = ra.Action
	}
	if effAct == "" {
		effAct = o.userAct
	}
	var actAbs string
	if effAct != "" {
		actAbs = infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot, o.projectRoot)
	}

	var image, dockerfileDir, contextDir string
	var tmpl string
	if im, dir, ok := infrastructure.TemplateBuild(o.repoRoot, effIso); ok {
		tmpl = effIso
		image, dockerfileDir, contextDir = im, dir, o.repoRoot
	} else {
		image = effIso
	}
	if image == "" {
		image, dockerfileDir = "dockpipe-base-dev", filepath.Join(infrastructure.CoreDir(o.repoRoot), "assets", "images", "base-dev")
		contextDir = o.repoRoot
	}
	image = infrastructure.MaybeVersionTag(o.repoRoot, image)

	actionPath := actAbs
	commitOnHost := false
	if actionPath != "" {
		if _, err := osStatFn(actionPath); err != nil {
			return nil, runOpts, "", "", nil, fmt.Errorf("action script not found: %s", actionPath)
		}
		if infrastructure.IsBundledCommitWorktree(actionPath, o.repoRoot) {
			if !o.strategyHandlesCommit {
				commitOnHost = true
				actionPath = ""
				applyBranchPrefix(envMap, branchResolverName(o, step, i), tmpl)
			} else {
				actionPath = ""
			}
		}
	}

	workHost := firstNonEmpty(envMap["DOCKPIPE_SOURCE_ROOT"], envMap["DOCKPIPE_WORKDIR"], runStepsWorkdir(o))
	workPath := strings.TrimSpace(runStepsWorkPath(o))
	if strings.TrimSpace(envMap["DOCKPIPE_STEP_CWD"]) != "" && strings.TrimSpace(envMap["DOCKPIPE_STEP_CWD"]) != strings.TrimSpace(workHost) {
		if rel, relErr := filepath.Rel(workHost, envMap["DOCKPIPE_STEP_CWD"]); relErr == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
			workPath = filepath.ToSlash(rel)
		} else {
			workHost = envMap["DOCKPIPE_STEP_CWD"]
			workPath = ""
		}
	}
	containerCfg := mergeWorkflowContainerConfig(o.wf.Container, step.Container)
	containerHostBase := firstNonEmpty(envMap["DOCKPIPE_SOURCE_ROOT"], envMap["DOCKPIPE_WORKDIR"], runStepsWorkdir(o), o.projectRoot, o.repoRoot)
	authoredMounts := []string(nil)
	var mountErr error
	workHost, workPath, authoredMounts, mountErr = resolveWorkflowContainerConfig(containerCfg, containerHostBase, workHost, workPath, o.opts.ExtraMounts)
	if mountErr != nil {
		return nil, runOpts, "", "", nil, mountErr
	}
	dockerForRun := maps.Clone(dockerEnv)
	if strings.TrimSpace(dockerForRun["DOCKPIPE_BIN"]) == "" {
		if dp := strings.TrimSpace(envMap["DOCKPIPE_BIN"]); dp != "" {
			dockerForRun["DOCKPIPE_BIN"] = dp
		}
	}
	mergeResolverAuthEnvFromHost(dockerForRun, envMap, ra)
	authoredMounts = append(authoredMounts, resolverAuthMountSpecs(ra, envMap)...)
	mergePolicyProxyEnvFromHost(dockerForRun, envMap)
	mergeWorktreeGitDockerEnv(dockerForRun, workHost)
	applyContainerPathEnv(dockerForRun, workHost, stepOutputsAbsPath(o, step, envMap))
	networkMode := infrastructure.DockerNetworkModeFromEnv(dockerForRun)
	if networkMode == "" {
		networkMode = strings.TrimSpace(envMap["DOCKPIPE_DOCKER_NETWORK"])
	}
	if networkMode == "" {
		networkMode = strings.TrimSpace(os.Getenv("DOCKPIPE_DOCKER_NETWORK"))
	}

	runOpts = infrastructure.RunOpts{
		Image:                   image,
		WorkdirHost:             workHost,
		WorkdirVolume:           envMap["DOCKPIPE_SESSION_VOLUME"],
		SkipVolumeWorkspaceSync: isTruthyString(envMap["DOCKPIPE_SESSION_VOLUME_AUTHORITATIVE"]),
		WorkPath:                workPath,
		ActionPath:              actionPath,
		ExtraMounts:             authoredMounts,
		NetworkMode:             networkMode,
		ExtraEnv:                domain.EnvMapToSlice(dockerForRun),
		DataVolume:              o.dataVol,
		DataDir:                 o.dataDir,
		Reinit:                  o.opts.Reinit,
		Force:                   o.opts.Force,
		Detach:                  o.opts.Detach,
		CommitOnHost:            commitOnHost,
		CommitMessage:           envMap["DOCKPIPE_COMMIT_MESSAGE"],
		BundleOut:               firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], o.opts.BundleOut),
		BundleAll:               strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1",
	}
	rm, err = runtimepolicy.ApplyCompiledRuntimePolicyForStep(&runOpts, o.wf, o.wfConfig, o.wfRoot, step, stepRunPolicyID(step, i))
	if err != nil {
		return nil, runOpts, "", "", nil, err
	}
	image, dockerfileDir, contextDir = applyCompiledImageSelectionInputs(o.repoRoot, o.wfRoot, rm, image, dockerfileDir, contextDir)
	runOpts.Image = image
	return argv, runOpts, dockerfileDir, contextDir, rm, nil
}

func applyContainerPathEnv(env map[string]string, workHost, outputsPath string) {
	for _, key := range []string{
		"DOCKPIPE_SOURCE_ROOT",
		"DOCKPIPE_WORKDIR",
		"DOCKPIPE_ARTIFACT_ROOT",
		"DOCKPIPE_OUTPUT_ROOT",
		"DOCKPIPE_STEP_CWD",
		"DOCKPIPE_BIN",
	} {
		if v := containerWorktreePath(env[key], workHost); v != "" {
			env[key] = v
		}
	}
	if v := containerWorktreePath(outputsPath, workHost); v != "" {
		env["DOCKPIPE_STEP_OUTPUTS_FILE"] = v
	}
}

func containerWorktreePath(path, workHost string) string {
	path = strings.TrimSpace(path)
	workHost = strings.TrimSpace(workHost)
	if path == "" || workHost == "" {
		return ""
	}
	rel, err := filepath.Rel(workHost, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	if rel == "." {
		return "/work"
	}
	return filepath.ToSlash(filepath.Join("/work", rel))
}

func runStepImageArtifactProvenance(repoRoot string, step domain.Step) domain.ImageArtifactProvenance {
	p := domain.ImageArtifactProvenance{DockpipeVersion: authoredPackageVersion(repoRoot)}
	switch {
	case strings.TrimSpace(step.Isolate) != "":
		p.Isolate = strings.TrimSpace(step.Isolate)
	case strings.TrimSpace(step.Resolver) != "":
		p.Resolver = strings.TrimSpace(step.Resolver)
	case strings.TrimSpace(step.Runtime) != "":
		p.Runtime = strings.TrimSpace(step.Runtime)
	}
	return p
}

func stepRunPolicyID(step domain.Step, idx int) string {
	if s := strings.TrimSpace(step.ID); s != "" {
		return s
	}
	return fmt.Sprintf("step-%d", idx+1)
}
