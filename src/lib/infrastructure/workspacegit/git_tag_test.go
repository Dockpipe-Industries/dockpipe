package workspacegit

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func workspaceGitTagFixture(t *testing.T) (string, string, *WorkspaceGitSnapshot, WorkspaceGitRequest) {
	t.Helper()
	root := initGitSessionTestRepo(t)
	target := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("next commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "add", "README.md")
	mustGitOutput(t, root, "commit", "-m", "next")
	remote := t.TempDir()
	mustGitOutput(t, remote, "init", "--bare")
	mustGitOutput(t, root, "remote", "add", "origin", remote)
	state := workspaceGitTagStatus(t, root)
	request := WorkspaceGitRequest{Workdir: root, Action: "tag-publish", Snapshot: state.Snapshot,
		Name: "reviewed/example", Commit: target, Confirm: true}
	return root, remote, state, request
}

func workspaceGitTagStatus(t *testing.T, root string) *WorkspaceGitSnapshot {
	t.Helper()
	state, err := RunWorkspaceGit(workspaceGitTestContext(t), WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func workspaceGitTagMetadata(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	files := map[string][32]byte{}
	gitDirectory := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "--absolute-git-dir"))
	if err := filepath.WalkDir(gitDirectory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(gitDirectory, path)
		if err != nil {
			return err
		}
		files[name] = sha256.Sum256(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestWorkspaceGitTagPublishExactAncestorAndIdempotent(t *testing.T) {
	root, remote, before, request := workspaceGitTagFixture(t)
	metadata := workspaceGitTagMetadata(t, root)
	after, err := RunWorkspaceGit(workspaceGitTestContext(t), request)
	if err != nil {
		t.Fatal(err)
	}
	proof := after.TagPublication
	if proof == nil || proof.Commit != request.Commit || proof.ObservedCommit != request.Commit || proof.Remote != "origin" ||
		proof.Name != request.Name || proof.DestinationRef != "refs/tags/"+request.Name || proof.AlreadyPublished || proof.Recovered || len(proof.RemoteIdentity) != 64 {
		t.Fatalf("publication proof = %+v", proof)
	}
	if after.Head != before.Head || after.Snapshot != before.Snapshot || len(after.Changes) != 0 {
		t.Fatal("tag publication changed the reviewed project")
	}
	if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
		t.Fatalf("published %s, want exact ancestor %s", got, request.Commit)
	}
	if refs := mustGitOutput(t, root, "for-each-ref", "refs/tags/"); refs != "" {
		t.Fatalf("created local tags: %s", refs)
	}
	if refs := mustGitOutput(t, remote, "for-each-ref", "refs/heads/"); refs != "" {
		t.Fatalf("changed remote branches: %s", refs)
	}
	if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
		t.Fatal("publication modified local Git metadata")
	}
	pushes := 0
	run := func(ctx context.Context, root string, arguments ...string) ([]byte, error) {
		if arguments[0] == "push" {
			pushes++
		}
		return workspaceGitCommand(ctx, root, arguments...)
	}
	repeated, err := publishWorkspaceGitTagWithCommand(workspaceGitTestContext(t), after, request, run)
	if err != nil || pushes != 0 || repeated.TagPublication == nil || !repeated.TagPublication.AlreadyPublished {
		t.Fatalf("idempotence = %+v, pushes=%d, err=%v", repeated, pushes, err)
	}
	if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
		t.Fatal("idempotent verification modified local Git metadata")
	}
}

func TestWorkspaceGitTagPublishMatchesRepositoryObjectFormat(t *testing.T) {
	t.Setenv("GIT_DEFAULT_HASH", "sha256")
	_, remote, _, request := workspaceGitTagFixture(t)
	if len(request.Commit) != 64 {
		t.Fatal("fixture did not create the selected SHA-256 repository")
	}
	result, err := RunWorkspaceGit(workspaceGitTestContext(t), request)
	if err != nil || result.TagPublication.ObservedCommit != request.Commit {
		t.Fatalf("SHA-256 publication = %+v, %v", result, err)
	}
	if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
		t.Fatal("remote object format or commit changed")
	}
}

