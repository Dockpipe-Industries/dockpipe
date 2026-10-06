package application

import (
	"encoding/json"
	"os"

	"dockpipe/src/lib/infrastructure"
)

func writePackageInventory(workdir string) error {
	inventory, err := infrastructure.ListInstalledPackages(workdir)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(inventory)
}
