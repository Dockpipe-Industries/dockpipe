// Package domain holds workflow config types and parsing — no I/O.
package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateWorkflowTypeField checks workflow_type when set (lowercase identifier, hyphens allowed).
func ValidateWorkflowTypeField(w *Workflow) error {
	if w == nil {
		return nil
	}
	t := strings.TrimSpace(w.WorkflowType)
	if t == "" {
		return nil
	}
	if !workflowTypePattern.MatchString(t) {
		return fmt.Errorf("workflow_type %q must match %s (e.g. secretstore, my-kind)", t, workflowTypePattern.String())
	}
	return nil
}

// ValidateWorkflowNamespaceField checks namespace when set (reserved words and pattern).
func ValidateWorkflowNamespaceField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateNamespace(w.Namespace)
}

// ValidateWorkflowVaultField checks workflow vault: when set, must be a supported token (see docs/runtime/vault.md).
func ValidateWorkflowVaultField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateVaultModeString(w.Vault)
}

// ValidateLoadedWorkflow checks workflow fields required for run and dockpipe workflow validate.
func ValidateLoadedWorkflow(w *Workflow) error {
	if w == nil {
		return nil
	}
	if err := ValidateWorkflowTypeField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowNamespaceField(w); err != nil {
		return err
	}
	if err := ValidatePlatformList("platforms", w.Platforms); err != nil {
		return err
	}
	if err := ValidateWorkflowVaultField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowComposeField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowSecurityField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowContainerField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowWorkspaceField(w); err != nil {
		return err
	}
	if err := ValidateDependencySpec("dependencies", w.Dependencies); err != nil {
		return err
	}
	if err := ValidateWorkflowSingleFlowFields(w); err != nil {
		return err
	}
	for i, s := range w.Steps {
		if err := validateWorkflowStep(i, s, w); err != nil {
			return err
		}
	}
	for i, s := range w.Finally {
		if err := validateWorkflowStep(i, s, w); err != nil {
			return fmt.Errorf("finally step %d: %w", i+1, err)
		}
	}
	return nil
}

func ValidateWorkflowSingleFlowFields(w *Workflow) error {
	if w == nil || (len(w.Steps) == 0 && len(w.Finally) == 0) {
		return nil
	}
	if len(w.Steps) == 0 && len(w.Finally) > 0 {
		return fmt.Errorf("workflow with finally requires at least one main step in steps")
	}
	if len(w.Run) > 0 {
		return fmt.Errorf("workflow with steps uses top-level run; move those host pre-scripts onto a step with run: or pre_script")
	}
	if strings.TrimSpace(w.Act) != "" || strings.TrimSpace(w.Action) != "" {
		return fmt.Errorf("workflow with steps: uses top-level act/action; set act/action on the specific step that needs it")
	}
	return nil
}

func validateWorkflowStep(i int, s Step, w *Workflow) error {
	if err := ValidateStepKind(i, s); err != nil {
		return err
	}
	if err := ValidateStepCWD(i, s); err != nil {
		return err
	}
	if err := ValidateStepScopes(i, s); err != nil {
		return err
	}
	if err := ValidateStepHostShape(i, s); err != nil {
		return err
	}
	if err := ValidateStepPackageInvocation(i, s); err != nil {
		return err
	}
	if err := ValidateStepSecurityField(i, s); err != nil {
		return err
	}
	if err := ValidateStepContainerField(i, s); err != nil {
		return err
	}
	if err := ValidateStepVMField(i, s); err != nil {
		return err
	}
	if err := ValidateStepHostBuiltin(i, s); err != nil {
		return err
	}
	if err := ValidateStepComposeBuiltin(i, s, w); err != nil {
		return err
	}
	return nil
}

func ValidateStepKind(i int, s Step) error {
	if s.EffectiveKind().IsValid() {
		return nil
	}
	return fmt.Errorf("step %d: kind must be host or container", i+1)
}

func ValidateStepCWD(i int, s Step) error {
	if s.EffectiveCWD().IsValid() {
		return nil
	}
	return fmt.Errorf("step %d: cwd must be source, repo, or artifacts", i+1)
}

