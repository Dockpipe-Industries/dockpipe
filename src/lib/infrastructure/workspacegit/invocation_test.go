package workspacegit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

func TestWorkspaceGitInvocationRejectsBeforeRepositoryAccess(t *testing.T) {
	target, identity := nativeinvocation.Local()
	request := WorkspaceGitRequest{Workdir: filepath.Join(t.TempDir(), "missing"), Action: "status"}
	denied := errors.New("denied by trusted adapter")
	called := false
	authorize := func(_ context.Context, binding invocation.Binding) error {
		called = true
		if binding.Contract != (invocation.ID{Capability: "dockpipe.git", Action: "status", Version: "1"}) || len(binding.Digest) != 64 {
			t.Fatalf("incorrect binding: %+v", binding)
		}
		return denied
	}
	if _, err := RunWorkspaceGitWithAuthority(context.Background(), request, target, identity, authorize); !errors.Is(err, denied) || !called {
		t.Fatalf("denied admission = %v; authorized = %v", err, called)
	}
	called = false
	request.Action = "unrecognized"
	if _, err := RunWorkspaceGitWithAuthority(context.Background(), request, target, identity, authorize); err == nil || !strings.Contains(err.Error(), "unsupported project Git action") || called {
		t.Fatalf("unknown action = %v; authorized = %v", err, called)
	}
	if _, err := RunWorkspaceGit(context.Background(), request); err == nil || !strings.Contains(err.Error(), "unsupported project Git action") {
		t.Fatalf("local unknown action = %v", err)
	}
}

func TestWorkspaceGitRemoteInvocationRequiresAuthority(t *testing.T) {
	_, identity := nativeinvocation.Local()
	target := invocation.Target{Kind: invocation.Remote, ID: "worker", Fingerprint: strings.Repeat("a", 64), Generation: "one"}
	request := WorkspaceGitRequest{Workdir: filepath.Join(t.TempDir(), "missing"), Action: "status"}
	if _, err := RunWorkspaceGitWithAuthority(context.Background(), request, target, identity, nil); !errors.Is(err, invocation.ErrAuthorizationUnavailable) {
		t.Fatalf("remote admission without authority = %v", err)
	}
	denied := errors.New("remote denied")
	if _, err := RunWorkspaceGitWithAuthority(context.Background(), request, target, identity, func(_ context.Context, binding invocation.Binding) error {
		if binding.Target != target || binding.Identity != identity || len(binding.Digest) != 64 {
			t.Fatalf("remote binding = %+v", binding)
		}
		return denied
	}); !errors.Is(err, denied) {
		t.Fatalf("remote denial = %v", err)
	}
}

func TestWorkspaceGitInvocationOwnsSelectedPaths(t *testing.T) {
	root := initGitSessionTestRepo(t)
	ctx := workspaceGitTestContext(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("selected change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "status"})
	request := WorkspaceGitRequest{Workdir: root, Action: "stage", Snapshot: before.Snapshot, Paths: []string{"README.md"}}
	target, identity := nativeinvocation.Local()
	_, err := RunWorkspaceGitWithAuthority(ctx, request, target, identity, func(_ context.Context, binding invocation.Binding) error {
		if binding.Revision != before.Snapshot || binding.Digest != workspaceGitInputDigest(request) {
			t.Fatal("authorization did not bind the reviewed request")
		}
		request.Paths[0] = "unselected"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if staged := strings.TrimSpace(mustGitOutput(t, root, "diff", "--cached", "--name-only")); staged != "README.md" {
		t.Fatalf("staged paths = %q", staged)
	}
}

func TestWorkspaceGitInvocationOwnsSelectedCommits(t *testing.T) {
	root := initGitSessionTestRepo(t)
	before := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "status"})
	mustGitOutput(t, root, "switch", "-c", "source")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("picked content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGitOutput(t, root, "commit", "-am", "selected commit")
	selected := strings.TrimSpace(mustGitOutput(t, root, "rev-parse", "HEAD"))
	mustGitOutput(t, root, "switch", before.Branch)
	current := gitWorkspaceAction(t, root, WorkspaceGitRequest{Action: "status"})
	request := WorkspaceGitRequest{Workdir: root, Action: "cherry-pick", Snapshot: current.Snapshot, Commits: []string{selected}, Confirm: true}
	target, identity := nativeinvocation.Local()
	_, err := RunWorkspaceGitWithAuthority(workspaceGitTestContext(t), request, target, identity, func(_ context.Context, binding invocation.Binding) error {
		if binding.Digest != workspaceGitInputDigest(request) {
			t.Fatal("authorization did not bind the selected commit")
		}
		request.Commits[0] = "invalid-commit"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if content := mustGitOutput(t, root, "show", "HEAD:README.md"); content != "picked content\n" {
		t.Fatalf("picked content = %q", content)
	}
}

func TestWorkspaceGitInputDigestBindsEveryRequestField(t *testing.T) {
	initial := WorkspaceGitRequest{Action: "status", Workdir: "/project"}
	want := workspaceGitInputDigest(initial)
	if again := workspaceGitInputDigest(initial); again != want {
		t.Fatal("digest is not stable")
	}
	requestType := reflect.TypeOf(initial)
	for index := 0; index < requestType.NumField(); index++ {
		t.Run(requestType.Field(index).Name, func(t *testing.T) {
			changed := initial
			field := reflect.ValueOf(&changed).Elem().Field(index)
			switch field.Kind() {
			case reflect.String:
				field.SetString(field.String() + "changed\xff")
			case reflect.Int:
				field.SetInt(1)
			case reflect.Bool:
				field.SetBool(true)
			case reflect.Slice:
				field.Set(reflect.ValueOf([]string{"selected"}))
			default:
				t.Fatalf("add digest coverage for %s", field.Kind())
			}
			if workspaceGitInputDigest(changed) == want {
				t.Fatal("request field is absent from the digest")
			}
		})
	}
}