func TestWorkspaceGitTagPublishRejectsInvalidNamesTargetsAndConfirmation(t *testing.T) {
	root, remote, _, valid := workspaceGitTagFixture(t)
	blob := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD:README.md"))
	unreachable := strings.TrimSpace(mustGitOutput(t, root, "commit-tree", "HEAD^{tree}", "-m", "unreachable root"))
	for _, name := range []string{"", "-option", "refs/heads/other", "bad..name", "bad name", "bad:name", "*.tag", "bad\nname", "bad.lock", strings.Repeat("a", 257)} {
		t.Run("name-"+strings.ReplaceAll(name, "\n", "newline"), func(t *testing.T) {
			request := valid
			request.Name = name
			if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
				t.Fatal("invalid tag name accepted")
			}
		})
	}
	for _, target := range []string{valid.Commit[:12], valid.Commit + "~1", strings.ToUpper(valid.Commit), strings.Repeat("1", 40), blob, unreachable} {
		t.Run("commit-"+target, func(t *testing.T) {
			request := valid
			request.Commit = target
			if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
				t.Fatal("invalid or unreachable tag commit accepted")
			}
		})
	}
	request := valid
	request.Confirm = false
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("missing confirmation = %v", err)
	}
	if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
		t.Fatalf("invalid request changed remote refs: %s", refs)
	}
}

func TestWorkspaceGitTagPublishRejectsStaleAndDirtyProjects(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(map[bool]string{false: "worktree", true: "index"}[staged], func(t *testing.T) {
			root, remote, _, request := workspaceGitTagFixture(t)
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("unreviewed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if staged {
				mustGitOutput(t, root, "add", "README.md")
			}
			if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "changed since review") {
				t.Fatalf("stale snapshot = %v", err)
			}
			request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
			if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "clean project") {
				t.Fatalf("reviewed dirty project = %v", err)
			}
			if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
				t.Fatal("dirty request changed remote refs")
			}
		})
	}
}

func TestWorkspaceGitTagPublishUsesSingleEffectivePushDestination(t *testing.T) {
	root, fetchRemote, _, request := workspaceGitTagFixture(t)
	pushRemote := t.TempDir()
	mustGitOutput(t, pushRemote, "init", "--bare")
	mustGitOutput(t, root, "remote", "set-url", "--push", "origin", pushRemote)
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(mustGitOutput(t, pushRemote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
		t.Fatal("did not verify the effective push destination")
	}
	if refs := mustGitOutput(t, fetchRemote, "for-each-ref"); refs != "" {
		t.Fatal("published to the fetch destination")
	}
	mustGitOutput(t, root, "remote", "set-url", "--add", "--push", "origin", fetchRemote)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("multiple push destinations = %v", err)
	}
}

func TestWorkspaceGitTagPublishRejectsConflictingRemoteAndAnnotatedObject(t *testing.T) {
	for _, annotated := range []bool{false, true} {
		t.Run(map[bool]string{false: "other-commit", true: "annotated-object"}[annotated], func(t *testing.T) {
			root, remote, _, request := workspaceGitTagFixture(t)
			ref := "refs/tags/" + request.Name
			mustGitOutput(t, root, "push", remote, "HEAD:"+ref)
			if annotated {
				mustGitOutput(t, remote, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "tag", "-a", "-f", request.Name, request.Commit, "-m", "annotation")
			}
			before := mustGitOutput(t, remote, "rev-parse", ref)
			if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "different object") {
				t.Fatalf("conflict = %v", err)
			}
			if got := mustGitOutput(t, remote, "rev-parse", ref); got != before {
				t.Fatal("conflicting remote object was overwritten")
			}
		})
	}
}