func ValidateStepScopes(i int, s Step) error {
	for name, scope := range map[string]StepPathScope{
		"source":    s.EffectiveSourceScope(),
		"artifacts": s.EffectiveArtifactsScope(),
	} {
		if !scope.IsValid() {
			return fmt.Errorf("step %d: scopes.%s must be source, repo, or artifacts", i+1, name)
		}
	}
	return nil
}

func ValidateWorkflowComposeField(w *Workflow) error {
	if w == nil {
		return nil
	}
	c := w.Compose
	if strings.TrimSpace(c.File) == "" &&
		strings.TrimSpace(c.Project) == "" &&
		strings.TrimSpace(c.ProjectDirectory) == "" &&
		len(c.Exports) == 0 &&
		len(c.Services) == 0 {
		return nil
	}
	if strings.TrimSpace(c.File) == "" {
		return fmt.Errorf("compose.file is required when compose settings are present")
	}
	return nil
}

func ValidateWorkflowSecurityField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateWorkflowSecurityConfig("security", w.Security)
}

func ValidateWorkflowContainerField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateWorkflowContainerConfig("container", w.Container)
}

func ValidateWorkflowWorkspaceField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateWorkflowWorkspaceConfig("workspace", w.Workspace)
}

func ValidateWorkflowWorkspaceConfig(fieldPrefix string, cfg WorkflowWorkspaceConfig) error {
	if cfg.IsEmpty() {
		return nil
	}
	mode := WorkspaceMode(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = WorkspaceModeManaged
	}
	if err := validateEnum(fieldPrefix+".mode", mode, validWorkspaceModes); err != nil {
		return err
	}
	if err := validateEnum(fieldPrefix+".lifecycle.checkpoint", WorkspaceCheckpointMode(cfg.Lifecycle.Checkpoint), validWorkspaceCheckpointModes); err != nil {
		return err
	}
	if err := validateEnum(fieldPrefix+".lifecycle.publish", WorkspacePublishMode(cfg.Lifecycle.Publish), validWorkspacePublishModes); err != nil {
		return err
	}
	if err := validateEnum(fieldPrefix+".storage", WorkspaceStorage(cfg.Storage), validAuthoredWorkspaceStorages); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Lifecycle.BranchPrefix) != "" && strings.ContainsAny(cfg.Lifecycle.BranchPrefix, " \t\r\n") {
		return fmt.Errorf("%s.lifecycle.branch_prefix must not contain whitespace", fieldPrefix)
	}
	if branch := strings.TrimSpace(cfg.Lifecycle.Branch); branch != "" {
		if strings.ContainsAny(branch, " \t\r\n") {
			return fmt.Errorf("%s.lifecycle.branch must not contain whitespace", fieldPrefix)
		}
		if strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "//") {
			return fmt.Errorf("%s.lifecycle.branch must be a relative git branch name without empty path segments", fieldPrefix)
		}
	}
	return nil
}

func ValidateWorkflowSecurityConfig(fieldPrefix string, cfg WorkflowSecurityConfig) error {
	if err := validateEnum(fieldPrefix+".profile", PolicyProfile(cfg.Profile), validPolicyProfiles); err != nil {
		return err
	}
	if err := validateEnum(fieldPrefix+".network.mode", NetworkMode(cfg.Network.Mode), validNetworkModes); err != nil {
		return err
	}
	if NetworkMode(strings.TrimSpace(cfg.Network.Mode)) == NetworkModeAllowlist && len(cfg.Network.Allow) == 0 {
		return fmt.Errorf("%s.network.mode allowlist requires at least one allow rule", fieldPrefix)
	}
	if NetworkMode(strings.TrimSpace(cfg.Network.Mode)) == NetworkModeOffline && (len(cfg.Network.Allow) > 0 || len(cfg.Network.Block) > 0) {
		return fmt.Errorf("%s.network.mode offline cannot be combined with allow/block rules", fieldPrefix)
	}
	if err := validateEnum(fieldPrefix+".filesystem.root", FilesystemRootPolicy(cfg.Filesystem.Root), validFilesystemRoots); err != nil {
		return err
	}
	if err := validateEnum(fieldPrefix+".filesystem.writes", FilesystemWritePolicy(cfg.Filesystem.Writes), validFilesystemWrites); err != nil {
		return err
	}
	if err := validateEnum(fieldPrefix+".process.user", ProcessUserPolicy(cfg.Process.User), validProcessUsers); err != nil {
		return err
	}
	if cfg.Process.PIDLimit < 0 {
		return fmt.Errorf("%s.process.pid_limit must be >= 0", fieldPrefix)
	}
	return nil
}

