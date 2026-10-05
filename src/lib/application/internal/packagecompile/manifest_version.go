package packagecompile

import (
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"gopkg.in/yaml.v3"
)

// readVersionedCompiledManifest persists an inherited version into the generated
// copy so store assembly sees the same identity as the archive filename.
func readVersionedCompiledManifest(path, fallback string) (*domain.PackageManifest, error) {
	manifest, err := domain.ParsePackageManifest(path)
	if err != nil || strings.TrimSpace(manifest.Version) != "" {
		return manifest, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := yaml.Unmarshal(content, &fields); err != nil {
		return nil, err
	}
	fields["version"] = fallback
	content, err = yaml.Marshal(fields)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return nil, err
	}
	manifest.Version = fallback
	return manifest, nil
}
