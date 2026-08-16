package packagecompile

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/imageartifact"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func selectCompiledImageArtifact(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	provenance := workflowImageArtifactProvenance(workdir, pm, wf)
	packages := normalizeAptPackages(wf.Image.Packages.Apt)
	if pm != nil && strings.TrimSpace(pm.Image.Ref) != "" {
		imageKey := packageImageKey(pm, wf)
		if len(packages) == 0 {
			sel, artifact, _, err := selectPackageImageArtifact(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), "", imageKey, pm, policyFingerprint, provenance)
			return sel, artifact, err
		}
		baseRef := strings.TrimSpace(pm.Image.Ref)
		derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, "workflow", baseRef, packages)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		ref := derivedImageRef(baseRef, packages)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(sourceRoot, sourceRoot),
			Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), imageKey, ref, derived, sourceRoot, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}
	identity := firstNonEmptyString(
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Resolver),
		strings.TrimSpace(wf.Runtime),
	)
	if identity == "" {
		return domain.CompiledImageSelection{}, nil, nil
	}

	if image, dockerfileDir, ok := infrastructure.TemplateBuild(workdir, identity); ok {
		manifestRoot := workdir
		contextDir := workdir
		if localDir := workflowLocalImageDir(sourceRoot, identity); localDir != "" {
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = localDir
		}
		ref := infrastructure.MaybeVersionTag(workdir, image)
		if len(packages) > 0 {
			derived, err := writeDerivedAptImageBuild(sourceRoot, "workflow", ref, dockerfileDir, packages)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = derived
			ref = derivedImageRef(ref, packages)
		}
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(manifestRoot, contextDir),
			Dockerfile: relOrAbs(manifestRoot, filepath.Join(dockerfileDir, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(manifestRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), identity, ref, dockerfileDir, contextDir, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}

	if len(packages) > 0 {
		derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, "workflow", identity, packages)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		ref := derivedImageRef(identity, packages)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(sourceRoot, sourceRoot),
			Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), identity, ref, derived, sourceRoot, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}
	return registryImageSelection(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), "", identity, identity, "never", policyFingerprint, provenance)
}

func selectCompiledImageArtifactForStep(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest, step domain.Step, stepID, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	provenance := stepImageArtifactProvenance(workdir, pm, wf, step)
	packages := normalizeAptPackages(append(append([]string{}, wf.Image.Packages.Apt...), step.Image.Packages.Apt...))
	if !stepHasImageSelectionOverride(step) {
		if pm != nil && strings.TrimSpace(pm.Image.Ref) != "" {
			if len(packages) == 0 {
				sel, artifact, _, err := selectPackageImageArtifact(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, stepID, pm, policyFingerprint, provenance)
				return sel, artifact, err
			}
			baseRef := strings.TrimSpace(pm.Image.Ref)
			derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, stepID, baseRef, packages)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			ref := derivedImageRef(baseRef, packages)
			buildSpec := &domain.CompiledImageBuildSpec{
				Context:    relOrAbs(sourceRoot, sourceRoot),
				Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
			}
			sel := domain.CompiledImageSelection{
				Source:    "build",
				Ref:       ref,
				AutoBuild: "if-stale",
				Build:     buildSpec,
			}
			artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, derived, sourceRoot, policyFingerprint, provenance)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			return sel, artifact, nil
		}
	}
	identity := firstNonEmptyString(
		strings.TrimSpace(step.Isolate),
		strings.TrimSpace(step.Runtime),
		strings.TrimSpace(step.Resolver),
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Runtime),
		strings.TrimSpace(wf.Resolver),
	)
	if identity == "" {
		return domain.CompiledImageSelection{}, nil, nil
	}

	if image, dockerfileDir, ok := infrastructure.TemplateBuild(workdir, identity); ok {
		manifestRoot := workdir
		contextDir := workdir
		if localDir := workflowLocalImageDir(sourceRoot, identity); localDir != "" {
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = localDir
		}
		ref := infrastructure.MaybeVersionTag(workdir, image)
		if len(packages) > 0 {
			derived, err := writeDerivedAptImageBuild(sourceRoot, stepID, ref, dockerfileDir, packages)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = derived
			ref = derivedImageRef(ref, packages)
		}
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(manifestRoot, contextDir),
			Dockerfile: relOrAbs(manifestRoot, filepath.Join(dockerfileDir, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(manifestRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, dockerfileDir, contextDir, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}

	if len(packages) > 0 {
		derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, stepID, identity, packages)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		ref := derivedImageRef(identity, packages)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(sourceRoot, sourceRoot),
			Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, derived, sourceRoot, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}
	return registryImageSelection(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, stepID, identity, "never", policyFingerprint, provenance)
}

func stepHasImageSelectionOverride(step domain.Step) bool {
	return strings.TrimSpace(step.Isolate) != "" ||
		strings.TrimSpace(step.Runtime) != "" ||
		strings.TrimSpace(step.Resolver) != ""
}

func workflowLocalImageDir(sourceRoot, identity string) string {
	sourceRoot = strings.TrimSpace(sourceRoot)
	identity = strings.TrimSpace(identity)
	if sourceRoot == "" || identity == "" {
		return ""
	}
	dir := filepath.Join(sourceRoot, "assets", "images", identity)
	if st, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil && !st.IsDir() {
		return dir
	}
	return ""
}

func workflowImageArtifactProvenance(workdir string, pm *domain.PackageManifest, wf *domain.Workflow) domain.ImageArtifactProvenance {
	p := baseImageArtifactProvenance(workdir, pm)
	if wf == nil {
		return p
	}
	switch {
	case strings.TrimSpace(wf.Isolate) != "":
		p.Isolate = strings.TrimSpace(wf.Isolate)
	case strings.TrimSpace(wf.Resolver) != "":
		p.Resolver = strings.TrimSpace(wf.Resolver)
	case strings.TrimSpace(wf.Runtime) != "":
		p.Runtime = strings.TrimSpace(wf.Runtime)
	}
	return p
}

func stepImageArtifactProvenance(workdir string, pm *domain.PackageManifest, wf *domain.Workflow, step domain.Step) domain.ImageArtifactProvenance {
	p := baseImageArtifactProvenance(workdir, pm)
	switch {
	case strings.TrimSpace(step.Isolate) != "":
		p.Isolate = strings.TrimSpace(step.Isolate)
	case strings.TrimSpace(step.Resolver) != "":
		p.Resolver = strings.TrimSpace(step.Resolver)
	case strings.TrimSpace(step.Runtime) != "":
		p.Runtime = strings.TrimSpace(step.Runtime)
	case wf != nil && strings.TrimSpace(wf.Isolate) != "":
		p.Isolate = strings.TrimSpace(wf.Isolate)
	case wf != nil && strings.TrimSpace(wf.Resolver) != "":
		p.Resolver = strings.TrimSpace(wf.Resolver)
	case wf != nil && strings.TrimSpace(wf.Runtime) != "":
		p.Runtime = strings.TrimSpace(wf.Runtime)
	}
	return p
}

func baseImageArtifactProvenance(workdir string, pm *domain.PackageManifest) domain.ImageArtifactProvenance {
	p := domain.ImageArtifactProvenance{
		DockpipeVersion: authoredPackageVersion(workdir),
	}
	if pm != nil {
		p.PackageVersion = strings.TrimSpace(pm.Version)
	}
	return p
}

func selectPackageImageArtifact(workflowName, packageName, stepID, imageKey string, pm *domain.PackageManifest, policyFingerprint string, provenance domain.ImageArtifactProvenance) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, bool, error) {
	if pm == nil {
		return domain.CompiledImageSelection{}, nil, false, nil
	}
	ref := strings.TrimSpace(pm.Image.Ref)
	if ref == "" {
		return domain.CompiledImageSelection{}, nil, false, nil
	}
	pullPolicy := firstNonEmptyString(strings.TrimSpace(pm.Image.PullPolicy), "never")
	sel, artifact, err := registryImageSelection(workflowName, packageName, stepID, imageKey, ref, pullPolicy, policyFingerprint, provenance)
	return sel, artifact, true, err
}

