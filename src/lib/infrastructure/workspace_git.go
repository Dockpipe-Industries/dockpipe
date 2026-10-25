package infrastructure

import (
	"bytes"
	"context"
	"errors"

	"dockpipe/src/lib/infrastructure/workspacegit"
)

// Workspace Git models retain their public infrastructure API while their
// implementation is owned by the workspacegit package.
type WorkspaceGitRequest = workspacegit.WorkspaceGitRequest

type WorkspaceGitChange = workspacegit.WorkspaceGitChange

type WorkspaceGitSnapshot = workspacegit.WorkspaceGitSnapshot

type WorkspaceGitHistory = workspacegit.WorkspaceGitHistory

type WorkspaceGitCommitReview = workspacegit.WorkspaceGitCommitReview

type WorkspaceGitStash = workspacegit.WorkspaceGitStash

type WorkspaceGitOperation = workspacegit.WorkspaceGitOperation

type WorkspaceGitReview = workspacegit.WorkspaceGitReview

type WorkspaceGitCommit = workspacegit.WorkspaceGitCommit

type WorkspaceGitBranch = workspacegit.WorkspaceGitBranch

type WorkspaceGitCatalog = workspacegit.WorkspaceGitCatalog

type WorkspaceGitTagPublication = workspacegit.WorkspaceGitTagPublication

// RunWorkspaceGit runs a project Git operation through its owning adapter.
func RunWorkspaceGit(ctx context.Context, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	return workspacegit.RunWorkspaceGit(ctx, request)
}

const workspaceGitBytes = 8 * 1024 * 1024 // Maximum review and command output, in bytes.

type workspaceGitBuffer struct{ bytes.Buffer }

func (buffer *workspaceGitBuffer) Write(data []byte) (int, error) {
	if len(data) > workspaceGitBytes-buffer.Len() {
		return 0, errors.New("git output exceeds the 8 MiB review limit")
	}
	return buffer.Buffer.Write(data)
}
