package domain

import "dockpipe/src/lib/domain/workflow"

type RunSpec = workflow.RunSpec
type Workflow = workflow.Workflow
type WorkflowComposeConfig = workflow.WorkflowComposeConfig
type WorkflowImageConfig = workflow.WorkflowImageConfig
type WorkflowImagePackagesConfig = workflow.WorkflowImagePackagesConfig
type WorkflowContainerConfig = workflow.WorkflowContainerConfig
type WorkflowContainerMount = workflow.WorkflowContainerMount
type WorkflowWorkspaceConfig = workflow.WorkflowWorkspaceConfig
type WorkflowWorkspaceLifecycleConfig = workflow.WorkflowWorkspaceLifecycleConfig
type WorkflowView = workflow.WorkflowView
type WorkflowViewEntry = workflow.WorkflowViewEntry
type WorkflowViewEntryOption = workflow.WorkflowViewEntryOption
type WorkflowViewPage = workflow.WorkflowViewPage
type WorkflowViewSection = workflow.WorkflowViewSection
type WorkflowSecurityConfig = workflow.WorkflowSecurityConfig
type StepAgentConfig = workflow.StepAgentConfig
type StepAgentAccessConfig = workflow.StepAgentAccessConfig
type StepAgentModelPolicyConfig = workflow.StepAgentModelPolicyConfig
type StepAgentModelAttemptPolicy = workflow.StepAgentModelAttemptPolicy
type StepAgentModelValidatePolicy = workflow.StepAgentModelValidatePolicy
type StepAgentModelEscalationPolicy = workflow.StepAgentModelEscalationPolicy
type StepAgentModelConfig = workflow.StepAgentModelConfig
type StepAgentOrchestrationConfig = workflow.StepAgentOrchestrationConfig
type StepAgentRequestConfig = workflow.StepAgentRequestConfig
type StepAgentPlanConfig = workflow.StepAgentPlanConfig
type StepAgentConcurrencyConfig = workflow.StepAgentConcurrencyConfig
type StepAgentSharedArtifact = workflow.StepAgentSharedArtifact
type StepAgentWorkerPolicyConfig = workflow.StepAgentWorkerPolicyConfig
type StepAgentTaskConfig = workflow.StepAgentTaskConfig
type StepAgentTaskContextConfig = workflow.StepAgentTaskContextConfig
type StepAgentMaterializeOutput = workflow.StepAgentMaterializeOutput
type StepAgentMergeConfig = workflow.StepAgentMergeConfig
type StepAgentVerifyConfig = workflow.StepAgentVerifyConfig
type StepAgentApplyConfig = workflow.StepAgentApplyConfig
type StepAgentApplyOutput = workflow.StepAgentApplyOutput
type WorkflowNetworkConfig = workflow.WorkflowNetworkConfig
type WorkflowFilesystemConfig = workflow.WorkflowFilesystemConfig
type WorkflowProcessConfig = workflow.WorkflowProcessConfig
type WorkflowResourceLimits = workflow.WorkflowResourceLimits
type Step = workflow.Step
type StepScopes = workflow.StepScopes
type InputBinding = workflow.InputBinding
type StepVMConfig = workflow.StepVMConfig
type StepVMMount = workflow.StepVMMount

const (
	WorkflowTypeSecretstore = workflow.WorkflowTypeSecretstore
	DefaultOutputsEnvRel    = workflow.DefaultOutputsEnvRel
)

func ParseWorkflowYAML(data []byte) (*Workflow, error) {
	return workflow.ParseWorkflowYAML(data)
}
