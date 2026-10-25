package secretenv

import (
	"fmt"
	"strings"
)

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
