package remotecmd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"dockpipe/src/lib/infrastructure/packageplatform"
)

// The definition binds a service to both its executable path and installed bytes.
// An application upgrade must not leave a background process silently unreviewed.
type serviceDefinition struct {
	role    string
	name    string
	path    string
	content string
	flatpak bool
}

type serviceInfo struct {
	State string `json:"state"`
}

func prepareService(ctx context.Context, root, role string) (serviceDefinition, error) {
	command := "serve"
	if role == "worker" {
		command = "worker"
	} else if role != "broker" {
		return serviceDefinition{}, errors.New("service role must be broker or worker")
	}
	executable, err := os.Executable()
	if err != nil {
		return serviceDefinition{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return serviceDefinition{}, err
	}
	digest := sha256.Sum256([]byte(root))
	name := fmt.Sprintf("com.dockpipe.remote.%s.%x", role, digest[:6])
	arguments := []string{executable, "remote", command, "--state", root}
	flatpak := packageplatform.IsFlatpak()
	if flatpak {
		applicationID, err := packageplatform.ApplicationID()
		if err != nil {
			return serviceDefinition{}, err
		}
		arguments = flatpakServiceArguments(applicationID, command, root)
	}
	var path, content string
	switch runtime.GOOS {
	case "darwin":
		path = filepath.Join(home, "Library", "LaunchAgents", name+".plist")
		bundleIdentifier, err := serviceBundleIdentifier(ctx, executable)
		if err != nil {
			return serviceDefinition{}, err
		}
		content = launchAgent(name, arguments, filepath.Join(root, role+".log"), bundleIdentifier)
	case "linux":
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		path = filepath.Join(base, "systemd", "user", name+".service")
		content = systemdUnit(arguments)
		if flatpak {
			// /app and the sandbox PATH do not exist in the host user manager.
			content = systemdUnitWithPath(arguments, "")
		}
	default:
		return serviceDefinition{}, errors.New("remote user services currently support Linux and macOS")
	}

	fingerprint, err := executableFingerprint(executable)
	if err != nil {
		return serviceDefinition{}, err
	}
	if runtime.GOOS == "darwin" {
		content = strings.Replace(content, "<plist version=", "<!-- Dockpipe executable sha256: "+fingerprint+" -->\n<plist version=", 1)
	} else {
		content = "# Dockpipe executable sha256: " + fingerprint + "\n" + content
	}
	return serviceDefinition{role: role, name: name, path: path, content: content, flatpak: flatpak}, nil
}

func executableFingerprint(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("service executable must be a regular file")
	}
	hash := sha256.New()
	if _, err := io.CopyN(hash, file, info.Size()); err != nil {
		return "", err
	}
	after, err := file.Stat()
	if err != nil {
		return "", err
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return "", errors.New("service executable changed during inspection; retry")
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func describeService(root, role string) *serviceInfo {
	definition, err := prepareService(context.Background(), root, role)
	if err != nil {
		return &serviceInfo{State: "unavailable"}
	}
	info := inspectServiceDefinition(definition)
	if info.State == "current" {
		_, err := os.Lstat(filepath.Join(root, role+"-service-update.json"))
		if err == nil {
			info.State = "update-required"
		} else if !os.IsNotExist(err) {
			info.State = "unavailable"
		}
	}
	return info
}

func inspectServiceDefinition(definition serviceDefinition) *serviceInfo {
	content, err := readServiceDefinition(definition.path)
	if os.IsNotExist(err) {
		return &serviceInfo{State: "missing"}
	}
	if err != nil {
		return &serviceInfo{State: "unavailable"}
	}
	if string(content) != definition.content {
		return &serviceInfo{State: "update-required"}
	}
	// This describes registration, not process health or remote reachability.
	return &serviceInfo{State: "current"}
}
