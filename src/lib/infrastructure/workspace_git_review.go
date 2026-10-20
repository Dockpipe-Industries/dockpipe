package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A source-neutral two-sided comparison. Missing sides, conflicts and binary
// content are explicit; consumers must never interpret them as empty text diffs.
type WorkspaceGitReview struct {
	Path         string `json:"path"`
	OriginalPath string `json:"original_path,omitempty"`
	Comparison   string `json:"comparison"`
	BeforeLabel  string `json:"before_label"`
	AfterLabel   string `json:"after_label"`
	Revision     string `json:"revision"`
	Patch        string `json:"patch"`
	Binary       bool   `json:"binary"`
	Conflict     bool   `json:"conflict"`
	Limited      bool   `json:"limited"`
	Notice       string `json:"notice,omitempty"`
}

type WorkspaceGitCommit struct {
	ID      string   `json:"id"`
	Parents []string `json:"parents"`
	Subject string   `json:"subject"`
	Author  string   `json:"author"`
}

type WorkspaceGitBranch struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	Upstream   string `json:"upstream"`
	Divergence string `json:"divergence"`
}

type WorkspaceGitCatalog struct {
	Commits      []WorkspaceGitCommit `json:"commits"`
	Branches     []WorkspaceGitBranch `json:"branches"`
	Remotes      []string             `json:"remotes"`
	Notice       string               `json:"notice"`
	PullRequests bool                 `json:"pull_requests"`
}

func readWorkspaceGitCatalog(ctx context.Context, snapshot *WorkspaceGitSnapshot) (*WorkspaceGitCatalog, error) {
	catalog := &WorkspaceGitCatalog{Commits: []WorkspaceGitCommit{}, Branches: []WorkspaceGitBranch{}, Remotes: []string{},
		Notice: "Showing up to 128 branches and 64 recent commits. No pull-request provider is connected to project Git."}
	branches, err := workspaceGitCommand(ctx, snapshot.Root, "for-each-ref", "--count=128", "--format=%(refname:short)%00%(objectname)%00%(upstream:short)%00%(upstream:track)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	for _, row := range strings.Split(strings.TrimSpace(string(branches)), "\n") {
		fields := strings.Split(row, "\x00")
		if len(fields) == 4 {
			catalog.Branches = append(catalog.Branches, WorkspaceGitBranch{fields[0], fields[1], fields[2], fields[3]})
		}
	}
	remotes, err := workspaceGitCommand(ctx, snapshot.Root, "remote")
	if err != nil {
		return nil, err
	}
	catalog.Remotes = strings.Fields(string(remotes))
	if snapshot.Head != "" {
		commits, err := workspaceGitCommand(ctx, snapshot.Root, "log", "-64", "--format=%H%x00%P%x00%an%x00%s")
		if err != nil {
			return nil, err
		}
		for _, row := range strings.Split(strings.TrimSpace(string(commits)), "\n") {
			fields := strings.SplitN(row, "\x00", 4)
			if len(fields) == 4 {
				catalog.Commits = append(catalog.Commits, WorkspaceGitCommit{fields[0], strings.Fields(fields[1]), fields[3], fields[2]})
			}
		}
	}
	return catalog, nil
}

func workspaceGitSelected(snapshot *WorkspaceGitSnapshot, paths []string) (*WorkspaceGitChange, error) {
	if len(paths) != 1 {
		return nil, errors.New("select exactly one changed file")
	}
	for index := range snapshot.Changes {
		if snapshot.Changes[index].Path == paths[0] {
			return &snapshot.Changes[index], nil
		}
	}
	return nil, errors.New("selected file is not in the reviewed changes")
}

func workspaceGitConflict(status string) bool {
	return strings.Contains(status, "U") || status == "AA" || status == "DD"
}

