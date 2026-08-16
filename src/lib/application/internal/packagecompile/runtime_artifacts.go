package packagecompile

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dockpipe/src/lib/application/internal/imageartifact"
	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/application/internal/textvalue"
	"dockpipe/src/lib/domain"
)

func writeCompiledWorkflowRuntimeArtifacts(workdir, staging, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest) error {
	rm, im, stepArtifacts, err := compileWorkflowRuntimeArtifacts(workdir, staging, pkgName, wf, pm)
	if err != nil {
		return err
	}
	manifestDir := filepath.Join(staging, domain.RuntimeManifestDirName)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(manifestDir, domain.RuntimeManifestFileName), rm); err != nil {
		return err
	}
	if im != nil {
		if err := writeJSONFile(filepath.Join(manifestDir, domain.ImageArtifactFileName), im); err != nil {
			return err
		}
	}
	for _, a := range stepArtifacts {
		if a.Manifest == nil {
			continue
		}
		p := filepath.Join(manifestDir, domain.RuntimeManifestPathForStep(a.StepID))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := writeJSONFile(p, a.Manifest); err != nil {
			return err
		}
		if a.Image != nil {
			ip := filepath.Join(manifestDir, domain.ImageArtifactPathForStep(a.StepID))
			if err := os.MkdirAll(filepath.Dir(ip), 0o755); err != nil {
				return err
			}
			if err := writeJSONFile(ip, a.Image); err != nil {
				return err
			}
		}
	}
	return nil
}

type compiledStepRuntimeArtifacts struct {
	StepID   string
	Manifest *domain.CompiledRuntimeManifest
	Image    *domain.ImageArtifactManifest
}

func compileWorkflowRuntimeArtifacts(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, []compiledStepRuntimeArtifacts, error) {
	policyProfile := runtimepolicy.NormalizeWorkflowPolicyProfile(wf)
	security, policySources := runtimepolicy.CompileSecurityPolicyForWorkflow(wf, policyProfile)
	rm := &domain.CompiledRuntimeManifest{
		Schema:          2,
		Kind:            domain.RuntimeManifestKind,
		WorkflowName:    strings.TrimSpace(wf.Name),
		PackageName:     strings.TrimSpace(pkgName),
		RuntimeProfile:  strings.TrimSpace(wf.Runtime),
		ResolverProfile: strings.TrimSpace(wf.Resolver),
		PolicyProfile:   policyProfile,
		PolicySources:   policySources,
		Security:        security,
	}

	policyFingerprint, err := domain.FingerprintJSON(rm.Security)
	if err != nil {
		return nil, nil, nil, err
	}
	rm.PolicyFingerprint = policyFingerprint

	imageSel, artifact, err := selectCompiledImageArtifact(workdir, sourceRoot, pkgName, wf, pm, policyFingerprint)
	if err != nil {
		return nil, nil, nil, err
	}
	rm.Image = imageSel
	if fp, err := domain.FingerprintJSON(rm.Image); err == nil {
		rm.ImageFingerprint = fp
	}
	rm.EnforcementSummaries = runtimepolicy.CompiledEnforcementSummaries(rm)
	rm.RuleIDs = runtimepolicy.CompiledRuleIDs(rm)
	if err := domain.ValidateCompiledRuntimeManifest(rm); err != nil {
		return nil, nil, nil, err
	}
	if artifact != nil {
		if err := domain.ValidateImageArtifactManifest(artifact); err != nil {
			return nil, nil, nil, err
		}
	}
	stepArtifacts, err := compileStepRuntimeArtifacts(workdir, sourceRoot, pkgName, wf, pm)
	if err != nil {
		return nil, nil, nil, err
	}
	return rm, artifact, stepArtifacts, nil
}

