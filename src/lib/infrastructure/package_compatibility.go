package infrastructure

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/engineversion"
	"gopkg.in/yaml.v3"
)

// Match the package catalog's manifest budget for installed and local packages.
const maxCompatibilityMetadataBytes = 8 << 20

func checkPackageEngineManifest(data []byte, current string) error {
	// Read the compatibility envelope before newer package fields are validated.
	var manifest struct {
		Name    string `yaml:"name"`
		Minimum string `yaml:"min_dockpipe_version"`
	}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("package compatibility metadata: %w", err)
	}
	return domain.CheckPackageEngineVersion(manifest.Name, manifest.Minimum, current)
}

func checkPackageEngineFile(configPath string) error {
	file, err := os.Open(filepath.Join(filepath.Dir(configPath), PackageManifestFilename))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCompatibilityMetadataBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxCompatibilityMetadataBytes {
		return fmt.Errorf("package compatibility metadata exceeds size limit")
	}
	return checkPackageEngineManifest(data, engineversion.Current())
}
