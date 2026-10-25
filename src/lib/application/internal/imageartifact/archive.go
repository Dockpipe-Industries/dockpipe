package imageartifact

import (
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// IsTarEntry identifies image artifact manifests inside a compiled workflow archive.
func IsTarEntry(entry string) bool {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	return strings.HasSuffix(entry, "/"+domain.RuntimeManifestDirName+"/"+domain.ImageArtifactFileName) ||
		(strings.Contains(entry, "/"+domain.RuntimeManifestDirName+"/"+domain.StepArtifactsDirName+"/") && strings.HasSuffix(entry, ".image-artifact.json"))
}
