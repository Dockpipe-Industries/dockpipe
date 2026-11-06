// Package mcp validates configured-server contracts without network or process IO.
package mcp

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	mcpmodel "dockpipe/src/lib/model/mcp"
)

var identity = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)
var variable = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var header = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
var platform = regexp.MustCompile(`^(linux|darwin|windows)/(amd64|arm64)$`)
var windowsAbsolute = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// Validate checks authored data only; no secret resolver, runtime, or server is
// invoked. At most 64 imports and 128 arguments per import keep management output
// reviewable. Larger installations need an explicit capacity contract first.
func Validate(config *mcpmodel.Config, environments map[string]map[string]string) error {
	if config == nil {
		return nil
	}
	if len(config.Servers) > 64 {
		return fmt.Errorf("mcp supports at most 64 configured servers")
	}
	names := make([]string, 0, len(config.Servers))
	for name := range config.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !identity.MatchString(name) || name == "core" || name == "native" || name == "dockpipe" {
			return fmt.Errorf("mcp server identity must be a non-reserved lowercase identifier")
		}
		if err := validateServer(config.Servers[name], environments); err != nil {
			return fmt.Errorf("mcp.servers[%s]: %w", name, err)
		}
	}
	return nil
}

func validateServer(server mcpmodel.Server, environments map[string]map[string]string) error {
	if server.Source != nil {
		if server.Source.Registry != mcpmodel.OfficialRegistry || ValidateRegistryName(server.Source.Name) != nil ||
			ValidateRegistryVersion(server.Source.Version) != nil || server.Source.Version == "latest" {
			return fmt.Errorf("source requires the official registry, a server name, and an exact version")
		}
	}
	switch server.Transport {
	case "stdio":
		if server.Local == nil || server.Remote != nil {
			return fmt.Errorf("stdio requires local configuration only")
		}
		local := server.Local
		if !platform.MatchString(local.Platform) {
			return fmt.Errorf("local platform must be linux, darwin or windows with amd64 or arm64")
		}
		if !absolute(local.Executable, local.Platform) || !absolute(local.WorkingDirectory, local.Platform) {
			return fmt.Errorf("executable and working_directory must be absolute paths for the declared platform")
		}
		if !identity.MatchString(local.Runtime) {
			return fmt.Errorf("local runtime must name an explicit runtime profile")
		}
		if len(local.Arguments) > 128 {
			return fmt.Errorf("local arguments exceed the 128 argument limit")
		}
		for _, argument := range local.Arguments {
			if len(argument) > 8192 || strings.ContainsRune(argument, 0) {
				return fmt.Errorf("invalid or oversized local argument")
			}
		}
		return environmentExists(local.SecretEnvironment, environments)
	case "streamable-http":
		if server.Remote == nil || server.Local != nil {
			return fmt.Errorf("streamable-http requires remote configuration only")
		}
		remote := server.Remote
		if err := ValidateRemoteURL(remote.URL); err != nil {
			return err
		}
		if err := environmentExists(remote.SecretEnvironment, environments); err != nil {
			return err
		}
		if len(remote.HeaderBindings) > 32 {
			return fmt.Errorf("remote header bindings exceed the 32 header limit")
		}
		seen := map[string]bool{}
		for name, binding := range remote.HeaderBindings {
			lower := strings.ToLower(name)
			if !header.MatchString(name) || seen[lower] || lower == "host" || lower == "content-length" ||
				lower == "transfer-encoding" || lower == "connection" || lower == "proxy-authorization" {
				return fmt.Errorf("invalid, duplicate or transport-owned header binding")
			}
			seen[lower] = true
			if !variable.MatchString(binding) || remote.SecretEnvironment == "" || environments[remote.SecretEnvironment][binding] == "" {
				return fmt.Errorf("header binding must reference a variable in the selected secret environment")
			}
		}
		return nil
	default:
		return fmt.Errorf("transport must be stdio or streamable-http; legacy SSE is not supported")
	}
}

func environmentExists(name string, environments map[string]map[string]string) error {
	if name != "" {
		if _, exists := environments[name]; !exists {
			return fmt.Errorf("secret_environment must select an existing project secret environment")
		}
	}
	return nil
}

func validText(value string, max int) bool {
	return value != "" && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func absolute(value, target string) bool {
	if !validText(value, 4096) {
		return false
	}
	if strings.HasPrefix(target, "windows/") {
		return windowsAbsolute.MatchString(value) && !strings.ContainsAny(value, "*?\"<>|")
	}
	return strings.HasPrefix(value, "/")
}

// ValidateRemoteURL intentionally excludes inline credentials, query tokens,
// templates and fragments. This is configuration validation, not SSRF containment:
// a future client must enforce egress policy at resolution and connection time.
func ValidateRemoteURL(raw string) error {
	if !validText(raw, 4096) || strings.ContainsAny(raw, "{}") {
		return fmt.Errorf("remote URL must be an explicit HTTPS URL without credentials, query or fragment")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return fmt.Errorf("remote URL must be an explicit HTTPS URL without credentials, query or fragment")
	}
	return nil
}
