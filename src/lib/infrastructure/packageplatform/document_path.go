package packageplatform

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const flatpakDocumentMount = "/run/flatpak/doc"

// FlatpakDocumentPath uses the per-app mount for paths returned by the
// document portal. It leaves every link below the mount for the caller to validate.
func FlatpakDocumentPath(path string) string {
	mountPath, _ := FlatpakDocumentGrantPath(path)
	return mountPath
}

// FlatpakDocumentGrantPath identifies paths below the app's document mount.
// Callers still need to verify that the grant is accessible.
func FlatpakDocumentGrantPath(path string) (string, bool) {
	if !IsFlatpak() {
		return path, false
	}

	alias := filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "doc")
	mountPath := filepath.Clean(flatpakDocumentPath(path, alias))
	if strings.HasPrefix(mountPath, flatpakDocumentMount+"/") {
		return mountPath, true
	}

	return path, false
}

func flatpakDocumentPath(path, alias string) string {
	relative, ok := strings.CutPrefix(filepath.Clean(path), alias+string(filepath.Separator))
	if !ok {
		return path
	}

	// Flatpak creates this runtime-directory alias when it mounts the app's
	// document grant view. Leave unexpected links to the normal path checks.
	target, err := os.Readlink(alias)
	if err != nil || target != "../../flatpak/doc" {
		return path
	}

	return filepath.Join(flatpakDocumentMount, relative)
}
