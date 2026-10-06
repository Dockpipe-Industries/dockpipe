package remote

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/domain"
	contract "dockpipe/src/lib/domain/remote"
	"gopkg.in/yaml.v3"
)

// Validate the declared package closure again on the receiving side; protocol
// clients need not have used BuildBundle. Package code stays opaque to core.
func validateBundlePackages(bundle contract.Bundle) error {
	manifests := map[string]domain.PackageManifest{}
	roots := map[string]bool{}
	manifestRoots := map[string]bool{}
	for _, file := range bundle.Files {
		if !strings.HasPrefix(file.Path, "store/") {
			continue
		}
		parts := strings.Split(file.Path, "/")
		if len(parts) < 4 || !safePackageName(parts[2]) {
			return fmt.Errorf("invalid delivered package layout")
		}
		root := strings.Join(parts[:3], "/")
		roots[root] = true
		if len(parts) != 4 || parts[3] != "package.yml" {
			continue
		}
		var manifest domain.PackageManifest
		if err := yaml.Unmarshal(file.Data, &manifest); err != nil {
			return err
		}
		if err := domain.ValidatePackageManifest(&manifest); err != nil {
			return err
		}
		category := map[string]string{"workflow": "workflows", "resolver": "resolvers", "assets": "assets"}[manifest.Kind]
		if manifest.Name != parts[2] || category == "" || category != parts[1] {
			return fmt.Errorf("delivered package identity does not match its store path")
		}
		if _, exists := manifests[manifest.Name]; exists {
			return fmt.Errorf("duplicate delivered package identity")
		}
		manifests[manifest.Name] = manifest
		manifestRoots[root] = true
	}
	for root := range roots {
		name := strings.Split(root, "/")[2]
		if !manifestRoots[root] {
			return fmt.Errorf("delivered package %s is missing package.yml", name)
		}
	}
	for name, manifest := range manifests {
		for _, dependency := range manifest.Depends {
			if _, exists := manifests[dependency]; !exists {
				return fmt.Errorf("delivered package %s requires missing package %s", name, dependency)
			}
		}
	}
	return nil
}

func safePackageName(name string) bool {
	return len(name) <= 128 && contract.SafeBundlePath(name) && !strings.Contains(name, "/")
}
