package domain

import "testing"

func TestRuntimePolicyValueConstantsMatchWireVocabulary(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"policy profile", string(PolicyProfileSecureDefault), "secure-default"},
		{"network mode", string(NetworkModeAllowlist), "allowlist"},
		{"network enforcement", string(NetworkEnforcementProxy), "proxy"},
		{"filesystem root", string(FilesystemRootReadonly), "readonly"},
		{"filesystem writes", string(FilesystemWritesWorkspaceOnly), "workspace-only"},
		{"process user", string(ProcessUserNonRoot), "non-root"},
		{"image source", string(ImageSourceRegistry), "registry"},
		{"image auto build", string(ImageAutoBuildIfStale), "if-stale"},
		{"image pull policy", string(ImagePullIfMissing), "if-missing"},
		{"image artifact state", string(ImageArtifactMaterialized), "materialized"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q want %q", test.got, test.want)
			}
		})
	}
}

func TestRuntimeKindValueNormalizesKnownAndPreservesUnknownInput(t *testing.T) {
	known := NormalizeRuntimeKind(" Agent ")
	if known != RuntimeKind(RuntimeKindAgent) || !known.IsValid() {
		t.Fatalf("known runtime kind = %q valid=%v", known, known.IsValid())
	}
	unknown := NormalizeRuntimeKind(" Future-Kind ")
	if unknown != RuntimeKind("future-kind") || unknown.IsValid() {
		t.Fatalf("unknown runtime kind = %q valid=%v", unknown, unknown.IsValid())
	}
}

func TestRuntimePolicyWireFieldsRemainStrings(t *testing.T) {
	// These assignments intentionally protect source compatibility for callers that
	// construct the exported YAML/JSON shapes with ordinary string variables.
	var policyProfile string = (CompiledRuntimeManifest{}).PolicyProfile
	var policySourceProfile string = (PolicySources{}).PolicyProfile
	var preset string = (CompiledSecurityPolicy{}).Preset
	var networkMode string = (CompiledNetworkPolicy{}).Mode
	var networkEnforcement string = (CompiledNetworkPolicy{}).Enforcement
	var filesystemRoot string = (CompiledFilesystemPolicy{}).Root
	var filesystemWrites string = (CompiledFilesystemPolicy{}).Writes
	var processUser string = (CompiledProcessPolicy{}).User
	var imageSource string = (CompiledImageSelection{}).Source
	var imageAutoBuild string = (CompiledImageSelection{}).AutoBuild
	var imagePullPolicy string = (CompiledImageSelection{}).PullPolicy
	var artifactSource string = (ImageArtifactManifest{}).Source
	var artifactState string = (ImageArtifactManifest{}).ArtifactState
	var workflowProfile string = (WorkflowSecurityConfig{}).Profile
	var workflowNetworkMode string = (WorkflowNetworkConfig{}).Mode
	var workflowFilesystemRoot string = (WorkflowFilesystemConfig{}).Root
	var workflowFilesystemWrites string = (WorkflowFilesystemConfig{}).Writes
	var workflowProcessUser string = (WorkflowProcessConfig{}).User

	_ = []string{
		policyProfile,
		policySourceProfile,
		preset,
		networkMode,
		networkEnforcement,
		filesystemRoot,
		filesystemWrites,
		processUser,
		imageSource,
		imageAutoBuild,
		imagePullPolicy,
		artifactSource,
		artifactState,
		workflowProfile,
		workflowNetworkMode,
		workflowFilesystemRoot,
		workflowFilesystemWrites,
		workflowProcessUser,
	}
}

func TestRuntimePolicyValidationAcceptsTrimmedWireValues(t *testing.T) {
	manifest := &CompiledRuntimeManifest{
		Schema:        2,
		Kind:          RuntimeManifestKind,
		PolicyProfile: " secure-default ",
		Security: CompiledSecurityPolicy{
			Preset: " secure-default ",
			Network: CompiledNetworkPolicy{
				Mode:        " internet ",
				Enforcement: " native ",
			},
			FS: CompiledFilesystemPolicy{
				Root:   " readonly ",
				Writes: " workspace-only ",
			},
			Process: CompiledProcessPolicy{User: " non-root "},
		},
		Image: CompiledImageSelection{
			Source:     " registry ",
			Ref:        "example.test/tool:latest",
			PullPolicy: " if-missing ",
		},
	}
	if err := ValidateCompiledRuntimeManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.PolicyProfile != " secure-default " || manifest.Image.Source != " registry " {
		t.Fatal("validation mutated authored wire values")
	}
}
