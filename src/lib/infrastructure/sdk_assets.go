package infrastructure

import (
	"fmt"
	"path/filepath"

	"dockpipe/src/lib/sdkassets"
)

// SDKHelperPath materializes a selected portable library helper without depending
// on the source checkout, workflow roots or another helper's filesystem location.
func SDKHelperPath(workdir, language string) (string, error) {
	var name string
	switch language {
	case "shell":
		name = "dockpipe-sdk.sh"
	case "powershell":
		name = "repo-tools.ps1"
	case "python":
		name = "repo_tools.py"
	default:
		return "", fmt.Errorf("unsupported SDK language %q (use shell, powershell, python)", language)
	}
	source, err := sdkassets.Scripts.ReadFile("scripts/" + name)
	if err != nil {
		return "", err
	}
	directory, err := materializeLibraryResources(workdir, "sdk", map[string][]byte{name: source})
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, name), nil
}

// ShellSDKPath materializes only the requested library-owned shell helper.
func ShellSDKPath(workdir string) (string, error) {
	return SDKHelperPath(workdir, "shell")
}