func compareWorkspaceGit(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	change, err := workspaceGitSelected(current, request.Paths)
	if err != nil {
		return nil, err
	}
	review := &WorkspaceGitReview{Path: change.Path, OriginalPath: change.OriginalPath, Comparison: request.Comparison, Revision: change.Revision,
		Conflict: workspaceGitConflict(change.Status)}
	current.Review = review
	switch request.Comparison {
	case "staged":
		review.BeforeLabel, review.AfterLabel = "HEAD", "Index (staged)"
	case "unstaged":
		review.BeforeLabel, review.AfterLabel = "Index", "Working tree (unstaged)"
	case "combined":
		review.BeforeLabel, review.AfterLabel = "HEAD", "Working tree (combined)"
	default:
		return nil, errors.New("comparison must be staged, unstaged or combined")
	}
	if review.Conflict {
		review.Notice = "Unmerged index: resolve the conflict in the conventional editor. This is not an ordinary two-way diff."
		return current, nil
	}
	if change.Bytes > 1024*1024 {
		review.Limited = true
		review.Notice = "File exceeds the 1 MiB focused-code limit. Content identity is retained; open the conventional editor to inspect it."
		return current, nil
	}
	// Untracked and unborn combined comparisons have no original side.
	if (change.Status == "??" && request.Comparison != "staged") || (current.Head == "" && request.Comparison == "combined") {
		absolute, err := workspaceGitSafePath(current.Root, change.Path)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(absolute)
		if os.IsNotExist(err) {
			review.Notice = "Both sides are absent."
			return current, nil
		}
		if err != nil {
			return nil, err
		}
		var content []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(absolute)
			content, err = []byte(target), readErr
		} else if info.Mode().IsRegular() {
			file, openErr := os.Open(absolute)
			if openErr != nil {
				return nil, openErr
			}
			content, err = io.ReadAll(io.LimitReader(file, 1024*1024+1))
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			if len(content) > 1024*1024 {
				return nil, errors.New("file grew beyond the 1 MiB focused-code limit; refresh")
			}
		} else {
			review.Limited = true
			review.Notice = "Directory or submodule: inspect it in conventional Git."
			return current, nil
		}
		if err != nil {
			return nil, err
		}
		review.BeforeLabel += " (absent)"
		if bytes.ContainsRune(content, '\x00') || !utf8.Valid(content) {
			review.Binary = true
			review.Notice = "Binary content; no text comparison."
			return current, nil
		}
		lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
		if len(content) == 0 {
			lines = nil
		}
		var patch strings.Builder
		fmt.Fprintf(&patch, "@@ -0,0 +1,%d @@\n", len(lines))
		for _, line := range lines {
			fmt.Fprintf(&patch, "+%s\n", line)
		}
		if len(content) > 0 && !bytes.HasSuffix(content, []byte{'\n'}) {
			patch.WriteString("\\ No newline at end of file\n")
		}
		review.Patch = patch.String()
	} else {
		contextLines := request.Context
		if contextLines < 3 {
			contextLines = 3
		}
		if contextLines > 1000 {
			return nil, errors.New("comparison context exceeds 1000 lines")
		}
		args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--unified=" + strconv.Itoa(contextLines)}
		if request.Comparison == "staged" {
			args = append(args, "--cached")
		}
		if request.Comparison == "combined" && current.Head != "" {
			args = append(args, "HEAD")
		}
		args = append(args, "--", change.Path)
		if change.OriginalPath != "" {
			args = append(args, change.OriginalPath)
		}
		patch, err := workspaceGitCommand(ctx, current.Root, args...)
		if err != nil {
			return nil, err
		}
		if len(patch) > 2*1024*1024 {
			review.Limited = true
			review.Notice = "Diff exceeds the 2 MiB focused review limit. Open conventional Git to inspect it."
		} else if !utf8.Valid(patch) || bytes.Contains(patch, []byte("Binary files ")) || bytes.Contains(patch, []byte("GIT binary patch")) {
			review.Binary = true
			review.Notice = "Binary change; inspect the file externally."
		} else {
			review.Patch = string(patch)
			if !bytes.Contains(patch, []byte("\n@@ ")) {
				review.Notice = "No text hunks in this comparison. " + string(patch)
			}
		}
	}
	// Detect edits while assembling a comparison instead of tagging newer code
	// with an older reviewed fingerprint.
	after, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return nil, err
	}
	if after.Snapshot != current.Snapshot {
		return nil, errors.New("the project changed while loading the comparison; refresh before continuing")
	}
	return current, nil
}

func mutateWorkspaceGitIndex(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	if current.Branch == "" {
		return nil, errors.New("select a branch before writing")
	}
	var args []string
	if request.Action == "commit-index" {
		if !request.Confirm || strings.TrimSpace(request.Message) == "" || len(request.Message) > 4096 {
			return nil, errors.New("explicitly confirm a commit message of 1 to 4096 bytes")
		}
		staged := false
		for _, change := range current.Changes {
			if workspaceGitConflict(change.Status) {
				return nil, errors.New("resolve conflicts before committing")
			}
			staged = staged || (change.Status != "??" && change.Status[0] != ' ')
		}
		if !staged {
			return nil, errors.New("there are no staged changes to commit")
		}
		if len(request.Paths) != 0 {
			return nil, errors.New("commit-index commits the reviewed index, not working-tree path selections")
		}
		args = []string{"commit", "--message", request.Message}
	} else {
		change, err := workspaceGitSelected(current, request.Paths)
		if err != nil {
			return nil, err
		}
		if workspaceGitConflict(change.Status) {
			return nil, errors.New("resolve and stage conflicted files before continuing")
		}
		paths := []string{change.Path}
		if change.OriginalPath != "" {
			paths = append(paths, change.OriginalPath)
		}
		switch request.Action {
		case "stage":
			// A staged rename has already removed its old path from the index.
			args = []string{"add", "--all", "--", change.Path}
		case "unstage":
			if change.Status == "??" || change.Status[0] == ' ' {
				return nil, errors.New("file has no staged changes")
			}
			if current.Head == "" {
				args = append([]string{"rm", "--cached", "--force", "--"}, paths...)
			} else {
				args = append([]string{"restore", "--staged", "--source=HEAD", "--"}, paths...)
			}
		case "discard":
			if !request.Confirm {
				return nil, errors.New("discard requires explicit confirmation")
			}
			if change.Status == "??" || change.Status[1] == ' ' || strings.ContainsAny(change.Status, "RC") {
				return nil, errors.New("only ordinary tracked working-tree changes can be discarded; the index is preserved")
			}
			args = append([]string{"restore", "--worktree", "--"}, paths...)
		default:
			return nil, errors.New("unsupported index operation")
		}
	}
	if _, err := workspaceGitCommand(ctx, current.Root, args...); err != nil {
		return nil, fmt.Errorf("%w; refresh to inspect state before retrying", err)
	}
	updated, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return nil, fmt.Errorf("%s completed but refresh failed: %w", request.Action, err)
	}
	updated.Notice = request.Action + " completed"
	return updated, nil
}
