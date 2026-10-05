package application

import (
	"dockpipe/src/lib/application/internal/imageartifact"
	"dockpipe/src/lib/application/internal/textvalue"
	"dockpipe/src/lib/domain"
)

func buildImageArtifactManifest(repoRoot, workflowName, packageName, imageKey, imageRef, buildDir, contextDir, policyFingerprint string, provenance domain.ImageArtifactProvenance) (*domain.ImageArtifactManifest, error) {
	return imageartifact.BuildManifest(imageartifact.ManifestInput{
		RepoRoot:          repoRoot,
		WorkflowName:      workflowName,
		PackageName:       packageName,
		ImageKey:          imageKey,
		ImageRef:          imageRef,
		BuildDir:          buildDir,
		ContextDir:        contextDir,
		PolicyFingerprint: policyFingerprint,
		Provenance:        provenance,
	})
}

func trimImageArtifactProvenance(p domain.ImageArtifactProvenance) domain.ImageArtifactProvenance {
	return imageartifact.NormalizeProvenance(p)
}

func marshalArtifactJSON(v any) ([]byte, error) {
	return imageartifact.MarshalJSON(v)
}

func firstNonEmptyString(values ...string) string {
	return textvalue.FirstNonEmpty(values...)
}