func TestWorkspaceGitTagPublishRaceCannotOverwriteCompetingTag(t *testing.T) {
	root, remote, state, request := workspaceGitTagFixture(t)
	ref := "refs/tags/" + request.Name
	pushes := 0
	run := func(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
		if arguments[0] == "push" {
			pushes++
			mustGitOutput(t, root, "push", remote, "HEAD:"+ref)
			for _, argument := range arguments {
				if argument == "--force" || argument == "-f" || argument == "--delete" || strings.HasPrefix(argument, "+") {
					t.Fatalf("unsafe push argument %q", argument)
				}
			}
		}
		return workspaceGitCommand(ctx, directory, arguments...)
	}
	_, err := publishWorkspaceGitTagWithCommand(workspaceGitTestContext(t), state, request, run)
	if err == nil || !strings.Contains(err.Error(), "different object") || pushes != 1 {
		t.Fatalf("race result = %v, pushes=%d", err, pushes)
	}
	if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", ref)); got != state.Head {
		t.Fatal("competing remote tag was overwritten")
	}
}

func TestWorkspaceGitTagPublishTransportFailuresAndExactRecovery(t *testing.T) {
	for _, mode := range []string{"observe-before", "push-before", "lost-response", "observe-after", "ambiguous-after"} {
		t.Run(mode, func(t *testing.T) {
			root, remote, state, request := workspaceGitTagFixture(t)
			observations := 0
			pushes := 0
			run := func(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
				if arguments[0] == "ls-remote" && arguments[1] == "--refs" {
					observations++
					if mode == "observe-before" || mode == "observe-after" && observations == 2 {
						return nil, errors.New("private transport credential diagnostic")
					}
					if mode == "ambiguous-after" && observations == 2 {
						row := request.Commit + "\trefs/tags/" + request.Name + "\n"
						return []byte(row + row), nil
					}
				}
				if arguments[0] == "push" {
					pushes++
					if mode == "push-before" {
						return nil, errors.New("private transport credential diagnostic")
					}
					output, err := workspaceGitCommand(ctx, directory, arguments...)
					if mode == "lost-response" && err == nil {
						return nil, errors.New("response lost after remote accepted push")
					}
					return output, err
				}
				return workspaceGitCommand(ctx, directory, arguments...)
			}
			result, err := publishWorkspaceGitTagWithCommand(workspaceGitTestContext(t), state, request, run)
			if mode == "lost-response" {
				if err != nil || result.TagPublication == nil || !result.TagPublication.Recovered || result.TagPublication.AlreadyPublished {
					t.Fatalf("exact recovery = %+v, %v", result, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "private transport") {
				t.Fatalf("failure or diagnostic leak = %v", err)
			}
			if pushes > 1 {
				t.Fatal("transport failure caused a blind retry")
			}
			if mode == "observe-before" || mode == "push-before" {
				if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
					t.Fatal("failed transport unexpectedly published")
				}
			} else if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
				t.Fatal("uncertain result did not preserve the actually accepted tag")
			}
			if got := workspaceGitTagStatus(t, root); got.Snapshot != state.Snapshot {
				t.Fatal("transport handling changed local Git state")
			}
		})
	}
}

func TestWorkspaceGitTagPublishRevalidatesProjectAndDestinationBeforePush(t *testing.T) {
	for _, change := range []string{"worktree", "destination"} {
		t.Run(change, func(t *testing.T) {
			root, remote, state, request := workspaceGitTagFixture(t)
			other := t.TempDir()
			mustGitOutput(t, other, "init", "--bare")
			pushes := 0
			run := func(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
				if arguments[0] == "push" {
					pushes++
				}
				output, err := workspaceGitCommand(ctx, directory, arguments...)
				if arguments[0] == "ls-remote" && arguments[1] == "--refs" && err == nil {
					if change == "worktree" {
						if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("concurrent change\n"), 0o644); err != nil {
							t.Fatal(err)
						}
					} else {
						mustGitOutput(t, root, "remote", "set-url", "--push", "origin", other)
					}
				}
				return output, err
			}
			if _, err := publishWorkspaceGitTagWithCommand(workspaceGitTestContext(t), state, request, run); err == nil || pushes != 0 {
				t.Fatalf("concurrent %s was accepted: pushes=%d, err=%v", change, pushes, err)
			}
			if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
				t.Fatal("concurrent change published a tag")
			}
		})
	}
}

