// Package projectmodel owns the authored project-configuration wire shape.
package projectmodel

import (
	"encoding/json"
	"fmt"
)

// DockpipeProjectConfig is repo-root metadata (JSON). Schema may grow; unknown keys are ignored by encoding/json.
type DockpipeProjectConfig struct {
	Schema   int                    `json:"schema,omitempty"`
	Compile  DockpipeCompileConfig  `json:"compile,omitempty"`
	Secrets  DockpipeSecretsConfig  `json:"secrets,omitempty"`
	Packages DockpipePackagesConfig `json:"packages,omitempty"`
}

// DockpipeSecretsConfig points at host-side vault mapping (secret references → env), not plaintext secrets.
type DockpipeSecretsConfig struct {
	// VaultTemplate is the preferred repo-relative or absolute path to the vault env template (e.g. .env.vault.template).
	// Same role as op_inject_template; takes precedence when both are set.
	VaultTemplate *string `json:"vault_template,omitempty"`
	// OpInjectTemplate is a legacy alias for VaultTemplate (1Password op inject format). Use vault_template in new projects.
	OpInjectTemplate *string `json:"op_inject_template,omitempty"`
	// Vault is the default vault mode when workflow YAML omits vault: (see docs/runtime/vault.md).
	Vault *string `json:"vault,omitempty"`
	// Notes is optional human-readable context for maintainers (shown by dockpipe doctor when present).
	Notes *string `json:"notes,omitempty"`
}

// DockpipeCompileConfig lists directories (repo-relative or absolute) used by `dockpipe package compile`.
// Pointer slices distinguish JSON "key absent" (nil → use CLI defaults) from "empty array" (non-nil, len 0 → compile nothing from that category).
type DockpipeCompileConfig struct {
	CoreFrom  *string   `json:"core_from,omitempty"` // optional override for compile core --from
	Workflows *[]string `json:"workflows,omitempty"` // roots to scan for workflow/resolver trees (e.g. workflows/, packages/, custom vendor roots); same walk for tarballs and resolver discovery (+ src/core/resolvers, templates/core/resolvers)
}

// UnmarshalJSON rejects retired compile-root keys while preserving the project config's
// forward-compatible handling of every other unknown key.
func (c *DockpipeCompileConfig) UnmarshalJSON(data []byte) error {
	type plain DockpipeCompileConfig
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	for _, retired := range []string{"resolvers", "bundles"} {
		if _, ok := keys[retired]; ok {
			return fmt.Errorf("compile.%s is not supported; use compile.workflows", retired)
		}
	}
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = DockpipeCompileConfig(decoded)
	return nil
}

// DockpipePackagesConfig holds optional defaults for packaged workflows/resolvers and tarball resolution.
type DockpipePackagesConfig struct {
	// TarballDir is a repo-relative directory containing dockpipe-workflow-*.tar.gz (after package build store).
	// When unset, release/artifacts is used if that directory exists. Resolution also checks
	// <workdir>/bin/.dockpipe/internal/packages/workflows/ first.
	TarballDir *string `json:"tarball_dir,omitempty"`
	// Namespace: default author/org label for compile (package.yml) when workflow/resolver metadata omits it;
	// when set, tarball resolution prefers archives whose config.yml namespace matches.
	Namespace *string `json:"namespace,omitempty"`
	// RegistryURLs optional HTTPS bases for future package id resolution (e.g. https://packages.dockpipe.com).
	// Not wired in the runner yet; compile and resolution use compile.workflows paths and local stores.
	RegistryURLs *[]string `json:"registry_urls,omitempty"`
	// Sources extends package resolution with explicit local filesystem sources for unpublished or shared package stores.
	// This supplements, but does not replace, DOCKPIPE_PACKAGES_ROOT, global package roots, or remote tarball flows.
	Sources *[]DockpipePackageSourceConfig `json:"sources,omitempty"`
}

// DockpipePackageSourceConfig points package resolution at an explicit local filesystem source.
// Supported kinds:
// - "store": a package store root containing workflows/, resolvers/, and core/
// - "tarball_dir": a directory containing dockpipe-*.tar.gz artifacts
type DockpipePackageSourceConfig struct {
	Kind string `json:"kind,omitempty"`
	Path string `json:"path,omitempty"`
}
