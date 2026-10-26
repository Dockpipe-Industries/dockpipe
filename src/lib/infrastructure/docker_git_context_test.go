package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func dockerGitFixture(t *testing.T, root string, input string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull, "-C", root}, args...)...)
	command.Env = dockerGitEnvironment()
	command.Stdin = strings.NewReader(input)
	var output, diagnostics workspaceGitBuffer
	command.Stdout = &output
	command.Stderr = &diagnostics
	if err := command.Run(); err != nil {
		t.Fatalf("fixture Git %v: %v\n%s\n%s", args, err, output.Bytes(), diagnostics.Bytes())
	}
	return strings.TrimSpace(output.String())
}

func dockerLinkedFixture(t *testing.T) (string, string) {
	return dockerLinkedFixtureFormat(t, "sha1")
}

func dockerLinkedFixtureFormat(t *testing.T, format string) (string, string) {
	t.Helper()
	source := t.TempDir()
	dockerGitFixture(t, source, "", "init", "--object-format="+format)
	dockerGitFixture(t, source, "", "config", "user.name", "Fixture")
	dockerGitFixture(t, source, "", "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dockerGitFixture(t, source, "", "add", "tracked.txt")
	dockerGitFixture(t, source, "", "commit", "-m", "fixture")
	linked := filepath.Join(t.TempDir(), "linked")
	dockerGitFixture(t, source, "", "worktree", "add", "-b", "fixture-branch", linked)
	t.Cleanup(func() { dockerGitFixture(t, source, "", "worktree", "remove", "--force", linked) })
	return source, linked
}

func TestDockerGitSnapshotPreservesSplitIndexAndIsolatesPrivateState(t *testing.T) {
	source, linked := dockerLinkedFixture(t)
	if err := os.WriteFile(filepath.Join(source, "private.txt"), []byte("private stash contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dockerGitFixture(t, source, "", "add", "private.txt")
	dockerGitFixture(t, source, "", "stash", "push", "-m", "private-stash")
	stash := dockerGitFixture(t, source, "", "rev-parse", "refs/stash")
	dockerGitFixture(t, source, "", "config", "credential.helper", "!touch credential-helper-must-not-run")
	dockerGitFixture(t, source, "", "config", "core.fsmonitor", "!touch fsmonitor-must-not-run")
	if err := os.WriteFile(filepath.Join(linked, "staged.txt"), []byte("staged blob outside HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dockerGitFixture(t, linked, "", "add", "staged.txt")
	dockerGitFixture(t, linked, "", "update-index", "--assume-unchanged", "tracked.txt")
	cachedTree := dockerGitFixture(t, linked, "", "write-tree")
	dockerGitFixture(t, linked, "", "update-index", "--split-index")
	originalIndex := dockerGitFixture(t, linked, "", "rev-parse", "--git-path", "index")
	indexBefore, err := os.ReadFile(originalIndex)
	if err != nil {
		t.Fatal(err)
	}
	gitfileBefore, _ := os.ReadFile(filepath.Join(linked, ".git"))
	expectedEntries := dockerGitFixture(t, linked, "", "ls-files", "--stage", "-v")
	t.Setenv("GIT_DIR", "/inherited/poison")
	t.Setenv("GIT_WORK_TREE", "/inherited/poison-worktree")
	t.Setenv("GIT_INDEX_FILE", "/inherited/poison-index")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", "/inherited/poison-config")
	snapshot, err := prepareDockerGitContext(linked, false)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.close()
	directory := filepath.Join(snapshot.temporary, "git")
	dockerGitFixture(t, linked, "", "--git-dir="+directory, "cat-file", "-e", cachedTree)
	if actual := dockerGitFixture(t, linked, "", "--git-dir="+directory, "ls-files", "--stage", "-v"); actual != expectedEntries {
		t.Fatalf("index flags/entries differ:\n%s\n%s", actual, expectedEntries)
	}
	if expected, actual := dockerGitFixture(t, linked, "", "diff", "--cached", "--stat"), dockerGitFixture(t, linked, "", "--git-dir="+directory, "diff", "--cached", "--stat"); expected != actual {
		t.Fatalf("cached tree comparison differs: %q vs %q", expected, actual)
	}
	if data := dockerGitFixture(t, linked, "", "--git-dir="+directory, "show", ":staged.txt"); data != "staged blob outside HEAD" {
		t.Fatalf("staged object differs: %q", data)
	}
	command := exec.Command("git", "--git-dir="+directory, "cat-file", "-e", stash)
	command.Env = dockerGitEnvironment()
	if err := command.Run(); err == nil {
		t.Fatal("private stash object leaked into snapshot")
	}
	for _, name := range []string{"hooks", "commondir", "worktrees", "logs", "objects/info/alternates"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("snapshot unexpectedly includes %s", name)
		}
	}
	config, _ := os.ReadFile(filepath.Join(directory, "config"))
	if strings.Contains(string(config), "credential") || strings.Contains(string(config), source) {
		t.Fatal("source-local configuration leaked")
	}
	afterIndex, _ := os.ReadFile(originalIndex)
	afterGitfile, _ := os.ReadFile(filepath.Join(linked, ".git"))
	if string(indexBefore) != string(afterIndex) || string(gitfileBefore) != string(afterGitfile) {
		t.Fatal("original worktree metadata changed")
	}
}

func TestDockerGitSnapshotPreservesUnmergedStages(t *testing.T) {
	_, linked := dockerLinkedFixture(t)
	first := dockerGitFixture(t, linked, "base\n", "hash-object", "-w", "--stdin")
	second := dockerGitFixture(t, linked, "ours\n", "hash-object", "-w", "--stdin")
	third := dockerGitFixture(t, linked, "theirs\n", "hash-object", "-w", "--stdin")
	input := "100644 " + first + " 1\tconflict.txt\n100644 " + second + " 2\tconflict.txt\n100644 " + third + " 3\tconflict.txt\n"
	dockerGitFixture(t, linked, input, "update-index", "--index-info")
	snapshot, err := prepareDockerGitContext(linked, false)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.close()
	directory := filepath.Join(snapshot.temporary, "git")
	if expected, actual := dockerGitFixture(t, linked, "", "ls-files", "--unmerged"), dockerGitFixture(t, linked, "", "--git-dir="+directory, "ls-files", "--unmerged"); expected != actual {
		t.Fatalf("unmerged index differs: %q vs %q", expected, actual)
	}
	for _, object := range []string{first, second, third} {
		dockerGitFixture(t, linked, "", "--git-dir="+directory, "cat-file", "-e", object)
	}
}

func TestDockerGitSnapshotOrdinaryDirectoryAndPartialClone(t *testing.T) {
	root := t.TempDir()
	snapshot, err := prepareDockerGitContext(root, false)
	if err != nil || snapshot != nil {
		t.Fatalf("ordinary directory changed: %v %v", snapshot, err)
	}
	_, linked := dockerLinkedFixture(t)
	dockerGitFixture(t, linked, "", "config", "remote.origin.promisor", "false")
	ordinary, err := prepareDockerGitContext(linked, false)
	if err != nil {
		probe := dockerGitFailureDiagnostics(t, linked)
		data, marshalErr := json.Marshal(probe)
		if marshalErr != nil {
			t.Logf("Git snapshot diagnostic serialization failed: %v", marshalErr)
		} else {
			t.Logf("Git snapshot failure diagnostics: %s", data)
		}
		t.Fatalf("inactive promisor setting was treated as partial clone: %v", err)
	}
	ordinary.close()
	dockerGitFixture(t, linked, "", "config", "remote.origin.promisor", "true")
	if _, err := prepareDockerGitContext(linked, false); err == nil || !strings.Contains(err.Error(), "partial-clone") {
		t.Fatalf("expected explicit local-only failure, got %v", err)
	}
}

func TestDockerGitSnapshotPackBoundAndCancellation(t *testing.T) {
	var output strings.Builder
	writer := &dockerGitPackWriter{destination: &output, remaining: 3}
	if _, err := writer.Write([]byte("four")); err == nil || output.Len() != 0 {
		t.Fatal("pack capacity was not enforced before writing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := dockerGitOutput(ctx, t.TempDir(), "status"); err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("cancelled Git command did not stop promptly: %v", err)
	}
}

func TestDockerGitSnapshotRejectsConcurrentIndexChangeAndCleansUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git interception fixture; native snapshot coverage is separate")
	}
	_, linked := dockerLinkedFixture(t)
	actualGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := t.TempDir()
	script := `#!/bin/sh
case " $* " in
    *" pack-objects "*)
        printf 'concurrent index change\n' > "$DOCKPIPE_GIT_FIXTURE_ROOT/concurrent.txt"
        "$DOCKPIPE_GIT_FIXTURE_REAL" -C "$DOCKPIPE_GIT_FIXTURE_ROOT" add concurrent.txt || exit 1
        ;;
esac
exec "$DOCKPIPE_GIT_FIXTURE_REAL" "$@"
`
	if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	temporary := t.TempDir()
	t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMPDIR", temporary)
	t.Setenv("DOCKPIPE_GIT_FIXTURE_ROOT", linked)
	t.Setenv("DOCKPIPE_GIT_FIXTURE_REAL", actualGit)
	snapshot, err := prepareDockerGitContext(linked, false)
	if snapshot != nil || err == nil || !strings.Contains(err.Error(), "changed while preparing") {
		t.Fatalf("concurrent index change was accepted: %v %v", snapshot, err)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed snapshot left temporary Git data: %v %v", entries, err)
	}
}

func TestDockerGitSnapshotRejectsSparseCheckout(t *testing.T) {
	_, linked := dockerLinkedFixture(t)
	dockerGitFixture(t, linked, "", "config", "core.sparseCheckout", "true")
	if _, err := prepareDockerGitContext(linked, false); err == nil || !strings.Contains(err.Error(), "sparse") {
		t.Fatalf("unsupported sparse checkout was not explicit: %v", err)
	}
}

func TestDockerGitSnapshotSHA256AndShallowHistory(t *testing.T) {
	t.Run("sha256", func(t *testing.T) {
		_, linked := dockerLinkedFixtureFormat(t, "sha256")
		snapshot, err := prepareDockerGitContext(linked, false)
		if err != nil {
			t.Fatal(err)
		}
		defer snapshot.close()
		directory := filepath.Join(snapshot.temporary, "git")
		if format := dockerGitFixture(t, linked, "", "--git-dir="+directory, "rev-parse", "--show-object-format"); format != "sha256" {
			t.Fatalf("object format differs: %q", format)
		}
		dockerGitFixture(t, linked, "", "--git-dir="+directory, "cat-file", "-e", "HEAD:tracked.txt")
	})
	t.Run("shallow", func(t *testing.T) {
		source, _ := dockerLinkedFixture(t)
		shallow := filepath.Join(t.TempDir(), "shallow")
		dockerGitFixture(t, source, "", "clone", "--depth=1", "file://"+filepath.ToSlash(source), shallow)
		linked := filepath.Join(t.TempDir(), "linked-shallow")
		dockerGitFixture(t, shallow, "", "worktree", "add", "-b", "shallow-branch", linked)
		t.Cleanup(func() { dockerGitFixture(t, shallow, "", "worktree", "remove", "--force", linked) })
		snapshot, err := prepareDockerGitContext(linked, false)
		if err != nil {
			t.Fatal(err)
		}
		defer snapshot.close()
		directory := filepath.Join(snapshot.temporary, "git")
		if value := dockerGitFixture(t, linked, "", "--git-dir="+directory, "rev-parse", "--is-shallow-repository"); value != "true" {
			t.Fatalf("shallow boundary was lost: %q", value)
		}
		dockerGitFixture(t, linked, "", "--git-dir="+directory, "log", "--oneline", "HEAD")
	})
}

// Diagnostics run only after the original snapshot has already failed. They do
// not retry snapshot acceptance or change the original error or WaitDelay.
func dockerGitFailureDiagnostics(t *testing.T, root string) map[string]any {
	t.Helper()
	report := map[string]any{"probe_only": true}
	git, err := exec.LookPath("git")
	if err != nil {
		report["lookup_error"] = err.Error()
		return report
	}
	report["git_path"] = git
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	version, err := dockerGitOutput(ctx, root, "--version")
	report["git_version"] = strings.TrimSpace(string(version))
	if err != nil {
		report["version_error"] = err.Error()
		return report
	}
	head, err := dockerGitOutput(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		report["head_error"] = err.Error()
		return report
	}
	directory := t.TempDir()
	pack, err := os.Create(filepath.Join(directory, "objects.pack"))
	if err != nil {
		report["pack_error"] = err.Error()
		return report
	}
	defer pack.Close()
	stderr, err := os.Create(filepath.Join(directory, "stderr.log"))
	if err != nil {
		report["stderr_error"] = err.Error()
		return report
	}
	defer stderr.Close()
	trace := filepath.Join(directory, "trace2.jsonl")
	command := dockerGitCommand(ctx, root, "pack-objects", "--stdout", "--revs", "--indexed-objects", "--delta-base-offset", "--threads=2", "--window-memory=32m")
	// dockerGitCommand removes inherited GIT_* configuration. Add only the
	// fixture-local trace destination after sanitization; never dump environment.
	command.Env = append(command.Env, "GIT_TRACE2_EVENT="+trace)
	input := strings.NewReader(strings.TrimSpace(string(head)) + "\n")
	started := time.Now()
	output := &dockerGitDiagnosticWriter{
		destination: &dockerGitPackWriter{destination: pack, remaining: dockerGitPackLimit},
		started:     started,
	}
	command.Stdin = input
	command.Stdout = output
	command.Stderr = stderr
	err = command.Run()
	report["command"] = command.Args
	report["elapsed_ms"] = time.Since(started).Milliseconds()
	report["wait_delay_ms"] = command.WaitDelay.Milliseconds()
	report["wait_delay_expired"] = errors.Is(err, exec.ErrWaitDelay)
	report["stdin_bytes_remaining"] = input.Len()
	report["stdout_bytes"] = output.bytes
	report["stdout_first_write_ms"] = output.firstWrite.Milliseconds()
	report["stdout_last_write_ms"] = output.lastWrite.Milliseconds()
	report["stdout_last_return_ms"] = output.lastReturn.Milliseconds()
	if command.ProcessState != nil {
		report["exit_code"] = command.ProcessState.ExitCode()
		report["primary_process_success"] = command.ProcessState.Success()
	}
	if err != nil {
		report["error"] = err.Error()
	}
	if ctx.Err() != nil {
		report["context_error"] = ctx.Err().Error()
	}
	for name, path := range map[string]string{"stderr": stderr.Name(), "trace2": trace} {
		data, truncated, readErr := dockerGitDiagnosticRead(path)
		report[name] = string(data)
		report[name+"_truncated"] = truncated
		if readErr != nil {
			report[name+"_read_error"] = readErr.Error()
		}
	}
	return report
}

type dockerGitDiagnosticWriter struct {
	destination io.Writer
	started     time.Time
	firstWrite  time.Duration
	lastWrite   time.Duration
	lastReturn  time.Duration
	bytes       int64
}

func (writer *dockerGitDiagnosticWriter) Write(data []byte) (int, error) {
	if writer.bytes == 0 {
		writer.firstWrite = time.Since(writer.started)
	}
	writer.lastWrite = time.Since(writer.started)
	count, err := writer.destination.Write(data)
	writer.bytes += int64(count)
	writer.lastReturn = time.Since(writer.started)
	return count, err
}

func dockerGitDiagnosticRead(path string) ([]byte, bool, error) {
	const limit = 64 << 10
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if len(data) > limit {
		return data[:limit], true, err
	}
	return data, false, err
}

func TestDockerGitFailureDiagnosticProbeAndReadBound(t *testing.T) {
	_, linked := dockerLinkedFixture(t)
	report := dockerGitFailureDiagnostics(t, linked)
	if report["error"] != nil || report["exit_code"] != 0 || report["primary_process_success"] != true || report["wait_delay_ms"] != int64(1000) {
		t.Fatalf("diagnostic probe did not preserve the actual pack contract: %+v", report)
	}
	if count, ok := report["stdout_bytes"].(int64); !ok || count == 0 || report["stdin_bytes_remaining"] != 0 {
		t.Fatalf("diagnostic probe did not consume the fixture input and pack: %+v", report)
	}
	if report["trace2"] == "" || report["trace2_truncated"] != false {
		t.Fatalf("fixture-local trace was not captured: %+v", report)
	}
	path := filepath.Join(t.TempDir(), "wide.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", (64<<10)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	data, truncated, err := dockerGitDiagnosticRead(path)
	if err != nil || !truncated || len(data) != 64<<10 {
		t.Fatalf("diagnostic read exceeded its log bound: bytes=%d truncated=%v error=%v", len(data), truncated, err)
	}
}
