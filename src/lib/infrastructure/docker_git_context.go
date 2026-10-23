package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const dockerGitDirectory = "/dockpipe-worktree-git"
const dockerGitIndexLimit = 64 << 20
const dockerGitPackLimit = 2 << 30

var dockerGitObjectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type dockerGitContext struct {
	temporary string
	mounts    []string
}

// Each snapshot file is mounted separately. Linux bind mounts retain those file
// inodes after the host temporary directory is removed, including detached runs.
func (snapshot *dockerGitContext) close() {
	if snapshot != nil {
		_ = os.RemoveAll(snapshot.temporary)
	}
}

func dockerGitEnvironment() []string {
	environment := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			environment = append(environment, entry)
		}
	}
	return append(environment, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1",
		"GIT_GRAFT_FILE="+os.DevNull, "GIT_NO_LAZY_FETCH=1")
}

func dockerGitCommand(ctx context.Context, root string, arguments ...string) *exec.Cmd {
	prefix := []string{"--no-pager", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "core.attributesFile=" + os.DevNull, "-C", root}
	command := exec.CommandContext(ctx, "git", append(prefix, arguments...)...)
	command.Env = dockerGitEnvironment()
	command.WaitDelay = time.Second
	return command
}

func dockerGitOutput(ctx context.Context, root string, arguments ...string) ([]byte, error) {
	var output workspaceGitBuffer
	command := dockerGitCommand(ctx, root, arguments...)
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("prepare Git snapshot %s: %w", arguments[0], err)
	}
	return output.Bytes(), nil
}

