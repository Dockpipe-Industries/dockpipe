package runtimepolicy

import (
	"strings"

	"dockpipe/src/lib/domain"
)

func NormalizeWorkflowPolicyProfile(wf *domain.Workflow) string {
	if wf == nil {
		return "secure-default"
	}
	if p := strings.TrimSpace(wf.Security.Profile); p != "" {
		return p
	}
	return "secure-default"
}

func CompileSecurityPolicyForWorkflow(wf *domain.Workflow, profile string) (domain.CompiledSecurityPolicy, domain.PolicySources) {
	security := engineDefaultSecurityPolicy()
	baselineName, baseline := runtimeBaselineSecurityPolicy(wf)
	mergeCompiledSecurityPolicy(&security, baseline)
	mergeCompiledSecurityPolicy(&security, securityPolicyProfile(profile))
	workflowOverride := applyWorkflowSecurityOverrides(&security, wf)
	security.Preset = profile
	security.Network.Enforcement = CompiledNetworkEnforcement(security.Network.Mode, profile)
	security.Network.InternalDNS = true
	return security, domain.PolicySources{
		EngineDefault:    true,
		RuntimeBaseline:  baselineName,
		PolicyProfile:    profile,
		WorkflowOverride: workflowOverride,
	}
}

func engineDefaultSecurityPolicy() domain.CompiledSecurityPolicy {
	return domain.CompiledSecurityPolicy{
		Preset: "secure-default",
		Network: domain.CompiledNetworkPolicy{
			Mode: "offline",
		},
	}
}

func runtimeBaselineSecurityPolicy(wf *domain.Workflow) (string, domain.CompiledSecurityPolicy) {
	if wf == nil {
		return "container-default", domain.CompiledSecurityPolicy{}
	}
	if !workflowUsesContainerSecurityPolicy(wf) {
		return "host-only", domain.CompiledSecurityPolicy{}
	}
	name := firstNonEmptyString(strings.TrimSpace(wf.Runtime), strings.TrimSpace(wf.Isolate), strings.TrimSpace(wf.Resolver), "container-default")
	return name, domain.CompiledSecurityPolicy{
		FS: domain.CompiledFilesystemPolicy{
			Root:      "readonly",
			Writes:    "workspace-only",
			TempPaths: []string{"/tmp"},
		},
		Process: domain.CompiledProcessPolicy{
			User:            "non-root",
			NoNewPrivileges: true,
			DropCaps:        []string{"ALL"},
			PIDLimit:        256,
		},
	}
}

func workflowUsesContainerSecurityPolicy(wf *domain.Workflow) bool {
	if wf == nil {
		return false
	}
	if len(wf.Steps) == 0 {
		return true
	}
	return wf.AnyContainerStep()
}

func securityPolicyProfile(name string) domain.CompiledSecurityPolicy {
	switch strings.TrimSpace(name) {
	case "internet-client":
		return domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "internet"},
		}
	case "build-online":
		return domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "internet"},
			FS: domain.CompiledFilesystemPolicy{
				Root:          "writable",
				Writes:        "declared",
				WritablePaths: []string{"/tmp", "/var/tmp"},
				TempPaths:     []string{"/tmp", "/var/tmp"},
			},
		}
	case "sidecar-client":
		return domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "restricted"},
		}
	default:
		return domain.CompiledSecurityPolicy{}
	}
}

func applyWorkflowSecurityOverrides(dst *domain.CompiledSecurityPolicy, wf *domain.Workflow) bool {
	if dst == nil || wf == nil {
		return false
	}
	changed := false
	if v := strings.TrimSpace(wf.Security.Network.Mode); v != "" {
		dst.Network.Mode = v
		changed = true
	}
	if len(wf.Security.Network.Allow) > 0 {
		dst.Network.Allow = append([]string(nil), wf.Security.Network.Allow...)
		changed = true
	}
	if len(wf.Security.Network.Block) > 0 {
		dst.Network.Block = append([]string(nil), wf.Security.Network.Block...)
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Filesystem.Root); v != "" {
		dst.FS.Root = v
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Filesystem.Writes); v != "" {
		dst.FS.Writes = v
		changed = true
	}
	if len(wf.Security.Filesystem.WritablePaths) > 0 {
		dst.FS.WritablePaths = append([]string(nil), wf.Security.Filesystem.WritablePaths...)
		changed = true
	}
	if len(wf.Security.Filesystem.TempPaths) > 0 {
		dst.FS.TempPaths = append([]string(nil), wf.Security.Filesystem.TempPaths...)
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Process.User); v != "" {
		dst.Process.User = v
		changed = true
	}
	if wf.Security.Process.PIDLimit > 0 {
		dst.Process.PIDLimit = wf.Security.Process.PIDLimit
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Process.Resources.CPU); v != "" {
		dst.Process.Resources.CPU = v
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Process.Resources.Memory); v != "" {
		dst.Process.Resources.Memory = v
		changed = true
	}
	return changed
}

