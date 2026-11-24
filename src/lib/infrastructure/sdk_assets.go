package infrastructure

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/sdkassets"
)

// SDKHelperPath materializes a selected portable library helper without depending
// on the source checkout, workflow roots or another helper's filesystem location.
func SDKHelperPath(workdir, language string) (string, error) {
	if language == "pipelang" {
		return PipeLangSDKPath(workdir)
	}

	var name string
	switch language {
	case "shell":
		name = "dockpipe-sdk.sh"
	case "powershell":
		name = "repo-tools.ps1"
	case "python":
		name = "repo_tools.py"
	default:
		return "", fmt.Errorf("unsupported SDK language %q (use shell, powershell, python, pipelang)", language)
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

// PipeLangSDKPath materializes the complete library-owned SDK bundle and returns
// its pinned source-library descriptor. Native support files share its cache
// identity, so a transport change cannot reuse an older bundle directory.
func PipeLangSDKPath(workdir string) (string, error) {
	files := make(map[string][]byte)
	err := fs.WalkDir(sdkassets.PipeLang, "pipelang", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := sdkassets.PipeLang.ReadFile(path)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(path, "pipelang/")] = content
		return nil
	})
	if err != nil {
		return "", err
	}
	directory, err := materializeLibraryResources(workdir, "sdk-pipelang", files)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "library.json"), nil
}

// ShellSDKPath materializes only the requested library-owned shell helper.
func ShellSDKPath(workdir string) (string, error) {
	return SDKHelperPath(workdir, "shell")
}
