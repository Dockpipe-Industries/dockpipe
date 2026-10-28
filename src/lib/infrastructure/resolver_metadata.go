package infrastructure

import (
	"errors"
	"os"
	"path/filepath"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/domain/dependency"
)

// ResolverMetadata is a public catalog projection. Profile values and credentials
// are never included. RemoteSetup describes the generic setup contract, not a vendor.
type ResolverMetadata struct {
	Name                string          `json:"name"`
	Title               string          `json:"title,omitempty"`
	Version             string          `json:"version,omitempty"`
	Description         string          `json:"description,omitempty"`
	Capability          string          `json:"capability,omitempty"`
	RemoteSetup         string          `json:"remote_setup,omitempty"`
	DependencyChanges   bool            `json:"dependency_changes,omitempty"`
	DependencyInventory bool            `json:"dependency_inventory,omitempty"`
	Surface             *PackageSurface `json:"surface,omitempty"`
}

func DescribeResolver(profilePath, name string) (ResolverMetadata, error) {
	result := ResolverMetadata{Name: name}
	manifest, err := domain.ParsePackageManifest(filepath.Join(filepath.Dir(profilePath), PackageManifestFilename))
	if err == nil {
		result.Title = manifest.Title
		result.Version = manifest.Version
		result.Description = manifest.Description
		result.Capability = manifest.Capability
		if manifest.Surface != "" {
			result.Surface, err = LoadPackageSurface(filepath.Dir(profilePath), manifest.Surface)
			if err != nil {
				return result, err
			}
		}
	} else if !os.IsNotExist(err) {
		return result, err
	}
	profile, err := LoadResolverFile(profilePath)
	if err != nil {
		return result, err
	}
	result.DependencyInventory = profile["DOCKPIPE_DEPENDENCY_LIFECYCLE_PROTOCOL"] == dependency.InventorySchema &&
		profile["DOCKPIPE_DEPENDENCY_LIFECYCLE_WORKFLOW"] != ""
	result.DependencyChanges = profile["DOCKPIPE_DEPENDENCY_CHANGE_PROTOCOL"] == dependency.ChangeSchema && profile["DOCKPIPE_DEPENDENCY_CHANGE_WORKFLOW"] != ""
	edge := profile["DOCKPIPE_REMOTE_EDGE_SETUP"] != ""
	hosted := profile["DOCKPIPE_REMOTE_BROKER_SETUP"] != ""
	if edge && hosted {
		return result, errors.New("resolver declares conflicting remote setup contracts")
	}
	if edge && (result.Capability == "" || result.Capability == "remote.edge") {
		result.RemoteSetup = "local"
	}
	if hosted && result.Capability == "remote.broker" {
		result.RemoteSetup = "hosted"
	}
	return result, nil
}
