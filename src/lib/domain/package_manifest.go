package domain

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	modelpackage "dockpipe/src/lib/model/package"

	"gopkg.in/yaml.v3"
)

// PackageManifest remains as a source-compatible Domain facade for the authored model.
type PackageManifest = modelpackage.PackageManifest

// PackageImageSpec remains as a source-compatible Domain facade for the authored model.
type PackageImageSpec = modelpackage.PackageImageSpec

// PackageScriptContract remains as a source-compatible Domain facade for the authored model.
type PackageScriptContract = modelpackage.PackageScriptContract

// PackageStateSpec remains as a source-compatible Domain facade for the authored model.
type PackageStateSpec = modelpackage.PackageStateSpec

// PackageBuildSpec remains as a source-compatible Domain facade for the authored model.
type PackageBuildSpec = modelpackage.PackageBuildSpec

// PackageSourceBuildSpec remains as a source-compatible Domain facade for the authored model.
type PackageSourceBuildSpec = modelpackage.PackageSourceBuildSpec

// PackageTestSpec remains as a source-compatible Domain facade for the authored model.
type PackageTestSpec = modelpackage.PackageTestSpec

// ParsePackageManifest reads and parses package.yml from path.
func ParsePackageManifest(path string) (*PackageManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m PackageManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	NormalizePackageManifestYAMLAliases(&m)
	if err := ValidatePackageManifest(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// ValidatePackageManifest checks optional fields (e.g. namespace) after YAML decode.
func ValidatePackageManifest(m *PackageManifest) error {
	if m == nil {
		return nil
	}
	if err := ValidatePackageVersion(m.Version); err != nil {
		return err
	}
	if err := ValidateNamespace(m.Namespace); err != nil {
		return err
	}
	if err := ValidateProvider(m.Provider); err != nil {
		return err
	}
	if err := ValidateCapabilityID(m.Capability); err != nil {
		return err
	}
	for _, p := range m.RequiresCapabilities {
		if err := ValidateCapabilityID(p); err != nil {
			return err
		}
	}
	for _, injectable := range m.ScriptContract.Inject {
		if err := ValidateScriptContractInjectable(injectable); err != nil {
			return err
		}
	}
	if err := ValidatePackageStateSpec(&m.PackageState); err != nil {
		return err
	}
	if err := ValidatePackageImageSpec(&m.Image); err != nil {
		return err
	}
	if err := ValidatePackageBuildSpec(&m.Build); err != nil {
		return err
	}
	if err := ValidatePackageTestSpec(&m.Test); err != nil {
		return err
	}
	if err := ValidatePlatformList("platforms", m.Platforms); err != nil {
		return err
	}
	if err := ValidateDependencySpec("dependencies", m.Dependencies); err != nil {
		return err
	}
	// kind-specific required fields kept minimal — capability / requires_capabilities are optional metadata.
	return nil
}

// ValidatePackageStateSpec keeps the engine/package split explicit: maintained packages may own
// exact cohort migration for declared durable owner IDs, while an absent declaration means that
// the generic public compatibility importer must preserve the legacy scope whole.
func ValidatePackageStateSpec(spec *PackageStateSpec) error {
	if spec == nil {
		return nil
	}
	mode := strings.TrimSpace(spec.CompatibilityImport)
	if mode == "" {
		if len(spec.OwnerIDs) != 0 {
			return fmt.Errorf("package_state.owner_ids requires compatibility_import: package-owned")
		}
		return nil
	}
	if mode != "package-owned" {
		return fmt.Errorf("package_state.compatibility_import: %q is invalid (expected package-owned)", mode)
	}
	if len(spec.OwnerIDs) == 0 {
		return fmt.Errorf("package_state.owner_ids is required for package-owned compatibility import")
	}
	seen := map[string]bool{}
	for _, raw := range spec.OwnerIDs {
		ownerID := strings.TrimSpace(raw)
		if ownerID == "" || ownerID != raw {
			return fmt.Errorf("package_state.owner_ids contains an empty or untrimmed owner ID")
		}
		for _, character := range ownerID {
			if character < 0x20 || character > 0x7e {
				return fmt.Errorf("package_state.owner_ids owner %q must use printable ASCII", ownerID)
			}
		}
		if seen[ownerID] {
			return fmt.Errorf("package_state.owner_ids contains duplicate owner %q", ownerID)
		}
		seen[ownerID] = true
	}
	return nil
}

// NormalizePackageManifestYAMLAliases merges deprecated primitive / requires_primitives keys into Capability / RequiresCapabilities.
func NormalizePackageManifestYAMLAliases(m *PackageManifest) {
	if m == nil {
		return
	}
	if strings.TrimSpace(m.Capability) == "" {
		m.Capability = strings.TrimSpace(m.PrimitiveYAMLDeprecated)
	}
	if len(m.RequiresCapabilities) == 0 && len(m.RequiresPrimitivesYAMLDeprecated) > 0 {
		m.RequiresCapabilities = append([]string(nil), m.RequiresPrimitivesYAMLDeprecated...)
	}
}

// ValidateProvider checks optional provider metadata (platform/vendor id for filtering).
func ValidateProvider(s string) error {
	return validateOptionalMetadataString(s, "provider")
}

// ValidateCapabilityID checks optional dotted capability id (e.g. cli.codex) — see docs/concepts/capabilities.md.
func ValidateCapabilityID(s string) error {
	return validateOptionalMetadataString(s, "capability")
}

// ValidatePrimitive is deprecated: use ValidateCapabilityID. Kept for transitional call sites.
func ValidatePrimitive(s string) error {
	return ValidateCapabilityID(s)
}

var packageVersionPattern = regexp.MustCompile(`^v?[0-9]+(?:\.[0-9]+){2}(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
var packageImageDigestPattern = regexp.MustCompile(`@sha256:[0-9a-fA-F]{64}$`)

// ValidatePackageVersion checks optional package version metadata.
// Keep this semver-shaped so tarball names and CDN paths stay predictable.
func ValidatePackageVersion(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !packageVersionPattern.MatchString(s) {
		return fmt.Errorf("version: %q is not semver-like (expected 1.2.3, 1.2.3-rc1, or v1.2.3)", s)
	}
	return nil
}

func ValidatePackageImageSpec(img *PackageImageSpec) error {
	if img == nil {
		return nil
	}
	source := strings.TrimSpace(img.Source)
	ref := strings.TrimSpace(img.Ref)
	pullPolicy := strings.TrimSpace(img.PullPolicy)
	switch source {
	case "", "registry":
	default:
		return fmt.Errorf("image.source: %q is invalid (expected registry)", source)
	}
	switch pullPolicy {
	case "", "never", "if-missing":
	default:
		return fmt.Errorf("image.pull_policy: %q is invalid (expected never or if-missing)", pullPolicy)
	}
	if ref == "" {
		if source != "" || pullPolicy != "" {
			return fmt.Errorf("image.ref is required when image metadata is set")
		}
		return nil
	}
	if strings.Contains(ref, "@") && !packageImageDigestPattern.MatchString(ref) {
		return fmt.Errorf("image.ref: %q has an invalid digest-pinned format", ref)
	}
	return nil
}

func validateOptionalMetadataString(s, field string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > 256 {
		return fmt.Errorf("%s: length exceeds 256", field)
	}
	for _, r := range s {
		if r < 0x20 {
			return fmt.Errorf("%s: control characters not allowed", field)
		}
	}
	return nil
}

var validScriptContractInjectables = map[string]struct{}{
	"workdir":       {},
	"workflow_name": {},
	"script_dir":    {},
	"package_root":  {},
	"assets_dir":    {},
	"dockpipe_bin":  {},
}

func ValidateScriptContractInjectable(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if _, ok := validScriptContractInjectables[s]; ok {
		return nil
	}
	return fmt.Errorf("script_contract.inject: unknown injectable %q (expected one of: workdir, workflow_name, script_dir, package_root, assets_dir, dockpipe_bin)", s)
}

func ValidatePackageBuildSpec(spec *PackageBuildSpec) error {
	if spec == nil || spec.Source == nil {
		return nil
	}
	return validateRelativePackageScriptPath(spec.Source.Script, "build.source.script")
}

func ValidatePackageTestSpec(test *PackageTestSpec) error {
	if test == nil || strings.TrimSpace(test.Script) == "" {
		return nil
	}
	return validateRelativePackageScriptPath(test.Script, "test.script")
}

func validateRelativePackageScriptPath(raw, field string) error {
	script := strings.TrimSpace(raw)
	if script == "" {
		return fmt.Errorf("%s is required when %s is set", field, strings.TrimSuffix(field, ".script"))
	}
	if filepath.IsAbs(script) {
		return fmt.Errorf("%s must be relative to package.yml", field)
	}
	if strings.Contains(script, `\`) {
		return fmt.Errorf("%s must use forward slashes", field)
	}
	cleaned := path.Clean(script)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%s must stay inside the package tree", field)
	}
	return nil
}
