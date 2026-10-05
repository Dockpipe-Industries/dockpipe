//go:build !windows

package application

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestCmdCleanRejectsSpecialFileWithoutMutation(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	root := filepath.Join(workdir, infrastructure.DockpipeDirRel)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("preserve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	if err := cmdClean([]string{"--workdir", workdir, "--dry-run"}); err == nil || !strings.Contains(err.Error(), "not a regular file or directory") {
		t.Fatalf("special-file clean tree was accepted: %v", err)
	}
	if _, err := os.Lstat(marker); err != nil {
		t.Fatalf("special-file rejection mutated marker: %v", err)
	}
}
