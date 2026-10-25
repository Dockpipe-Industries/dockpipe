package domain

import "dockpipe/src/lib/domain/workflow"

type ContainerMountMode = workflow.ContainerMountMode
type WorkspaceMode = workflow.WorkspaceMode
type WorkspaceStorage = workflow.WorkspaceStorage
type WorkspaceStorageBackend = workflow.WorkspaceStorageBackend
type WorkspaceCheckpointMode = workflow.WorkspaceCheckpointMode
type WorkspacePublishMode = workflow.WorkspacePublishMode

const (
	ContainerMountReadOnly              = workflow.ContainerMountReadOnly
	ContainerMountReadWrite             = workflow.ContainerMountReadWrite
	WorkspaceModeManaged                = workflow.WorkspaceModeManaged
	WorkspaceModeBind                   = workflow.WorkspaceModeBind
	WorkspaceStorageBind                = workflow.WorkspaceStorageBind
	WorkspaceStorageVolume              = workflow.WorkspaceStorageVolume
	WorkspaceStorageWorktree            = workflow.WorkspaceStorageWorktree
	WorkspaceStorageClone               = workflow.WorkspaceStorageClone
	WorkspaceStorageBackendBind         = workflow.WorkspaceStorageBackendBind
	WorkspaceStorageBackendWorktree     = workflow.WorkspaceStorageBackendWorktree
	WorkspaceStorageBackendDockerVolume = workflow.WorkspaceStorageBackendDockerVolume
	WorkspaceCheckpointManual           = workflow.WorkspaceCheckpointManual
	WorkspaceCheckpointAuto             = workflow.WorkspaceCheckpointAuto
	WorkspaceCheckpointStep             = workflow.WorkspaceCheckpointStep
	WorkspacePublishNone                = workflow.WorkspacePublishNone
	WorkspacePublishBranch              = workflow.WorkspacePublishBranch
	WorkspacePublishReview              = workflow.WorkspacePublishReview
)

func NormalizeWorkspaceStorageBackend(value string) WorkspaceStorageBackend {
	return workflow.NormalizeWorkspaceStorageBackend(value)
}
