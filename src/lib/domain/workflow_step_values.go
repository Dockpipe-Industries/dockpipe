package domain

import "strings"

// StepKind identifies where a workflow step executes.
type StepKind string

const (
	StepKindContainer StepKind = "container"
	StepKindHost      StepKind = "host"
)

// NormalizeStepKind returns the effective step kind. An omitted kind defaults
// to container; unknown values remain available for validation.
func NormalizeStepKind(value string) StepKind {
	kind := StepKind(strings.ToLower(strings.TrimSpace(value)))
	if kind == "" {
		return StepKindContainer
	}
	return kind
}

// IsValid reports whether kind names a supported workflow step execution kind.
func (kind StepKind) IsValid() bool {
	switch kind {
	case StepKindContainer, StepKindHost:
		return true
	default:
		return false
	}
}

// StepPathScope identifies the effective source or artifact root selected for
// a step's process cwd or explicit source/artifact binding.
type StepPathScope string

const (
	StepPathScopeSource    StepPathScope = "source"
	StepPathScopeArtifacts StepPathScope = "artifacts"

	stepPathScopeRepoAlias     = "repo"
	stepPathScopeWorkdirAlias  = "workdir"
	stepPathScopeArtifactAlias = "artifact"
)

func normalizeStepPathScope(value string, emptyDefault StepPathScope) StepPathScope {
	scope := StepPathScope(strings.ToLower(strings.TrimSpace(value)))
	if scope == "" {
		return emptyDefault
	}
	switch scope {
	case StepPathScopeSource, stepPathScopeRepoAlias, stepPathScopeWorkdirAlias:
		return StepPathScopeSource
	case StepPathScopeArtifacts, stepPathScopeArtifactAlias:
		return StepPathScopeArtifacts
	default:
		return scope
	}
}

// IsValid reports whether scope names an effective workflow step path root.
func (scope StepPathScope) IsValid() bool {
	switch scope {
	case StepPathScopeSource, StepPathScopeArtifacts:
		return true
	default:
		return false
	}
}

// StepHostBuiltin identifies an engine-owned action executed by a host step.
type StepHostBuiltin string

const (
	StepHostBuiltinPackageBuildStore StepHostBuiltin = "package_build_store"
	StepHostBuiltinComposeUp         StepHostBuiltin = "compose_up"
	StepHostBuiltinComposeDown       StepHostBuiltin = "compose_down"
	StepHostBuiltinComposePS         StepHostBuiltin = "compose_ps"
)

// NormalizeStepHostBuiltin trims the authored value without changing its case.
// Unknown values remain available for the existing validation and runtime errors.
func NormalizeStepHostBuiltin(value string) StepHostBuiltin {
	return StepHostBuiltin(strings.TrimSpace(value))
}

// IsValid reports whether builtin names a supported non-empty host action.
func (builtin StepHostBuiltin) IsValid() bool {
	switch builtin {
	case StepHostBuiltinPackageBuildStore,
		StepHostBuiltinComposeUp,
		StepHostBuiltinComposeDown,
		StepHostBuiltinComposePS:
		return true
	default:
		return false
	}
}

// NeedsCompose reports whether builtin delegates to the compose lifecycle.
func (builtin StepHostBuiltin) NeedsCompose() bool {
	switch builtin {
	case StepHostBuiltinComposeUp, StepHostBuiltinComposeDown, StepHostBuiltinComposePS:
		return true
	default:
		return false
	}
}

// NeedsDocker reports whether builtin requires Docker reachability.
func (builtin StepHostBuiltin) NeedsDocker() bool {
	return builtin.NeedsCompose()
}

// ComposeAction returns the unchanged compose lifecycle action suffix.
func (builtin StepHostBuiltin) ComposeAction() string {
	return strings.TrimPrefix(string(builtin), "compose_")
}
