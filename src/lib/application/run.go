package application

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

var (
	repoRootAppFn          = infrastructure.RepoRoot
	loadWorkflowAppFn      = infrastructure.LoadWorkflow
	loadResolverFileAppFn  = infrastructure.LoadResolverFile
	templateBuildAppFn     = infrastructure.ResolveTemplateBuild
	maybeVersionTagAppFn   = infrastructure.MaybeVersionTag
	resolveActionPathFn    = infrastructure.ResolveActionPath
	sourceHostScriptAppFn  = infrastructure.SourceHostScript
	runHostScriptAppFn     = infrastructure.RunHostScript
	dockerBuildAppFn       = infrastructure.DockerBuild
	dockerImageExistsAppFn = infrastructure.DockerImageExists
	dockerPullAppFn        = infrastructure.DockerPull
	runContainerAppFn      = infrastructure.RunContainer
	commitOnHostAppFn      = infrastructure.CommitOnHostWithResult
	resolvePreScriptAppFn  = infrastructure.ResolvePreScriptPath
	resolveWorkflowAppFn   = infrastructure.ResolveWorkflowScript
	isBundledCommitAppFn   = infrastructure.IsBundledCommitWorktree
	runStepsAppFn          = runSteps
	osExitAppFn            = os.Exit
)

func preScriptsIncludeCloneWorktree(paths []string) bool {
	for _, p := range paths {
		if strings.HasSuffix(p, "clone-worktree.sh") {
			return true
		}
	}
	return false
}

// setUserRepoRootForWorktree sets DOCKPIPE_USER_REPO_ROOT when cwd/--workdir is a clone whose
// origin matches repoURL, so clone-worktree.sh can use git worktree add from that checkout
// (current HEAD; uncommitted changes copied to worktree by default, or stash if DOCKPIPE_STASH_UNCOMMITTED=1) instead of a mirror from origin/HEAD.
func setUserRepoRootForWorktree(env map[string]string, opts *CliOpts, repoURL string) {
	gitDir, err := os.Getwd()
	if err != nil {
		return
	}
	if opts.Workdir != "" {
		gitDir = opts.Workdir
	}
	top, err := infrastructure.GitTopLevel(gitDir)
	if err != nil {
		return
	}
	origin, err := infrastructure.GitRemoteGetURL(gitDir, "origin")
	if err != nil {
		return
	}
	if !infrastructure.RepoURLsEquivalent(origin, repoURL) {
		return
	}
	env["DOCKPIPE_USER_REPO_ROOT"] = top
	infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
		Unit:       "run.worktree.base",
		Status:     infrastructure.OperationStatusDone,
		DurationMs: 0,
		IDs: map[string]string{
			"base":      filepath.ToSlash(top),
			"repo_mode": "same_origin",
		},
	})
}