func TestWorkspaceGitTagPublishDeadlineAndCancellationPreserveOutcome(t *testing.T) {
	_, _, state, request := workspaceGitTagFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	run := func(ctx context.Context, root string, arguments ...string) ([]byte, error) {
		if arguments[0] == "ls-remote" && arguments[1] == "--refs" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return workspaceGitCommand(ctx, root, arguments...)
	}
	started := time.Now()
	if _, err := publishWorkspaceGitTagWithCommand(ctx, state, request, run); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancelled observation did not finish promptly")
	}

	_, remote, state, request := workspaceGitTagFixture(t)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	run = func(ctx context.Context, root string, arguments ...string) ([]byte, error) {
		output, err := workspaceGitCommand(ctx, root, arguments...)
		if arguments[0] == "push" && err == nil {
			cancel()
		}
		return output, err
	}
	if _, err := publishWorkspaceGitTagWithCommand(ctx, state, request, run); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation after accepted push = %v", err)
	}
	if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
		t.Fatal("cancellation attempted to roll back an accepted remote tag")
	}
}

func TestWorkspaceGitTagObservationRejectsAmbiguityAndWrongObjectFormat(t *testing.T) {
	ref := "refs/tags/example"
	oid := strings.Repeat("a", 40)
	for _, value := range []string{"\n", oid + " " + ref, oid + "\trefs/tags/other\n", oid + "\t" + ref + "\nextra", strings.Repeat("b", 64) + "\t" + ref, strings.Repeat("A", 40) + "\t" + ref, oid + "\t" + ref + "\r\n"} {
		if _, err := parseWorkspaceGitTagObservation([]byte(value), ref, 40); err == nil {
			t.Fatalf("accepted ambiguous observation %q", value)
		}
	}
	for _, length := range []int{40, 64} {
		oid := strings.Repeat("a", length)
		if got, err := parseWorkspaceGitTagObservation([]byte(oid+"\t"+ref+"\n"), ref, length); err != nil || got != oid {
			t.Fatalf("canonical OID = %q, %v", got, err)
		}
	}
}

func TestWorkspaceGitTagPublishRelativeDestinationCannotBecomeRemoteAlias(t *testing.T) {
	root, remote, _, request := workspaceGitTagFixture(t)
	relative, err := filepath.Rel(root, remote)
	if err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "remote", "set-url", "--push", "origin", relative)
	state := workspaceGitTagStatus(t, root)
	request.Snapshot = state.Snapshot
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "remote", "add", "other-remote", remote)
	mustGitOutput(t, root, "remote", "set-url", "--push", "origin", "other-remote")
	state = workspaceGitTagStatus(t, root)
	request.Snapshot = state.Snapshot
	request.Name = "must-not-publish-through-alias"
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "observation failed") {
		t.Fatalf("relative URL reinterpreted as remote alias: %v", err)
	}
	if refs := mustGitOutput(t, remote, "for-each-ref", "refs/tags/"+request.Name); refs != "" {
		t.Fatal("published through a remote alias instead of the literal path")
	}
}

func TestWorkspaceGitTagPublishRejectsActiveOperationAndMissingOrigin(t *testing.T) {
	root, remote, _, request := workspaceGitTagFixture(t)
	gitDirectory := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "--absolute-git-dir"))
	operation := filepath.Join(gitDirectory, "CHERRY_PICK_HEAD")
	if err := os.WriteFile(operation, []byte(request.Commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "active Git operation") {
		t.Fatalf("active operation = %v", err)
	}
	if err := os.Remove(operation); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "remote", "remove", "origin")
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "cannot resolve") {
		t.Fatalf("missing origin = %v", err)
	}
	if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
		t.Fatal("rejected request changed remote refs")
	}
}

func TestWorkspaceGitTagPublishPreservesLocalTagsAndDisablesFollowTags(t *testing.T) {
	root, remote, _, request := workspaceGitTagFixture(t)
	mustGitOutput(t, root, "tag", request.Name, "HEAD")
	mustGitOutput(t, root, "tag", "-a", "unrelated-annotated", request.Commit, "-m", "unrelated")
	mustGitOutput(t, root, "config", "push.followTags", "true")
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	metadata := workspaceGitTagMetadata(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
		t.Fatal("local tags or Git metadata changed")
	}
	refs := strings.Fields(mustGitOutput(t, remote, "for-each-ref", "--format=%(refname)"))
	if !reflect.DeepEqual(refs, []string{"refs/tags/" + request.Name}) {
		t.Fatalf("published unrelated refs: %v", refs)
	}
	if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
		t.Fatal("published local tag object instead of explicit immutable commit")
	}
}

