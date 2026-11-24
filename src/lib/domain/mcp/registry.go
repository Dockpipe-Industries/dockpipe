package mcp

import (
	"fmt"
	"strings"
)

// ValidateRegistryName validates a namespaced upstream identity before URL encoding.
func ValidateRegistryName(value string) error { return validateRegistryComponent(value, true) }

// ValidateRegistryVersion accepts an opaque version or the read-only latest selector.
func ValidateRegistryVersion(value string) error { return validateRegistryComponent(value, false) }

func validateRegistryComponent(value string, isName bool) error {
	if value == "" || len(value) > 512 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\x00\r\n\\?#%") || value == "." || value == ".." {
		return fmt.Errorf("invalid MCP registry name or version")
	}
	if isName && (strings.Count(value, "/") != 1 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/")) {
		return fmt.Errorf("MCP registry name must contain namespace/server")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("invalid MCP registry path segment")
		}
	}
	if !isName && strings.Contains(value, "/") {
		return fmt.Errorf("invalid MCP registry version")
	}
	return nil
}
