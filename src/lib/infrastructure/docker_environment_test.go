package infrastructure

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDockerProviderDiscoveryUsesCapturedEnvironment(t *testing.T) {
	directory := t.TempDir()
	name := "docker"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	selected := filepath.Join(directory, name)
	if err := os.WriteFile(selected, nil, 0755); err != nil {
		t.Fatal(err)
	}
	captured := []string{"PATH=" + directory, "PATHEXT=.exe"}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "other-docker"))
	actual, err := resolveDockerCommandFromEnvironment(captured)
	if err != nil || actual != selected {
		t.Fatalf("ambient environment redirected discovery: %q, %v", actual, err)
	}
	if _, err := resolveDockerCommandFromEnvironment([]string{"PATH=."}); runtime.GOOS != "windows" && !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("implicit current-directory Docker lookup accepted: %v", err)
	}
}
