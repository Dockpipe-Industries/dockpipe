package domain

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/domain/secretenv"
)

// EffectiveVaultString returns the secret injection mode. Workflow YAML wins when `vault:` is set;
// otherwise secrets.vault from dockpipe.config.json applies when present.
func EffectiveVaultString(wf *Workflow, cfg *DockpipeProjectConfig) string {
	if wf != nil {
		v := strings.TrimSpace(wf.Vault)
		if v != "" {
			return v
		}
	}
	if cfg != nil && cfg.Secrets.Vault != nil {
		return strings.TrimSpace(*cfg.Secrets.Vault)
	}
	return ""
}

// ValidateVaultModeString checks a vault backend token (workflow vault: or secrets.vault).
func ValidateVaultModeString(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "op", "1password", "environment", "none", "off", "false", "no", "0":
		return nil
	default:
		return fmt.Errorf("vault %q is not supported (see docs/runtime/vault.md)", v)
	}
}

// ValidateDockpipeProjectConfig checks optional fields after JSON decode.
func ValidateDockpipeProjectConfig(c *DockpipeProjectConfig) error {
	if c == nil {
		return nil
	}
	if c.Secrets.Vault != nil {
		if err := ValidateVaultModeString(*c.Secrets.Vault); err != nil {
			return fmt.Errorf("secrets.vault: %w", err)
		}
	}
	for name, environment := range c.Secrets.Environments {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(environment.Resolver) == "" {
			return fmt.Errorf("secrets.environments requires nonempty names and resolvers")
		}
		if err := secretenv.ValidateBindings(environment.Bindings); err != nil {
			return fmt.Errorf("secrets.environments[%s]: %w", name, err)
		}
	}
	if c.Secrets.Environment != "" {
		if _, exists := c.Secrets.Environments[c.Secrets.Environment]; !exists {
			return fmt.Errorf("secrets.environment must name a configured environment")
		}
	}
	if c.Packages.Sources != nil {
		for i, src := range *c.Packages.Sources {
			if strings.TrimSpace(src.Path) == "" {
				return fmt.Errorf("packages.sources[%d].path: must not be empty", i)
			}
			kind := NormalizePackageSourceKind(src.Kind)
			if !kind.IsValid() {
				return fmt.Errorf("packages.sources[%d].kind: %q is not supported", i, src.Kind)
			}
		}
	}
	return nil
}
