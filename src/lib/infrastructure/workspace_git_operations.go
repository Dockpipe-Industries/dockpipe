package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceGitHistory struct {
	Commits []WorkspaceGitCommit `json:"commits"`
	Offset  int                  `json:"offset"`
	Limit   int                  `json:"limit"`
	HasMore bool                 `json:"has_more"`
}

type WorkspaceGitCommitReview struct {
	Commit  WorkspaceGitCommit `json:"commit"`
	Body    string             `json:"body"`
	Patch   string             `json:"patch"`
	Limited bool               `json:"limited"`
	Notice  string             `json:"notice,omitempty"`
}

type WorkspaceGitStash struct {
	ID       string `json:"id"`
	Selector string `json:"selector"`
	Subject  string `json:"subject"`
}

type WorkspaceGitOperation struct {
	Kind        string   `json:"kind"`
	Current     string   `json:"current"`
	Remaining   []string `json:"remaining"`
	Conflicts   []string `json:"conflicts"`
	CanContinue bool     `json:"can_continue"`
	CanSkip     bool     `json:"can_skip"`
	CanAbort    bool     `json:"can_abort"`
}

// Git's own metadata is the recovery journal. It is included in the reviewed
// fingerprint so an externally advanced sequencer invalidates pending actions.
func workspaceGitOperationState(ctx context.Context, root string, changes []WorkspaceGitChange) (*WorkspaceGitOperation, string, error) {
	state := &WorkspaceGitOperation{Remaining: []string{}, Conflicts: []string{}}
	hash := sha256.New()
	directory, err := workspaceGitCommand(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, "", err
	}
	gitDirectory := strings.TrimSpace(string(directory))
	names := []string{"CHERRY_PICK_HEAD", "REVERT_HEAD", "MERGE_HEAD", "MERGE_MSG", "MERGE_MODE", "ORIG_HEAD", "MERGE_AUTOSTASH", "AUTO_MERGE", "sequencer/todo", "sequencer/head", "sequencer/abort-safety", "sequencer/opts", "rebase-merge", "rebase-apply"}
	for _, name := range names {
		absolute := filepath.Join(gitDirectory, filepath.FromSlash(name))
		info, err := os.Lstat(absolute)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		if info.IsDir() {
			if strings.HasPrefix(name, "rebase-") {
				state.Kind = "rebase"
				fmt.Fprintf(hash, "%s\x00", name)
			}
			continue
		}
		if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			return nil, "", errors.New("unsupported or oversized Git operation metadata")
		}
		data, err := readWorkspaceGitBoundedFile(absolute, 1024*1024)
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(hash, "%s\x00%s\x00", name, data)
		switch name {
		case "CHERRY_PICK_HEAD":
			state.Kind, state.Current = "cherry-pick", strings.TrimSpace(string(data))
		case "REVERT_HEAD":
			state.Kind, state.Current = "revert", strings.TrimSpace(string(data))
		case "MERGE_HEAD":
			state.Kind, state.Current = "merge", strings.TrimSpace(string(data))
		case "sequencer/todo":
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && (fields[0] == "pick" || fields[0] == "revert") {
					if len(state.Remaining) >= 128 {
						return nil, "", errors.New("Git sequencer exceeds 128 commits")
					}
					if state.Kind == "" {
						if fields[0] == "pick" {
							state.Kind = "cherry-pick"
						} else {
							state.Kind = "revert"
						}
					}
					state.Remaining = append(state.Remaining, fields[1])
				}
			}
		}
	}
	if len(state.Remaining) > 0 {
		for _, id := range state.Remaining {
			if len(id) < 4 || len(id) > 64 {
				return nil, "", errors.New("unsupported Git sequencer commit identity")
			}
			for _, character := range id {
				if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
					return nil, "", errors.New("unsupported Git sequencer commit identity")
				}
			}
		}
		resolved, err := workspaceGitCommand(ctx, root, append([]string{"rev-parse", "--revs-only", "--end-of-options"}, state.Remaining...)...)
		if err != nil {
			return nil, "", err
		}
		identities := strings.Fields(string(resolved))
		if len(identities) != len(state.Remaining) {
			return nil, "", errors.New("Git sequencer contains an unresolved commit")
		}
		state.Remaining = identities
	}
	for _, change := range changes {
		if workspaceGitConflict(change.Status) {
			state.Conflicts = append(state.Conflicts, change.Path)
		}
	}
	if state.Kind == "" && len(state.Conflicts) != 0 {
		state.Kind = "conflicts"
	}
	if state.Kind == "" {
		return nil, hex.EncodeToString(hash.Sum(nil)), nil
	}
	supportedOperation := state.Kind == "merge" || state.Kind == "cherry-pick" || state.Kind == "revert"
	state.CanContinue = len(state.Conflicts) == 0 && supportedOperation
	state.CanSkip = state.Kind == "cherry-pick" || state.Kind == "revert"
	state.CanAbort = supportedOperation
	return state, hex.EncodeToString(hash.Sum(nil)), nil
}