func packageImageKey(pm *domain.PackageManifest, wf *domain.Workflow) string {
	if pm != nil && strings.TrimSpace(pm.Name) != "" {
		return strings.TrimSpace(pm.Name)
	}
	if wf != nil && strings.TrimSpace(wf.Name) != "" {
		return strings.TrimSpace(wf.Name)
	}
	return "workflow-image"
}

func registryImageSelection(workflowName, packageName, stepID, imageKey, ref, pullPolicy, policyFingerprint string, provenance domain.ImageArtifactProvenance) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	provenance = imageartifact.NormalizeProvenance(provenance)
	expectedDigest := registryExpectedDigest(ref)
	sel := domain.CompiledImageSelection{
		Source:         "registry",
		Ref:            ref,
		PullPolicy:     pullPolicy,
		ExpectedDigest: expectedDigest,
	}
	sourceFingerprint, err := domain.FingerprintJSON(struct {
		StepID         string `json:"step_id,omitempty"`
		ImageKey       string `json:"image_key"`
		Ref            string `json:"ref"`
		PullPolicy     string `json:"pull_policy"`
		ExpectedDigest string `json:"expected_digest"`
	}{
		StepID:         stepID,
		ImageKey:       imageKey,
		Ref:            ref,
		PullPolicy:     pullPolicy,
		ExpectedDigest: expectedDigest,
	})
	if err != nil {
		return domain.CompiledImageSelection{}, nil, err
	}
	fingerprint, err := domain.FingerprintJSON(struct {
		SourceFingerprint string                         `json:"source_fingerprint"`
		Provenance        domain.ImageArtifactProvenance `json:"provenance,omitempty"`
	}{
		SourceFingerprint: sourceFingerprint,
		Provenance:        provenance,
	})
	if err != nil {
		return domain.CompiledImageSelection{}, nil, err
	}
	artifact := &domain.ImageArtifactManifest{
		Schema:                      3,
		Kind:                        domain.ImageArtifactManifestKind,
		WorkflowName:                workflowName,
		PackageName:                 packageName,
		StepID:                      stepID,
		ImageKey:                    imageKey,
		Source:                      "registry",
		ArtifactState:               "referenced",
		Fingerprint:                 fingerprint,
		SourceFingerprint:           sourceFingerprint,
		SecurityManifestFingerprint: policyFingerprint,
		ImageRef:                    ref,
		ExpectedDigest:              expectedDigest,
		Provenance:                  provenance,
	}
	return sel, artifact, nil
}

func registryExpectedDigest(ref string) string {
	ref = strings.TrimSpace(ref)
	if i := strings.LastIndex(ref, "@sha256:"); i >= 0 {
		return ref[i+1:]
	}
	return ""
}
