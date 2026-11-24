package remotecmd

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func terminalHomePermissions(root string) (remoteio.FeaturePermissions, error) {
	var permissions remoteio.FeaturePermissions
	if os.Getenv("DOCKPIPE_REMOTE_WORKER") == "1" {
		return permissions, errors.New("enable home Terminal from the paired worker's local Dockpipe installation")
	}

	path := filepath.Join(root, "features.json")
	if _, err := os.Lstat(path); err == nil {
		permissions, err = remoteio.LoadFeaturePermissions(root)
		if err != nil {
			return permissions, err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		permissions.Schema = contract.FeatureVersion
	} else {
		return permissions, err
	}

	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return permissions, errors.New("the worker account's home directory is unavailable")
	}
	home = filepath.Clean(home)
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		return permissions, errors.New("the worker account's home directory is unavailable")
	}

	if permissions.TerminalProfile == nil {
		profile := defaultRemoteShell()
		if !filepath.IsAbs(profile.Executable) {
			return permissions, errors.New("the worker account's default shell is unavailable")
		}
		if info, err := os.Stat(profile.Executable); err != nil || !info.Mode().IsRegular() {
			return permissions, errors.New("the worker account's default shell is unavailable")
		}
		permissions.TerminalProfile = &profile
	}
	for index, existing := range permissions.Roots {
		if existing.ID != "home" {
			continue
		}
		configuredPath := filepath.Clean(existing.Path)
		sameHome := configuredPath == home || runtime.GOOS == "windows" && strings.EqualFold(configuredPath, home)
		if !sameHome {
			return permissions, errors.New("the home root ID already belongs to a different folder")
		}
		permissions.Roots[index].Terminal = true
		return permissions, nil
	}
	permissions.Roots = append(permissions.Roots, remoteio.FeatureRootPermission{
		FeatureRoot: contract.FeatureRoot{ID: "home", Name: "Home", Terminal: true},
		Path:        home,
	})
	return permissions, nil
}

func defaultRemoteShell() remoteio.TerminalProfile {
	if runtime.GOOS == "windows" {
		return remoteio.TerminalProfile{
			Executable: filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
			Args:       []string{"-NoLogo"},
		}
	}
	shell := os.Getenv("SHELL")
	if !filepath.IsAbs(shell) {
		shell = "/bin/sh"
	}
	return remoteio.TerminalProfile{Executable: shell, Args: []string{"-i"}}
}
