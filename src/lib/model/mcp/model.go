// Package mcpmodel owns MCP discovery and configured-server wire shapes.
// Registry descriptions are untrusted metadata, never executable configuration.
package mcpmodel

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const OfficialRegistry = "https://registry.modelcontextprotocol.io"

// Config belongs to dockpipe.config.json, not a second configuration store.
type Config struct {
	Servers map[string]Server `json:"servers,omitempty"`
}

// Server describes operator-selected configuration. Validation does not enable a
// server, grant authority, provision dependencies, or resolve secret references.
type Server struct {
	Transport string          `json:"transport"`
	Source    *RegistrySource `json:"source,omitempty"`
	Local     *Local          `json:"local,omitempty"`
	Remote    *Remote         `json:"remote,omitempty"`
}

type RegistrySource struct {
	Registry string `json:"registry"`
	Name     string `json:"name"`
	Version  string `json:"version"`
}

// Local binds an already installed executable to a Dockpipe runtime profile and
// target platform. Provisioning remains with existing runtime/package owners.
type Local struct {
	Executable        string   `json:"executable"`
	Arguments         []string `json:"arguments,omitempty"`
	WorkingDirectory  string   `json:"working_directory"`
	Runtime           string   `json:"runtime"`
	Platform          string   `json:"platform"`
	SecretEnvironment string   `json:"secret_environment,omitempty"`
}

// HeaderBindings maps HTTP header names to variables from the selected existing
// secret environment. It contains references, never resolved header values.
type Remote struct {
	URL               string            `json:"url"`
	SecretEnvironment string            `json:"secret_environment,omitempty"`
	HeaderBindings    map[string]string `json:"header_bindings,omitempty"`
}

// RegistryRecord is normalized discovery metadata. Package/remote declarations
// remain verbatim JSON so a future importer cannot mistake partial decoding for
// a complete executable contract. Unknown upstream extensions survive discovery.
type RegistryRecord struct {
	Source      string          `json:"source"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Status      string          `json:"status"`
	IsLatest    bool            `json:"is_latest"`
	Document    json.RawMessage `json:"document"`
}

type RegistryPage struct {
	Servers    []RegistryRecord `json:"servers"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// UnmarshalJSON rejects misspelled transport/security fields instead of silently
// accepting an incomplete configuration. The surrounding project stays extensible.
func (c *Config) UnmarshalJSON(data []byte) error {
	if len(data) > 8<<20 {
		return fmt.Errorf("mcp configuration exceeds 8 MiB")
	}
	type plain Config
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*c = Config(value)
	return nil
}