func executeWorkflowRun(argv []string, baseEnviron []string, services runExecutionServices) error {

	repoRoot, err := repoRootAppFn()
	if err != nil {
		return err
	}

	rest, opts, err := ParseFlags(repoRoot, argv)
	if err != nil {
		return err
	}
	if opts.Help {
		printUsage()
		return nil
	}
	if opts.WorkflowsDir != "" {
		infrastructure.SetWorkflowsDirForProcess(opts.WorkflowsDir)
		defer infrastructure.SetWorkflowsDirForProcess("")
	}
	selection, err := selectWorkflow(workflowSelectionRequest{
		repoRoot:          repoRoot,
		effectiveWorkdir:  effectiveWorkdirForWorkflowOpts(opts),
		workflowName:      opts.Workflow,
		workflowFile:      opts.WorkflowFile,
		packageName:       opts.Package,
		compileDepsWanted: compileDepsWanted(opts),
		force:             opts.Force,
	}, loadWorkflowAppFn)
	if err != nil {
		return err
	}
	wf, wfRoot, wfConfig := selection.workflow, selection.root, selection.config
	stepsMode := selection.stepsMode
	if selection.editorWorkdirOverride != "" {
		opts.Workdir = selection.editorWorkdirOverride
	}
	if selection.editorLaunchWithoutDebugger {
		fmt.Fprintln(os.Stderr, "[dockpipe] VS Code launch: running without a debugger")
	}

	effWd := effectiveWorkdirForWorkflowOpts(opts)
	projectRoot := effWd
	if ap, err := filepath.Abs(effWd); err == nil {
		projectRoot = ap
	}
	if wf != nil {
		if err := domain.ValidateLoadedWorkflow(wf); err != nil {
			return err
		}
		if err := infrastructure.CheckWorkflowResolverScriptDependencies(effWd, repoRoot, wf, wfRoot, wfConfig); err != nil {
			return err
		}
		if err := infrastructure.CheckWorkflowPackageRequiresCapabilities(effWd, repoRoot, wfRoot, wfConfig); err != nil {
			return err
		}
		if err := checkWorkflowHostDependencies(wf, wfRoot, wfConfig, opts); err != nil {
			return err
		}
	}

	var gitSession *infrastructure.GitSession
	if wf != nil && !wf.Workspace.IsEmpty() {
		session, err := createWorkflowGitSession(wf, effWd, wfRoot, opts)
		if err != nil {
			return err
		}
		gitSession = session
		opts.Workdir = session.Storage.Workspace
		effWd = effectiveWorkdirForWorkflowOpts(opts)
		projectRoot = effWd
		if ap, err := filepath.Abs(effWd); err == nil {
			projectRoot = ap
		}
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "run.workspace",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs: map[string]string{
				"workspace": session.WorkspaceID,
				"session":   session.SessionID,
				"branch":    session.Repo.SessionRef,
				"mode":      session.Storage.Mode,
				"result":    "prepared",
			},
		})
	}

	session := newRunSession(baseEnviron, services)
	envMap := session.env
	workflowRunID, identityErr := infrastructure.NewWorkflowRunID()
	if identityErr != nil {
		return identityErr
	}
	envMap["DOCKPIPE_WORKFLOW_RUN_ID"] = workflowRunID
	if gitSession != nil {
		envMap["DOCKPIPE_WORKSPACE_ID"] = gitSession.WorkspaceID
		envMap["DOCKPIPE_SESSION_ID"] = gitSession.SessionID
		envMap["DOCKPIPE_SESSION_BRANCH"] = gitSession.Repo.SessionRef
		envMap["DOCKPIPE_SESSION_WORKSPACE"] = gitSession.Storage.Workspace
		if strings.TrimSpace(gitSession.Storage.Volume) != "" {
			envMap["DOCKPIPE_SESSION_VOLUME"] = gitSession.Storage.Volume
			if domain.NormalizeWorkspaceStorageBackend(gitSession.Storage.Backend) == domain.WorkspaceStorageBackendDockerVolume {
				envMap["DOCKPIPE_SESSION_VOLUME_AUTHORITATIVE"] = "1"
			}
		}
		envMap["DOCKPIPE_AGENT_GIT_POLICY"] = "runtime-owned"
	}
	if len(rest) > 0 {
		if b, err := json.Marshal(rest); err == nil {
			envMap["DOCKPIPE_ARGS_JSON"] = string(b)
		}
		envMap["DOCKPIPE_ARGS"] = shellJoinArgs(rest)
	}
	if wf != nil {
		if err := buildWorkflowEnvInto(envMap, wf, wfConfig, wfRoot, repoRoot, opts); err != nil {
			return err
		}
		wn := strings.TrimSpace(wf.Name)
		if wn == "" {
			wn = strings.TrimSpace(opts.Workflow)
		}
		if wn != "" {
			domain.MergeIfUnset(envMap, map[string]string{"DOCKPIPE_WORKFLOW_NAME": wn})
		}
	}
	mergePromptSafetyCLIIntoEnv(envMap, opts)
	if err := mergeTerraformCLIIntoEnv(envMap, opts); err != nil {
		return fmt.Errorf("terraform flags: %w", err)
	}
	if stepsMode {
		MergeExtraEnvCLIIntoSteps(envMap, opts.ExtraEnvLines, opts.VarOverrides)
		mergeVSCodeInputEnv(envMap, opts.ExtraEnvLines, opts.VarOverrides)
	}
	// Explicit workdir remains authoritative after setup exports are merged.
	if opts.Workdir != "" {
		envMap["DOCKPIPE_WORKDIR"] = opts.Workdir
	}
	if strings.TrimSpace(envMap["DOCKPIPE_BIN"]) == "" {
		if dp, derr := resolveDockpipeBinForChildProcess(effWd); derr == nil && strings.TrimSpace(dp) != "" {
			envMap["DOCKPIPE_BIN"] = dp
		}
	}
	if dp := strings.TrimSpace(envMap["DOCKPIPE_BIN"]); dp != "" {
		if filepath.IsAbs(dp) {
			dpDir := filepath.Dir(dp)
			envMap["PATH"] = prependPATHDir(envMap["PATH"], dpDir)
		}
	}

	effStrat := EffectiveStrategyName(opts, wf)
	var stratBeforeAbs, stratAfterAbs []string
	strategyHandlesCommit := false
	if effStrat != "" {
		if err := ValidateStrategyAllowlist(wf, effStrat); err != nil {
			return err
		}
		sa, _, err := LoadStrategyAssignments(repoRoot, wfRoot, effStrat)
		if err != nil {
			return err
		}
		stratBeforeAbs = ResolveStrategyScriptPaths(sa.Before, wfRoot, repoRoot, projectRoot)
		stratAfterAbs = ResolveStrategyScriptPaths(sa.After, wfRoot, repoRoot, projectRoot)
		strategyHandlesCommit = StrategyAfterHandlesBundledCommit(stratAfterAbs, repoRoot)
		if wf != nil {
			if err := ValidateNoDuplicateClone(wf, wfRoot, repoRoot, projectRoot, effStrat == "worktree", stratBeforeAbs); err != nil {
				return err
			}
		}
	}
	if effStrat == "commit" && opts.RepoURL != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] warning: --repo set with strategy commit (no clone; strategy only commits after success)\n")
	}

	rtName := EffectiveRuntimeProfileName(opts, wf, stepsMode)
	rsName := EffectiveResolverProfileName(opts, wf, stepsMode)
	rtName, rsName, err = applyWorkflowCapabilityIsolation(effWd, repoRoot, wf, rtName, rsName)
	if err != nil {
		return err
	}
	rtName = infrastructure.NormalizeRuntimeProfileName(rtName)
	if rtName == "" && rsName == "" {
		if leg := EffectiveLegacyIsolateName(wf); leg != "" {
			rtName, rsName = leg, leg
		}
	}
	applyPackageManifestContext(envMap, wfRoot)
	if err := applyDockpipeStateEnv(envMap, effWd, workflowStateScopeHint(opts, wfRoot, wf, rtName, rsName)); err != nil {
		return err
	}
	if err := applyCIArtifactEnv(envMap, effWd); err != nil {
		return err
	}
	if err := applyWorkflowArtifactEnv(envMap, effWd, workflowStateScopeHint(opts, wfRoot, wf, rtName, rsName)); err != nil {
		return err
	}
	restoreEventLogEnv := applyProcessEventLogEnv(envMap)
	defer restoreEventLogEnv()
	profileLabel := ProfileLabelForEnv(rtName, rsName)
	if rtName != "" {
		if err := ValidateRuntimeAllowlist(wf, rtName); err != nil {
			return err
		}
	}

	templateName := ""
	var preFromResolver, actFromResolver string
	hostIsolate := ""
	resolverWorkflow := ""
	isolationProfileEnv := map[string]string(nil)
	var resolverAssignments *domain.ResolverAssignments
	var resolverEnvHint string
	if rtName != "" || rsName != "" {
		rm, err := loadMergedIsolationProfile(repoRoot, projectRoot, rtName, rsName)
		if err != nil {
			return fmt.Errorf("isolation profile: %w", err)
		}
		isolationProfileEnv = rm
		ra := domain.FromResolverMap(rm)
		resolverAssignments = &ra
		resolverEnvHint = ra.EnvHint
		if rtName != "" && rsName != "" {
		}
		profileIDs := map[string]string{"profile": profileLabel}
		if strings.TrimSpace(rtName) != "" {
			profileIDs["runtime"] = rtName
		}
		if strings.TrimSpace(rsName) != "" {
			profileIDs["resolver"] = rsName
		}
		if rk := strings.TrimSpace(ra.RuntimeKind); rk != "" {
			profileIDs["runtime_kind"] = rk
		}
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "run.profile",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs:        profileIDs,
		})
		templateName = ra.Template
		hostIsolate = strings.TrimSpace(ra.HostIsolate)
		resolverWorkflow = strings.TrimSpace(ra.Workflow)
		if resolverWorkflow != "" && hostIsolate != "" {
			return fmt.Errorf("profile %q: set only one of DOCKPIPE_RUNTIME_WORKFLOW / DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RUNTIME_HOST_SCRIPT / DOCKPIPE_RESOLVER_HOST_ISOLATE", profileLabel)
		}
		preFromResolver = ra.PreScript
		actFromResolver = ra.Action
		if ra.Experimental {
			fmt.Fprintf(os.Stderr, "[dockpipe] Profile %q is experimental — see its package documentation.\n", profileLabel)
		}
		if opts.Workflow == "" && opts.WorkflowFile == "" {
			if len(opts.PreScripts) == 0 && preFromResolver != "" {
				opts.PreScripts = []string{filepath.Join(repoRoot, preFromResolver)}
			}
			if opts.Action == "" && actFromResolver != "" {
				opts.Action = filepath.Join(repoRoot, actFromResolver)
			}
		}
	}

	var preparedResolver *preparedWorkflow
	if !stepsMode && resolverWorkflow != "" {
		preparedResolver, err = prepareResolverWorkflow(loadWorkflowAppFn, resolverWorkflow, repoRoot, opts)
		if err != nil {
			return err
		}
	}

	// Banner after resolver: workflow / host-isolate resolvers may omit argv after --.
	if !opts.Detach && (stepsMode || (opts.SeenDash && (len(rest) > 0 || hostIsolate != "" || resolverWorkflow != ""))) {
		infrastructure.PrintLaunchBanner(os.Stdout, os.Stderr)
	}
	if wf != nil && (opts.Workflow != "" || opts.WorkflowFile != "") {
		disp := strings.TrimSpace(wf.Name)
		if disp == "" {
			if opts.Workflow != "" {
				disp = opts.Workflow
			} else {
				disp = filepath.Base(opts.WorkflowFile)
			}
		}
		stepCount := 0
		if stepsMode {
			stepCount = len(wf.Steps)
		}
		fmt.Fprint(os.Stderr, workflowTaskLines(disp, wf.Description, stepCount))
	}

	locked := lockedKeys(opts.VarOverrides)

	if !stepsMode {
		if wf != nil {
			cliPre := opts.PreScripts
			opts.PreScripts = nil
			opts.PreScripts = append(opts.PreScripts, stratBeforeAbs...)
			if len(cliPre) == 0 && len(wf.Run) > 0 {
				for _, r := range wf.Run {
					opts.PreScripts = append(opts.PreScripts, resolveWorkflowAppFn(r, wfRoot, repoRoot, projectRoot))
				}
			}
			opts.PreScripts = append(opts.PreScripts, cliPre...)
			if opts.Action == "" {
				act := wf.Act
				if act == "" {
					act = wf.Action
				}
				if act != "" && !(strategyHandlesCommit && ActWouldBeBundledCommit(act, wfRoot, repoRoot, projectRoot)) {
					opts.Action = resolveWorkflowAppFn(act, wfRoot, repoRoot, projectRoot)
				}
			}
		} else {
			opts.PreScripts = append(stratBeforeAbs, opts.PreScripts...)
		}
	}

	// worktree strategy flow: clone-worktree.sh needs DOCKPIPE_REPO_URL. Without --repo we infer
	// origin from the current checkout (or --workdir) so workflow runs don't silently skip.
	if !stepsMode && opts.RepoURL == "" && preScriptsIncludeCloneWorktree(opts.PreScripts) {
		gitDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot infer --repo (getwd): %w", err)
		}
		if opts.Workdir != "" {
			gitDir = opts.Workdir
		}
		u, err := infrastructure.GitRemoteGetURL(gitDir, "origin")
		if err != nil {
			return fmt.Errorf("workflow run includes clone-worktree.sh but --repo was not set and origin URL could not be read from %s: %w\n(hint: pass --repo <url> or run from a git clone with `git remote add origin ...`)", gitDir, err)
		}
		opts.RepoURL = u
		fmt.Fprintf(os.Stderr, "[dockpipe] Repo: using origin from %s\n", gitDir)
	}

	userIso := opts.Isolate
	userAct := opts.Action

	var image, buildDir, buildCtx string
	effectiveTemplate := templateName
	if effectiveTemplate == "" && (hostIsolate != "" || resolverWorkflow != "") {
		effectiveTemplate = profileLabel
	}
	commitOnHost := false
	actionForContainer := opts.Action

	if !stepsMode {
		if hostIsolate != "" || resolverWorkflow != "" {
			// Host script or embedded workflow replaces docker isolate (e.g. cursor-dev / vscode templates).
			image, buildDir, buildCtx = "", "", ""
		} else if opts.Isolate != "" {
			if im, dir, ctx, ok, err := templateBuildForRun(repoRoot, projectRoot, opts.Isolate); err != nil {
				return err
			} else if ok {
				effectiveTemplate = opts.Isolate
				image, buildDir, buildCtx = im, dir, ctx
			} else {
				image = opts.Isolate
			}
		} else if templateName != "" {
			effectiveTemplate = templateName
			if im, dir, ctx, ok, err := templateBuildForRun(repoRoot, projectRoot, templateName); err != nil {
				return err
			} else if ok {
				image, buildDir, buildCtx = im, dir, ctx
			}
		}
		if hostIsolate == "" && resolverWorkflow == "" {
			if image == "" {
				image, buildDir, buildCtx, _, err = infrastructure.BuiltinContainerBuild(firstNonEmpty(projectRoot, repoRoot), "base-dev")
				if err != nil {
					return err
				}
			}
			image = maybeVersionTagAppFn(repoRoot, image)
		}

		cwd, _ := os.Getwd()
		if opts.Action != "" {
			ap, err := resolveActionPathFn(opts.Action, repoRoot, cwd, projectRoot)
			if err != nil {
				return err
			}
			if _, err := os.Stat(ap); err != nil {
				return fmt.Errorf("action script not found: %s", ap)
			}
			opts.Action = ap
			actionForContainer = ap
			if isBundledCommitAppFn(ap, repoRoot) {
				if strategyHandlesCommit {
					return fmt.Errorf("strategy %q already runs commit in its after hook; omit --act / workflow act that points at commit-worktree.sh", effStrat)
				}
				commitOnHost = true
				actionForContainer = ""
				applyBranchPrefix(envMap, profileLabel, effectiveTemplate)
				mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
			}
		}
	}
	compiledWorkflowManifest, err := runtimepolicy.LoadCompiledRuntimeManifestForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return err
	}
	image, buildDir, buildCtx = applyCompiledImageSelectionInputs(repoRoot, wfRoot, compiledWorkflowManifest, image, buildDir, buildCtx)

	dataVol := opts.DataVolume
	dataDir := opts.DataDir
	if opts.NoData {
		dataVol, dataDir = "", ""
	} else if dataDir == "" && dataVol == "" {
		dataVol = "dockpipe-data"
	}

	if opts.RepoURL != "" && opts.RepoBranch == "" {
		prefix := envMap["DOCKPIPE_BRANCH_PREFIX"]
		if prefix == "" {
			prefix = profileLabel
		}
		if prefix == "" {
			prefix = domain.BranchPrefixForTemplate(effectiveTemplate)
		}
		wb := opts.WorkBranch
		if wb == "" {
			slug, err := domain.RandomWorkBranchSlug()
			if err != nil {
				return fmt.Errorf("generate work branch name: %w", err)
			}
			wb = prefix + "/" + slug
		}
		opts.RepoBranch = wb
		envMap["DOCKPIPE_REPO_BRANCH"] = wb
		fmt.Fprintf(os.Stderr, "[dockpipe] Branch: %s (new)\n", wb)
	}

	if opts.RepoURL != "" && len(opts.PreScripts) == 0 && !stepsMode {
		if dataDir == "" {
			home, _ := os.UserHomeDir()
			if home == "" {
				home = "/tmp"
			}
			dataDir = filepath.Join(home, ".dockpipe")
			fmt.Fprintf(os.Stderr, "[dockpipe] Data dir for worktrees: %s (override with --data-dir)\n", dataDir)
			dataVol = ""
		}
		cloneHook, err := infrastructure.LegacyGitHookPath(projectRoot, "clone-worktree")
		if err != nil {
			return fmt.Errorf("prepare repository clone hook: %w", err)
		}
		opts.PreScripts = []string{cloneHook}
	}

	// Drop inherited DOCKPIPE_WORKDIR before pre-scripts: a leftover export (or workflow .env)
	// would be passed into bash and survive env -0 if clone-worktree ever exited without exporting.
	if !stepsMode && preScriptsIncludeCloneWorktree(opts.PreScripts) {
		if opts.RepoURL != "" {
			setUserRepoRootForWorktree(envMap, opts, opts.RepoURL)
		}
		delete(envMap, "DOCKPIPE_WORKDIR")
		if opts.Workdir != "" {
			envMap["DOCKPIPE_WORKDIR"] = opts.Workdir
		}
	}

	session.launchOverrides = make(map[string]string)
	// Initial command-only values are not implicitly exported to step workflows.
	session.launchOverrides["DOCKPIPE_REPO_ROOT"] = repoRoot
	if opts.RepoURL != "" {
		session.launchOverrides["DOCKPIPE_REPO_URL"] = opts.RepoURL
	}
	if opts.RepoBranch != "" {
		session.launchOverrides["DOCKPIPE_REPO_BRANCH"] = opts.RepoBranch
	}
	if dataDir != "" {
		session.launchOverrides["DOCKPIPE_DATA_DIR"] = dataDir
	}
	if rsName != "" {
		session.launchOverrides["RESOLVER"] = rsName
	}
	if rtName != "" {
		session.launchOverrides["RUNTIME"] = rtName
	}
	if rsName == "" && rtName == "" && profileLabel != "" {
		session.launchOverrides["RESOLVER"] = profileLabel
		session.launchOverrides["RUNTIME"] = profileLabel
	}
	if templateName != "" {
		session.launchOverrides["TEMPLATE"] = templateName
	}
	if opts.Workdir != "" {
		session.launchOverrides["DOCKPIPE_WORKDIR"] = opts.Workdir
	}

	var firstStepExtra []string
	if stepsMode {
		for _, p := range opts.PreScripts {
			firstStepExtra = append(firstStepExtra, resolvePreScriptAppFn(p, repoRoot, projectRoot))
		}
		opts.PreScripts = nil
	} else {
		resolvedPre := make([]string, 0, len(opts.PreScripts))
		for _, p := range opts.PreScripts {
			resolvedPre = append(resolvedPre, resolvePreScriptAppFn(p, repoRoot, projectRoot))
		}
		opts.PreScripts = resolvedPre
	}

	needsHostGit := opts.RepoURL != "" || commitOnHost
	if !stepsMode {
		needsHostGit = needsHostGit || preScriptsIncludeCloneWorktree(opts.PreScripts)
	} else {
		for _, p := range firstStepExtra {
			if strings.HasSuffix(p, "clone-worktree.sh") {
				needsHostGit = true
				break
			}
		}
		if !needsHostGit {
			for _, p := range stratBeforeAbs {
				if strings.HasSuffix(p, "clone-worktree.sh") {
					needsHostGit = true
					break
				}
			}
		}
	}
	if needsHostGit {
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git not found in PATH — required on the host for clone/worktree/commit flows (by design; dockpipe invokes your normal git). Install git (e.g. Git for Windows on Windows, git from your OS package manager on Linux). HTTPS/SSH auth uses your existing git setup (Credential Manager, SSH keys, etc.)")
		}
	}

	if stepsMode {
		for _, p := range stratBeforeAbs {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("strategy before script not found: %s: %w", p, err)
			}
			ids := map[string]string{
				"kind":   "strategy",
				"script": filepath.Base(p),
			}
			var em map[string]string
			_, err := infrastructure.RunOperationWithResultOptions(os.Stderr, "host.setup", hostSpinnerLabel(p), ids, hostSetupOperationOptions(), func() error {
				var opErr error
				em, opErr = services.sourceScript(p, session.commandEnvironment())
				return opErr
			})
			if err != nil {
				return err
			}
			session.mergeSetupEnvironment(em)
		}
	}

	if !stepsMode {
		for _, p := range opts.PreScripts {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("pre-script not found: %s", p)
			}
			ids := map[string]string{
				"kind":   "pre",
				"script": filepath.Base(p),
			}
			var em map[string]string
			_, err := infrastructure.RunOperationWithResultOptions(os.Stderr, "host.setup", hostSpinnerLabel(p), ids, hostSetupOperationOptions(), func() error {
				var opErr error
				em, opErr = services.sourceScript(p, session.commandEnvironment())
				return opErr
			})
			if err != nil {
				return err
			}
			session.mergeSetupEnvironment(em)
		}
	}

	if commitOnHost {
		mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
		session.launchOverrides = nil
	}

	if !opts.SeenDash && !stepsMode {
		return fmt.Errorf("expected -- before command (e.g. dockpipe -- ls -la)")
	}
	if len(rest) == 0 && !stepsMode && hostIsolate == "" && resolverWorkflow == "" {
		return fmt.Errorf("no command given after --")
	}

	completion := runCompletion{
		workflow:       wf,
		gitSession:     gitSession,
		repoRoot:       repoRoot,
		workdir:        opts.Workdir,
		bundleOut:      opts.BundleOut,
		afterScripts:   stratAfterAbs,
		extraEnv:       opts.ExtraEnvLines,
		strategyCommit: strategyHandlesCommit,
		profile:        profileLabel,
		template:       effectiveTemplate,
	}
	if stepsMode {
		completion.template = templateName
		session.launchOverrides = nil
	}

	if !stepsMode && resolverWorkflow != "" {
		if err := runPreparedResolverWorkflow(preparedResolver, services.executeSteps, repoRoot, envMap, opts, rest, locked, dataVol, dataDir, profileLabel, templateName); err != nil {
			return err
		}
		workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
		if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
			logRunMount(envMap["DOCKPIPE_WORKDIR"])
		}
		return session.complete(completion, workHost, commitOnHost)
	}

	if !stepsMode && hostIsolate != "" {
		if hostDelegateRequiresDocker(isolationProfileEnv) {
			if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
				return err
			}
		}
		scriptAbs := resolveWorkflowAppFn(hostIsolate, wfRoot, repoRoot, projectRoot)
		if _, err := os.Stat(scriptAbs); err != nil {
			return fmt.Errorf("host isolate script not found: %s: %w", scriptAbs, err)
		}
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "run.host_isolate",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs: map[string]string{
				"script": hostIsolate,
			},
		})
		workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
		if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
			logRunMount(envMap["DOCKPIPE_WORKDIR"])
		}
		hostEnv := hostDelegateEnvSlice(
			envSliceWithScriptContext(session.commandEnvironment(), scriptAbs),
			isolationProfileEnv,
			shellJoinArgs(rest),
			filepath.Join(workHost, domain.DefaultOutputsEnvRel),
		)
		if err := services.hostScript(scriptAbs, hostEnv); err != nil {
			return err
		}
		dockerEnvMap := domain.EnvSliceToMap(opts.ExtraEnvLines)
		applyOutputsFile(filepath.Join(workHost, domain.DefaultOutputsEnvRel), envMap, dockerEnvMap, locked, nil, "")
		session.launchOverrides = nil
		return session.complete(completion, workHost, commitOnHost)
	}

	dockerEnvMap := domain.EnvSliceToMap(opts.ExtraEnvLines)
	workHostForEnv := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	mergeWorktreeGitDockerEnv(dockerEnvMap, workHostForEnv)
	mergeResolverAuthEnvFromHost(dockerEnvMap, envMap, resolverAssignments)
	if resolverAssignments == nil {
		mergeEnvHintKeys(dockerEnvMap, envMap, resolverEnvHint)
	}
	mergePolicyProxyEnvFromHost(dockerEnvMap, envMap)
	networkMode := infrastructure.DockerNetworkModeFromEnv(dockerEnvMap)
	if networkMode == "" {
		networkMode = strings.TrimSpace(envMap["DOCKPIPE_DOCKER_NETWORK"])
	}
	if networkMode == "" {
		networkMode = strings.TrimSpace(os.Getenv("DOCKPIPE_DOCKER_NETWORK"))
	}
	extraDocker := domain.EnvMapToSlice(dockerEnvMap)

	if stepsMode {
		if rm, err := runtimepolicy.ApplyCompiledRuntimePolicy(nil, wfConfig, wfRoot); err != nil {
			return err
		} else {
			for _, line := range runtimepolicy.CompiledRuntimePolicyLogLines(rm) {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
			}
		}
		if wf != nil && WorkflowNeedsDockerReachableResolved(wf, effWd, repoRoot) {
			if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
				return err
			}
		}
		if err := services.executeSteps(runStepsOpts{
			wf:                    wf,
			wfRoot:                wfRoot,
			wfConfig:              wfConfig,
			repoRoot:              repoRoot,
			projectRoot:           projectRoot,
			cliArgs:               rest,
			envMap:                envMap,
			locked:                locked,
			userIsolate:           userIso,
			userAct:               userAct,
			firstStepExtra:        firstStepExtra,
			opts:                  opts,
			dataVol:               dataVol,
			dataDir:               dataDir,
			resolver:              profileLabel,
			templateName:          templateName,
			strategyHandlesCommit: strategyHandlesCommit,
		}); err != nil {
			if code, ok := exitCodeFromError(err); ok {
				return &runProcessExit{code: code}
			}
			return err
		}
		return session.complete(completion, "", false)
	}

	if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
		return err
	}

	imageDecision := ""
	if buildDir != "" && buildCtx != "" {
		skipBuild, msg, err := maybeSkipDockerBuildForWorkflow(effWd, wfConfig, wfRoot, image, buildDir, buildCtx)
		if err != nil {
			return err
		}
		if skipBuild {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			imageDecision = msg
		} else {
			if strings.TrimSpace(msg) != "" {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] image: materializing image artifact (docker)…\n")
			if err := services.buildImage(image, buildDir, buildCtx); err != nil {
				return err
			}
			wfName := strings.TrimSpace(opts.Workflow)
			if wf != nil && wfName == "" {
				wfName = strings.TrimSpace(wf.Name)
			}
			policyFingerprint, _ := runtimePolicyFingerprintForRun(wfConfig, wfRoot)
			if artifact, err := buildImageArtifactManifest(repoRoot, wfName, "", templateName, image, buildDir, buildCtx, policyFingerprint, domain.ImageArtifactProvenance{Isolate: templateName, DockpipeVersion: authoredPackageVersion(repoRoot)}); err == nil {
				persistMaterializedImageArtifactForRun(effWd, image, artifact)
			} else {
				logImageArtifactOperationResult("run.image_artifact.manifest", image, err)
			}
			imageDecision = "image: materialized image artifact for current run"
		}
	} else if compiledWorkflowManifest != nil {
		msg, err := ensureCompiledRegistryImageForWorkflow(compiledWorkflowManifest)
		if err != nil {
			return err
		}
		if msg != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			imageDecision = msg
		}
	}

	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
		logRunMount(envMap["DOCKPIPE_WORKDIR"])
	}
	if strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
		if b := dockerEnvMap["DOCKPIPE_WORKTREE_BRANCH"]; b != "" {
			infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
				Unit:       "run.container.context",
				Status:     infrastructure.OperationStatusDone,
				DurationMs: 0,
				IDs: map[string]string{
					"branch": b,
				},
			})
		} else if dockerEnvMap["DOCKPIPE_WORKTREE_DETACHED"] == "1" {
			infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
				Unit:       "run.container.context",
				Status:     infrastructure.OperationStatusDone,
				DurationMs: 0,
				IDs: map[string]string{
					"detached": "1",
				},
			})
		}
	}
	workPath := opts.WorkPath
	authoredMounts := opts.ExtraMounts
	if wf != nil {
		containerHostBase := firstNonEmpty(envMap["DOCKPIPE_SOURCE_ROOT"], envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
		var mountErr error
		workHost, workPath, authoredMounts, mountErr = resolveWorkflowContainerConfig(wf.Container, containerHostBase, workHost, workPath, opts.ExtraMounts)
		if mountErr != nil {
			return mountErr
		}
	}
	authoredMounts = append(authoredMounts, resolverAuthMountSpecs(resolverAssignments, envMap)...)

	runOpts := infrastructure.RunOpts{
		Image:                   image,
		WorkdirHost:             workHost,
		WorkdirVolume:           envMap["DOCKPIPE_SESSION_VOLUME"],
		SkipVolumeWorkspaceSync: isTruthyString(envMap["DOCKPIPE_SESSION_VOLUME_AUTHORITATIVE"]),
		WorkPath:                workPath,
		ActionPath:              actionForContainer,
		ExtraMounts:             authoredMounts,
		NetworkMode:             networkMode,
		ExtraEnv:                extraDocker,
		DataVolume:              dataVol,
		DataDir:                 dataDir,
		Reinit:                  opts.Reinit,
		Force:                   opts.Force,
		Detach:                  opts.Detach,
		CommitOnHost:            commitOnHost && !strategyHandlesCommit,
		CommitMessage:           envMap["DOCKPIPE_COMMIT_MESSAGE"],
		BundleOut:               firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], opts.BundleOut),
		BundleAll:               strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1",
	}
	if rm, err := runtimepolicy.ApplyCompiledRuntimePolicy(&runOpts, wfConfig, wfRoot); err != nil {
		return err
	} else {
		for _, line := range runtimepolicy.CompiledRuntimePolicyLogLines(rm) {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
		}
		wfName := strings.TrimSpace(opts.Workflow)
		if wf != nil && wfName == "" {
			wfName = strings.TrimSpace(wf.Name)
		}
		if err := writeRunPolicyRecord(effWd, wfName, wfConfig, "", image, imageDecision, rm); err != nil {
			return err
		}
	}

	rc, err := services.container(runOpts, rest)
	if err != nil {
		return err
	}
	if rc != 0 {
		return &runProcessExit{code: rc}
	}
	return session.complete(completion, "", false)
}

func effectiveWorkdirForWorkflowOpts(opts *CliOpts) string {
	if opts != nil && strings.TrimSpace(opts.Workdir) != "" {
		return infrastructure.HostPathForGit(opts.Workdir)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return infrastructure.HostPathForGit(wd)
}

func logRunMount(hostPath string) {
	hostPath = strings.TrimSpace(hostPath)
	if hostPath == "" {
		return
	}
	infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
		Unit:       "run.mount",
		Status:     infrastructure.OperationStatusDone,
		DurationMs: 0,
		IDs: map[string]string{
			"source": filepath.ToSlash(hostPath),
			"target": "/work",
		},
	})
}

func compileDepsWanted(opts *CliOpts) bool {
	if opts == nil {
		return false
	}
	if opts.NoCompileDeps {
		return false
	}
	v := strings.TrimSpace(os.Getenv("DOCKPIPE_COMPILE_DEPS"))
	if v != "" {
		vl := strings.ToLower(v)
		return vl != "0" && vl != "false" && vl != "no" && vl != "off"
	}
	// Env unset: default on for named --workflow (not --workflow-file); same as package compile for-workflow.
	if opts.Workflow != "" && opts.WorkflowFile == "" {
		return true
	}
	return opts.CompileDeps
}
