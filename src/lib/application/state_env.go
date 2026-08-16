package application

import (
	"fmt"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func preferredStateScope(parts ...string) string {
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			return part
		}
	}
	return "default"
}

func workflowStateScopeHint(opts *CliOpts, wfRoot string, wf *domain.Workflow, rtName, rsName string) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Workflow); s != "" {
			return s
		}
		if s := strings.TrimSpace(opts.WorkflowFile); s != "" {
			base := filepath.Base(filepath.Dir(s))
			if base == "." || base == string(filepath.Separator) {
				base = filepath.Base(s)
			}
			if base != "" && base != "." {
				return base
			}
		}
	}
	wfName := ""
	if wf != nil {
		wfName = strings.TrimSpace(wf.Name)
	}
	return preferredStateScope(filepath.Base(wfRoot), wfName, rsName, rtName)
}

func applyDockpipeStateEnv(envMap map[string]string, workdir, scope string) error {
	stateDir, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return err
	}
	scope = strings.TrimSpace(scope)
	packageState, err := infrastructure.PreparePackageStateDirWithManifests(workdir, scope, envMap["DOCKPIPE_PACKAGE_MANIFEST"])
	if err != nil {
		return err
	}
	envMap[infrastructure.EnvStateDir] = stateDir
	envMap[infrastructure.EnvPackageID] = scope
	envMap[infrastructure.EnvPackageStateDir] = packageState.Dir
	return nil
}

func applyPackageManifestContext(envMap map[string]string, sourceRoot string) {
	delete(envMap, "DOCKPIPE_PACKAGE_MANIFEST")
	if packageRoot := nearestPackageRoot(sourceRoot); packageRoot != "" {
		envMap["DOCKPIPE_PACKAGE_ROOT"] = packageRoot
		envMap["DOCKPIPE_PACKAGE_MANIFEST"] = filepath.Join(packageRoot, infrastructure.PackageManifestFilename)
	}
}

func applyCIArtifactEnv(envMap map[string]string, workdir string) error {
	if strings.TrimSpace(envMap["DOCKPIPE_CI_RAW_DIR"]) != "" && strings.TrimSpace(envMap["DOCKPIPE_CI_ANALYSIS_DIR"]) != "" {
		return nil
	}
	binding := strings.TrimSpace(envMap["DOCKPIPE_CI_ARTIFACT_SCOPE"])
	workflowName := strings.TrimSpace(envMap["DOCKPIPE_WORKFLOW_NAME"])
	binding, err := resolveCIArtifactBinding(binding, workflowName, strings.TrimSpace(envMap[infrastructure.EnvPackageID]))
	if err != nil {
		return err
	}
	if binding == "" {
		return nil
	}
	rawDir, analysisDir, err := ciArtifactDirs(workdir, binding)
	if err != nil {
		return err
	}
	if strings.TrimSpace(envMap["DOCKPIPE_CI_RAW_DIR"]) == "" {
		envMap["DOCKPIPE_CI_RAW_DIR"] = rawDir
	}
	if strings.TrimSpace(envMap["DOCKPIPE_CI_ANALYSIS_DIR"]) == "" {
		envMap["DOCKPIPE_CI_ANALYSIS_DIR"] = analysisDir
	}
	return nil
}

func resolveCIArtifactBinding(binding, workflowName, packageID string) (string, error) {
	binding = strings.TrimSpace(binding)
	workflowName = strings.TrimSpace(workflowName)
	packageID = strings.TrimSpace(packageID)
	if binding == "" && workflowName != "" {
		return "workflow:" + workflowName, nil
	}
	switch binding {
	case "":
		return "", nil
	case "workflow":
		if workflowName == "" {
			return "", fmt.Errorf("DOCKPIPE_CI_ARTIFACT_SCOPE=workflow requires DOCKPIPE_WORKFLOW_NAME")
		}
		return "workflow:" + workflowName, nil
	case "package":
		if packageID == "" {
			return "", fmt.Errorf("DOCKPIPE_CI_ARTIFACT_SCOPE=package requires %s", infrastructure.EnvPackageID)
		}
		return "package:" + packageID, nil
	default:
		return binding, nil
	}
}

func applyWorkflowArtifactEnv(envMap map[string]string, workdir, workflowName string) error {
	sourceRoot := strings.TrimSpace(workdir)
	if sourceRoot == "" {
		return nil
	}
	sourceRoot, err := filepath.Abs(filepath.Clean(sourceRoot))
	if err != nil {
		return err
	}
	artifactRoot := strings.TrimSpace(envMap["DOCKPIPE_ARTIFACT_ROOT"])
	if artifactRoot == "" {
		artifactRoot, err = workflowArtifactRoot(sourceRoot, workflowName)
		if err != nil {
			return err
		}
		envMap["DOCKPIPE_ARTIFACT_ROOT"] = artifactRoot
	}
	if strings.TrimSpace(envMap[infrastructure.EnvDockpipeEventLog]) == "" {
		envMap[infrastructure.EnvDockpipeEventLog] = filepath.Join(artifactRoot, "events.jsonl")
	}
	if strings.TrimSpace(envMap[infrastructure.EnvDockpipeEventIndex]) == "" {
		envMap[infrastructure.EnvDockpipeEventIndex] = filepath.Join(artifactRoot, "events-index.json")
	}
	if strings.TrimSpace(envMap["DOCKPIPE_SOURCE_ROOT"]) == "" {
		envMap["DOCKPIPE_SOURCE_ROOT"] = sourceRoot
	}
	return nil
}

func workflowArtifactRoot(workdir, workflowName string) (string, error) {
	stateDir, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return "", err
	}
	scope := sanitizeWorkflowStateScope(workflowName)
	return filepath.Join(stateDir, "workflows", scope, "artifacts"), nil
}

func ciArtifactDirs(workdir, binding string) (string, string, error) {
	binding = strings.TrimSpace(binding)
	if strings.HasPrefix(binding, "package:") {
		scope := strings.TrimSpace(strings.TrimPrefix(binding, "package:"))
		if scope == "" {
			return "", "", fmt.Errorf("CI artifact package binding requires a package name")
		}
		root, err := infrastructure.PackageRuntimeDir(workdir, scope)
		if err != nil {
			return "", "", err
		}
		return filepath.Join(root, "ci", "raw"), filepath.Join(root, "ci", "analysis"), nil
	}
	workflowName := binding
	if strings.HasPrefix(binding, "workflow:") {
		workflowName = strings.TrimSpace(strings.TrimPrefix(binding, "workflow:"))
	}
	if workflowName == "" {
		return "", "", fmt.Errorf("CI artifact workflow binding requires a workflow name")
	}
	root, err := workflowArtifactRoot(workdir, workflowName)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(root, "ci-raw"), filepath.Join(root, "ci-analysis"), nil
}

func sanitizeWorkflowStateScope(scope string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(scope) {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}
