package application

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTerminalStreamOwnsAdmissionWithoutProjectConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte("deliberately malformed project config"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := checkCLIProjectVersion([]string{"terminal", "--stdio"}); err != nil {
		t.Fatalf("native terminal incorrectly loaded project config: %v", err)
	}
	if err := checkCLIProjectVersion([]string{"--workflow", "example"}); err == nil {
		t.Fatal("workflow lost its project-config admission")
	}
}

func TestTerminalCannotBeRetargetedByAmbientWindowsBridge(t *testing.T) {
	previous := windowsGoosFn
	windowsGoosFn = func() string { return "windows" }
	t.Cleanup(func() { windowsGoosFn = previous })
	t.Setenv(EnvUseWSLBridge, "1")
	previousGetwd := windowsGetwdFn
	windowsGetwdFn = func() (string, error) {
		t.Fatal("terminal attempted WSL route resolution")
		return "", nil
	}
	t.Cleanup(func() { windowsGetwdFn = previousGetwd })
	if handled, code := TryWindowsWSLBridge([]string{"terminal", "--stdio"}, nil, io.Discard, io.Discard); handled || code != 0 {
		t.Fatal("native terminal was forwarded into WSL")
	}
}
