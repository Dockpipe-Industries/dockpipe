package domain

import "dockpipe/src/lib/domain/workflow"

type RuntimeKind = workflow.RuntimeKind

const (
	RuntimeKindExecution = workflow.RuntimeKindExecution
	RuntimeKindIDE       = workflow.RuntimeKindIDE
	RuntimeKindAgent     = workflow.RuntimeKindAgent
)

// ValidRuntimeKinds lists accepted DOCKPIPE_RUNTIME_TYPE values.
var ValidRuntimeKinds = workflow.ValidRuntimeKinds

func IsValidRuntimeKind(s string) bool {
	return workflow.IsValidRuntimeKind(s)
}

func NormalizeRuntimeKind(s string) RuntimeKind {
	return workflow.NormalizeRuntimeKind(s)
}