func workspaceGitExactCommit(ctx context.Context, root, id string) error {
	if len(id) != 40 && len(id) != 64 {
		return errors.New("select an exact full commit object ID")
	}
	for _, char := range id {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return errors.New("invalid commit object ID")
		}
	}
	output, err := workspaceGitCommand(ctx, root, "rev-parse", "--verify", id+"^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(output)) != id {
		return errors.New("object ID is not an exact commit")
	}
	return nil
}

func workspaceGitLocalBranch(ctx context.Context, root, name string) error {
	if name == "" || len(name) > 256 || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\x00\r\n") {
		return errors.New("select a valid local branch name")
	}
	_, err := workspaceGitCommand(ctx, root, "check-ref-format", "refs/heads/"+name)
	return err
}

func handledWorkspaceGitAction(action string) bool {
	switch action {
	case "branch-create", "branch-switch", "branch-delete", "fetch", "merge", "stash-save", "stash-apply", "stash-pop", "stash-drop", "revert", "cherry-pick", "operation-continue", "operation-skip", "operation-abort", "resolve-stage", "resolve-ours", "resolve-theirs", "resolve-delete":
		return true
	}
	return false
}

func mutateWorkspaceGitExtended(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	args := []string{}
	switch request.Action {
	case "resolve-stage", "resolve-ours", "resolve-theirs", "resolve-delete":
		return resolveWorkspaceGitConflict(ctx, current, request)
	case "branch-create", "branch-switch", "branch-delete":
		if err := workspaceGitLocalBranch(ctx, current.Root, request.Name); err != nil {
			return nil, err
		}
		if request.Action == "branch-create" {
			args = []string{"branch", "--", request.Name}
		}
		if request.Action == "branch-switch" {
			args = []string{"switch", "--no-guess", "--no-overwrite-ignore", "--", request.Name}
		}
		if request.Action == "branch-delete" {
			if !request.Confirm {
				return nil, errors.New("branch deletion requires explicit confirmation")
			}
			args = []string{"branch", "--delete", "--", request.Name}
		}
	case "fetch":
		args = []string{"fetch", "--all", "--no-recurse-submodules", "--no-tags", "--no-prune", "--no-prune-tags"}
	case "merge", "revert", "cherry-pick":
		if !request.Confirm {
			return nil, errors.New("review and explicitly confirm the selected commits")
		}
		if current.Branch == "" {
			return nil, errors.New("select a branch before writing")
		}
		commits := request.Commits
		if request.Action != "cherry-pick" {
			commits = []string{request.Commit}
		}
		if len(commits) == 0 || len(commits) > 128 {
			return nil, errors.New("select between 1 and 128 commits in the desired order")
		}
		seen := map[string]bool{}
		for _, id := range commits {
			if seen[id] {
				return nil, errors.New("duplicate selected commit")
			}
			seen[id] = true
			if err := workspaceGitExactCommit(ctx, current.Root, id); err != nil {
				return nil, err
			}
			if request.Action != "merge" {
				parents, err := workspaceGitCommand(ctx, current.Root, "rev-list", "--parents", "-n", "1", id)
				if err != nil {
					return nil, err
				}
				if len(strings.Fields(string(parents))) > 2 {
					return nil, errors.New("merge commits need an explicit mainline policy; select ordinary commits")
				}
			}
		}
		if request.Action == "merge" {
			args = []string{"merge", "--no-edit", "--no-autostash", "--no-overwrite-ignore", "--", commits[0]}
		} else {
			if err := workspaceGitProtectIgnoredPaths(ctx, current.Root, commits); err != nil {
				return nil, err
			}
			args = append([]string{request.Action}, commits...)
		}
	case "stash-save":
		if len(current.Changes) == 0 {
			return nil, errors.New("there are no local changes to stash")
		}
		if len(request.Message) > 4096 || strings.ContainsRune(request.Message, '\x00') {
			return nil, errors.New("stash message exceeds the supported limit")
		}
		args = []string{"stash", "push"}
		if request.IncludeUntracked {
			args = append(args, "--include-untracked")
		}
		if request.Message != "" {
			args = append(args, "--message", request.Message)
		}
	case "stash-apply", "stash-pop", "stash-drop":
		stashes, err := readWorkspaceGitStashes(ctx, current.Root)
		if err != nil {
			return nil, err
		}
		var selected *WorkspaceGitStash
		for index := range stashes {
			if stashes[index].ID != request.Stash {
				continue
			}
			if request.StashSelector != "" && stashes[index].Selector != request.StashSelector {
				continue
			}
			if selected != nil {
				return nil, errors.New("multiple stash entries share this object ID; select an exact stash selector")
			}
			selected = &stashes[index]
		}
		if selected == nil {
			return nil, errors.New("selected stash is no longer available; refresh")
		}
		action := strings.TrimPrefix(request.Action, "stash-")
		if action == "drop" && !request.Confirm {
			return nil, errors.New("stash deletion requires explicit confirmation")
		}
		args = []string{"stash", action, selected.Selector}
	case "operation-continue", "operation-skip", "operation-abort":
		operation := current.Operation
		if operation == nil || (operation.Kind != "merge" && operation.Kind != "cherry-pick" && operation.Kind != "revert") {
			return nil, errors.New("no supported Git operation is active")
		}
		verb := strings.TrimPrefix(request.Action, "operation-")
		if verb == "continue" && !operation.CanContinue {
			return nil, errors.New("resolve and stage all conflicts before continuing")
		}
		if verb == "skip" && !operation.CanSkip {
			return nil, errors.New("this Git operation cannot skip a commit")
		}
		if (verb == "abort" || verb == "skip") && !request.Confirm {
			return nil, errors.New("skipping or aborting requires explicit confirmation and discards operation resolutions")
		}
		args = []string{operation.Kind, "--" + verb}
	default:
		return nil, errors.New("unsupported Git operation")
	}
	// Preserve the entire reviewed working tree. Git's own looser checkout and
	// stash-apply allowances are intentionally narrowed at this human UI boundary.
	cleanRequired := request.Action == "branch-switch" || request.Action == "merge" || request.Action == "revert" || request.Action == "cherry-pick" || request.Action == "stash-apply" || request.Action == "stash-pop"
	if cleanRequired && len(current.Changes) != 0 {
		return nil, errors.New("commit or stash local changes before this operation")
	}
	_, commandErr := workspaceGitCommand(ctx, current.Root, args...)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w; refresh before retrying because Git may have changed state", ctx.Err())
	}
	updated, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return nil, fmt.Errorf("Git action ended; refresh failed: %w", err)
	}
	if commandErr != nil {
		if updated.Operation != nil || hasWorkspaceGitConflicts(updated) {
			updated.Notice = "Git stopped for resolution. Review conflicted files, save and stage each resolved file, then continue, skip or abort as offered. " + commandErr.Error()
			return updated, nil
		}
		return nil, fmt.Errorf("%w; refresh before retrying", commandErr)
	}
	updated.Notice = request.Action + " completed"
	return updated, nil
}

