package imageartifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestBuildManifestSeparatesSourceFromPolicy(t *testing.T) {
	wd := t.TempDir()
	buildDir := filepath.Join(wd, "templates", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input := ManifestInput{
		RepoRoot:          wd,
		WorkflowName:      " wf ",
		PackageName:       " pkg ",
		ImageKey:          " codex ",
		ImageRef:          "dockpipe-codex:1.2.3",
		BuildDir:          buildDir,
		ContextDir:        wd,
		PolicyFingerprint: " sha256:policy-a ",
		Provenance: domain.ImageArtifactProvenance{
			Isolate:         " codex ",
			PackageVersion:  " 1.2.3 ",
			DockpipeVersion: " 1.2.3 ",
		},
	}
	a, err := BuildManifest(input)
	if err != nil {
		t.Fatal(err)
	}
	input.PolicyFingerprint = "sha256:policy-b"
	b, err := BuildManifest(input)
	if err != nil {
		t.Fatal(err)
	}
	if a.SourceFingerprint != b.SourceFingerprint {
		t.Fatalf("source fingerprint should ignore runtime policy: %q != %q", a.SourceFingerprint, b.SourceFingerprint)
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("artifact fingerprint should ignore runtime-only policy: %q != %q", a.Fingerprint, b.Fingerprint)
	}
	if a.SecurityManifestFingerprint == b.SecurityManifestFingerprint {
		t.Fatalf("security manifest fingerprint should keep policy distinction")
	}

	const sourceFingerprint = "sha256:940b0a3eb7a4b1706f96214d19943aa3ff52a9983bba35915f79585a9d862b7b"
	const artifactFingerprint = "sha256:ae5afa7a7ade448a343030109e9c4ae9195c1750093236644681f68ec0e5d6ac"
	if a.Schema != 3 || a.Kind != domain.ImageArtifactManifestKind || a.Source != "build" || a.ArtifactState != "planned" {
		t.Fatalf("unexpected manifest contract: %+v", a)
	}
	if a.WorkflowName != "wf" || a.PackageName != "pkg" || a.ImageKey != "codex" || a.ImageRef != "dockpipe-codex:1.2.3" {
		t.Fatalf("unexpected normalized identity: %+v", a)
	}
	if a.SourceFingerprint != sourceFingerprint || a.Fingerprint != artifactFingerprint || a.SecurityManifestFingerprint != "sha256:policy-a" {
		t.Fatalf("unexpected fingerprint separation: %+v", a)
	}
	if a.Build == nil || a.Build.Context != "." || a.Build.Dockerfile != "templates/core/assets/images/codex/Dockerfile" {
		t.Fatalf("unexpected relative build paths: %+v", a.Build)
	}
	if a.Provenance != (domain.ImageArtifactProvenance{Isolate: "codex", PackageVersion: "1.2.3", DockpipeVersion: "1.2.3"}) {
		t.Fatalf("unexpected provenance: %+v", a.Provenance)
	}

	gotJSON, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	gotJSON = append(gotJSON, '\n')
	wantJSON := "{\n" +
		"  \"schema\": 3,\n" +
		"  \"kind\": \"docker-image-artifact\",\n" +
		"  \"workflow_name\": \"wf\",\n" +
		"  \"package_name\": \"pkg\",\n" +
		"  \"image_key\": \"codex\",\n" +
		"  \"source\": \"build\",\n" +
		"  \"artifact_state\": \"planned\",\n" +
		"  \"fingerprint\": \"" + artifactFingerprint + "\",\n" +
		"  \"source_fingerprint\": \"" + sourceFingerprint + "\",\n" +
		"  \"security_manifest_fingerprint\": \"sha256:policy-a\",\n" +
		"  \"image_ref\": \"dockpipe-codex:1.2.3\",\n" +
		"  \"build\": {\n" +
		"    \"context\": \".\",\n" +
		"    \"dockerfile\": \"templates/core/assets/images/codex/Dockerfile\"\n" +
		"  },\n" +
		"  \"provenance\": {\n" +
		"    \"isolate\": \"codex\",\n" +
		"    \"package_version\": \"1.2.3\",\n" +
		"    \"dockpipe_version\": \"1.2.3\"\n" +
		"  }\n" +
		"}\n"
	if string(gotJSON) != wantJSON {
		t.Fatalf("unexpected artifact JSON:\n%s", gotJSON)
	}
}
