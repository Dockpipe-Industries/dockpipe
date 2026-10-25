package domain

import "dockpipe/src/lib/domain/workflow"

func ValidateWorkflowTypeField(w *Workflow) error {
	return workflow.ValidateWorkflowTypeField(w)
}

func ValidateWorkflowNamespaceField(w *Workflow) error {
	return workflow.ValidateWorkflowNamespaceField(w)
}

func ValidateWorkflowVaultField(w *Workflow) error {
	return workflow.ValidateWorkflowVaultField(w)
}

func ValidateLoadedWorkflow(w *Workflow) error {
	return workflow.ValidateLoadedWorkflow(w)
}

func ValidateWorkflowSingleFlowFields(w *Workflow) error {
	return workflow.ValidateWorkflowSingleFlowFields(w)
}

func ValidateStepKind(i int, s Step) error {
	return workflow.ValidateStepKind(i, s)
}

func ValidateStepCWD(i int, s Step) error {
	return workflow.ValidateStepCWD(i, s)
}

func ValidateStepScopes(i int, s Step) error {
	return workflow.ValidateStepScopes(i, s)
}

func ValidateWorkflowComposeField(w *Workflow) error {
	return workflow.ValidateWorkflowComposeField(w)
}

func ValidateWorkflowSecurityField(w *Workflow) error {
	return workflow.ValidateWorkflowSecurityField(w)
}

func ValidateWorkflowContainerField(w *Workflow) error {
	return workflow.ValidateWorkflowContainerField(w)
}

func ValidateWorkflowWorkspaceField(w *Workflow) error {
	return workflow.ValidateWorkflowWorkspaceField(w)
}

func ValidateWorkflowWorkspaceConfig(fieldPrefix string, cfg WorkflowWorkspaceConfig) error {
	return workflow.ValidateWorkflowWorkspaceConfig(fieldPrefix, cfg)
}

func ValidateWorkflowSecurityConfig(fieldPrefix string, cfg WorkflowSecurityConfig) error {
	return workflow.ValidateWorkflowSecurityConfig(fieldPrefix, cfg)
}

func ValidateWorkflowContainerConfig(fieldPrefix string, cfg WorkflowContainerConfig) error {
	return workflow.ValidateWorkflowContainerConfig(fieldPrefix, cfg)
}

func WorkflowSecurityConfigIsEmpty(cfg WorkflowSecurityConfig) bool {
	return workflow.WorkflowSecurityConfigIsEmpty(cfg)
}

func ValidateStepPackageInvocation(i int, s Step) error {
	return workflow.ValidateStepPackageInvocation(i, s)
}

func ValidateStepSecurityField(i int, s Step) error {
	return workflow.ValidateStepSecurityField(i, s)
}

func ValidateStepContainerField(i int, s Step) error {
	return workflow.ValidateStepContainerField(i, s)
}

func ValidateStepVMField(i int, s Step) error {
	return workflow.ValidateStepVMField(i, s)
}

func ValidateStepHostBuiltin(i int, s Step) error {
	return workflow.ValidateStepHostBuiltin(i, s)
}

func ValidateStepHostShape(i int, s Step) error {
	return workflow.ValidateStepHostShape(i, s)
}

func ValidateStepComposeBuiltin(i int, s Step, w *Workflow) error {
	return workflow.ValidateStepComposeBuiltin(i, s, w)
}