func TestWorkspaceGitTagPublishRejectsLiteralPushRewriteBeforeMutation(t *testing.T) {
	root, remoteA, _, request := workspaceGitTagFixture(t)
	remoteB := t.TempDir()
	mustGitOutput(t, remoteB, "init", "--bare")
	mustGitOutput(t, root, "remote", "set-url", "--push", "origin", remoteA)
	mustGitOutput(t, root, "config", "url."+remoteB+".pushInsteadOf", remoteA)
	if got := strings.TrimSpace(mustGitOutput(t, root, "remote", "get-url", "--push", "origin")); got != remoteA {
		t.Fatalf("fixture explicit pushurl did not remain A: %q", got)
	}
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	metadata := workspaceGitTagMetadata(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil {
		t.Fatal("literal push rewrite was accepted")
	}
	for _, remote := range []string{remoteA, remoteB} {
		if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
			t.Fatalf("rewrite guard mutated a remote before rejection: %s", refs)
		}
	}
	if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
		t.Fatal("rewrite rejection modified local Git configuration or metadata")
	}
}

func TestWorkspaceGitTagPublishRejectsChainedInsteadOfBeforeMutation(t *testing.T) {
	root, remoteA, _, request := workspaceGitTagFixture(t)
	remoteB := t.TempDir()
	mustGitOutput(t, remoteB, "init", "--bare")
	mustGitOutput(t, root, "remote", "set-url", "origin", "rewrite-start")
	mustGitOutput(t, root, "config", "url."+remoteA+".insteadOf", "rewrite-start")
	mustGitOutput(t, root, "config", "url."+remoteB+".insteadOf", remoteA)
	if got := strings.TrimSpace(mustGitOutput(t, root, "remote", "get-url", "--push", "origin")); got != remoteA {
		t.Fatalf("fixture origin did not resolve the first rewrite: %q", got)
	}
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	metadata := workspaceGitTagMetadata(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "additional URL rewrites") {
		t.Fatalf("chained rewrite = %v", err)
	}
	for _, remote := range []string{remoteA, remoteB} {
		if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
			t.Fatalf("chained rewrite mutated a remote: %s", refs)
		}
	}
	if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
		t.Fatal("chained rewrite rejection modified local metadata")
	}
}

func TestWorkspaceGitTagPublishAcceptsOrdinaryResolvedOriginRewrite(t *testing.T) {
	for _, kind := range []string{"insteadOf", "pushInsteadOf"} {
		t.Run(kind, func(t *testing.T) {
			root, remote, _, request := workspaceGitTagFixture(t)
			mustGitOutput(t, root, "remote", "set-url", "origin", "ordinary-rewrite-start")
			mustGitOutput(t, root, "config", "url."+remote+"."+kind, "ordinary-rewrite-start")
			request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
			metadata := workspaceGitTagMetadata(t, root)
			result, err := RunWorkspaceGit(workspaceGitTestContext(t), request)
			if err != nil || result.TagPublication == nil || result.TagPublication.ObservedCommit != request.Commit {
				t.Fatalf("ordinary rewrite = %+v, %v", result, err)
			}
			if got := strings.TrimSpace(mustGitOutput(t, remote, "rev-parse", "refs/tags/"+request.Name)); got != request.Commit {
				t.Fatal("ordinary rewrite published the wrong commit or destination")
			}
			if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
				t.Fatal("ordinary rewrite publication modified local metadata")
			}
		})
	}
}

