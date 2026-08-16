// Package packagescript owns portable command construction for package and compile scripts.
package packagescript

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// UpsertEnv replaces duplicate entries for key with one value or appends it when absent.
func UpsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

// ScriptCommand constructs the host command used to execute a package script.
func ScriptCommand(scriptAbs string) (*exec.Cmd, string, error) {
	lower := strings.ToLower(scriptAbs)
	switch {
	case strings.HasSuffix(lower, ".ps1"):
		return exec.Command("pwsh", "-File", scriptAbs), "", nil
	case strings.HasSuffix(lower, ".cmd"), strings.HasSuffix(lower, ".bat"):
		if runtime.GOOS != "windows" {
			return nil, "", fmt.Errorf("script %q requires cmd.exe on Windows", scriptAbs)
		}
		return exec.Command("cmd", "/c", scriptAbs), "", nil
	default:
		bashExe, bashArg, err := bashCommandParts(scriptAbs)
		if err != nil {
			return nil, "", err
		}
		return exec.Command(bashExe, bashArg), bashExe, nil
	}
}

// BashShellCommand constructs a Bash command for an authored shell fragment.
func BashShellCommand(command string) (*exec.Cmd, string, error) {
	if runtime.GOOS == "windows" {
		if bashExe := gitBashWindowsPath(); bashExe != "" {
			return exec.Command(bashExe, "-lc", command), bashExe, nil
		}
	}
	bashExe, err := exec.LookPath("bash")
	if err != nil {
		return nil, "", fmt.Errorf("bash not found for shell command %q", command)
	}
	return exec.Command(bashExe, "-lc", command), bashExe, nil
}

// PathForBashEnv translates a host path for the selected Bash implementation on Windows.
func PathForBashEnv(bashExe, path string) string {
	if strings.TrimSpace(path) == "" {
		return path
	}
	if runtime.GOOS != "windows" {
		return path
	}
	if bashIsWSLPath(bashExe) {
		return pathForWSLBash(path)
	}
	return pathForGitBash(path)
}

func bashCommandParts(scriptAbs string) (string, string, error) {
	if runtime.GOOS == "windows" {
		if bashExe := gitBashWindowsPath(); bashExe != "" {
			return bashExe, pathForGitBash(scriptAbs), nil
		}
	}
	bashExe, err := exec.LookPath("bash")
	if err != nil {
		return "", "", fmt.Errorf("bash not found for script %q", scriptAbs)
	}
	return bashExe, scriptAbs, nil
}

func gitBashWindowsPath() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Programs", "Git", "bin", "bash.exe"),
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	seen := map[string]bool{}
	for _, path := range candidates {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			return path
		}
	}
	return ""
}

func pathForGitBash(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	volume := filepath.VolumeName(abs)
	if len(volume) >= 2 && volume[1] == ':' {
		drive := strings.ToLower(string(volume[0]))
		rest := abs[len(volume):]
		for len(rest) > 0 && (rest[0] == '\\' || rest[0] == '/') {
			rest = rest[1:]
		}
		rest = filepath.ToSlash(rest)
		return "/" + drive + "/" + rest
	}
	return filepath.ToSlash(abs)
}

func bashIsWSLPath(bashExe string) bool {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(bashExe), `\`, "/"))
	return strings.Contains(value, "/system32/bash") ||
		strings.Contains(value, "windowsapps") ||
		strings.Contains(value, "/wsl/")
}

func pathForWSLBash(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	volume := filepath.VolumeName(abs)
	if len(volume) >= 2 && volume[1] == ':' {
		drive := strings.ToLower(string(volume[0]))
		rest := abs[len(volume):]
		for len(rest) > 0 && (rest[0] == '\\' || rest[0] == '/') {
			rest = rest[1:]
		}
		rest = filepath.ToSlash(rest)
		return "/mnt/" + drive + "/" + rest
	}
	return filepath.ToSlash(abs)
}
