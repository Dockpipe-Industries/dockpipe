package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure"
)

func cmdWorkspaceGit(args []string) error {
	if len(args) != 2 || args[0] != "--request" {
		return errors.New("usage: dockpipe git --request <project-action.json>")
	}
	info, err := os.Stat(args[1])
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("project Git request must be a regular JSON file")
	}
	file, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer file.Close()
	const maxRequestBytes = 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(file, maxRequestBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxRequestBytes {
		return errors.New("project Git request exceeds 1 MiB")
	}
	var request infrastructure.WorkspaceGitRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("project Git request: %w", err)
	}
	switch request.Action {
	case "status", "diff", "commit", "pull", "push", "review-status", "compare", "catalog", "stage", "unstage", "discard", "commit-index", "history", "commit-review", "branch-create", "branch-switch", "branch-delete", "fetch", "merge", "stash-list", "stash-save", "stash-apply", "stash-pop", "stash-drop", "revert", "cherry-pick", "operation-continue", "operation-skip", "operation-abort", "resolve-stage", "resolve-ours", "resolve-theirs", "resolve-delete":
	default:
		return errors.New("unsupported project Git action")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var result *infrastructure.WorkspaceGitSnapshot
	err = infrastructure.RunOperationWithOptions(os.Stderr, "workspace.git."+request.Action,
		"Working with project Git", nil, infrastructure.OperationOptions{}, func() error {
			var operationErr error
			result, operationErr = infrastructure.RunWorkspaceGit(ctx, request)
			return operationErr
		})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
