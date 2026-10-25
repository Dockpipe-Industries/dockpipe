package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// workflowSelectionRequest captures selection inputs before execution changes the workdir.
type workflowSelectionRequest struct {
	repoRoot          string
	effectiveWorkdir  string
	workflowName      string
	workflowFile      string
	packageName       string
	compileDepsWanted bool
	force             bool
}

type workflowSelection struct {
	workflow                    *domain.Workflow
	root                        string
	config                      string
	stepsMode                   bool
	editorWorkdirOverride       string
	editorLaunchWithoutDebugger bool
}

func selectWorkflow(request workflowSelectionRequest, loadWorkflow func(string) (*domain.Workflow, error)) (workflowSelection, error) {
	if request.compileDepsWanted && request.workflowName != "" && !isVSCodeWorkflow(request.workflowName) && request.workflowFile == "" && strings.TrimSpace(request.packageName) == "" {
		effWd := request.effectiveWorkdir
		if err := compileClosureForWorkflow(effWd, request.workflowName, request.force); err != nil {
			return workflowSelection{}, err
		}
	}

	var err error
	var wf *domain.Workflow
	var wfRoot, wfConfig string
	stepsMode := false
	editorWorkdirOverride := ""
	editorLaunchWithoutDebugger := false
	if request.workflowName != "" && request.workflowFile != "" {
		return workflowSelection{}, fmt.Errorf("use only one of --workflow and --workflow-file")
	}
	if strings.TrimSpace(request.packageName) != "" && request.workflowName == "" {
		return workflowSelection{}, fmt.Errorf("--package requires --workflow")
	}
	if strings.TrimSpace(request.packageName) != "" && request.workflowFile != "" {
		return workflowSelection{}, fmt.Errorf("--package cannot be used with --workflow-file")
	}
	if request.workflowFile != "" {
		wfPath, err := filepath.Abs(request.workflowFile)
		if err != nil {
			return workflowSelection{}, fmt.Errorf("workflow file: %w", err)
		}
		if _, err := os.Stat(wfPath); err != nil {
			return workflowSelection{}, fmt.Errorf("workflow file: %w", err)
		}
		wf, err = loadWorkflow(wfPath)
		if err != nil {
			return workflowSelection{}, fmt.Errorf("parse config: %w", err)
		}
		wfRoot = filepath.Dir(wfPath)
		wfConfig = wfPath
		if len(wf.Steps) > 0 {
			stepsMode = true
		}
	} else if isVSCodeWorkflow(request.workflowName) && strings.TrimSpace(request.packageName) == "" {
		wf, wfRoot, err = loadVSCodeWorkflow(request.effectiveWorkdir, request.workflowName)
		if err != nil {
			return workflowSelection{}, fmt.Errorf("VS Code workflow: %w", err)
		}
		// The adapter produces normal host steps in memory. The editor JSON is
		// provenance, not a YAML config to parse or ship to a remote machine.
		stepsMode = true
		editorWorkdirOverride = wfRoot
		editorLaunchWithoutDebugger = wf.Category == "app"
	} else if request.workflowName != "" {
		var statErr error
		effWd := request.effectiveWorkdir
		if strings.TrimSpace(request.packageName) != "" {
			wfConfig, statErr = infrastructure.ResolvePackageWorkflowConfigPath(request.repoRoot, effWd, request.workflowName, request.packageName)
		} else {
			wfConfig, statErr = infrastructure.ResolveWorkflowConfigPathWithWorkdir(request.repoRoot, effWd, request.workflowName)
		}
		if statErr != nil {
			names, _ := infrastructure.ListWorkflowNamesInRepoRootAndPackages(request.repoRoot, effWd)
			msg := fmt.Sprintf("workflow %q not found — tried project workflows, declared compile roots, and installed or configured workflow packages", request.workflowName)
			if strings.TrimSpace(request.packageName) != "" {
				msg = fmt.Sprintf("workflow %q in package %q not found — tried unpacked packaged workflows and configured workflow roots", request.workflowName, request.packageName)
			}
			if len(names) > 0 {
				msg += fmt.Sprintf(" (available: %s)", strings.Join(names, ", "))
			}
			if strings.TrimSpace(request.packageName) != "" {
				msg += ". Ensure the nearest package.yml sets name: to the requested package."
			} else {
				msg += ". Install the workflow package or use --workflow-file path/to/config.yml."
			}
			return workflowSelection{}, fmt.Errorf("%s", msg)
		}
		if strings.HasPrefix(wfConfig, "tar://") {
			configOnDisk, extractErr := infrastructure.WorkflowConfigOnDiskPath(effWd, wfConfig)
			if extractErr != nil {
				return workflowSelection{}, extractErr
			}
			wfRoot = filepath.Dir(configOnDisk)
		} else {
			wfRoot = filepath.Dir(wfConfig)
		}
		wf, err = loadWorkflow(wfConfig)
		if err != nil {
			return workflowSelection{}, fmt.Errorf("parse config: %w", err)
		}
		if len(wf.Steps) > 0 {
			stepsMode = true
		}
	}

	return workflowSelection{
		workflow:                    wf,
		root:                        wfRoot,
		config:                      wfConfig,
		stepsMode:                   stepsMode,
		editorWorkdirOverride:       editorWorkdirOverride,
		editorLaunchWithoutDebugger: editorLaunchWithoutDebugger,
	}, nil
}
