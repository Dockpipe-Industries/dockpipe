package workspacegit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

// RunWorkspaceGit admits local human-driven Git actions using native process
// identity. Repository, review, confirmation and credential checks remain owned
// by the Git backend.
func RunWorkspaceGit(ctx context.Context, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	target, identity := nativeinvocation.Local()
	return RunWorkspaceGitWithAuthority(ctx, request, target, identity, nil)
}

// RunWorkspaceGitWithAuthority is for trusted native or authenticated transport
// adapters. The adapter asserts verified identity, target and project scope;
// request JSON cannot supply authority. Remote targets always require an
// authorizer and never fall back to the local process identity.
func RunWorkspaceGitWithAuthority(ctx context.Context, request WorkspaceGitRequest, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) (*WorkspaceGitSnapshot, error) {
	id := invocation.ID{Capability: "dockpipe.git", Action: request.Action, Version: "1"}
	requireAuthorization := target.Kind == invocation.Remote || authorize != nil
	contract, err := invocation.New(invocation.Definition{
		ID: id, Target: target, RequireAuthorization: requireAuthorization,
	}, prepareWorkspaceGit(requireAuthorization), authorize)
	if err != nil {
		return nil, err
	}
	return contract.Invoke(ctx, invocation.Call{Contract: id, Identity: identity, Target: target}, request)
}

func prepareWorkspaceGit(requireDigest bool) invocation.Prepare[WorkspaceGitRequest, *WorkspaceGitSnapshot] {
	return func(ctx context.Context, request WorkspaceGitRequest) (invocation.Prepared[*WorkspaceGitSnapshot], error) {
		if err := ctx.Err(); err != nil {
			return invocation.Prepared[*WorkspaceGitSnapshot]{}, err
		}
		if !knownWorkspaceGitAction(request.Action) {
			return invocation.Prepared[*WorkspaceGitSnapshot]{}, fmt.Errorf("unsupported project Git action %q", request.Action)
		}
		if strings.TrimSpace(request.Workdir) == "" {
			return invocation.Prepared[*WorkspaceGitSnapshot]{}, errors.New("select a project directory first")
		}
		request.Paths = append([]string(nil), request.Paths...)
		request.Commits = append([]string(nil), request.Commits...)
		prepared := invocation.Prepared[*WorkspaceGitSnapshot]{
			Revision: request.Snapshot,
			Execute: func(ctx context.Context) (*WorkspaceGitSnapshot, error) {
				return executeWorkspaceGit(ctx, request)
			},
		}
		if requireDigest {
			prepared.Digest = workspaceGitInputDigest(request)
		}
		return prepared, nil
	}
}

func knownWorkspaceGitAction(action string) bool {
	switch action {
	case "status", "review-status", "catalog", "history", "commit-review", "stash-list",
		"tag-publish", "compare", "stage", "unstage", "discard", "commit-index", "diff",
		"commit", "pull", "push":
		return true
	default:
		return handledWorkspaceGitAction(action)
	}
}

func workspaceGitInputDigest(request WorkspaceGitRequest) string {
	hash := sha256.New()
	// Length framing preserves exact bytes, including invalid UTF-8 and NULs.
	// Collection counts distinguish element boundaries and empty collections.
	writeString := func(value string) {
		fmt.Fprintf(hash, "%d:", len(value))
		hash.Write([]byte(value))
	}
	for _, value := range []string{request.Workdir, request.Action, request.Snapshot, request.Message,
		request.Comparison, strconv.Itoa(request.Context), strconv.FormatBool(request.Confirm),
		request.Query, request.Match, request.Ref, strconv.Itoa(request.Offset), strconv.Itoa(request.Limit),
		request.Commit, request.Name, request.Stash, request.StashSelector, strconv.FormatBool(request.IncludeUntracked)} {
		writeString(value)
	}
	for _, values := range [][]string{request.Paths, request.Commits} {
		writeString(strconv.Itoa(len(values)))
		for _, value := range values {
			writeString(value)
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}
