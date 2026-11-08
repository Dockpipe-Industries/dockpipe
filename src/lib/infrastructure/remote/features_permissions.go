package remote

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
)

const MaxFeaturePermissionBytes = 1 << 20

// FeaturePermissions is private worker-local configuration. Enrollment and
// workflow delivery never create these grants. Root paths are never advertised.
type FeaturePermissions struct {
	Schema          string                  `json:"schema"`
	Roots           []FeatureRootPermission `json:"roots"`
	DockerRead      bool                    `json:"docker_read"`
	DockerControl   bool                    `json:"docker_control"`
	TerminalProfile *TerminalProfile        `json:"terminal_profile,omitempty"`
}

type FeatureRootPermission struct {
	contract.FeatureRoot
	Path string `json:"path"`
}

func (permissions FeaturePermissions) Validate() error {
	if permissions.Schema != contract.FeatureVersion || len(permissions.Roots) > 32 {
		return errors.New("invalid worker-local feature permissions")
	}
	if permissions.DockerControl && !permissions.DockerRead {
		return errors.New("Docker control requires Docker read permission")
	}
	seen := map[string]bool{}
	terminalRoots := 0
	for _, root := range permissions.Roots {
		if root.Terminal {
			terminalRoots++
		}
		if root.FilesWrite && !root.FilesRead || (root.GitWrite || root.GitPublish) && !root.GitRead {
			return errors.New("file and Git mutations require their corresponding read permission")
		}
		if !contract.ValidID(root.ID) || seen[root.ID] || !filepath.IsAbs(root.Path) || len(root.Path) > 32768 || strings.ContainsRune(root.Path, 0) {
			return errors.New("feature roots require distinct IDs and absolute local paths")
		}
		seen[root.ID] = true
	}
	if terminalRoots > 1 || (terminalRoots == 1) != (permissions.TerminalProfile != nil) {
		return errors.New("terminal authority requires exactly one owner root and shell profile")
	}
	if permissions.TerminalProfile != nil {
		if err := permissions.TerminalProfile.Validate(); err != nil {
			return err
		}
	}
	return permissions.catalog(contract.FeatureTarget{Node: "validation", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)}).Validate()
}

func (permissions FeaturePermissions) catalog(target contract.FeatureTarget) contract.FeatureCatalog {
	roots := make([]contract.FeatureRoot, 0, len(permissions.Roots))
	for _, root := range permissions.Roots {
		published := root.FeatureRoot
		published.Terminal = published.Terminal && terminalSupported()
		roots = append(roots, published)
	}
	return contract.FeatureCatalog{Schema: contract.FeatureVersion, Target: target, Roots: roots, DockerRead: permissions.DockerRead, DockerControl: permissions.DockerControl}
}

func loadFeaturePermissions(root string) (FeaturePermissions, error) {
	return LoadFeaturePermissions(root)
}

func LoadFeaturePermissions(root string) (FeaturePermissions, error) {
	var permissions FeaturePermissions
	if err := readPrivateBounded(filepath.Join(root, "features.json"), &permissions, MaxFeaturePermissionBytes); err != nil {
		return FeaturePermissions{}, err
	}
	return permissions, permissions.Validate()
}

// SaveFeaturePermissions is a worker-owner configuration operation. It cannot
// be invoked through the remote feature protocol to grant the broker more power.
func SaveFeaturePermissions(root string, permissions FeaturePermissions) error {
	if err := permissions.Validate(); err != nil {
		return err
	}
	if permissions.TerminalProfile != nil && !terminalSupported() {
		return errors.New("remote terminal is unavailable on this platform; a qualified ConPTY backend is required on Windows")
	}
	encoded, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	if len(encoded) > MaxFeaturePermissionBytes {
		return errors.New("worker-local feature permissions exceed 1 MiB")
	}
	if err := PrivateDirectory(root); err != nil {
		return err
	}
	return WritePrivate(filepath.Join(root, "features.json"), permissions)
}
