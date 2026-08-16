package domain

import "strings"

// RuntimeKind is the behavioral classification of a runtime profile.
type RuntimeKind string

// runtime.type (DOCKPIPE_RUNTIME_TYPE): three behavior classifications only — not Docker vs EC2, not tool names.
// See docs/concepts/architecture-model.md (normative).
const (
	RuntimeKindExecution = "execution" // non-interactive command/test execution
	RuntimeKindIDE       = "ide"       // interactive development environment
	RuntimeKindAgent     = "agent"     // autonomous task execution
)

// ValidRuntimeKinds lists accepted DOCKPIPE_RUNTIME_TYPE values.
var ValidRuntimeKinds = []string{
	RuntimeKindExecution,
	RuntimeKindIDE,
	RuntimeKindAgent,
}

// IsValidRuntimeKind reports whether s is one of the three runtime.type values (after trim).
func IsValidRuntimeKind(s string) bool {
	return NormalizeRuntimeKind(s).IsValid()
}

// NormalizeRuntimeKind converts a wire value into its canonical domain value.
// Unknown values remain available to callers for forward-compatible handling.
func NormalizeRuntimeKind(s string) RuntimeKind {
	return RuntimeKind(strings.TrimSpace(strings.ToLower(s)))
}

// IsValid reports whether kind is one of DockPipe's three runtime classifications.
func (kind RuntimeKind) IsValid() bool {
	switch kind {
	case RuntimeKindExecution, RuntimeKindIDE, RuntimeKindAgent:
		return true
	default:
		return false
	}
}