func ValidateWorkflowContainerConfig(fieldPrefix string, cfg WorkflowContainerConfig) error {
	if cfg.IsEmpty() {
		return nil
	}
	if wp := strings.TrimSpace(cfg.WorkPath); wp != "" && (strings.HasPrefix(wp, "/") || filepath.IsAbs(wp)) {
		return fmt.Errorf("%s.work_path must be relative to the container work root", fieldPrefix)
	}
	for idx, mount := range cfg.Mounts {
		if strings.TrimSpace(mount.Host) == "" || strings.TrimSpace(mount.Guest) == "" {
			return fmt.Errorf("%s.mounts[%d] requires both host and guest", fieldPrefix, idx)
		}
		if err := validateEnum(fmt.Sprintf("%s.mounts[%d].mode", fieldPrefix, idx), ContainerMountMode(mount.Mode), validContainerMountModes); err != nil {
			return err
		}
	}
	return nil
}

func WorkflowSecurityConfigIsEmpty(cfg WorkflowSecurityConfig) bool {
	return strings.TrimSpace(cfg.Profile) == "" &&
		strings.TrimSpace(cfg.Network.Mode) == "" &&
		len(cfg.Network.Allow) == 0 &&
		len(cfg.Network.Block) == 0 &&
		strings.TrimSpace(cfg.Filesystem.Root) == "" &&
		strings.TrimSpace(cfg.Filesystem.Writes) == "" &&
		len(cfg.Filesystem.WritablePaths) == 0 &&
		len(cfg.Filesystem.TempPaths) == 0 &&
		strings.TrimSpace(cfg.Process.User) == "" &&
		cfg.Process.PIDLimit == 0 &&
		strings.TrimSpace(cfg.Process.Resources.CPU) == "" &&
		strings.TrimSpace(cfg.Process.Resources.Memory) == ""
}

func ValidateStepPackageInvocation(i int, s Step) error {
	if strings.EqualFold(strings.TrimSpace(s.Runtime), "package") {
		return fmt.Errorf("step %d: runtime: package is no longer supported; use workflow: <name> and package: <namespace>", i+1)
	}
	if !s.UsesPackagedWorkflow() {
		return nil
	}
	if strings.TrimSpace(s.WorkflowName) == "" {
		return fmt.Errorf("step %d: packaged workflow step requires workflow: <name>", i+1)
	}
	if strings.TrimSpace(s.Package) == "" {
		return fmt.Errorf("step %d: packaged workflow step requires package: <namespace>", i+1)
	}
	if strings.TrimSpace(s.Resolver) != "" {
		return fmt.Errorf("step %d: packaged workflow step uses workflow: <name>; do not also set resolver", i+1)
	}
	if strings.TrimSpace(s.Isolate) != "" {
		return fmt.Errorf("step %d: packaged workflow step uses workflow/package; do not also set isolate", i+1)
	}
	if strings.TrimSpace(s.CmdLine()) != "" {
		return fmt.Errorf("step %d: packaged workflow step uses workflow/package; do not also set cmd/command", i+1)
	}
	if strings.TrimSpace(s.ActPath()) != "" {
		return fmt.Errorf("step %d: packaged workflow step uses workflow/package; do not also set act/action", i+1)
	}
	if s.IsHostStep() {
		return fmt.Errorf("step %d: packaged workflow step cannot use kind: host", i+1)
	}
	if !WorkflowSecurityConfigIsEmpty(s.Security) {
		return fmt.Errorf("step %d: packaged workflow step uses workflow/package; do not also set security", i+1)
	}
	return nil
}

