package domain

import "dockpipe/src/lib/domain/workflow"

type CompiledRuntimeManifest = workflow.CompiledRuntimeManifest
type PolicySources = workflow.PolicySources
type CompiledSecurityPolicy = workflow.CompiledSecurityPolicy
type CompiledNetworkPolicy = workflow.CompiledNetworkPolicy
type CompiledFilesystemPolicy = workflow.CompiledFilesystemPolicy
type CompiledProcessPolicy = workflow.CompiledProcessPolicy
type CompiledResourceLimits = workflow.CompiledResourceLimits
type CompiledImageSelection = workflow.CompiledImageSelection
type CompiledImageBuildSpec = workflow.CompiledImageBuildSpec
type ImageArtifactManifest = workflow.ImageArtifactManifest
type ImageArtifactProvenance = workflow.ImageArtifactProvenance

const (
	RuntimeManifestKind       = workflow.RuntimeManifestKind
	ImageArtifactManifestKind = workflow.ImageArtifactManifestKind
	RuntimeManifestDirName    = workflow.RuntimeManifestDirName
	RuntimeManifestFileName   = workflow.RuntimeManifestFileName
	ImageArtifactFileName     = workflow.ImageArtifactFileName
	StepArtifactsDirName      = workflow.StepArtifactsDirName
)

func ValidateCompiledRuntimeManifest(m *CompiledRuntimeManifest) error {
	return workflow.ValidateCompiledRuntimeManifest(m)
}

func ValidateCompiledSecurityPolicy(p *CompiledSecurityPolicy) error {
	return workflow.ValidateCompiledSecurityPolicy(p)
}

func ValidateCompiledImageSelection(i *CompiledImageSelection) error {
	return workflow.ValidateCompiledImageSelection(i)
}

func ValidateImageArtifactManifest(m *ImageArtifactManifest) error {
	return workflow.ValidateImageArtifactManifest(m)
}

func FingerprintJSON(v any) (string, error) {
	return workflow.FingerprintJSON(v)
}

func RuntimeManifestPathForStep(stepID string) string {
	return workflow.RuntimeManifestPathForStep(stepID)
}

func ImageArtifactPathForStep(stepID string) string {
	return workflow.ImageArtifactPathForStep(stepID)
}