func compileStepRuntimeArtifacts(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest) ([]compiledStepRuntimeArtifacts, error) {
	if wf == nil || len(wf.Steps) == 0 {
		return nil, nil
	}
	out := make([]compiledStepRuntimeArtifacts, 0, len(wf.Steps))
	for i, step := range wf.Steps {
		if step.IsHostStep() || step.UsesPackagedWorkflow() {
			continue
		}
		stepID := compiledStepID(step, i)
		rm, im, err := compileStepRuntimeManifest(workdir, sourceRoot, pkgName, wf, pm, step, stepID)
		if err != nil {
			return nil, err
		}
		out = append(out, compiledStepRuntimeArtifacts{
			StepID:   stepID,
			Manifest: rm,
			Image:    im,
		})
	}
	return out, nil
}

func compileStepRuntimeManifest(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest, step domain.Step, stepID string) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, error) {
	policyProfile := runtimepolicy.NormalizeWorkflowPolicyProfile(wf)
	if p := strings.TrimSpace(step.Security.Profile); p != "" {
		policyProfile = p
	}
	security, policySources := runtimepolicy.CompileSecurityPolicyForWorkflow(wf, policyProfile)
	stepOverride := runtimepolicy.ApplyStepSecurityOverrides(&security, step)
	if strings.TrimSpace(step.Security.Profile) != "" {
		stepOverride = true
	}
	security.Preset = policyProfile
	security.Network.Enforcement = runtimepolicy.CompiledNetworkEnforcement(security.Network.Mode, policyProfile)
	security.Network.InternalDNS = true

	rm := &domain.CompiledRuntimeManifest{
		Schema:          2,
		Kind:            domain.RuntimeManifestKind,
		WorkflowName:    strings.TrimSpace(wf.Name),
		PackageName:     strings.TrimSpace(pkgName),
		StepID:          stepID,
		RuntimeProfile:  firstNonEmptyString(strings.TrimSpace(step.Runtime), strings.TrimSpace(wf.Runtime)),
		ResolverProfile: firstNonEmptyString(strings.TrimSpace(step.Resolver), strings.TrimSpace(wf.Resolver)),
		PolicyProfile:   policyProfile,
		PolicySources: domain.PolicySources{
			EngineDefault:    policySources.EngineDefault,
			RuntimeBaseline:  firstNonEmptyString(stepBaselineName(step, wf), policySources.RuntimeBaseline),
			PolicyProfile:    policyProfile,
			WorkflowOverride: policySources.WorkflowOverride,
			StepOverride:     stepOverride,
		},
		Security: security,
	}
	policyFingerprint, err := domain.FingerprintJSON(rm.Security)
	if err != nil {
		return nil, nil, err
	}
	rm.PolicyFingerprint = policyFingerprint
	imageSel, artifact, err := selectCompiledImageArtifactForStep(workdir, sourceRoot, pkgName, wf, pm, step, stepID, policyFingerprint)
	if err != nil {
		return nil, nil, err
	}
	rm.Image = imageSel
	if fp, err := domain.FingerprintJSON(rm.Image); err == nil {
		rm.ImageFingerprint = fp
	}
	rm.EnforcementSummaries = runtimepolicy.CompiledEnforcementSummaries(rm)
	rm.RuleIDs = runtimepolicy.CompiledRuleIDs(rm)
	if err := domain.ValidateCompiledRuntimeManifest(rm); err != nil {
		return nil, nil, err
	}
	if artifact != nil {
		if err := domain.ValidateImageArtifactManifest(artifact); err != nil {
			return nil, nil, err
		}
	}
	return rm, artifact, nil
}

func compiledStepID(step domain.Step, idx int) string {
	if s := strings.TrimSpace(step.ID); s != "" {
		return s
	}
	return "step-" + strings.TrimSpace(strconv.Itoa(idx+1))
}

func stepBaselineName(step domain.Step, wf *domain.Workflow) string {
	return firstNonEmptyString(
		strings.TrimSpace(step.Runtime),
		strings.TrimSpace(step.Isolate),
		strings.TrimSpace(step.Resolver),
		strings.TrimSpace(wf.Runtime),
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Resolver),
		"container-default",
	)
}

func writeJSONFile(path string, v any) error {
	b, err := imageartifact.MarshalJSON(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func relOrAbs(base, path string) string {
	if base == "" {
		return path
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}

func firstNonEmptyString(values ...string) string {
	return textvalue.FirstNonEmpty(values...)
}
