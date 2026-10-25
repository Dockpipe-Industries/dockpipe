package workspacegit

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

func readWorkspaceGitExtended(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	switch request.Action {
	case "stash-list":
		var err error
		current.Stashes, err = readWorkspaceGitStashes(ctx, current.Root)
		return current, err
	case "commit-review":
		if err := workspaceGitExactCommit(ctx, current.Root, request.Commit); err != nil {
			return nil, err
		}
		metadata, err := workspaceGitCommand(ctx, current.Root, "show", "--no-patch", "--format=%H%x00%P%x00%an%x00%s%x00%B", request.Commit)
		if err != nil {
			return nil, err
		}
		fields := strings.SplitN(string(metadata), "\x00", 5)
		if len(fields) != 5 {
			return nil, errors.New("unsupported commit metadata")
		}
		patch, err := workspaceGitCommand(ctx, current.Root, "show", "--format=", "--no-ext-diff", "--no-textconv", "--no-color", "--stat", "--patch", request.Commit, "--")
		if err != nil {
			return nil, err
		}
		review := &WorkspaceGitCommitReview{Commit: WorkspaceGitCommit{ID: fields[0], Parents: strings.Fields(fields[1]), Author: fields[2], Subject: fields[3]}, Body: fields[4]}
		if len(metadata) > 256*1024 || len(patch) > 2*1024*1024 {
			review.Limited = true
			review.Notice = "Commit exceeds the focused review limit (256 KiB metadata or 2 MiB patch); inspect it in conventional Git."
			if len(metadata) > 256*1024 {
				review.Body = ""
			}
		} else {
			review.Patch = strings.ToValidUTF8(string(patch), "�")
		}
		current.CommitReview = review
		return current, nil
	case "history":
		return readWorkspaceGitHistory(ctx, current, request)
	}
	return nil, errors.New("unsupported Git read action")
}

func workspaceGitHistoryPage(request WorkspaceGitRequest) (int, error) {
	limit := request.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 || request.Offset < 0 || request.Offset > 100000 || len(request.Query) > 256 || strings.ContainsAny(request.Query, "\x00\r\n") {
		return 0, errors.New("history requires limit 1 to 200, offset 0 to 100000 and a query up to 256 bytes")
	}
	if request.Match != "" && request.Match != "keyword" && request.Match != "prefix" {
		return 0, errors.New("history match must be keyword or prefix")
	}
	return limit, nil
}

func readWorkspaceGitHistory(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	limit, err := workspaceGitHistoryPage(request)
	if err != nil {
		return nil, err
	}
	args := []string{"log", "--format=%H%x00%P%x00%an%x00%s"}
	if request.Ref == "all" {
		args = append(args, "--all")
	} else if request.Ref != "" {
		if err := workspaceGitLocalBranch(ctx, current.Root, request.Ref); err != nil {
			return nil, err
		}
		resolved := "refs/heads/" + request.Ref
		if _, err := workspaceGitCommand(ctx, current.Root, "show-ref", "--verify", "--quiet", resolved); err != nil {
			if !workspaceGitMissingRef(err) {
				return nil, err
			}
			resolved = "refs/remotes/" + request.Ref
			if _, err := workspaceGitCommand(ctx, current.Root, "show-ref", "--verify", "--quiet", resolved); err != nil {
				return nil, err
			}
		}
		args = append(args, resolved)
	} else if current.Head == "" {
		current.History = &WorkspaceGitHistory{Commits: []WorkspaceGitCommit{}, Offset: request.Offset, Limit: limit}
		return current, nil
	}
	// Native grep searches the complete reachable history, including commit bodies.
	// Prefix mode also matches full SHA prefixes, so it filters a bounded raw log.
	if request.Query != "" && request.Match != "prefix" {
		args = append(args, "--regexp-ignore-case", "--fixed-strings", "--grep="+request.Query)
	}
	if request.Match != "prefix" || request.Query == "" {
		args = append(args, "--skip="+strconv.Itoa(request.Offset), "--max-count="+strconv.Itoa(limit+1))
	}
	args = append(args, "--")
	output, err := workspaceGitCommand(ctx, current.Root, args...)
	if err != nil {
		return nil, err
	}
	history := &WorkspaceGitHistory{Commits: []WorkspaceGitCommit{}, Offset: request.Offset, Limit: limit}
	skipped := 0
	for _, row := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.SplitN(row, "\x00", 4)
		if len(fields) != 4 {
			continue
		}
		if request.Match == "prefix" && request.Query != "" {
			query := strings.ToLower(request.Query)
			if !strings.HasPrefix(strings.ToLower(fields[0]), query) && !strings.HasPrefix(strings.ToLower(fields[3]), query) {
				continue
			}
			if skipped < request.Offset {
				skipped++
				continue
			}
		}
		if len(history.Commits) == limit {
			history.HasMore = true
			break
		}
		history.Commits = append(history.Commits, WorkspaceGitCommit{ID: fields[0], Parents: strings.Fields(fields[1]), Author: fields[2], Subject: fields[3]})
	}
	current.History = history
	return current, nil
}
