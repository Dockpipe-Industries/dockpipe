// Package runtimeartifact owns the compiled runtime and image artifact wire shapes.
package runtimeartifact

const (
	RuntimeManifestKind       = "dockpipe-runtime-manifest"
	ImageArtifactManifestKind = "docker-image-artifact"
	RuntimeManifestDirName    = ".dockpipe"
	RuntimeManifestFileName   = "runtime.effective.json"
	ImageArtifactFileName     = "image-artifact.json"
	StepArtifactsDirName      = "steps"
)

type CompiledRuntimeManifest struct {
	Schema               int                    `json:"schema" yaml:"schema"`
	Kind                 string                 `json:"kind" yaml:"kind"`
	WorkflowName         string                 `json:"workflow_name,omitempty" yaml:"workflow_name,omitempty"`
	PackageName          string                 `json:"package_name,omitempty" yaml:"package_name,omitempty"`
	StepID               string                 `json:"step_id,omitempty" yaml:"step_id,omitempty"`
	RuntimeProfile       string                 `json:"runtime_profile,omitempty" yaml:"runtime_profile,omitempty"`
	ResolverProfile      string                 `json:"resolver_profile,omitempty" yaml:"resolver_profile,omitempty"`
	PolicyProfile        string                 `json:"policy_profile,omitempty" yaml:"policy_profile,omitempty"`
	PolicySources        PolicySources          `json:"policy_sources,omitempty" yaml:"policy_sources,omitempty"`
	PolicyFingerprint    string                 `json:"policy_fingerprint,omitempty" yaml:"policy_fingerprint,omitempty"`
	ImageFingerprint     string                 `json:"image_fingerprint,omitempty" yaml:"image_fingerprint,omitempty"`
	Security             CompiledSecurityPolicy `json:"security" yaml:"security"`
	Image                CompiledImageSelection `json:"image" yaml:"image"`
	EnforcementSummaries []string               `json:"enforcement_summaries,omitempty" yaml:"enforcement_summaries,omitempty"`
	RuleIDs              []string               `json:"rule_ids,omitempty" yaml:"rule_ids,omitempty"`
}

type PolicySources struct {
	EngineDefault    bool   `json:"engine_default,omitempty" yaml:"engine_default,omitempty"`
	RuntimeBaseline  string `json:"runtime_baseline,omitempty" yaml:"runtime_baseline,omitempty"`
	PolicyProfile    string `json:"policy_profile,omitempty" yaml:"policy_profile,omitempty"`
	WorkflowOverride bool   `json:"workflow_override,omitempty" yaml:"workflow_override,omitempty"`
	StepOverride     bool   `json:"step_override,omitempty" yaml:"step_override,omitempty"`
}

