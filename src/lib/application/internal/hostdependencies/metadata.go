package hostdependencies

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

type packageDependencyMetadata struct {
	Platforms    []string
	Dependencies domain.DependencySpec
}

func packageDependenciesForWorkflow(wfRoot, wfConfig string) (packageDependencyMetadata, error) {
	if strings.HasPrefix(wfConfig, "tar://") {
		tarPath, entry, ok := infrastructure.SplitTarWorkflowURI(wfConfig)
		if !ok {
			return packageDependencyMetadata{}, nil
		}
		pmEntry := filepath.ToSlash(filepath.Join(filepath.Dir(entry), infrastructure.PackageManifestFilename))
		b, err := packagebuild.ReadFileFromTarGz(tarPath, pmEntry)
		if err != nil {
			return packageDependencyMetadata{}, nil
		}
		var pm domain.PackageManifest
		if err := yamlUnmarshalPackageManifest(b, &pm); err != nil {
			return packageDependencyMetadata{}, fmt.Errorf("package manifest: %w", err)
		}
		return packageDependencyMetadata{Platforms: pm.Platforms, Dependencies: pm.Dependencies}, nil
	}
	for cur := strings.TrimSpace(wfRoot); cur != ""; cur = filepath.Dir(cur) {
		p := filepath.Join(cur, infrastructure.PackageManifestFilename)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			pm, err := domain.ParsePackageManifest(p)
			if err != nil {
				return packageDependencyMetadata{}, err
			}
			return packageDependencyMetadata{Platforms: pm.Platforms, Dependencies: pm.Dependencies}, nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
	}
	return packageDependencyMetadata{}, nil
}

func yamlUnmarshalPackageManifest(b []byte, pm *domain.PackageManifest) error {
	if err := yaml.Unmarshal(b, pm); err != nil {
		return err
	}
	domain.NormalizePackageManifestYAMLAliases(pm)
	return domain.ValidatePackageManifest(pm)
}
