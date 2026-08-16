package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelproject "dockpipe/src/lib/model/project"
)

// DockpipeProjectConfigFileName is the repo-root JSON file for project-level DockPipe settings
// (compile source lists, future package registry hints). Optional — compile uses built-in defaults when absent.
const DockpipeProjectConfigFileName = "dockpipe.config.json"

// DockpipeProjectConfig remains as a source-compatible Domain facade for the authored model.
type DockpipeProjectConfig = modelproject.DockpipeProjectConfig

// DockpipeSecretsConfig remains as a source-compatible Domain facade for the authored model.
type DockpipeSecretsConfig = modelproject.DockpipeSecretsConfig

// DockpipeCompileConfig remains as a source-compatible Domain facade for the authored model.
type DockpipeCompileConfig = modelproject.DockpipeCompileConfig

// DockpipePackagesConfig remains as a source-compatible Domain facade for the authored model.
type DockpipePackagesConfig = modelproject.DockpipePackagesConfig

// DockpipePackageSourceConfig remains as a source-compatible Domain facade for the authored model.
type DockpipePackageSourceConfig = modelproject.DockpipePackageSourceConfig

// LoadDockpipeProjectConfig reads dockpipe.config.json from repoRoot. Returns (nil, nil) if the file is missing.
func LoadDockpipeProjectConfig(repoRoot string) (*DockpipeProjectConfig, error) {
	p := filepath.Join(repoRoot, DockpipeProjectConfigFileName)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c DockpipeProjectConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if err := ValidateDockpipeProjectConfig(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &c, nil
}

// FindProjectRootWithDockpipeConfig walks up from startDir until it finds a directory
// containing DockpipeProjectConfigFileName. Returns the absolute path to that directory.
// If the file is not found in any parent, returns abs(startDir) so callers can still use
// cwd-based defaults (e.g. compile without a config file).
func FindProjectRootWithDockpipeConfig(startDir string) (string, error) {
	startAbs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for d := startAbs; ; {
		p := filepath.Join(d, DockpipeProjectConfigFileName)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return startAbs, nil
}

// ResolveOpInjectTemplatePath returns the absolute path to the op inject template when secrets.op_inject_template is set.
func ResolveOpInjectTemplatePath(cfg *DockpipeProjectConfig, repoRoot string) (string, bool) {
	if cfg == nil || cfg.Secrets.OpInjectTemplate == nil {
		return "", false
	}
	p := strings.TrimSpace(*cfg.Secrets.OpInjectTemplate)
	if p == "" {
		return "", false
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), true
	}
	return filepath.Join(repoRoot, filepath.Clean(p)), true
}

// ResolveVaultTemplatePath returns the absolute path to the vault env template.
// secrets.vault_template takes precedence; secrets.op_inject_template is the legacy alias when vault_template is unset or empty.
func ResolveVaultTemplatePath(cfg *DockpipeProjectConfig, repoRoot string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	var p string
	if cfg.Secrets.VaultTemplate != nil {
		p = strings.TrimSpace(*cfg.Secrets.VaultTemplate)
	}
	if p == "" && cfg.Secrets.OpInjectTemplate != nil {
		p = strings.TrimSpace(*cfg.Secrets.OpInjectTemplate)
	}
	if p == "" {
		return "", false
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), true
	}
	return filepath.Join(repoRoot, filepath.Clean(p)), true
}

// DefaultDockpipeProjectConfigBytes returns indented JSON for a new project (dockpipe init).
// Paths are repo-relative; compile skips any that do not exist on disk.
func DefaultDockpipeProjectConfigBytes() ([]byte, error) {
	wf := []string{"workflows"}
	vaultT := ".env.vault.template"
	notes := "Vault env template (op:// references resolved via op inject — 1Password CLI today). Keep references here; do not commit plaintext secrets."
	cfg := DockpipeProjectConfig{
		Schema: 1,
		Compile: DockpipeCompileConfig{
			Workflows: &wf,
		},
		Secrets: DockpipeSecretsConfig{
			VaultTemplate: &vaultT,
			Notes:         &notes,
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}
