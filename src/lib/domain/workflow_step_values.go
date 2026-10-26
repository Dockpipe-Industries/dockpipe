package domain

import "dockpipe/src/lib/domain/workflow"

type StepKind = workflow.StepKind
type StepPathScope = workflow.StepPathScope
type StepHostBuiltin = workflow.StepHostBuiltin

const (
	StepKindContainer                = workflow.StepKindContainer
	StepKindHost                     = workflow.StepKindHost
	StepPathScopeSource              = workflow.StepPathScopeSource
	StepPathScopeArtifacts           = workflow.StepPathScopeArtifacts
	StepHostBuiltinPackageBuildStore = workflow.StepHostBuiltinPackageBuildStore
	StepHostBuiltinComposeUp         = workflow.StepHostBuiltinComposeUp
	StepHostBuiltinComposeDown       = workflow.StepHostBuiltinComposeDown
	StepHostBuiltinComposePS         = workflow.StepHostBuiltinComposePS
)

func NormalizeStepKind(value string) StepKind {
	return workflow.NormalizeStepKind(value)
}

func NormalizeStepHostBuiltin(value string) StepHostBuiltin {
	return workflow.NormalizeStepHostBuiltin(value)
}