type CompiledSecurityPolicy struct {
	Preset  string                   `json:"preset,omitempty" yaml:"preset,omitempty"`
	Network CompiledNetworkPolicy    `json:"network,omitempty" yaml:"network,omitempty"`
	FS      CompiledFilesystemPolicy `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
	Process CompiledProcessPolicy    `json:"process,omitempty" yaml:"process,omitempty"`
}

type CompiledNetworkPolicy struct {
	Mode        string   `json:"mode,omitempty" yaml:"mode,omitempty"`
	Enforcement string   `json:"enforcement,omitempty" yaml:"enforcement,omitempty"`
	Allow       []string `json:"allow,omitempty" yaml:"allow,omitempty"`
	Block       []string `json:"block,omitempty" yaml:"block,omitempty"`
	InternalDNS bool     `json:"internal_dns,omitempty" yaml:"internal_dns,omitempty"`
}

type CompiledFilesystemPolicy struct {
	Root          string   `json:"root,omitempty" yaml:"root,omitempty"`
	Writes        string   `json:"writes,omitempty" yaml:"writes,omitempty"`
	WritablePaths []string `json:"writable_paths,omitempty" yaml:"writable_paths,omitempty"`
	TempPaths     []string `json:"temp_paths,omitempty" yaml:"temp_paths,omitempty"`
}

type CompiledProcessPolicy struct {
	User            string                 `json:"user,omitempty" yaml:"user,omitempty"`
	NoNewPrivileges bool                   `json:"no_new_privileges,omitempty" yaml:"no_new_privileges,omitempty"`
	DropCaps        []string               `json:"drop_caps,omitempty" yaml:"drop_caps,omitempty"`
	AddCaps         []string               `json:"add_caps,omitempty" yaml:"add_caps,omitempty"`
	PIDLimit        int                    `json:"pid_limit,omitempty" yaml:"pid_limit,omitempty"`
	Resources       CompiledResourceLimits `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type CompiledResourceLimits struct {
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
}

type CompiledImageSelection struct {
	Source         string                  `json:"source,omitempty" yaml:"source,omitempty"`
	Ref            string                  `json:"ref,omitempty" yaml:"ref,omitempty"`
	AutoBuild      string                  `json:"auto_build,omitempty" yaml:"auto_build,omitempty"`
	PullPolicy     string                  `json:"pull_policy,omitempty" yaml:"pull_policy,omitempty"`
	Build          *CompiledImageBuildSpec `json:"build,omitempty" yaml:"build,omitempty"`
	ExpectedDigest string                  `json:"expected_digest,omitempty" yaml:"expected_digest,omitempty"`
}

type CompiledImageBuildSpec struct {
	Context    string            `json:"context,omitempty" yaml:"context,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Target     string            `json:"target,omitempty" yaml:"target,omitempty"`
	Platform   string            `json:"platform,omitempty" yaml:"platform,omitempty"`
	Args       map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
}

type ImageArtifactManifest struct {
	Schema                      int                     `json:"schema" yaml:"schema"`
	Kind                        string                  `json:"kind" yaml:"kind"`
	WorkflowName                string                  `json:"workflow_name,omitempty" yaml:"workflow_name,omitempty"`
	PackageName                 string                  `json:"package_name,omitempty" yaml:"package_name,omitempty"`
	StepID                      string                  `json:"step_id,omitempty" yaml:"step_id,omitempty"`
	ImageKey                    string                  `json:"image_key,omitempty" yaml:"image_key,omitempty"`
	Source                      string                  `json:"source,omitempty" yaml:"source,omitempty"`
	ArtifactState               string                  `json:"artifact_state,omitempty" yaml:"artifact_state,omitempty"`
	Fingerprint                 string                  `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	SourceFingerprint           string                  `json:"source_fingerprint,omitempty" yaml:"source_fingerprint,omitempty"`
	SecurityManifestFingerprint string                  `json:"security_manifest_fingerprint,omitempty" yaml:"security_manifest_fingerprint,omitempty"`
	ImageRef                    string                  `json:"image_ref,omitempty" yaml:"image_ref,omitempty"`
	ExpectedDigest              string                  `json:"expected_digest,omitempty" yaml:"expected_digest,omitempty"`
	ResolvedRef                 string                  `json:"resolved_ref,omitempty" yaml:"resolved_ref,omitempty"`
	ImageID                     string                  `json:"image_id,omitempty" yaml:"image_id,omitempty"`
	RepoDigest                  string                  `json:"repo_digest,omitempty" yaml:"repo_digest,omitempty"`
	Build                       *CompiledImageBuildSpec `json:"build,omitempty" yaml:"build,omitempty"`
	Provenance                  ImageArtifactProvenance `json:"provenance,omitempty" yaml:"provenance,omitempty"`
}

type ImageArtifactProvenance struct {
	Runtime         string `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Resolver        string `json:"resolver,omitempty" yaml:"resolver,omitempty"`
	Isolate         string `json:"isolate,omitempty" yaml:"isolate,omitempty"`
	PackageVersion  string `json:"package_version,omitempty" yaml:"package_version,omitempty"`
	DockpipeVersion string `json:"dockpipe_version,omitempty" yaml:"dockpipe_version,omitempty"`
}
