// Package imageartifact constructs deterministic build-source image artifact manifests.
package imageartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// MarshalJSON writes the stable indented JSON form used by runtime artifact files.
func MarshalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ManifestInput contains the source and provenance used to construct a build image artifact.
type ManifestInput struct {
	RepoRoot          string
	WorkflowName      string
	PackageName       string
	ImageKey          string
	ImageRef          string
	BuildDir          string
	ContextDir        string
	PolicyFingerprint string
	Provenance        domain.ImageArtifactProvenance
}

// BuildManifest constructs the planned image artifact for a deterministic build tree.
func BuildManifest(input ManifestInput) (*domain.ImageArtifactManifest, error) {
	provenance := NormalizeProvenance(input.Provenance)
	buildFingerprint, err := fingerprintDirTree(input.BuildDir)
	if err != nil {
		return nil, err
	}
	buildSpec := &domain.CompiledImageBuildSpec{
		Context:    relOrAbs(input.RepoRoot, input.ContextDir),
		Dockerfile: relOrAbs(input.RepoRoot, filepath.Join(input.BuildDir, "Dockerfile")),
	}
	sourceFingerprint, err := domain.FingerprintJSON(struct {
		ImageRef         string                         `json:"image_ref"`
		Build            *domain.CompiledImageBuildSpec `json:"build"`
		BuildFingerprint string                         `json:"build_fingerprint"`
	}{
		ImageRef:         input.ImageRef,
		Build:            buildSpec,
		BuildFingerprint: buildFingerprint,
	})
	if err != nil {
		return nil, err
	}
	fingerprint, err := domain.FingerprintJSON(struct {
		SourceFingerprint string                         `json:"source_fingerprint"`
		Provenance        domain.ImageArtifactProvenance `json:"provenance,omitempty"`
	}{
		SourceFingerprint: sourceFingerprint,
		Provenance:        provenance,
	})
	if err != nil {
		return nil, err
	}
	return &domain.ImageArtifactManifest{
		Schema:                      3,
		Kind:                        domain.ImageArtifactManifestKind,
		WorkflowName:                strings.TrimSpace(input.WorkflowName),
		PackageName:                 strings.TrimSpace(input.PackageName),
		ImageKey:                    strings.TrimSpace(input.ImageKey),
		Source:                      string(domain.ImageSourceBuild),
		ArtifactState:               string(domain.ImageArtifactPlanned),
		Fingerprint:                 fingerprint,
		SourceFingerprint:           sourceFingerprint,
		SecurityManifestFingerprint: strings.TrimSpace(input.PolicyFingerprint),
		ImageRef:                    strings.TrimSpace(input.ImageRef),
		Build:                       buildSpec,
		Provenance:                  provenance,
	}, nil
}

// NormalizeProvenance trims the authored provenance fields used in artifact fingerprints.
func NormalizeProvenance(p domain.ImageArtifactProvenance) domain.ImageArtifactProvenance {
	return domain.ImageArtifactProvenance{
		Runtime:         strings.TrimSpace(p.Runtime),
		Resolver:        strings.TrimSpace(p.Resolver),
		Isolate:         strings.TrimSpace(p.Isolate),
		PackageVersion:  strings.TrimSpace(p.PackageVersion),
		DockpipeVersion: strings.TrimSpace(p.DockpipeVersion),
	}
}

func fingerprintDirTree(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			_, _ = io.WriteString(h, "dir:"+rel+"\n")
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		_, _ = io.WriteString(h, "file:"+rel+"\n")
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		_, _ = io.WriteString(h, "\n")
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
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
