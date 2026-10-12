package cloudflare

import (
	"os"

	"dockpipe/src/lib/infrastructure"
)

func validateCredentialAccess(path string, _ os.FileInfo) error {
	return infrastructure.ValidatePrivatePath(path, false)
}
