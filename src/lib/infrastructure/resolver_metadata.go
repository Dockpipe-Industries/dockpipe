package infrastructure

import (
	"errors"
	"os"
	"path/filepath"

	"dockpipe/src/lib/domain"
)

// ResolverMetadata is a public catalog projection. Profile values and credentials
// are never included. RemoteSetup describes the generic setup contract, not a vendor.
type ResolverMetadata struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Capability  string `json:"capability,omitempty"`
	RemoteSetup string `json:"remote_setup,omitempty"`
}

func DescribeResolver(profilePath, name string) (ResolverMetadata, error) {
	result := ResolverMetadata{Name: name}
	manifest, err := domain.ParsePackageManifest(filepath.Join(filepath.Dir(profilePath), PackageManifestFilename))
	if err == nil {
		result.Title = manifest.Title
		result.Version = manifest.Version
		result.Description = manifest.Description
		result.Capability = manifest.Capability
	} else if !os.IsNotExist(err) {
		return result, err
	}
	profile, err := LoadResolverFile(profilePath)
	if err != nil {
		return result, err
	}
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
