package hostdependencies

import (
	"os"
	"runtime"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packageplatform"
)

func InstallCommandForCurrentPlatform(dep domain.HostDependency) string {
	hint := dep.Install
	switch CurrentPlatform() {
	case "flatpak":
		// Runtime dependencies are provided by the app or the selected package.
		// Native distribution installers cannot install into the immutable runtime.
		return ""
	case "windows":
		return strings.TrimSpace(hint.Windows)
	case "macos":
		return strings.TrimSpace(hint.MacOS)
	case "deb":
		if v := strings.TrimSpace(hint.Deb); v != "" {
			return v
		}
		return strings.TrimSpace(hint.Linux)
	default:
		return strings.TrimSpace(hint.Linux)
	}
}

func PlatformsSupportCurrent(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	current := CurrentPlatform()
	for _, platform := range platforms {
		if strings.EqualFold(strings.TrimSpace(platform), current) {
			return true
		}
		if current == "deb" && strings.EqualFold(strings.TrimSpace(platform), "linux") {
			return true
		}
	}
	return false
}

func CurrentPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	case "linux":
		if packageplatform.IsFlatpak() {
			return "flatpak"
		}
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			return "deb"
		}
		return "linux"
	default:
		return runtime.GOOS
	}
}
