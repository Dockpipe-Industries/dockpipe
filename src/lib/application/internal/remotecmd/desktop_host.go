package remotecmd

import (
	"errors"

	"dockpipe/src/lib/infrastructure/processenvironment"
)

// Restrict only incoming desktop hosting. A contained operator may still use
// desktop viewers, relays and independently authorized remote features.
func requireDesktopHosting() error {
	if processenvironment.IsContained() {
		return errors.New("incoming desktop capture and control are unavailable in contained applications; connect to a desktop provider on another host")
	}
	return nil
}
