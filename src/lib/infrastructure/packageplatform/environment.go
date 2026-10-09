package packageplatform

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
)

var safeToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
var applicationIDPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_-]*){2,}$`)

// ApplicationID identifies the installed app when creating a host integration.
// Never use a sandbox executable path as a host service command.
func ApplicationID() (string, error) {
	id := os.Getenv("FLATPAK_ID")
	if len(id) > 255 || !applicationIDPattern.MatchString(id) {
		return "", fmt.Errorf("host integration requires a valid Flatpak application ID")
	}
	return id, nil
}

// IsFlatpak identifies the process boundary, including incomplete environments
// which must not fall back to native dependency installers.
func IsFlatpak() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("FLATPAK_ID") != "" {
		return true
	}
	_, err := os.Stat("/.flatpak-info")
	return err == nil || !os.IsNotExist(err)
}

// Current selects artifacts for the process environment, not the host
// distribution. Merely having Flatpak installed must not change native installs.
func Current() (string, error) {
	return processPlatform(runtime.GOOS, runtime.GOARCH, os.Getenv("FLATPAK_ID"), "/.flatpak-info")
}

func processPlatform(goos, goarch, flatpakID, infoPath string) (string, error) {
	native := goos + "-" + goarch
	if goos != "linux" {
		return native, nil
	}
	file, err := os.Open(infoPath)
	if os.IsNotExist(err) && flatpakID == "" {
		return native, nil
	}
	if err != nil {
		return "", fmt.Errorf("read Flatpak execution environment: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil || len(data) > 65536 {
		return "", fmt.Errorf("cannot read bounded Flatpak execution metadata")
	}
	section := ""
	runtimeRef := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && section == "[Application]" && strings.TrimSpace(key) == "runtime" {
			if runtimeRef != "" {
				return "", fmt.Errorf("duplicate Flatpak runtime metadata")
			}
			runtimeRef = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("parse Flatpak execution metadata: %w", err)
	}
	// Flatpak uses a full ref in /.flatpak-info; older metadata may omit
	// the runtime/ prefix. Both describe the same execution environment.
	parts := strings.Split(strings.TrimPrefix(runtimeRef, "runtime/"), "/")
	if len(parts) != 3 || !safeToken.MatchString(parts[0]) || !safeToken.MatchString(parts[2]) {
		return "", fmt.Errorf("Flatpak execution metadata needs a valid runtime reference")
	}
	arches := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}
	if arches[goarch] == "" || parts[1] != arches[goarch] {
		return "", fmt.Errorf("Flatpak runtime architecture does not match %s", goarch)
	}
	return native + "-flatpak-" + parts[0] + "-" + parts[2], nil
}
