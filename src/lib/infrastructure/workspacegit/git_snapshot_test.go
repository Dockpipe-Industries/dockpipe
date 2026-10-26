package workspacegit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWorkspaceGitSafePathNormalizesRootSeparators(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	invalidPaths := []string{"../outside", "nested/../../outside", "/outside"}
	if runtime.GOOS == "windows" {
		invalidPaths = append(invalidPaths, `\outside`, `C:outside`, `C:/outside`, `\\server\share\outside`, "NUL")
	}
	for _, reportedRoot := range []string{root, filepath.ToSlash(root), root + string(filepath.Separator) + "."} {
		for _, path := range []string{"file.txt", "nested/file.txt"} {
			absolute, err := workspaceGitSafePath(reportedRoot, path)
			want := filepath.Join(root, filepath.FromSlash(path))
			if err != nil || absolute != want {
				t.Fatalf("root %q path %q: got %q, %v; want %q", reportedRoot, path, absolute, err, want)
			}
		}
		for _, path := range invalidPaths {
			if absolute, err := workspaceGitSafePath(reportedRoot, path); err == nil {
				t.Fatalf("accepted escaped path %q as %q", path, absolute)
			}
		}
	}
}

func TestWorkspaceGitSafePathRejectsLinkedParent(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "linked")); err != nil {
		t.Skipf("host cannot create directory symlinks: %v", err)
	}
	_, err := workspaceGitSafePath(filepath.ToSlash(root), "linked/file.txt")
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("linked parent was not rejected: %v", err)
	}
}
