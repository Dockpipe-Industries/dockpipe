package domain

import "strings"

// ContainerMountMode identifies the access mode of an authored container mount.
type ContainerMountMode string

const (
	ContainerMountReadOnly  ContainerMountMode = "ro"
	ContainerMountReadWrite ContainerMountMode = "rw"
)

var validContainerMountModes = enumValues(
	ContainerMountMode(""),
	ContainerMountReadOnly,
	ContainerMountReadWrite,
)

// WorkspaceMode identifies who owns workspace lifecycle behavior.
type WorkspaceMode string

const (
	WorkspaceModeManaged WorkspaceMode = "managed"
	WorkspaceModeBind    WorkspaceMode = "bind"
)

// WorkspaceStorage identifies the storage implementation requested for a workspace.
type WorkspaceStorage string

const (
	WorkspaceStorageBind     WorkspaceStorage = "bind"
	WorkspaceStorageVolume   WorkspaceStorage = "volume"
	WorkspaceStorageWorktree WorkspaceStorage = "worktree"
	WorkspaceStorageClone    WorkspaceStorage = "clone"
)

// WorkspaceStorageBackend identifies the effective runtime storage backend recorded for a session.
type WorkspaceStorageBackend string

const (
	WorkspaceStorageBackendBind         WorkspaceStorageBackend = "bind"
	WorkspaceStorageBackendWorktree     WorkspaceStorageBackend = "worktree"
	WorkspaceStorageBackendDockerVolume WorkspaceStorageBackend = "docker_volume"
)

// NormalizeWorkspaceStorageBackend normalizes a recorded backend for internal decisions.
func NormalizeWorkspaceStorageBackend(value string) WorkspaceStorageBackend {
	return WorkspaceStorageBackend(strings.ToLower(strings.TrimSpace(value)))
}

// WorkspaceCheckpointMode identifies when a runtime checkpoints workspace changes.
type WorkspaceCheckpointMode string

const (
	WorkspaceCheckpointManual WorkspaceCheckpointMode = "manual"
	WorkspaceCheckpointAuto   WorkspaceCheckpointMode = "auto"
	WorkspaceCheckpointStep   WorkspaceCheckpointMode = "step"
)

// WorkspacePublishMode identifies the requested workspace publication outcome.
type WorkspacePublishMode string

const (
	WorkspacePublishNone   WorkspacePublishMode = "none"
	WorkspacePublishBranch WorkspacePublishMode = "branch"
	WorkspacePublishReview WorkspacePublishMode = "review"
)

var (
	validWorkspaceModes = enumValues(
		WorkspaceModeManaged,
		WorkspaceModeBind,
	)
	validAuthoredWorkspaceStorages = enumValues(
		WorkspaceStorage(""),
		WorkspaceStorageVolume,
		WorkspaceStorageWorktree,
		WorkspaceStorageClone,
	)
	validWorkspaceCheckpointModes = enumValues(
		WorkspaceCheckpointMode(""),
		WorkspaceCheckpointManual,
		WorkspaceCheckpointAuto,
		WorkspaceCheckpointStep,
	)
	validWorkspacePublishModes = enumValues(
		WorkspacePublishMode(""),
		WorkspacePublishNone,
		WorkspacePublishBranch,
		WorkspacePublishReview,
	)
)
