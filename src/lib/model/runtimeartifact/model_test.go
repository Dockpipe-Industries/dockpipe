package runtimeartifact

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWireConstants(t *testing.T) {
	want := map[string]string{
		"runtime manifest kind":        "dockpipe-runtime-manifest",
		"image artifact manifest kind": "docker-image-artifact",
		"runtime manifest directory":   ".dockpipe",
		"runtime manifest filename":    "runtime.effective.json",
		"image artifact filename":      "image-artifact.json",
		"step artifacts directory":     "steps",
	}
	got := map[string]string{
		"runtime manifest kind":        RuntimeManifestKind,
		"image artifact manifest kind": ImageArtifactManifestKind,
		"runtime manifest directory":   RuntimeManifestDirName,
		"runtime manifest filename":    RuntimeManifestFileName,
		"image artifact filename":      ImageArtifactFileName,
		"step artifacts directory":     StepArtifactsDirName,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime artifact wire constants changed:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestPayloadWireTags(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{"compiled runtime manifest", CompiledRuntimeManifest{}, []string{"schema", "kind", "workflow_name,omitempty", "package_name,omitempty", "step_id,omitempty", "runtime_profile,omitempty", "resolver_profile,omitempty", "policy_profile,omitempty", "policy_sources,omitempty", "policy_fingerprint,omitempty", "image_fingerprint,omitempty", "security", "image", "enforcement_summaries,omitempty", "rule_ids,omitempty"}},
		{"policy sources", PolicySources{}, []string{"engine_default,omitempty", "runtime_baseline,omitempty", "policy_profile,omitempty", "workflow_override,omitempty", "step_override,omitempty"}},
		{"compiled security policy", CompiledSecurityPolicy{}, []string{"preset,omitempty", "network,omitempty", "filesystem,omitempty", "process,omitempty"}},
		{"compiled network policy", CompiledNetworkPolicy{}, []string{"mode,omitempty", "enforcement,omitempty", "allow,omitempty", "block,omitempty", "internal_dns,omitempty"}},
		{"compiled filesystem policy", CompiledFilesystemPolicy{}, []string{"root,omitempty", "writes,omitempty", "writable_paths,omitempty", "temp_paths,omitempty"}},
		{"compiled process policy", CompiledProcessPolicy{}, []string{"user,omitempty", "no_new_privileges,omitempty", "drop_caps,omitempty", "add_caps,omitempty", "pid_limit,omitempty", "resources,omitempty"}},
		{"compiled resource limits", CompiledResourceLimits{}, []string{"cpu,omitempty", "memory,omitempty"}},
		{"compiled image selection", CompiledImageSelection{}, []string{"source,omitempty", "ref,omitempty", "auto_build,omitempty", "pull_policy,omitempty", "build,omitempty", "expected_digest,omitempty"}},
		{"compiled image build spec", CompiledImageBuildSpec{}, []string{"context,omitempty", "dockerfile,omitempty", "target,omitempty", "platform,omitempty", "args,omitempty"}},
		{"image artifact manifest", ImageArtifactManifest{}, []string{"schema", "kind", "workflow_name,omitempty", "package_name,omitempty", "step_id,omitempty", "image_key,omitempty", "source,omitempty", "artifact_state,omitempty", "fingerprint,omitempty", "source_fingerprint,omitempty", "security_manifest_fingerprint,omitempty", "image_ref,omitempty", "expected_digest,omitempty", "resolved_ref,omitempty", "image_id,omitempty", "repo_digest,omitempty", "build,omitempty", "provenance,omitempty"}},
		{"image artifact provenance", ImageArtifactProvenance{}, []string{"runtime,omitempty", "resolver,omitempty", "isolate,omitempty", "package_version,omitempty", "dockpipe_version,omitempty"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(tt.value)
			jsonTags := make([]string, 0, typ.NumField())
			yamlTags := make([]string, 0, typ.NumField())
			for i := 0; i < typ.NumField(); i++ {
				jsonTags = append(jsonTags, typ.Field(i).Tag.Get("json"))
				yamlTags = append(yamlTags, typ.Field(i).Tag.Get("yaml"))
			}
			if !reflect.DeepEqual(jsonTags, tt.want) {
				t.Fatalf("JSON wire tags changed:\nwant: %#v\ngot:  %#v", tt.want, jsonTags)
			}
			if !reflect.DeepEqual(yamlTags, tt.want) {
				t.Fatalf("YAML wire tags changed:\nwant: %#v\ngot:  %#v", tt.want, yamlTags)
			}
		})
	}
}

func TestPayloadJSONAndYAMLRoundTrip(t *testing.T) {
	build := &CompiledImageBuildSpec{
		Context:    ".",
		Dockerfile: "Dockerfile",
		Target:     "runtime",
		Platform:   "linux/amd64",
		Args:       map[string]string{"MODE": "release"},
	}
	runtimeManifest := CompiledRuntimeManifest{
		Schema:          1,
		Kind:            RuntimeManifestKind,
		WorkflowName:    "build",
		PackageName:     "demo",
		StepID:          "compile",
		RuntimeProfile:  "dockerfile",
		ResolverProfile: "codex",
		PolicyProfile:   "secure-default",
		PolicySources: PolicySources{
			EngineDefault:    true,
			RuntimeBaseline:  "base",
			PolicyProfile:    "secure-default",
			WorkflowOverride: true,
			StepOverride:     true,
		},
		PolicyFingerprint: "sha256:policy",
		ImageFingerprint:  "sha256:image",
		Security: CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: CompiledNetworkPolicy{
				Mode:        "allowlist",
				Enforcement: "proxy",
				Allow:       []string{"example.com"},
				Block:       []string{"blocked.example"},
				InternalDNS: true,
			},
			FS: CompiledFilesystemPolicy{
				Root:          "repo",
				Writes:        "allowlist",
				WritablePaths: []string{"out"},
				TempPaths:     []string{"tmp"},
			},
			Process: CompiledProcessPolicy{
				User:            "nonroot",
				NoNewPrivileges: true,
				DropCaps:        []string{"ALL"},
				AddCaps:         []string{"NET_BIND_SERVICE"},
				PIDLimit:        64,
				Resources: CompiledResourceLimits{
					CPU:    "2",
					Memory: "1g",
				},
			},
		},
		Image: CompiledImageSelection{
			Source:         "build",
			Ref:            "demo:latest",
			AutoBuild:      "always",
			PullPolicy:     "if-missing",
			Build:          build,
			ExpectedDigest: "sha256:expected",
		},
		EnforcementSummaries: []string{"network=proxy"},
		RuleIDs:              []string{"rule-1"},
	}
	imageManifest := ImageArtifactManifest{
		Schema:                      1,
		Kind:                        ImageArtifactManifestKind,
		WorkflowName:                "build",
		PackageName:                 "demo",
		StepID:                      "compile",
		ImageKey:                    "demo/compile",
		Source:                      "build",
		ArtifactState:               "materialized",
		Fingerprint:                 "sha256:image",
		SourceFingerprint:           "sha256:source",
		SecurityManifestFingerprint: "sha256:policy",
		ImageRef:                    "demo:latest",
		ExpectedDigest:              "sha256:expected",
		ResolvedRef:                 "demo@sha256:resolved",
		ImageID:                     "sha256:id",
		RepoDigest:                  "demo@sha256:repo",
		Build:                       build,
		Provenance: ImageArtifactProvenance{
			Runtime:         "dockerfile",
			Resolver:        "codex",
			Isolate:         "dockpipe-codex",
			PackageVersion:  "1.2.3",
			DockpipeVersion: "0.6.0",
		},
	}

	assertRoundTrip(t, "runtime manifest", runtimeManifest)
	assertRoundTrip(t, "image artifact manifest", imageManifest)
}

func assertRoundTrip[T any](t *testing.T, name string, want T) {
	t.Helper()

	jsonBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal %s JSON: %v", name, err)
	}
	var fromJSON T
	if err := json.Unmarshal(jsonBytes, &fromJSON); err != nil {
		t.Fatalf("unmarshal %s JSON: %v", name, err)
	}
	if !reflect.DeepEqual(fromJSON, want) {
		t.Fatalf("%s JSON round trip changed shape:\nwant: %#v\ngot:  %#v", name, want, fromJSON)
	}

	yamlBytes, err := yaml.Marshal(want)
	if err != nil {
		t.Fatalf("marshal %s YAML: %v", name, err)
	}
	var fromYAML T
	if err := yaml.Unmarshal(yamlBytes, &fromYAML); err != nil {
		t.Fatalf("unmarshal %s YAML: %v", name, err)
	}
	if !reflect.DeepEqual(fromYAML, want) {
		t.Fatalf("%s YAML round trip changed shape:\nwant: %#v\ngot:  %#v", name, want, fromYAML)
	}
}
