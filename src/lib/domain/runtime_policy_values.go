package domain

import "dockpipe/src/lib/domain/workflow"

type PolicyProfile = workflow.PolicyProfile
type NetworkMode = workflow.NetworkMode
type NetworkEnforcement = workflow.NetworkEnforcement
type FilesystemRootPolicy = workflow.FilesystemRootPolicy
type FilesystemWritePolicy = workflow.FilesystemWritePolicy
type ProcessUserPolicy = workflow.ProcessUserPolicy
type ImageSource = workflow.ImageSource
type ImageAutoBuildMode = workflow.ImageAutoBuildMode
type ImagePullPolicy = workflow.ImagePullPolicy
type ImageArtifactState = workflow.ImageArtifactState

const (
	PolicyProfileSecureDefault    = workflow.PolicyProfileSecureDefault
	PolicyProfileInternetClient   = workflow.PolicyProfileInternetClient
	PolicyProfileBuildOnline      = workflow.PolicyProfileBuildOnline
	PolicyProfileSidecarClient    = workflow.PolicyProfileSidecarClient
	NetworkModeOffline            = workflow.NetworkModeOffline
	NetworkModeAllowlist          = workflow.NetworkModeAllowlist
	NetworkModeRestricted         = workflow.NetworkModeRestricted
	NetworkModeInternet           = workflow.NetworkModeInternet
	NetworkEnforcementNative      = workflow.NetworkEnforcementNative
	NetworkEnforcementProxy       = workflow.NetworkEnforcementProxy
	NetworkEnforcementAdvisory    = workflow.NetworkEnforcementAdvisory
	FilesystemRootReadonly        = workflow.FilesystemRootReadonly
	FilesystemRootWritable        = workflow.FilesystemRootWritable
	FilesystemWritesWorkspaceOnly = workflow.FilesystemWritesWorkspaceOnly
	FilesystemWritesDeclared      = workflow.FilesystemWritesDeclared
	ProcessUserAuto               = workflow.ProcessUserAuto
	ProcessUserRoot               = workflow.ProcessUserRoot
	ProcessUserNonRoot            = workflow.ProcessUserNonRoot
	ImageSourceAuto               = workflow.ImageSourceAuto
	ImageSourceBuild              = workflow.ImageSourceBuild
	ImageSourceRegistry           = workflow.ImageSourceRegistry
	ImageAutoBuildIfMissing       = workflow.ImageAutoBuildIfMissing
	ImageAutoBuildIfStale         = workflow.ImageAutoBuildIfStale
	ImageAutoBuildNever           = workflow.ImageAutoBuildNever
	ImagePullIfMissing            = workflow.ImagePullIfMissing
	ImagePullNever                = workflow.ImagePullNever
	ImageArtifactPlanned          = workflow.ImageArtifactPlanned
	ImageArtifactMaterialized     = workflow.ImageArtifactMaterialized
	ImageArtifactReferenced       = workflow.ImageArtifactReferenced
	ImageArtifactCached           = workflow.ImageArtifactCached
)
