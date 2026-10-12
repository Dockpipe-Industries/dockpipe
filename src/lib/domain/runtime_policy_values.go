package domain

// PolicyProfile identifies a compiled security-policy profile.
type PolicyProfile string

const (
	PolicyProfileSecureDefault  PolicyProfile = "secure-default"
	PolicyProfileInternetClient PolicyProfile = "internet-client"
	PolicyProfileBuildOnline    PolicyProfile = "build-online"
	PolicyProfileSidecarClient  PolicyProfile = "sidecar-client"
)

// NetworkMode identifies the network access model enforced for a runtime.
type NetworkMode string

const (
	NetworkModeOffline    NetworkMode = "offline"
	NetworkModeAllowlist  NetworkMode = "allowlist"
	NetworkModeRestricted NetworkMode = "restricted"
	NetworkModeInternet   NetworkMode = "internet"
)

// NetworkEnforcement identifies how a network policy is enforced.
type NetworkEnforcement string

const (
	NetworkEnforcementNative   NetworkEnforcement = "native"
	NetworkEnforcementProxy    NetworkEnforcement = "proxy"
	NetworkEnforcementAdvisory NetworkEnforcement = "advisory"
)

// FilesystemRootPolicy identifies the root-filesystem access model.
type FilesystemRootPolicy string

const (
	FilesystemRootReadonly FilesystemRootPolicy = "readonly"
	FilesystemRootWritable FilesystemRootPolicy = "writable"
)

// FilesystemWritePolicy identifies where a runtime may write.
type FilesystemWritePolicy string

const (
	FilesystemWritesWorkspaceOnly FilesystemWritePolicy = "workspace-only"
	FilesystemWritesDeclared      FilesystemWritePolicy = "declared"
)

// ProcessUserPolicy identifies the user model for a runtime process.
type ProcessUserPolicy string

const (
	ProcessUserAuto    ProcessUserPolicy = "auto"
	ProcessUserRoot    ProcessUserPolicy = "root"
	ProcessUserNonRoot ProcessUserPolicy = "non-root"
)

// ImageSource identifies how a compiled runtime image is obtained.
type ImageSource string

const (
	ImageSourceAuto     ImageSource = "auto"
	ImageSourceBuild    ImageSource = "build"
	ImageSourceRegistry ImageSource = "registry"
)

// ImageAutoBuildMode identifies when a source image is rebuilt.
type ImageAutoBuildMode string

const (
	ImageAutoBuildIfMissing ImageAutoBuildMode = "if-missing"
	ImageAutoBuildIfStale   ImageAutoBuildMode = "if-stale"
	ImageAutoBuildNever     ImageAutoBuildMode = "never"
)

// ImagePullPolicy identifies when a registry image is pulled.
type ImagePullPolicy string

const (
	ImagePullIfMissing ImagePullPolicy = "if-missing"
	ImagePullNever     ImagePullPolicy = "never"
)

// ImageArtifactState identifies the lifecycle state of an image artifact.
type ImageArtifactState string

const (
	ImageArtifactPlanned      ImageArtifactState = "planned"
	ImageArtifactMaterialized ImageArtifactState = "materialized"
	ImageArtifactReferenced   ImageArtifactState = "referenced"
	ImageArtifactCached       ImageArtifactState = "cached"
)

var (
	validPolicyProfiles = enumValues(
		PolicyProfile(""),
		PolicyProfileSecureDefault,
		PolicyProfileInternetClient,
		PolicyProfileBuildOnline,
		PolicyProfileSidecarClient,
	)
	validNetworkModes = enumValues(
		NetworkMode(""),
		NetworkModeOffline,
		NetworkModeAllowlist,
		NetworkModeRestricted,
		NetworkModeInternet,
	)
	validNetworkEnforcement = enumValues(
		NetworkEnforcement(""),
		NetworkEnforcementNative,
		NetworkEnforcementProxy,
		NetworkEnforcementAdvisory,
	)
	validFilesystemRoots = enumValues(
		FilesystemRootPolicy(""),
		FilesystemRootReadonly,
		FilesystemRootWritable,
	)
	validFilesystemWrites = enumValues(
		FilesystemWritePolicy(""),
		FilesystemWritesWorkspaceOnly,
		FilesystemWritesDeclared,
	)
	validProcessUsers = enumValues(
		ProcessUserPolicy(""),
		ProcessUserAuto,
		ProcessUserRoot,
		ProcessUserNonRoot,
	)
	validImageSources = enumValues(
		ImageSource(""),
		ImageSourceAuto,
		ImageSourceBuild,
		ImageSourceRegistry,
	)
	validImageAutoBuildModes = enumValues(
		ImageAutoBuildMode(""),
		ImageAutoBuildIfMissing,
		ImageAutoBuildIfStale,
		ImageAutoBuildNever,
	)
	validImagePullPolicies = enumValues(
		ImagePullPolicy(""),
		ImagePullIfMissing,
		ImagePullNever,
	)
	validImageArtifactStates = enumValues(
		ImageArtifactState(""),
		ImageArtifactPlanned,
		ImageArtifactMaterialized,
		ImageArtifactReferenced,
		ImageArtifactCached,
	)
)

func enumValues[T comparable](values ...T) map[T]struct{} {
	result := make(map[T]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
