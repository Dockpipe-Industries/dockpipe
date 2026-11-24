package remote

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/workspacegit"
)

func TestGitFeatureRequiresExactGrantedRepositoryRoot(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git is required for the native workspace fixture")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	directory := t.TempDir()
	repository := filepath.Join(directory, "Repository")
	child := filepath.Join(repository, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}

	gitFixture := func(args ...string) {
		t.Helper()
		config := []string{"-C", repository, "-c", "init.templateDir=", "-c", "commit.gpgSign=false", "-c", "user.name=Dockpipe Test", "-c", "user.email=fixture@example.invalid"}
		if output, err := exec.CommandContext(ctx, git, append(config, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("Git fixture %s failed: %v\n%s", args[0], err, output)
		}
	}
	gitFixture("init", "-q")
	gitFixture("commit", "--allow-empty", "-qm", "Fixture baseline")
	worktree := filepath.Join(directory, "Linked Worktree")
	gitFixture("worktree", "add", "-qb", "linked-fixture", worktree)

	// The local Git contract still discovers a repository from its subdirectory.
	if snapshot, err := workspacegit.RunWorkspaceGit(ctx, workspacegit.WorkspaceGitRequest{Action: "status", Workdir: child}); err != nil || snapshot == nil || snapshot.Head == "" {
		t.Fatalf("local Git did not discover the fixture repository: %v", err)
	}

	target := contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)}
	request, err := NewFeatureRequest(target, "fixture", "git.status", workspacegit.WorkspaceGitRequest{Action: "status"}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, identity := nativeinvocation.Local()
	run := func(path string) contract.FeatureResult {
		permissions := FeaturePermissions{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{{
			FeatureRoot: contract.FeatureRoot{ID: "fixture", Name: "Fixture", GitRead: true},
			Path:        path,
		}}}
		return executeFeature(ctx, permissions, request, identity, func(context.Context, invocation.Binding) error {
			return nil
		})
	}

	t.Run("subdirectory", func(t *testing.T) {
		result := run(child)
		if result.Status != "failure" || len(result.Payload) != 0 || result.Error != "Native feature operation rejected or failed; refresh the selected resource." {
			t.Fatalf("subdirectory grant should fail without disclosing repository data, status %q", result.Status)
		}
	})
	t.Run("repository", func(t *testing.T) {
		if result := run(repository); result.Status != "success" || len(result.Payload) == 0 {
			t.Fatalf("exact repository grant failed: %s", result.Error)
		}
	})
	t.Run("linked worktree", func(t *testing.T) {
		if result := run(worktree); result.Status != "success" || len(result.Payload) == 0 {
			t.Fatalf("exact linked worktree grant failed: %s", result.Error)
		}
	})
	if runtime.GOOS == "windows" {
		t.Run("Windows path spelling", func(t *testing.T) {
			alias := filepath.Join(directory, "rEPOSITORY")
			if _, err := os.Stat(alias); os.IsNotExist(err) {
				t.Skip("the fixture is on a case-sensitive Windows filesystem")
			} else if err != nil {
				t.Fatal(err)
			}
			if result := run(filepath.ToSlash(alias)); result.Status != "success" || len(result.Payload) == 0 {
				t.Fatalf("equivalent Windows grant path rejected: %s", result.Error)
			}
		})
	}
}