func hasWorkspaceGitConflicts(snapshot *WorkspaceGitSnapshot) bool {
	for _, change := range snapshot.Changes {
		if workspaceGitConflict(change.Status) {
			return true
		}
	}
	return false
}

func readWorkspaceGitStashes(ctx context.Context, root string) ([]WorkspaceGitStash, error) {
	output, err := workspaceGitCommand(ctx, root, "stash", "list", "--format=%H%x00%gd%x00%gs")
	if err != nil {
		return nil, err
	}
	result := []WorkspaceGitStash{}
	for _, row := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.SplitN(row, "\x00", 3)
		if len(fields) != 3 {
			continue
		}
		if len(result) >= 1024 {
			return nil, errors.New("stash list exceeds 1024 entries")
		}
		result = append(result, WorkspaceGitStash{ID: fields[0], Selector: fields[1], Subject: fields[2]})
	}
	return result, nil
}

func resolveWorkspaceGitConflict(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	if !request.Confirm {
		return nil, errors.New("conflict resolution requires explicit confirmation")
	}
	change, err := workspaceGitSelected(current, request.Paths)
	if err != nil {
		return nil, err
	}
	if !workspaceGitConflict(change.Status) {
		return nil, errors.New("selected path is not an unresolved conflict")
	}
	absolute, err := workspaceGitSafePath(current.Root, change.Path)
	if err != nil {
		return nil, err
	}
	switch request.Action {
	case "resolve-stage":
		info, err := os.Lstat(absolute)
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			if info.Size() > 1024*1024 {
				return nil, errors.New("manual resolution exceeds the 1 MiB text review limit; use conventional Git")
			}
			content, err := readWorkspaceGitBoundedFile(absolute, 1024*1024)
			if err != nil {
				return nil, err
			}
			for _, line := range strings.Split(string(content), "\n") {
				if strings.HasPrefix(line, "<<<<<<< ") || line == "=======" || strings.HasPrefix(line, ">>>>>>> ") || strings.HasPrefix(line, "||||||| ") {
					return nil, errors.New("conflict markers remain; save the resolved file before staging")
				}
			}
		} else if info.Mode()&os.ModeSymlink == 0 {
			return nil, errors.New("unsupported resolution file type")
		}
	case "resolve-ours", "resolve-theirs":
		side := strings.TrimPrefix(request.Action, "resolve-")
		if _, err := workspaceGitCommand(ctx, current.Root, "checkout", "--"+side, "--", change.Path); err != nil {
			return nil, err
		}
	case "resolve-delete":
		if _, err := workspaceGitCommand(ctx, current.Root, "rm", "--", change.Path); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported conflict resolution")
	}
	if request.Action != "resolve-delete" {
		if _, err := workspaceGitCommand(ctx, current.Root, "add", "--", change.Path); err != nil {
			return nil, fmt.Errorf("%w; refresh to inspect the selected resolution", err)
		}
	}
	updated, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return nil, err
	}
	updated.Notice = "Selected conflict resolution staged; review before continuing"
	return updated, nil
}

func readWorkspaceGitBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, errors.New("Git review file grew beyond the supported limit; refresh")
	}
	return content, nil
}

// Cherry-pick/revert can replace ignored untracked files without warning. Check
// only incoming paths, keeping large ignored build trees out of the review scan.
func workspaceGitProtectIgnoredPaths(ctx context.Context, root string, commits []string) error {
	seen := map[string]bool{}
	existing := []string{}
	for _, commit := range commits {
		changed, err := workspaceGitCommand(ctx, root, "diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", "--root", "-z", commit, "--")
		if err != nil {
			return err
		}
		for _, path := range strings.Split(string(changed), "\x00") {
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			if len(seen) > 2048 {
				return errors.New("selected commits exceed 2048 changed paths; narrow the queue")
			}
			absolute, err := workspaceGitSafePath(root, path)
			if err != nil {
				return err
			}
			_, err = os.Lstat(absolute)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			existing = append(existing, path)
		}
	}
	if len(existing) == 0 {
		return nil
	}
	ignored, err := workspaceGitCommand(ctx, root, append([]string{"--no-literal-pathspecs", "check-ignore", "--"}, existing...)...)
	if err != nil && !workspaceGitMissingRef(err) {
		return err
	}
	if len(ignored) != 0 {
		return errors.New("selected commits would replace ignored local files; move or protect those files first")
	}
	return nil
}
