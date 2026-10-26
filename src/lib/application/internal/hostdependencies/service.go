package hostdependencies

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"dockpipe/src/lib/application/internal/operationids"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// Service checks host requirements using dependencies captured for one invocation.
// Process environment and operation logging retain their existing host behavior.
type Service struct {
	adapters Adapters
}

// New captures host adapters for this service.
func New(adapters Adapters) Service {
	return Service{adapters: adapters}
}

type hostDependencyCandidate struct {
	Dep       domain.HostDependency
	Platforms []string
	ToolsDir  string
}

type missingHostDependency = hostDependencyCandidate

// Check admits the workflow and its owning package before execution.
func (service Service) Check(request Request) error {
	wf := request.Workflow
	wfRoot := request.WorkflowRoot
	wfConfig := request.Config
	var deps []hostDependencyCandidate
	if wf != nil {
		if err := checkHostPlatformSupported("workflow", wf.Platforms); err != nil {
			return err
		}
		deps = appendDependencyCandidates(deps, wf.Dependencies, wf.Platforms)
	}
	if pkgMeta, err := packageDependenciesForWorkflow(wfRoot, wfConfig); err != nil {
		return err
	} else {
		if err := checkHostPlatformSupported("package", pkgMeta.Platforms); err != nil {
			return err
		}
		deps = appendDependencyCandidates(deps, pkgMeta.Dependencies, pkgMeta.Platforms)
	}
	if len(deps) > 0 {
		toolsRoot := wfRoot
		if strings.TrimSpace(wfConfig) != "" {
			configPath, err := infrastructure.WorkflowConfigOnDiskPath(request.Workdir, wfConfig)
			if err != nil {
				return err
			}
			toolsRoot = filepath.Dir(configPath)
		}
		toolsDir := service.adapters.PackageToolsDir(toolsRoot)
		for i := range deps {
			deps[i].ToolsDir = toolsDir
		}
	}
	missing := service.preflight(deduplicateHostDependencies(deps))
	if len(missing) == 0 {
		return nil
	}
	if err := service.installMissing(missing, request.ApproveSystemChanges); err != nil {
		return err
	}
	missing = service.preflight(deduplicateHostDependencies(deps))
	if len(missing) == 0 {
		return nil
	}
	return missingHostDependenciesError(missing)
}

func appendDependencyCandidates(out []hostDependencyCandidate, deps domain.DependencySpec, platforms []string) []hostDependencyCandidate {
	for _, dep := range deps.Host {
		out = append(out, hostDependencyCandidate{
			Dep:       dep,
			Platforms: append([]string(nil), platforms...),
		})
	}
	return out
}

func checkHostPlatformSupported(owner string, platforms []string) error {
	if PlatformsSupportCurrent(platforms) {
		return nil
	}
	return fmt.Errorf("%s does not support host platform %q (supported: %s)", owner, CurrentPlatform(), strings.Join(platforms, ", "))
}

func missingHostDependenciesError(missing []missingHostDependency) error {
	var b strings.Builder
	b.WriteString("missing required host dependencies:")
	for _, missingDep := range missing {
		dep := missingDep.Dep
		id := hostDependencyID(dep)
		command := hostDependencyCommand(dep)
		b.WriteString("\n  - ")
		b.WriteString(id)
		if command != "" && command != id {
			b.WriteString(" (command: ")
			b.WriteString(command)
			b.WriteString(")")
		}
		if desc := strings.TrimSpace(dep.Description); desc != "" {
			b.WriteString(": ")
			b.WriteString(desc)
		}
		if hint := InstallCommandForCurrentPlatform(dep); hint != "" && PlatformsSupportCurrent(missingDep.Platforms) {
			b.WriteString("\n    install: ")
			b.WriteString(hint)
		} else if !PlatformsSupportCurrent(missingDep.Platforms) {
			b.WriteString("\n    install: no installer declared for this host platform")
		}
	}
	b.WriteString("\nInstall the missing dependency or choose a workflow/package that does not require it, then rerun.")
	return fmt.Errorf("%s", b.String())
}

func (service Service) preflight(deps []hostDependencyCandidate) []missingHostDependency {
	var missing []missingHostDependency
	for _, candidate := range deps {
		dep := candidate.Dep
		required := dep.Required == nil || *dep.Required
		ids := operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
			"platform": CurrentPlatform(),
			"required": fmt.Sprintf("%t", required),
		})
		cmd := hostDependencyCommand(dep)
		if cmd == "" {
			logDependencyResult("dependency.host.preflight", infrastructure.OperationStatusDone, operationids.Merge(ids, map[string]string{
				"result":      "skip",
				"skip_reason": "no_command",
			}), "")
			continue
		}
		path, err := service.adapters.BundledCommandPath(candidate.ToolsDir, cmd)
		if err != nil {
			path, err = service.ResolveCommandPath(cmd)
		}
		if err == nil {
			logDependencyResult("dependency.host.preflight", infrastructure.OperationStatusDone, operationids.Merge(ids, map[string]string{
				"result": "found",
				"path":   path,
			}), "")
			continue
		}
		if !required {
			logDependencyResult("dependency.host.preflight", infrastructure.OperationStatusDone, operationids.Merge(ids, map[string]string{
				"result":      "skip",
				"skip_reason": "optional_missing",
			}), "")
			continue
		}
		logDependencyResult("dependency.host.preflight", infrastructure.OperationStatusFail, operationids.Merge(ids, map[string]string{
			"result":            "missing",
			"installer_present": fmt.Sprintf("%t", strings.TrimSpace(InstallCommandForCurrentPlatform(dep)) != ""),
		}), err.Error())
		missing = append(missing, missingHostDependency(candidate))
	}
	return missing
}

// ResolveCommandPath checks the host PATH and the native Windows command fallback.
func (service Service) ResolveCommandPath(command string) (string, error) {
	if path, err := service.adapters.LookupCommand(command); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		if path, err := service.adapters.LookupPowerShell(command); err == nil && strings.TrimSpace(path) != "" {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func deduplicateHostDependencies(deps []hostDependencyCandidate) []hostDependencyCandidate {
	seen := map[string]struct{}{}
	out := make([]hostDependencyCandidate, 0, len(deps))
	for _, candidate := range deps {
		dep := candidate.Dep
		key := hostDependencyCommand(dep)
		if key == "" {
			key = hostDependencyID(dep)
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func hostDependencyID(dep domain.HostDependency) string {
	if id := strings.TrimSpace(dep.ID); id != "" {
		return id
	}
	return hostDependencyCommand(dep)
}

func hostDependencyCommand(dep domain.HostDependency) string {
	if cmd := strings.TrimSpace(dep.Command); cmd != "" {
		return cmd
	}
	return strings.TrimSpace(dep.ID)
}

func hostDependencyResultIDs(dep domain.HostDependency) map[string]string {
	ids := map[string]string{}
	if id := strings.TrimSpace(hostDependencyID(dep)); id != "" {
		ids["dependency"] = id
	}
	if command := strings.TrimSpace(hostDependencyCommand(dep)); command != "" {
		ids["command"] = command
	}
	return ids
}

func logDependencyResult(unit, status string, ids map[string]string, err string) {
	result := infrastructure.OperationResult{
		Unit:       unit,
		Status:     status,
		DurationMs: 0,
		IDs:        ids,
	}
	if strings.TrimSpace(err) != "" {
		result.Error = strings.TrimSpace(err)
	}
	infrastructure.LogOperationResult(os.Stderr, result)
}
