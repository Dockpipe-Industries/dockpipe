package workspacegitcmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure"
)

func TestWorkspaceGitTagPublishPublicRequestRequiresConfirmation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	for _, arguments := range [][]string{{"init"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.invalid"}} {
		output, err := exec.Command("git", append([]string{"-C", root}, arguments...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Git: %v\n%s", err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "README.md"}, {"commit", "-m", "fixture"}} {
		output, err := exec.Command("git", append([]string{"-C", root}, arguments...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Git: %v\n%s", err, output)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, err := infrastructure.RunWorkspaceGit(ctx, infrastructure.WorkspaceGitRequest{Workdir: root, Action: "status"})
	if err != nil {
		t.Fatal(err)
	}
	request := map[string]any{"workdir": root, "action": "tag-publish", "snapshot": state.Snapshot,
		"name": "reviewed-example", "commit": state.Head, "confirm": false}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"--request", path}); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("public tag-publish routing = %v", err)
	}
}