func ApplyStepSecurityOverrides(dst *domain.CompiledSecurityPolicy, step domain.Step) bool {
	if dst == nil {
		return false
	}
	changed := false
	if v := strings.TrimSpace(step.Security.Network.Mode); v != "" {
		dst.Network.Mode = v
		changed = true
	}
	if len(step.Security.Network.Allow) > 0 {
		dst.Network.Allow = append([]string(nil), step.Security.Network.Allow...)
		changed = true
	}
	if len(step.Security.Network.Block) > 0 {
		dst.Network.Block = append([]string(nil), step.Security.Network.Block...)
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Filesystem.Root); v != "" {
		dst.FS.Root = v
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Filesystem.Writes); v != "" {
		dst.FS.Writes = v
		changed = true
	}
	if len(step.Security.Filesystem.WritablePaths) > 0 {
		dst.FS.WritablePaths = append([]string(nil), step.Security.Filesystem.WritablePaths...)
		changed = true
	}
	if len(step.Security.Filesystem.TempPaths) > 0 {
		dst.FS.TempPaths = append([]string(nil), step.Security.Filesystem.TempPaths...)
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Process.User); v != "" {
		dst.Process.User = v
		changed = true
	}
	if step.Security.Process.PIDLimit > 0 {
		dst.Process.PIDLimit = step.Security.Process.PIDLimit
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Process.Resources.CPU); v != "" {
		dst.Process.Resources.CPU = v
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Process.Resources.Memory); v != "" {
		dst.Process.Resources.Memory = v
		changed = true
	}
	return changed
}

func mergeCompiledSecurityPolicy(dst *domain.CompiledSecurityPolicy, src domain.CompiledSecurityPolicy) {
	if dst == nil {
		return
	}
	if strings.TrimSpace(src.Preset) != "" {
		dst.Preset = strings.TrimSpace(src.Preset)
	}
	if strings.TrimSpace(src.Network.Mode) != "" {
		dst.Network.Mode = strings.TrimSpace(src.Network.Mode)
	}
	if len(src.Network.Allow) > 0 {
		dst.Network.Allow = append([]string(nil), src.Network.Allow...)
	}
	if len(src.Network.Block) > 0 {
		dst.Network.Block = append([]string(nil), src.Network.Block...)
	}
	if strings.TrimSpace(src.FS.Root) != "" {
		dst.FS.Root = strings.TrimSpace(src.FS.Root)
	}
	if strings.TrimSpace(src.FS.Writes) != "" {
		dst.FS.Writes = strings.TrimSpace(src.FS.Writes)
	}
	if len(src.FS.WritablePaths) > 0 {
		dst.FS.WritablePaths = append([]string(nil), src.FS.WritablePaths...)
	}
	if len(src.FS.TempPaths) > 0 {
		dst.FS.TempPaths = append([]string(nil), src.FS.TempPaths...)
	}
	if strings.TrimSpace(src.Process.User) != "" {
		dst.Process.User = strings.TrimSpace(src.Process.User)
	}
	if src.Process.NoNewPrivileges {
		dst.Process.NoNewPrivileges = true
	}
	if len(src.Process.DropCaps) > 0 {
		dst.Process.DropCaps = append([]string(nil), src.Process.DropCaps...)
	}
	if len(src.Process.AddCaps) > 0 {
		dst.Process.AddCaps = append([]string(nil), src.Process.AddCaps...)
	}
	if src.Process.PIDLimit > 0 {
		dst.Process.PIDLimit = src.Process.PIDLimit
	}
	if strings.TrimSpace(src.Process.Resources.CPU) != "" {
		dst.Process.Resources.CPU = strings.TrimSpace(src.Process.Resources.CPU)
	}
	if strings.TrimSpace(src.Process.Resources.Memory) != "" {
		dst.Process.Resources.Memory = strings.TrimSpace(src.Process.Resources.Memory)
	}
}

func CompiledNetworkEnforcement(mode, profile string) string {
	switch strings.TrimSpace(mode) {
	case "offline", "internet":
		return "native"
	case "allowlist", "restricted":
		if strings.TrimSpace(profile) == "sidecar-client" {
			return "proxy"
		}
		return "advisory"
	default:
		return "advisory"
	}
}

func CompiledEnforcementSummaries(rm *domain.CompiledRuntimeManifest) []string {
	if rm == nil {
		return nil
	}
	ownership := "policy ownership: engine defaults + runtime baseline + selected profile + workflow overrides"
	if rm.PolicySources.StepOverride {
		ownership += " + step overrides"
	}
	lines := []string{ownership}
	if strings.TrimSpace(rm.PolicySources.RuntimeBaseline) == "host-only" {
		lines = append(lines, "container security policy applies only to container steps; host-only steps remain outside Docker enforcement")
	}
	switch strings.TrimSpace(rm.Security.Network.Enforcement) {
	case "proxy":
		lines = append([]string{"network policy requires a proxy-backed egress layer when this workflow runs"}, lines...)
	case "advisory":
		lines = append([]string{"network policy currently compiles as advisory until full Docker egress enforcement lands"}, lines...)
	}
	return lines
}

func CompiledRuleIDs(rm *domain.CompiledRuntimeManifest) []string {
	if rm == nil {
		return nil
	}
	rules := []string{
		"security.profile." + firstNonEmptyString(strings.TrimSpace(rm.PolicyProfile), "secure-default"),
		"network.mode." + firstNonEmptyString(strings.TrimSpace(rm.Security.Network.Mode), "offline"),
	}
	if strings.TrimSpace(rm.Security.FS.Root) != "" {
		rules = append(rules, "filesystem.root."+strings.TrimSpace(rm.Security.FS.Root))
	}
	if rm.Security.Process.NoNewPrivileges {
		rules = append(rules, "process.no-new-privileges")
	}
	if len(rm.Security.Process.DropCaps) > 0 {
		rules = append(rules, "process.drop-caps")
	}
	if rm.PolicySources.WorkflowOverride {
		rules = append(rules, "security.workflow-override")
	}
	if rm.PolicySources.StepOverride {
		rules = append(rules, "security.step-override")
	}
	return rules
}
