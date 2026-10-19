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
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Metadata is bounded to 2048 changes and the existing 8 MiB Git output limit.
// Content is hashed, never accumulated, with a 256 MiB aggregate read budget.
// This supports large changes without loading every diff into the UI at once.
func readWorkspaceGitSnapshot(ctx context.Context, workdir string) (*WorkspaceGitSnapshot, error) {
	rootBytes, err := workspaceGitCommand(ctx, workdir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root := strings.TrimSuffix(string(rootBytes), "\n")
	branch, err := workspaceGitCommand(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil && !workspaceGitMissingRef(err) {
		return nil, err
	}
	head, err := workspaceGitCommand(ctx, root, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil && !workspaceGitMissingRef(err) {
		return nil, err
	}
	status, err := workspaceGitCommand(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--renames")
	if err != nil {
		return nil, err
	}
	index, err := workspaceGitCommand(ctx, root, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	entries := map[string]string{}
	for _, row := range bytes.Split(index, []byte{0}) {
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(parts) == 2 {
			entries[string(parts[1])] += string(parts[0]) + "\n"
		}
	}
	result := &WorkspaceGitSnapshot{Root: root, Branch: strings.TrimSpace(string(branch)), Head: strings.TrimSpace(string(head)), Changes: []WorkspaceGitChange{}}
	digest := sha256.New()
	fmt.Fprintf(digest, "%s\x00%s\x00%s\x00%s\x00%s", root, result.Branch, result.Head, status, index)
	remaining := int64(256 * 1024 * 1024)
	rows := bytes.Split(status, []byte{0})
	for row := 0; row < len(rows); row++ {
		entry := rows[row]
		if len(entry) == 0 {
			continue
		}
		if len(entry) < 4 || !utf8.Valid(entry) {
			return nil, errors.New("git returned an unsupported filename or status")
		}
		if len(result.Changes) >= 2048 {
			return nil, errors.New("git review exceeds 2048 changed files; narrow the change set first")
		}
		change := WorkspaceGitChange{Status: string(entry[:2]), Path: string(entry[3:])}
		if strings.ContainsAny(change.Status, "RC") {
			row++
			if row >= len(rows) || !utf8.Valid(rows[row]) {
				return nil, errors.New("invalid Git rename record")
			}
			change.OriginalPath = string(rows[row])
		}
		content, size, binary, err := workspaceGitContentIdentity(ctx, root, change.Path, &remaining)
		if err != nil {
			return nil, err
		}
		change.Bytes, change.Binary = size, binary
		revision := sha256.Sum256([]byte(result.Head + "\x00" + change.Path + "\x00" + change.OriginalPath + "\x00" + change.Status + "\x00" + entries[change.Path] + entries[change.OriginalPath] + content))
		change.Revision = hex.EncodeToString(revision[:])
		fmt.Fprintf(digest, "\x00%s", change.Revision)
		result.Changes = append(result.Changes, change)
		if change.Status == "??" && binary {
			result.Diff += fmt.Sprintf("Binary file: %d bytes, SHA256 %s\n", size, strings.Split(content, ":")[0])
		}
	}
	result.Snapshot = hex.EncodeToString(digest.Sum(nil))
	return result, nil
}

func workspaceGitSafePath(root, path string) (string, error) {
	// Git reports slash-separated roots on Windows; filepath.Dir returns native
	// separators. Keep both sides of the parent walk in the same lexical form.
	root = filepath.Clean(filepath.FromSlash(root))
	if path == "" || strings.ContainsRune(path, '\x00') || filepath.IsAbs(path) || filepath.Clean(path) != filepath.FromSlash(path) || path == ".." || strings.HasPrefix(path, "../") {
		return "", errors.New("invalid repository path")
	}
	absolute := filepath.Join(root, filepath.FromSlash(path))
	// Do not follow a repository-controlled parent symlink outside the checkout.
	for parent := filepath.Dir(absolute); parent != root; parent = filepath.Dir(parent) {
		if parent == filepath.Dir(parent) {
			return "", errors.New("path is outside the repository")
		}
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("repository path crosses a symbolic link")
		}
	}
	return absolute, nil
}

func workspaceGitContentIdentity(ctx context.Context, root, path string, remaining *int64) (string, int64, bool, error) {
	absolute, err := workspaceGitSafePath(root, path)
	if err != nil {
		return "", 0, false, err
	}
	info, err := os.Lstat(absolute)
	if os.IsNotExist(err) {
		return "missing", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(absolute)
		sum := sha256.Sum256([]byte(target))
		return "symlink:" + hex.EncodeToString(sum[:]), int64(len(target)), false, err
	}
	if info.IsDir() {
		commit, err := workspaceGitCommand(ctx, absolute, "rev-parse", "--verify", "HEAD")
		if ctx.Err() != nil {
			return "", 0, false, ctx.Err()
		}
		if err != nil {
			return "directory", 0, false, nil
		}
		return "gitlink:" + string(commit), 0, false, nil
	}
	if !info.Mode().IsRegular() {
		return "", 0, false, fmt.Errorf("cannot review non-regular file %q", path)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return "", 0, false, err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 32*1024)
	size := int64(0)
	binary := false
	for {
		if err := ctx.Err(); err != nil {
			return "", 0, false, err
		}
		n, readErr := file.Read(buffer)
		if int64(n) > *remaining {
			return "", 0, false, errors.New("git review exceeds the 256 MiB content-hashing budget")
		}
		*remaining -= int64(n)
		size += int64(n)
		hash.Write(buffer[:n])
		binary = binary || bytes.ContainsRune(buffer[:n], '\x00')
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", 0, false, readErr
		}
	}
	return hex.EncodeToString(hash.Sum(nil)) + ":" + strconv.FormatUint(uint64(info.Mode()), 8), size, binary, nil
}
