package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"dockpipe/src/lib/infrastructure/process"
)

// WorkspaceGitRequest is a human-driven project action, separate from managed
// agent sessions. Writes bind to the exact snapshot last reviewed in the UI.
type WorkspaceGitRequest struct {
	Workdir  string   `json:"workdir"`
	Action   string   `json:"action"`
	Snapshot string   `json:"snapshot,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Message  string   `json:"message,omitempty"`
}

type WorkspaceGitChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type WorkspaceGitSnapshot struct {
	Root     string               `json:"root"`
	Branch   string               `json:"branch"`
	Head     string               `json:"head"`
	Snapshot string               `json:"snapshot"`
	Changes  []WorkspaceGitChange `json:"changes"`
	Diff     string               `json:"diff"`
	Notice   string               `json:"notice,omitempty"`
}

const workspaceGitBytes = 8 * 1024 * 1024 // Maximum review and command output, in bytes.

type workspaceGitBuffer struct{ bytes.Buffer }

func (buffer *workspaceGitBuffer) Write(data []byte) (int, error) {
	if len(data) > workspaceGitBytes-buffer.Len() {
		return 0, errors.New("Git output exceeds the 8 MiB review limit")
	}
	return buffer.Buffer.Write(data)
}

func workspaceGitCommand(ctx context.Context, root string, args ...string) ([]byte, error) {
	arguments := append([]string{"--no-pager", "--literal-pathspecs", "-C", root}, args...)
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	command.WaitDelay = time.Second
	var stdout, stderr workspaceGitBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := process.Run(command); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("git %s: %w\n%s", args[0], err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func readWorkspaceGitSnapshot(ctx context.Context, workdir string) (*WorkspaceGitSnapshot, error) {
	rootBytes, err := workspaceGitCommand(ctx, workdir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root := strings.TrimSuffix(string(rootBytes), "\n")
	branch, branchErr := workspaceGitCommand(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr != nil && !workspaceGitMissingRef(branchErr) {
		return nil, branchErr
	}
	head, headErr := workspaceGitCommand(ctx, root, "rev-parse", "--verify", "--quiet", "HEAD")
	if headErr != nil && !workspaceGitMissingRef(headErr) {
		return nil, headErr
	}
	status, err := workspaceGitCommand(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return nil, err
	}
	result := &WorkspaceGitSnapshot{Root: root, Branch: strings.TrimSpace(string(branch)), Head: strings.TrimSpace(string(head)), Changes: []WorkspaceGitChange{}}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--binary", "--no-renames"}
	if headErr == nil {
		args = append(args, "HEAD")
	}
	diff, err := workspaceGitCommand(ctx, root, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	var review workspaceGitBuffer
	if _, err := review.Write(diff); err != nil {
		return nil, err
	}
	staged, err := workspaceGitCommand(ctx, root, "diff", "--cached", "--no-ext-diff", "--no-textconv", "--binary", "--")
	if err != nil {
		return nil, err
	}
	if len(staged) > 0 {
		if _, err := review.Write(append([]byte("\n--- Staged changes ---\n"), staged...)); err != nil {
			return nil, err
		}
	}
	untrackedBudget := workspaceGitBytes - review.Len()
	for _, entry := range bytes.Split(status, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		if len(entry) < 4 || !utf8.Valid(entry) {
			return nil, errors.New("Git returned an unsupported filename or status")
		}
		change := WorkspaceGitChange{Status: string(entry[:2]), Path: string(entry[3:])}
		if len(result.Changes) >= 2048 {
			return nil, errors.New("Git review exceeds 2048 changed files; narrow the change set first")
		}
		result.Changes = append(result.Changes, change)
		if change.Status != "??" {
			continue
		}
		content, err := workspaceGitUntracked(root, change.Path, &untrackedBudget)
		if err != nil {
			return nil, err
		}

		if _, err := review.Write([]byte("\n--- Untracked: " + change.Path + " ---\n")); err != nil {
			return nil, err
		}
		if _, err := review.Write(content); err != nil {
			return nil, err
		}
	}
	result.Diff = strings.ToValidUTF8(review.String(), "�")
	digest := sha256.New()
	for _, value := range []string{root, result.Branch, result.Head, string(status), review.String()} {
		digest.Write([]byte(value))
		digest.Write([]byte{0})
	}
	result.Snapshot = hex.EncodeToString(digest.Sum(nil))
	return result, nil
}

func workspaceGitMissingRef(err error) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError) && exitError.ExitCode() == 1
}

// RunWorkspaceGit owns the Git lifecycle for project-level human actions. No
// force pushes, implicit commits, automatic stashes, rebases, or merges occur.
func RunWorkspaceGit(ctx context.Context, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	if strings.TrimSpace(request.Workdir) == "" {
		return nil, errors.New("select a project directory first")
	}
	current, err := readWorkspaceGitSnapshot(ctx, request.Workdir)
	if err != nil {
		return nil, err
	}
	if request.Action == "status" {
		return current, nil
	}
	if request.Snapshot == "" || request.Snapshot != current.Snapshot {
		return nil, errors.New("the project changed since review; refresh before continuing")
	}
	if request.Action == "diff" {
		if len(request.Paths) != 1 {
			return nil, errors.New("select exactly one changed file to review")
		}
		var selected *WorkspaceGitChange
		for index := range current.Changes {
			if current.Changes[index].Path == request.Paths[0] {
				selected = &current.Changes[index]
				break
			}
		}
		if selected == nil {
			return nil, errors.New("selected file is not in the reviewed changes")
		}
		if selected.Status == "??" {
			budget := workspaceGitBytes
			content, err := workspaceGitUntracked(current.Root, selected.Path, &budget)
			if err != nil {
				return nil, err
			}
			current.Diff = string(content)
		} else {
			args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames"}
			if current.Head != "" {
				args = append(args, "HEAD")
			} else {
				args = append(args, "--cached")
			}
			content, err := workspaceGitCommand(ctx, current.Root, append(args, "--", selected.Path)...)
			if err != nil {
				return nil, err
			}
			// A reverted working file may still have a different staged version.
			// Show that version rather than an unexplained empty review.
			if len(content) == 0 && selected.Status[0] != ' ' {
				content, err = workspaceGitCommand(ctx, current.Root, "diff", "--cached", "--no-ext-diff", "--no-textconv", "--no-renames", "--", selected.Path)
				if err != nil {
					return nil, err
				}
				current.Notice = "Staged version; working file matches HEAD. Committing uses the saved working file."
			}
			current.Diff = strings.ToValidUTF8(string(content), "�")
		}
		return current, nil
	}
	if current.Branch == "" {
		return nil, errors.New("Git is in detached HEAD state; select a branch before writing")
	}
	var args []string
	switch request.Action {
	case "commit":
		if strings.TrimSpace(request.Message) == "" || len(request.Message) > 4096 || len(request.Paths) == 0 {
			return nil, errors.New("select files and enter a commit message of at most 4096 bytes")
		}
		selected := map[string]bool{}
		for _, path := range request.Paths {
			if selected[path] {
				return nil, errors.New("duplicate selected path")
			}
			found := false
			for _, change := range current.Changes {
				if change.Path == path {
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("selected path %q is not in the reviewed changes", path)
			}
			selected[path] = true
		}
		for _, change := range current.Changes {
			if strings.Contains(change.Status, "U") || change.Status == "AA" || change.Status == "DD" {
				return nil, errors.New("resolve Git conflicts before committing")
			}
		}
		if _, err := workspaceGitCommand(ctx, current.Root, append([]string{"add", "--all", "--"}, request.Paths...)...); err != nil {
			return nil, err
		}
		args = append([]string{"commit", "--only", "--message", request.Message, "--"}, request.Paths...)
	case "pull":
		if len(current.Changes) != 0 {
			return nil, errors.New("commit or set aside local changes before pulling")
		}
		args = []string{"pull", "--ff-only", "--no-rebase"}
	case "push":
		args = []string{"push", "--no-follow-tags", "--set-upstream", "origin", current.Branch + ":refs/heads/" + current.Branch}
	default:
		return nil, fmt.Errorf("unsupported project Git action %q", request.Action)
	}
	if _, err := workspaceGitCommand(ctx, current.Root, args...); err != nil {
		return nil, fmt.Errorf("%w\nRefresh to inspect the current state before retrying. A failed commit may leave selected files staged.", err)
	}
	updated, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return nil, fmt.Errorf("%s completed but refresh failed: %w", request.Action, err)
	}
	updated.Notice = request.Action + " completed"
	return updated, nil
}

func workspaceGitUntracked(root, path string, budget *int) ([]byte, error) {
	absolute := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("cannot review non-regular untracked file %q", path)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(file, int64(*budget+1)))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(content) > *budget {
		return nil, errors.New("Git file content exceeds the 8 MiB review limit")
	}
	*budget -= len(content)
	if !utf8.Valid(content) || bytes.ContainsRune(content, '\x00') {
		digest := sha256.Sum256(content)
		content = []byte(fmt.Sprintf("Binary file: %d bytes, SHA256 %x\n", len(content), digest))
	}
	return content, nil
}
