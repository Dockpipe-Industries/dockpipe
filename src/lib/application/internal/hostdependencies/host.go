package hostdependencies

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"dockpipe/src/lib/domain/dependency"
)

func RunInstallShellCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("empty install command")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// LookupWindowsFeature uses Windows' read-only optional feature inventory.
func LookupWindowsFeature(name string) (bool, error) {
	state, err := readWindowsRequirement(name, "Win32_OptionalFeature", "InstallState")
	if err != nil {
		return false, err
	}

	switch state {
	case "1":
		return true, nil
	case "2", "3":
		return false, nil
	default:
		return false, fmt.Errorf("unknown or unavailable Windows optional feature %q", name)
	}
}

// LookupWindowsService checks registration. A demand-start service may be stopped.
func LookupWindowsService(name string) (bool, error) {
	registered, err := readWindowsRequirement(name, "Win32_Service", "Name")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(registered, name), nil
}

func readWindowsRequirement(name, class, field string) (string, error) {
	if !dependency.ValidWindowsRequirementName(name) {
		return "", fmt.Errorf("invalid Windows requirement name")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'; (Get-CimInstance -ClassName %s -Filter "Name = '%s'").%s`,
		class, name, field,
	)
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", fmt.Errorf("could not inspect Windows requirement %q: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func LookupViaPowerShell(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("empty command")
	}
	ps := fmt.Sprintf(
		"$cmd = Get-Command -Name '%s' -ErrorAction SilentlyContinue; if ($cmd) { $cmd.Source }",
		strings.ReplaceAll(command, "'", "''"),
	)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", os.ErrNotExist
	}
	return path, nil
}
