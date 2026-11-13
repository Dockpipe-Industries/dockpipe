package remote

import (
	"encoding/json"
	"errors"
	"path/filepath"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

// DesktopPermissions is an explicit worker-owner grant for the existing desktop.
// It does not enable delivery, terminals, clipboard, audio or credential sharing.
// The external provider owns executable selection and artifact verification.
type DesktopPermissions struct {
	Schema   string                   `json:"schema"`
	Provider contract.DesktopProvider `json:"provider"`
	View     bool                     `json:"view"`
	Input    bool                     `json:"input"`
}

func (permissions DesktopPermissions) Validate() error {
	if permissions.Schema != contract.DesktopVersion || permissions.Provider.Validate() != nil || permissions.Input && !permissions.View {
		return errors.New("invalid independent desktop owner permissions")
	}
	return nil
}

func LoadDesktopPermissions(root string) (DesktopPermissions, error) {
	var permissions DesktopPermissions
	if err := readPrivateBounded(filepath.Join(root, "desktop.json"), &permissions, 4096); err != nil {
		return permissions, err
	}
	return permissions, permissions.Validate()
}

func SaveDesktopPermissions(root string, permissions DesktopPermissions) error {
	if err := permissions.Validate(); err != nil {
		return err
	}
	if err := PrivateDirectory(root); err != nil {
		return err
	}
	return WritePrivate(filepath.Join(root, "desktop.json"), permissions)
}

// NewDesktopCatalog freezes independent owner policy and a bridge lifetime.
// Provider readiness is supplied separately by the trusted native composition.
func NewDesktopCatalog(config contract.WorkerConfig, permissions DesktopPermissions, instance string, view, input bool) (contract.DesktopCatalog, error) {
	if permissions.Validate() != nil || !validMachineNonce(instance) || !contract.ValidID(config.Node) ||
		remoteidentity.Fingerprint(config.MachineKey) == "" || input && !view || view && !permissions.View || input && !permissions.Input {
		return contract.DesktopCatalog{}, errors.New("desktop provider readiness exceeds frozen owner permissions")
	}
	encoded, _ := json.Marshal(permissions)
	catalog := contract.DesktopCatalog{Schema: contract.DesktopVersion, Provider: permissions.Provider, View: view, Input: input,
		Target: contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(config.MachineKey), Generation: Hash(instance + string(encoded))}}
	return catalog, catalog.Validate()
}

// CheckDesktopPolicy is required before every worker exchange and native effect.
// A policy change cannot be published under the previous target generation.
func CheckDesktopPolicy(root string, frozen DesktopPermissions) error {
	current, err := LoadDesktopPermissions(root)
	if err != nil || current != frozen {
		return errors.New("desktop owner permissions changed; close native channels before restarting the bridge")
	}
	return nil
}
