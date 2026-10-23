//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// installGuestTools is the removable-media entrypoint. Windows owns elevation;
// the installer and the durable boot task share the same provisioning helper.
func installGuestTools() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	helper := filepath.Join(filepath.Dir(executable), "provision-windows-ssh.ps1")
	info, err := os.Lstat(helper)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("provisioning helper must be a regular file")
	}
	userName := os.Getenv("USERNAME")
	if strings.TrimSpace(userName) == "" {
		return fmt.Errorf("the interactive Windows username is unavailable")
	}
	quote := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
	installation := "& " + quote(helper) + " -UserName " + quote(userName) +
		" -CreateUser $false -EnablePasswordAuth $false; if (-not $?) { exit 1 }"
	encodedInstallation := encodePowerShell(installation)
	launcher := "$ErrorActionPreference = 'Stop'; $process = Start-Process -FilePath " +
		"($env:WINDIR + '\\System32\\WindowsPowerShell\\v1.0\\powershell.exe') " +
		"-Verb RunAs -Wait -PassThru -ArgumentList " +
		quote("-NoProfile -NoExit -ExecutionPolicy Bypass -EncodedCommand "+encodedInstallation) +
		"; exit $process.ExitCode"
	powershell := filepath.Join(os.Getenv("WINDIR"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	command := exec.Command(powershell, "-NoProfile", "-EncodedCommand", encodePowerShell(launcher))
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