func TestWorkspaceGitTagPublishRechecksLiteralRewritesImmediatelyBeforePush(t *testing.T) {
	root, remoteA, _, request := workspaceGitTagFixture(t)
	remoteB := t.TempDir()
	mustGitOutput(t, remoteB, "init", "--bare")
	mustGitOutput(t, root, "remote", "set-url", "--push", "origin", remoteA)
	state := workspaceGitTagStatus(t, root)
	request.Snapshot = state.Snapshot
	pushes := 0
	run := func(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
		if arguments[0] == "push" {
			pushes++
		}
		output, err := workspaceGitCommand(ctx, directory, arguments...)
		if arguments[0] == "ls-remote" && arguments[1] == "--refs" && err == nil {
			mustGitOutput(t, root, "config", "url."+remoteB+".pushInsteadOf", remoteA)
		}
		return output, err
	}
	if _, err := publishWorkspaceGitTagWithCommand(workspaceGitTestContext(t), state, request, run); err == nil || !strings.Contains(err.Error(), "additional URL rewrite") || pushes != 0 {
		t.Fatalf("late push rewrite = %v, pushes=%d", err, pushes)
	}
	for _, remote := range []string{remoteA, remoteB} {
		if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
			t.Fatal("late rewrite mutated a remote before rejection")
		}
	}
}

func TestWorkspaceGitTagLiteralRewriteConfigIsBoundedAndUnambiguous(t *testing.T) {
	for _, fixture := range []struct {
		name        string
		rules       string
		destination string
		matches     bool
		invalid     bool
	}{
		{name: "none", destination: "/resolved/repo"},
		{name: "push-prefix", rules: "url.other.pushinsteadof\n/resolved/\x00", destination: "/resolved/repo", matches: true},
		{name: "fetch-prefix", rules: "url.other.insteadof\n/resolved/\x00", destination: "/resolved/repo", matches: true},
		{name: "ordinary-origin-rewrite", rules: "url./resolved/.insteadof\nshorthand:\x00", destination: "/resolved/repo"},
		{name: "multiple-unrelated", rules: "url.first.insteadof\nshorthand:\x00url.second.pushinsteadof\nother:\x00", destination: "/resolved/repo"},
		{name: "empty-prefix", rules: "url.other.pushinsteadof\n\x00", destination: "/resolved/repo", matches: true},
		{name: "multiline-key", rules: "url.other\nkey.pushinsteadof\n/resolved/\x00", destination: "/resolved/repo", invalid: true},
		{name: "multiline-value", rules: "url.other.pushinsteadof\nother\n/resolved/\x00", destination: "/resolved/repo", invalid: true},
		{name: "missing-nul", rules: "url.other.pushinsteadof\n/resolved/", destination: "/resolved/repo", invalid: true},
		{name: "unexpected-key", rules: "remote.origin.url\n/resolved/\x00", destination: "/resolved/repo", invalid: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			matches, err := workspaceGitTagLiteralRewrite([]byte(fixture.rules), fixture.destination)
			if (err != nil) != fixture.invalid || matches != fixture.matches {
				t.Fatalf("matches=%t, error=%v", matches, err)
			}
		})
	}
}

func TestWorkspaceGitTagPublishRejectsLiteralURLConfiguredAsRemoteName(t *testing.T) {
	root, remoteA, _, request := workspaceGitTagFixture(t)
	remoteB := t.TempDir()
	mustGitOutput(t, remoteB, "init", "--bare")
	mustGitOutput(t, root, "remote", "set-url", "--push", "origin", remoteA)
	mustGitOutput(t, root, "config", "remote."+remoteA+".url", remoteA)
	mustGitOutput(t, root, "config", "remote."+remoteA+".pushurl", remoteB)
	request.Snapshot = workspaceGitTagStatus(t, root).Snapshot
	metadata := workspaceGitTagMetadata(t, root)
	if _, err := RunWorkspaceGit(workspaceGitTestContext(t), request); err == nil || !strings.Contains(err.Error(), "configured remote name") {
		t.Fatalf("literal URL remote alias = %v", err)
	}
	for _, remote := range []string{remoteA, remoteB} {
		if refs := mustGitOutput(t, remote, "for-each-ref"); refs != "" {
			t.Fatal("literal URL remote alias changed a remote ref")
		}
	}
	if !reflect.DeepEqual(metadata, workspaceGitTagMetadata(t, root)) {
		t.Fatal("literal URL alias rejection changed persistent configuration")
	}
}