func dockerGitRead(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > dockerGitIndexLimit {
		return nil, errors.New("prepare Git snapshot: metadata must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, dockerGitIndexLimit+1))
	if err != nil || len(data) > dockerGitIndexLimit {
		return nil, errors.New("prepare Git snapshot: metadata exceeds capacity")
	}
	return data, nil
}

type dockerGitPackWriter struct {
	destination io.Writer
	remaining   int64
}

func (writer *dockerGitPackWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.remaining {
		return 0, errors.New("export Git snapshot: pack exceeds capacity")
	}
	count, err := writer.destination.Write(data)
	writer.remaining -= int64(count)
	return count, err
}

func prepareDockerGitContext(root string, detached bool) (_ *dockerGitContext, resultErr error) {
	info, err := os.Lstat(filepath.Join(root, ".git"))
	if errors.Is(err, os.ErrNotExist) || err == nil && info.IsDir() {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("external Git metadata requires a regular gitfile")
	}
	if detached && runtime.GOOS != "linux" {
		return nil, errors.New("detached external Git snapshots require qualified Linux file-bind lifetime support")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	// Partial clones could fetch objects through source-local credential helpers.
	// Snapshot preparation is entirely local and fails closed instead.
	promisor, err := dockerGitOutput(ctx, root, "config", "--get-regexp", `^(extensions\.partialclone|remote\..*\.promisor)$`)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(promisor)), "\n") {
			if line == "" {
				continue
			}
			key, value, _ := strings.Cut(line, " ")
			inactive := strings.ToLower(strings.TrimSpace(value))
			if strings.HasPrefix(key, "remote.") && (inactive == "false" || inactive == "no" || inactive == "off" || inactive == "0") {
				continue
			}
			return nil, errors.New("prepare Docker Git snapshot: partial-clone repositories are unsupported")
		}
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, err
		}
	}
	sparse, sparseErr := dockerGitOutput(ctx, root, "config", "--type=bool", "--get", "core.sparseCheckout")
	if sparseErr == nil && strings.TrimSpace(string(sparse)) == "true" {
		return nil, errors.New("prepare Docker Git snapshot: sparse checkouts are unsupported")
	}
	if sparseErr != nil {
		var exit *exec.ExitError
		if !errors.As(sparseErr, &exit) || exit.ExitCode() != 1 {
			return nil, sparseErr
		}
	}
	head, err := dockerGitOutput(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("prepare Docker Git snapshot: a committed HEAD is required: %w", err)
	}
	headID := strings.TrimSpace(string(head))
	if !dockerGitObjectID.MatchString(headID) {
		return nil, errors.New("invalid Git snapshot HEAD")
	}
	branch, branchErr := dockerGitOutput(ctx, root, "symbolic-ref", "--quiet", "HEAD")
	if branchErr != nil {
		var exit *exec.ExitError
		if !errors.As(branchErr, &exit) || exit.ExitCode() != 1 {
			return nil, branchErr
		}
	}
	branchName := strings.TrimSpace(string(branch))
	indexPath, err := dockerGitOutput(ctx, root, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return nil, err
	}
	index, err := dockerGitRead(strings.TrimSpace(string(indexPath)))
	if err != nil {
		return nil, err
	}
	temporary, err := os.MkdirTemp("", "dockpipe-git-snapshot-")
	if err != nil {
		return nil, err
	}
	snapshot := &dockerGitContext{temporary: temporary}
	defer func() {
		if resultErr != nil {
			snapshot.close()
		}
	}()
	directory := filepath.Join(temporary, "git")
	if err := os.MkdirAll(filepath.Join(directory, "objects", "pack"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(directory, "refs", "heads"), 0o755); err != nil {
		return nil, err
	}
	objectFormat, err := dockerGitOutput(ctx, root, "rev-parse", "--show-object-format")
	if err != nil {
		return nil, err
	}
	config := "[core]\n\trepositoryformatversion = 0\n\tbare = false\n"
	if strings.TrimSpace(string(objectFormat)) == "sha256" {
		config = "[extensions]\n\tobjectFormat = sha256\n[core]\n\trepositoryformatversion = 1\n\tbare = false\n"
	}
	for _, name := range []string{"core.filemode", "core.ignorecase", "core.symlinks"} {
		value, valueErr := dockerGitOutput(ctx, root, "config", "--type=bool", "--get", name)
		if valueErr == nil {
			config += "\t" + strings.TrimPrefix(name, "core.") + " = " + strings.TrimSpace(string(value)) + "\n"
		} else {
			var exit *exec.ExitError
			if !errors.As(valueErr, &exit) || exit.ExitCode() != 1 {
				return nil, valueErr
			}
		}
	}
	files := map[string][]byte{"config": []byte(config), "HEAD": []byte(headID + "\n")}
	if branchName != "" {
		if _, err := dockerGitOutput(ctx, root, "check-ref-format", branchName); err != nil {
			return nil, err
		}
		files["HEAD"] = []byte("ref: " + branchName + "\n")
		files["packed-refs"] = []byte(headID + " " + branchName + "\n")
	}
	if index != nil {
		files["index"] = index
	}
	shallowPath, err := dockerGitOutput(ctx, root, "rev-parse", "--path-format=absolute", "--git-path", "shallow")
	if err != nil {
		return nil, err
	}
	shallow, err := dockerGitRead(strings.TrimSpace(string(shallowPath)))
	if err != nil {
		return nil, err
	}
	if shallow != nil {
		files["shallow"] = shallow
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o644); err != nil {
			return nil, err
		}
	}
	sharedPath, err := dockerGitOutput(ctx, root, "rev-parse", "--shared-index-path")
	if err != nil {
		return nil, err
	}
	if shared := strings.TrimSpace(string(sharedPath)); shared != "" {
		if !filepath.IsAbs(shared) {
			shared = filepath.Join(root, shared)
		}
		data, err := dockerGitRead(shared)
		if err != nil || data == nil {
			return nil, errors.New("cannot read split Git index")
		}
		name := filepath.Base(shared)
		if !strings.HasPrefix(name, "sharedindex.") || !dockerGitObjectID.MatchString(strings.TrimPrefix(name, "sharedindex.")) {
			return nil, errors.New("invalid split Git index identity")
		}
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o644); err != nil {
			return nil, err
		}
		if _, err := dockerGitOutput(ctx, root, "--git-dir="+directory, "update-index", "--no-split-index"); err != nil {
			return nil, err
		}
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			return nil, err
		}
	}
	pack, err := os.Create(filepath.Join(temporary, "objects.pack"))
	if err != nil {
		return nil, err
	}
	defer pack.Close()
	export := dockerGitCommand(ctx, root, "pack-objects", "--stdout", "--revs", "--indexed-objects", "--delta-base-offset", "--threads=2", "--window-memory=32m")
	export.Stdin = strings.NewReader(headID + "\n")
	export.Stdout = &dockerGitPackWriter{destination: pack, remaining: dockerGitPackLimit}
	if err := export.Run(); err != nil {
		return nil, fmt.Errorf("export Docker Git snapshot: %w", err)
	}
	if _, err := pack.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	importPack := dockerGitCommand(ctx, root, "--git-dir="+directory, "index-pack", "--stdin")
	importPack.Stdin = pack
	if err := importPack.Run(); err != nil {
		return nil, fmt.Errorf("index Docker Git snapshot: %w", err)
	}
	afterHead, headErr := dockerGitOutput(ctx, root, "rev-parse", "--verify", "HEAD")
	afterBranch, afterBranchErr := dockerGitOutput(ctx, root, "symbolic-ref", "--quiet", "HEAD")
	if afterBranchErr != nil {
		var exit *exec.ExitError
		if !errors.As(afterBranchErr, &exit) || exit.ExitCode() != 1 {
			return nil, afterBranchErr
		}
	}
	afterIndex, indexErr := dockerGitRead(strings.TrimSpace(string(indexPath)))
	headUnchanged := headErr == nil && bytes.Equal(head, afterHead)
	branchUnchanged := bytes.Equal(branch, afterBranch)
	indexUnchanged := indexErr == nil && sha256.Sum256(index) == sha256.Sum256(afterIndex)
	if !headUnchanged || !branchUnchanged || !indexUnchanged {
		return nil, errors.New("source Git HEAD or index changed while preparing Docker snapshot")
	}
	gitfile := filepath.Join(temporary, "gitfile")
	if err := os.WriteFile(gitfile, []byte("gitdir: "+dockerGitDirectory+"\n"), 0o644); err != nil {
		return nil, err
	}
	snapshot.mounts = append(snapshot.mounts, HostPathForDocker(gitfile)+":"+containerWorkMount+"/.git:ro")
	// Git requires a refs directory even when its only branch is packed.
	snapshot.mounts = append(snapshot.mounts, HostPathForDocker(filepath.Join(directory, "refs"))+":"+dockerGitDirectory+"/refs:ro")
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		snapshot.mounts = append(snapshot.mounts, HostPathForDocker(path)+":"+dockerGitDirectory+"/"+filepath.ToSlash(relative)+":ro")
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(snapshot.mounts)
	return snapshot, nil
}
