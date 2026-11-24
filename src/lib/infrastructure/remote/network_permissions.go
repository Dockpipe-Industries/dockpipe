package remote

import (
	contract "dockpipe/src/lib/domain/remote"
	"path/filepath"
)

func LoadNetworkPermissions(root string) (contract.NetworkPermissions, error) {
	var permissions contract.NetworkPermissions
	if err := readPrivateBounded(filepath.Join(root, "network.json"), &permissions, 4096); err != nil {
		return permissions, err
	}
	return permissions, permissions.Validate()
}
