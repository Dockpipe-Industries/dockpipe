package workspacegit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceGitIgnoresMalformedPrivateRefsAndTracksActionRefs(t *testing.T) {
	root := initGitSessionTestRepo(t)
	privateRef := filepath.Join(root, ".git", "refs", "private-tool", "invalid")
	if err := os.MkdirAll(filepath.Dir(privateRef), 0o755); err != nil {
		t.Fatal(err)
	}
	invalid := strings.Repeat("0", 40) + "\n"
	if err := os.WriteFile(privateRef, []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := workspaceGitTestContext(t)
	before, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatalf("unrelated private ref disabled status: %v", err)
	}
	for _, reference := range []string{"refs/heads/review", "refs/remotes/origin/review", "refs/tags/review"} {
		mustGitOutput(t, root, "update-ref", reference, "HEAD")
		after, err := RunWorkspaceGit(ctx, WorkspaceGitRequest{Workdir: root, Action: "status"})
		if err != nil {
			t.Fatal(err)
		}
		if before.Snapshot == after.Snapshot {
			t.Fatalf("snapshot did not include %s", reference)
		}
		before = after
	}
	content, err := os.ReadFile(privateRef)
	if err != nil || string(content) != invalid {
		t.Fatal("status changed the private ref instead of leaving it alone")
	}
}
