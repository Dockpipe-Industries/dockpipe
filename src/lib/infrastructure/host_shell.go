package infrastructure

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/hostpython"
)

var hostShellLookPathFn = exec.LookPath

func hostShellForScript(selection, scriptPath, operatingSystem string) (domain.HostShell, error) {
	shell := domain.NormalizeHostShell(selection)
	if err := domain.ValidateHostScriptShell(shell, scriptPath); err != nil {
		return "", err
	}
	if shell != "" {
		if shell == domain.HostShellPowerShell && operatingSystem != "windows" {
			return "", fmt.Errorf("shell powershell requires Windows PowerShell; use shell: pwsh on %s", operatingSystem)
		}
		return shell, nil
	}
	switch strings.ToLower(filepath.Ext(scriptPath)) {
	case ".sh":
		return domain.HostShellBash, nil
	case ".ps1":
		if operatingSystem != "windows" {
			return domain.HostShellPwsh, nil
		}
	}
	if operatingSystem == "windows" {
		return domain.HostShellPowerShell, nil
	}
	return domain.HostShellBash, nil
}

func hostScriptCommand(scriptPath string, env []string, selection string) (*exec.Cmd, []string, error) {
	if domain.NormalizeHostShell(selection) == "" && strings.EqualFold(filepath.Ext(scriptPath), ".py") {
		absolutePath, err := filepath.Abs(scriptPath)
		if err != nil {
			return nil, nil, fmt.Errorf("host script path: %w", err)
		}
		command, err := hostpython.Command(absolutePath, env)
		return command, prepareHostNativeEnv(env), err
	}
	shell, err := hostShellForScript(selection, scriptPath, runtime.GOOS)
	if err != nil {
		return nil, nil, err
	}
	if shell == domain.HostShellBash {
		bashExecutable, err := resolveBashExe()
		if err != nil {
			return nil, nil, fmt.Errorf("bash not found for host script; install Bash or select an available native shell: %w", err)
		}
		bashPath, err := pathMapperFor(bashExecutable)(scriptPath)
		if err != nil {
			return nil, nil, fmt.Errorf("host script path: %w", err)
		}
		return exec.Command(bashExecutable, bashPath), prepareHostBashEnv(env, bashExecutable), nil
	}

	executableName := "pwsh"
	if shell == domain.HostShellPowerShell {
		executableName = "powershell.exe"
	}
	executable, err := hostShellLookPathFn(executableName)
	if err != nil {
		return nil, nil, fmt.Errorf("requested host shell %q was not found: %w", shell, err)
	}
	absolutePath, err := filepath.Abs(scriptPath)
	if err != nil {
		return nil, nil, fmt.Errorf("host script path: %w", err)
	}
	// An encoded wrapper avoids a second command-line quoting interpretation on
	// Windows, keeps script paths literal, and propagates native program failures.
	body := "$ErrorActionPreference = 'Stop'\n" +
		"$ProgressPreference = 'SilentlyContinue'\n" +
		"$global:LASTEXITCODE = 0\n" +
		"& '" + strings.ReplaceAll(absolutePath, "'", "''") + "'\n" +
		"$commandSucceeded = $?\n" +
		"if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\n" +
		"if (-not $commandSucceeded) { exit 1 }\n"
	command := exec.Command(executable, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-OutputFormat", "Text", "-EncodedCommand", encodeHostPowerShell(body))
	return command, prepareHostNativeEnv(env), nil
}

func encodeHostPowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, len(units)*2)
	for index, unit := range units {
		binary.LittleEndian.PutUint16(encoded[index*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

// Bash environment extraction may return an MSYS/WSL PATH. Native Windows
// children need native paths separated by semicolons, without a Bash launch.
func prepareHostNativeEnv(env []string) []string {
	if runtime.GOOS != "windows" {
		return env
	}
	currentPath := envGet(env, "PATH")
	if currentPath == "" || strings.Contains(currentPath, ";") || isSingleWindowsPath(currentPath) {
		return env
	}
	parts := strings.Split(currentPath, ":")
	for index, part := range parts {
		parts[index] = rewriteMsysOrMntToWindows(part)
	}
	return upsertEnv(env, "PATH", strings.Join(parts, ";"))
}

func isSingleWindowsPath(value string) bool {
	return len(value) >= 3 && value[1] == ':' &&
		(value[2] == '/' || value[2] == '\\') && !strings.Contains(value[2:], ":")
}
