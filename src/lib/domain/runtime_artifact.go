package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	modelruntimeartifact "dockpipe/src/lib/model/runtimeartifact"
)

const (
	RuntimeManifestKind       = modelruntimeartifact.RuntimeManifestKind
	ImageArtifactManifestKind = modelruntimeartifact.ImageArtifactManifestKind
	RuntimeManifestDirName    = modelruntimeartifact.RuntimeManifestDirName
	RuntimeManifestFileName   = modelruntimeartifact.RuntimeManifestFileName
	ImageArtifactFileName     = modelruntimeartifact.ImageArtifactFileName
	StepArtifactsDirName      = modelruntimeartifact.StepArtifactsDirName
)

type CompiledRuntimeManifest = modelruntimeartifact.CompiledRuntimeManifest
type PolicySources = modelruntimeartifact.PolicySources
type CompiledSecurityPolicy = modelruntimeartifact.CompiledSecurityPolicy
type CompiledNetworkPolicy = modelruntimeartifact.CompiledNetworkPolicy
type CompiledFilesystemPolicy = modelruntimeartifact.CompiledFilesystemPolicy
type CompiledProcessPolicy = modelruntimeartifact.CompiledProcessPolicy
type CompiledResourceLimits = modelruntimeartifact.CompiledResourceLimits
type CompiledImageSelection = modelruntimeartifact.CompiledImageSelection
type CompiledImageBuildSpec = modelruntimeartifact.CompiledImageBuildSpec
type ImageArtifactManifest = modelruntimeartifact.ImageArtifactManifest
type ImageArtifactProvenance = modelruntimeartifact.ImageArtifactProvenance

func ValidateCompiledRuntimeManifest(m *CompiledRuntimeManifest) error {
	if m == nil {
		return nil
	}
	if m.Schema <= 0 {
		return fmt.Errorf("schema must be > 0")
	}
	if strings.TrimSpace(m.Kind) == "" {
		m.Kind = RuntimeManifestKind
	}
	if m.Kind != RuntimeManifestKind {
		return fmt.Errorf("kind %q must be %q", m.Kind, RuntimeManifestKind)
	}
	if err := validateEnum("policy_profile", PolicyProfile(m.PolicyProfile), validPolicyProfiles); err != nil {
		return err
	}
	if err := ValidateCompiledSecurityPolicy(&m.Security); err != nil {
		return err
	}
	if err := ValidateCompiledImageSelection(&m.Image); err != nil {
		return err
	}
	return nil
}

func ValidateCompiledSecurityPolicy(p *CompiledSecurityPolicy) error {
	if p == nil {
		return nil
	}
	if err := validateEnum("preset", PolicyProfile(p.Preset), validPolicyProfiles); err != nil {
		return err
	}
	if err := validateEnum("network.mode", NetworkMode(p.Network.Mode), validNetworkModes); err != nil {
		return err
	}
	if err := validateEnum("network.enforcement", NetworkEnforcement(p.Network.Enforcement), validNetworkEnforcement); err != nil {
		return err
	}
	mode := NetworkMode(strings.TrimSpace(p.Network.Mode))
	enforcement := NetworkEnforcement(strings.TrimSpace(p.Network.Enforcement))
	if mode == NetworkModeOffline && (len(p.Network.Allow) > 0 || len(p.Network.Block) > 0) {
		return fmt.Errorf("network.mode offline cannot be combined with allow/block rules")
	}
	if mode == NetworkModeAllowlist && len(p.Network.Allow) == 0 {
		return fmt.Errorf("network.mode allowlist requires at least one allow rule")
	}
	switch mode {
	case NetworkModeOffline, NetworkModeInternet:
		if enforcement != "" && enforcement != NetworkEnforcementNative {
			return fmt.Errorf("network.mode %s requires native enforcement", p.Network.Mode)
		}
	case NetworkModeAllowlist, NetworkModeRestricted:
		if enforcement == NetworkEnforcementNative {
			return fmt.Errorf("network.mode %s cannot use native enforcement", p.Network.Mode)
		}
	}
	if err := validateEnum("filesystem.root", FilesystemRootPolicy(p.FS.Root), validFilesystemRoots); err != nil {
		return err
	}
	if err := validateEnum("filesystem.writes", FilesystemWritePolicy(p.FS.Writes), validFilesystemWrites); err != nil {
		return err
	}
	if err := validateEnum("process.user", ProcessUserPolicy(p.Process.User), validProcessUsers); err != nil {
		return err
	}
	if p.Process.PIDLimit < 0 {
		return fmt.Errorf("process.pid_limit must be >= 0")
	}
	return nil
}