func ValidateStepSecurityField(i int, s Step) error {
	if WorkflowSecurityConfigIsEmpty(s.Security) {
		return nil
	}
	if s.IsHostStep() {
		return fmt.Errorf("step %d: kind: host step does not use security; remove security: or switch to a container step", i+1)
	}
	if s.UsesPackagedWorkflow() {
		return fmt.Errorf("step %d: packaged workflow step does not use security; keep policy inside the child workflow", i+1)
	}
	return ValidateWorkflowSecurityConfig(fmt.Sprintf("steps[%d].security", i), s.Security)
}

func ValidateStepContainerField(i int, s Step) error {
	if s.Container.IsEmpty() {
		return nil
	}
	if s.IsHostStep() {
		return fmt.Errorf("step %d: kind: host step does not use container; remove container: or switch to a container step", i+1)
	}
	if s.UsesPackagedWorkflow() {
		return fmt.Errorf("step %d: packaged workflow step does not use container; keep child container mounts inside the child workflow", i+1)
	}
	return ValidateWorkflowContainerConfig(fmt.Sprintf("steps[%d].container", i), s.Container)
}

func ValidateStepVMField(i int, s Step) error {
	if s.VM.IsEmpty() {
		return nil
	}
	if s.IsHostStep() {
		return fmt.Errorf("step %d: kind: host step does not use vm; remove vm: or switch to a VM/container step", i+1)
	}
	if s.UsesPackagedWorkflow() {
		return fmt.Errorf("step %d: packaged workflow step does not use vm; keep VM settings inside the child workflow", i+1)
	}
	if strings.TrimSpace(s.VM.HostContext) != "" && strings.TrimSpace(s.VM.GuestPath) == "" {
		return fmt.Errorf("step %d: vm.host_context requires vm.guest_path", i+1)
	}
	for mountIdx, mount := range s.VM.Mounts {
		if strings.TrimSpace(mount.Host) == "" || strings.TrimSpace(mount.Guest) == "" {
			return fmt.Errorf("step %d: vm.mounts[%d] requires both host and guest", i+1, mountIdx)
		}
	}
	if s.VM.InteractiveDebug != nil && *s.VM.InteractiveDebug &&
		s.VM.InteractiveSSH != nil && *s.VM.InteractiveSSH {
		return fmt.Errorf("step %d: vm.interactive_debug and vm.interactive_ssh are mutually exclusive", i+1)
	}
	return nil
}

// ValidateStepHostBuiltin checks host_builtin steps (see Step.HostBuiltin).
func ValidateStepHostBuiltin(i int, s Step) error {
	b := s.EffectiveHostBuiltin()
	if b == "" {
		return nil
	}
	if !s.IsHostStep() {
		return fmt.Errorf("step %d: host_builtin %q requires kind: host", i+1, b)
	}
	if s.UsesPackagedWorkflow() {
		return fmt.Errorf("step %d: host_builtin is incompatible with packaged workflow steps", i+1)
	}
	if len(s.Run) > 0 || s.PreScript != "" {
		return fmt.Errorf("step %d: host_builtin cannot be combined with run: or pre_script", i+1)
	}
	if b.IsValid() {
		return nil
	}
	return fmt.Errorf("step %d: unknown host_builtin %q (allowed: package_build_store, compose_up, compose_down, compose_ps)", i+1, string(b))
}

func ValidateStepHostShape(i int, s Step) error {
	if !s.IsHostStep() {
		return nil
	}
	if s.UsesPackagedWorkflow() {
		return nil
	}
	if strings.TrimSpace(s.Runtime) != "" {
		return fmt.Errorf("step %d: kind: host step does not use runtime; remove runtime: or switch to a container step", i+1)
	}
	if strings.TrimSpace(s.Resolver) != "" {
		return fmt.Errorf("step %d: kind: host step does not use resolver; remove resolver: or switch to a container step", i+1)
	}
	if strings.TrimSpace(s.Isolate) != "" {
		return fmt.Errorf("step %d: kind: host step does not use isolate; remove isolate: or switch to a container step", i+1)
	}
	return nil
}

func ValidateStepComposeBuiltin(i int, s Step, w *Workflow) error {
	b := s.EffectiveHostBuiltin()
	if !b.NeedsCompose() {
		return nil
	}
	if w == nil || strings.TrimSpace(w.Compose.File) == "" {
		return fmt.Errorf("step %d: host_builtin %q requires workflow compose.file", i+1, string(b))
	}
	return nil
}
