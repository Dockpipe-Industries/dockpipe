package domain

import (
	"fmt"
	"strings"

	modeldependency "dockpipe/src/lib/model/dependency"
)

// DependencySpec remains as a source-compatible Domain facade for the authored model.
type DependencySpec = modeldependency.DependencySpec

// HostDependency remains as a source-compatible Domain facade for the authored model.
type HostDependency = modeldependency.HostDependency

// HostDependencyInstallHint remains as a source-compatible Domain facade for the authored model.
type HostDependencyInstallHint = modeldependency.HostDependencyInstallHint

func ValidateDependencySpec(fieldPrefix string, deps DependencySpec) error {
	seen := map[string]struct{}{}
	for i, dep := range deps.Host {
		prefix := fmt.Sprintf("%s.host[%d]", fieldPrefix, i)
		id := strings.TrimSpace(dep.ID)
		command := strings.TrimSpace(dep.Command)
		if id == "" && command == "" {
			return fmt.Errorf("%s requires id or command", prefix)
		}
		if id != "" && strings.ContainsAny(id, " \t\r\n") {
			return fmt.Errorf("%s.id must not contain whitespace", prefix)
		}
		if command != "" && strings.ContainsAny(command, `/\`) {
			return fmt.Errorf("%s.command must be an executable name, not a path", prefix)
		}
		key := firstNonEmptyString(command, id)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%s duplicates dependency %q", prefix, key)
		}
		seen[key] = struct{}{}
		if err := validateDependencyInstallHint(prefix+".install", dep.Install); err != nil {
			return err
		}
	}
	return nil
}

func validateDependencyInstallHint(fieldPrefix string, hint HostDependencyInstallHint) error {
	for k, v := range map[string]string{
		"windows": hint.Windows,
		"macos":   hint.MacOS,
		"linux":   hint.Linux,
		"deb":     hint.Deb,
	} {
		if strings.ContainsAny(v, "\x00\r\n") {
			return fmt.Errorf("%s.%s must be a single shell command or short instruction", fieldPrefix, k)
		}
	}
	return nil
}

func ValidatePlatformList(fieldPrefix string, platforms []string) error {
	seen := map[string]struct{}{}
	for i, platform := range platforms {
		platform = strings.TrimSpace(strings.ToLower(platform))
		switch platform {
		case "windows", "macos", "linux", "deb":
		default:
			return fmt.Errorf("%s[%d] must be one of windows, macos, linux, deb", fieldPrefix, i)
		}
		if _, ok := seen[platform]; ok {
			return fmt.Errorf("%s[%d] duplicates platform %q", fieldPrefix, i, platform)
		}
		seen[platform] = struct{}{}
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