func ValidateCompiledImageSelection(i *CompiledImageSelection) error {
	if i == nil {
		return nil
	}
	if err := validateEnum("image.source", ImageSource(i.Source), validImageSources); err != nil {
		return err
	}
	if err := validateEnum("image.auto_build", ImageAutoBuildMode(i.AutoBuild), validImageAutoBuildModes); err != nil {
		return err
	}
	if err := validateEnum("image.pull_policy", ImagePullPolicy(i.PullPolicy), validImagePullPolicies); err != nil {
		return err
	}
	switch ImageSource(strings.TrimSpace(i.Source)) {
	case ImageSourceBuild:
		if i.Build == nil {
			return fmt.Errorf("image.source build requires build settings")
		}
		if strings.TrimSpace(i.Build.Context) == "" {
			return fmt.Errorf("image.build.context is required for build source")
		}
		if strings.TrimSpace(i.Build.Dockerfile) == "" {
			return fmt.Errorf("image.build.dockerfile is required for build source")
		}
	case ImageSourceRegistry:
		if strings.TrimSpace(i.Ref) == "" {
			return fmt.Errorf("image.source registry requires ref")
		}
		if i.AutoBuild != "" {
			return fmt.Errorf("image.source registry cannot use auto_build")
		}
	}
	return nil
}

func ValidateImageArtifactManifest(m *ImageArtifactManifest) error {
	if m == nil {
		return nil
	}
	if m.Schema <= 0 {
		return fmt.Errorf("schema must be > 0")
	}
	if strings.TrimSpace(m.Kind) == "" {
		m.Kind = ImageArtifactManifestKind
	}
	if m.Kind != ImageArtifactManifestKind {
		return fmt.Errorf("kind %q must be %q", m.Kind, ImageArtifactManifestKind)
	}
	if err := validateEnum("source", ImageSource(m.Source), validImageSources); err != nil {
		return err
	}
	if err := validateEnum("artifact_state", ImageArtifactState(m.ArtifactState), validImageArtifactStates); err != nil {
		return err
	}
	source := ImageSource(strings.TrimSpace(m.Source))
	if source == ImageSourceBuild && m.Build == nil {
		return fmt.Errorf("build source requires build settings")
	}
	if source == ImageSourceRegistry && strings.TrimSpace(m.ImageRef) == "" {
		return fmt.Errorf("registry source requires image_ref")
	}
	if strings.TrimSpace(m.Fingerprint) == "" {
		return fmt.Errorf("fingerprint is required")
	}
	return nil
}

func validateEnum[T ~string](field string, value T, allowed map[T]struct{}) error {
	value = T(strings.TrimSpace(string(value)))
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("%s %q is invalid", field, value)
}

func FingerprintJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func RuntimeManifestPathForStep(stepID string) string {
	return StepArtifactsDirName + "/" + sanitizeStepArtifactID(stepID) + ".runtime.effective.json"
}

func ImageArtifactPathForStep(stepID string) string {
	return StepArtifactsDirName + "/" + sanitizeStepArtifactID(stepID) + ".image-artifact.json"
}

func sanitizeStepArtifactID(stepID string) string {
	stepID = strings.TrimSpace(stepID)
	if stepID == "" {
		return "step"
	}
	var out []rune
	lastDash := false
	for _, r := range stepID {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_'
		if ok {
			out = append(out, r)
			lastDash = false
			continue
		}
		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}
	s := strings.Trim(strings.ToLower(string(out)), "-")
	if s == "" {
		return "step"
	}
	return s
}
