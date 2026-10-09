package remotecmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlatpakServiceUsesHostLauncherWithoutSandboxPath(t *testing.T) {
	arguments := flatpakServiceArguments("com.example.Runtime", "worker", "/home/test/a space/%state")
	unit := systemdUnitWithPath(arguments, "")
	if !strings.Contains(unit, `ExecStart="/usr/bin/flatpak" "run" "--command=dockpipe" "com.example.Runtime" "remote" "worker"`) {
		t.Fatalf("unexpected service command: %s", unit)
	}
	if strings.Contains(unit, "Environment=") || strings.Contains(unit, "/app/") {
		t.Fatalf("sandbox-only environment leaked to host unit: %s", unit)
	}
	if !strings.Contains(unit, `"/home/test/a space/%%state"`) {
		t.Fatalf("state path was not escaped: %s", unit)
	}
	// Exercise the systemd parser when available without creating a user service.
	// verify also checks executable availability, so use an isolated fixture after
	// asserting the real host command above. CI need not have Flatpak installed.
	tool, err := exec.LookPath("systemd-analyze")
	if err != nil {
		return
	}
	directory := t.TempDir()
	launcher := filepath.Join(directory, "flatpak")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	arguments[0] = launcher
	unit = systemdUnitWithPath(arguments, "")
	path := filepath.Join(directory, "qualification.service")
	if err := os.WriteFile(path, []byte(unit), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(tool, "--user", "verify", path).CombinedOutput(); err != nil {
		t.Fatalf("invalid systemd unit: %v\n%s", err, output)
	}
}
