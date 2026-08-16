package application

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func runParallelBatch(o *runStepsOpts, from, to, n int, dockerEnv map[string]string) error {
	if err := validateParallelOutputPaths(o.wf, from, to); err != nil {
		return err
	}
	if err := validateParallelNoResolverDelegate(o, from, to); err != nil {
		return err
	}
	if err := validateParallelNoHostCommit(o, from, to); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[dockpipe] --- Parallel batch steps %d–%d (non-blocking) ---\n", from+1, to)
	baseEnv := maps.Clone(o.envMap)
	baseDocker := maps.Clone(dockerEnv)

	if err := prefetchDockerBuildsForBatch(o, from, to, n, baseEnv, baseDocker); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var batchErr error
	for idx := from; idx < to; idx++ {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runParallelStepWorker(o, idx, n, from, baseEnv, baseDocker); err != nil {
				mu.Lock()
				if batchErr == nil {
					batchErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if batchErr != nil {
		return batchErr
	}

	// Merge outputs in YAML / declaration order; later step wins on key collision (see [merge] logs).
	mergeLog := newParallelMergeState()
	for idx := from; idx < to; idx++ {
		step := o.wf.Steps[idx]
		src := step.DisplayName(idx)
		applyOutputsFile(stepOutputsAbsPath(o, step, o.envMap), o.envMap, dockerEnv, o.locked, mergeLog, src)
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)
	return nil
}

func validateParallelOutputPaths(wf *domain.Workflow, from, to int) error {
	seen := make(map[string]struct{})
	for i := from; i < to; i++ {
		p := wf.Steps[i].OutputsPath()
		if _, ok := seen[p]; ok {
			return fmt.Errorf("parallel steps %d+: duplicate outputs path %q (set distinct outputs: per step)", i+1, p)
		}
		seen[p] = struct{}{}
	}
	return nil
}

func validateParallelNoResolverDelegate(o *runStepsOpts, from, to int) error {
	for i := from; i < to; i++ {
		step := o.wf.Steps[i]
		if step.UsesPackagedWorkflow() {
			return fmt.Errorf("parallel step %d: packaged workflow steps are not supported in async groups (use is_blocking: true)", i+1)
		}
		ra, effRt, effRs, err := loadStepResolver(o, step, i)
		if err != nil {
			return err
		}
		if stepUsesResolverDelegate(ra) {
			return fmt.Errorf("parallel step %d: profile %q uses DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RESOLVER_HOST_ISOLATE — not supported in async groups (use is_blocking: true)", i+1, ProfileLabelForEnv(effRt, effRs))
		}
	}
	return nil
}

func validateParallelNoHostCommit(o *runStepsOpts, from, to int) error {
	for i := from; i < to; i++ {
		step := o.wf.Steps[i]
		if step.IsHostStep() {
			continue
		}
		effAct := step.ActPath()
		if effAct == "" {
			effAct = o.userAct
		}
		if effAct == "" {
			continue
		}
		actAbs := infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot, o.projectRoot)
		if infrastructure.IsBundledCommitWorktree(actAbs, o.repoRoot) {
			return fmt.Errorf("step %d: host commit-worktree action cannot run inside an async group", i+1)
		}
	}
	return nil
}

func prefetchDockerBuildsForBatch(o *runStepsOpts, from, to, n int, baseEnv, baseDocker map[string]string) error {
	done := make(map[string]struct{})
	buildAnnounced := false
	for idx := from; idx < to; idx++ {
		step := o.wf.Steps[idx]
		if step.IsHostStep() {
			continue
		}
		if step.UsesPackagedWorkflow() {
			continue
		}
		localEnv := maps.Clone(baseEnv)
		localDocker := maps.Clone(baseDocker)
		if err := applyStepEnvOverrides(o, step, idx, localEnv, localDocker); err != nil {
			return err
		}
		ra, _, _, err := loadStepResolver(o, step, idx)
		if err != nil {
			return err
		}
		if stepUsesResolverDelegate(ra) {
			return fmt.Errorf("internal: resolver delegate in parallel batch should have been rejected")
		}
		_, runOpts, buildDir, buildCtx, rm, err := buildStepContainer(o, idx, n, step, localEnv, localDocker, ra)
		if err != nil {
			return err
		}
		if buildDir == "" || buildCtx == "" {
			if rm != nil {
				msg, err := ensureCompiledRegistryImageForStep(rm)
				if err != nil {
					return err
				}
				if msg != "" {
					fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
				}
			}
			continue
		}
		key := runOpts.Image + "\x00" + buildDir + "\x00" + buildCtx
		if _, ok := done[key]; ok {
			continue
		}
		done[key] = struct{}{}
		policyFingerprint := ""
		if rm != nil {
			policyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
		}
		skipBuild, msg, err := maybeSkipDockerBuildForStep(o.projectRoot, o.repoRoot, o.wfConfig, o.wfRoot, stepRunPolicyID(step, idx), policyFingerprint, runOpts.Image, buildDir, buildCtx)
		if err != nil {
			return err
		}
		if skipBuild {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			continue
		}
		if strings.TrimSpace(msg) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
		}
		if !buildAnnounced {
			fmt.Fprintf(os.Stderr, "[dockpipe] image: materializing image artifact (docker)…\n")
			buildAnnounced = true
		}
		if err := dockerBuildFn(runOpts.Image, buildDir, buildCtx); err != nil {
			return err
		}
		policyFingerprint = ""
		if rm != nil {
			policyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
		}
		if artifact, err := buildImageArtifactManifest(o.repoRoot, strings.TrimSpace(o.wf.Name), "", stepRunPolicyID(step, idx), runOpts.Image, buildDir, buildCtx, policyFingerprint, runStepImageArtifactProvenance(o.repoRoot, step)); err == nil {
			persistMaterializedImageArtifactForRun(o.projectRoot, runOpts.Image, artifact)
		} else {
			logImageArtifactOperationResult("run.image_artifact.manifest", runOpts.Image, err)
		}
	}
	return nil
}

func runParallelStepWorker(o *runStepsOpts, idx, n, batchStart int, baseEnv, baseDocker map[string]string) error {
	step := o.wf.Steps[idx]
	localEnv := maps.Clone(baseEnv)
	localDocker := maps.Clone(baseDocker)

	if err := applyStepEnvOverrides(o, step, idx, localEnv, localDocker); err != nil {
		return err
	}
	envSlice := domain.EnvMapToSlice(localEnv)

	var pre []string
	for _, r := range step.RunPaths() {
		pre = append(pre, infrastructure.ResolveWorkflowScript(r, o.wfRoot, o.repoRoot, o.projectRoot))
	}
	if idx == batchStart && idx == 0 {
		pre = append(pre, o.firstStepExtra...)
	}
	for _, p := range pre {
		if p == "" {
			continue
		}
		if _, err := osStatFn(p); err != nil {
			return fmt.Errorf("pre-script not found: %s", p)
		}
		ids := map[string]string{
			"parallel": fmt.Sprintf("%d", idx+1),
			"script":   filepath.Base(p),
		}
		if step.IsHostStep() {
			_, err := infrastructure.RunOperationWithResultOptions(os.Stderr, "host.setup", hostSpinnerLabel(p), ids, hostSetupOperationOptions(), func() error {
				return runHostScriptFn(p, appendUniqueEnv(envSliceWithScriptContext(envSlice, p), "DOCKPIPE_HOST_SCRIPT_SPINNER=false"))
			})
			if err != nil {
				return err
			}
			continue
		}
		var em map[string]string
		_, err := infrastructure.RunOperationWithResultOptions(os.Stderr, "host.setup", hostSpinnerLabel(p), ids, hostSetupOperationOptions(), func() error {
			var opErr error
			em, opErr = sourceHostScriptFn(p, envSliceWithScriptContext(envSlice, p))
			return opErr
		})
		if err != nil {
			return err
		}
		for k, v := range em {
			localEnv[k] = v
		}
		envSlice = domain.EnvMapToSlice(localEnv)
	}

	if step.UsesPackagedWorkflow() {
		return fmt.Errorf("parallel step %d: packaged workflow steps are not supported in async groups (use is_blocking: true)", idx+1)
	}

	if step.IsHostStep() {
		if cmd := strings.TrimSpace(step.CmdLine()); cmd != "" {
			if err := runHostCommandFn(cmd, envSlice); err != nil {
				return err
			}
		}
		return nil
	}

	ra, _, _, err := loadStepResolver(o, step, idx)
	if err != nil {
		return err
	}
	if stepUsesResolverDelegate(ra) {
		return fmt.Errorf("internal: resolver delegate in parallel batch should have been rejected")
	}
	argv, runOpts, _, _, rm, err := buildStepContainer(o, idx, n, step, localEnv, localDocker, ra)
	if err != nil {
		return err
	}
	workdir := firstNonEmpty(o.projectRoot, o.opts.Workdir, localEnv["DOCKPIPE_WORKDIR"], o.repoRoot, mustGetwd())
	if rm != nil && rm.PolicySources.StepOverride {
		for _, line := range runtimepolicy.CompiledRuntimePolicyLogLines(rm) {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
		}
	}
	if err := writeRunPolicyRecord(workdir, strings.TrimSpace(o.wf.Name), o.wfConfig, stepRunPolicyID(step, idx), runOpts.Image, "", rm); err != nil {
		return err
	}
	rc, err := runContainerFn(runOpts, argv)
	if err != nil {
		return err
	}
	if rc != 0 {
		fmt.Fprintf(os.Stderr, "[dockpipe] Parallel step %d failed with exit code %d\n", idx+1, rc)
		return &workflowExitCodeError{Code: rc}
	}
	return nil
}
