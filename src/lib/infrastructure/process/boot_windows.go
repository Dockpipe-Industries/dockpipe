//go:build windows

package process

import (
	"strings"

	"dockpipe/src/lib/infrastructure/hostboot"
)

func HostBootIdentity() (string, error) {
	identifier, err := hostboot.WindowsIdentifier()
	if err != nil {
		return "", err
	}

	return strings.ToLower(strings.Trim(identifier.String(), "{}")), nil
}

func hostBootSequence() (uint32, error) {
	return hostboot.WindowsSequence()
}
