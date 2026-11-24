package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveDockerCommandFromEnvironment performs filesystem discovery only in an
// admitted executor. Provider selection uses the captured native environment;
// an authorizer cannot redirect PATH, the configured executable or Windows
// installation roots by changing the process environment after preparation.
func resolveDockerCommandFromEnvironment(environment []string) (string, error) {
	if configured := strings.TrimSpace(envGet(environment, "DOCKPIPE_DOCKER_BIN")); configured != "" {
		absolute, err := filepath.Abs(configured)
		if err != nil {
			return "", err
		}
		selected, err := exec.LookPath(absolute)
		if err != nil {
			return "", err
		}
		if selected != absolute {
			return "", &exec.Error{Name: absolute, Err: exec.ErrNotFound}
		}
		return selected, nil
	}

	extensions := []string{""}
	if runtime.GOOS == "windows" {
		extensions = []string{".com", ".exe", ".bat", ".cmd"}
		if configured := envGet(environment, "PATHEXT"); configured != "" {
			extensions = nil
			for _, extension := range strings.Split(strings.ToLower(configured), ";") {
				if extension != "" && !strings.ContainsAny(extension, `/\:`) {
					if !strings.HasPrefix(extension, ".") {
						extension = "." + extension
					}
					extensions = append(extensions, extension)
				}
			}
		}
	}
	for _, directory := range filepath.SplitList(envGet(environment, "PATH")) {
		// Native Go command lookup rejects implicit current-directory providers.
		// Keep that boundary instead of resolving a relative PATH after admission.
		if !filepath.IsAbs(directory) {
			continue
		}
		for _, extension := range extensions {
			candidate := filepath.Join(directory, "docker"+extension)
			info, err := os.Stat(candidate)
			if err != nil || info.IsDir() {
				continue
			}
			if selected, err := exec.LookPath(candidate); err == nil && selected == candidate {
				return selected, nil
			}
		}
	}
	switch runtime.GOOS {
	case "darwin":
		// LaunchAgents need the same standard installations as interactive
		// shells even when the shell profile never ran.
		for _, candidate := range []string{
			"/Applications/Docker.app/Contents/Resources/bin/docker",
			"/opt/homebrew/bin/docker",
			"/usr/local/bin/docker",
		} {
			if selected, err := exec.LookPath(candidate); err == nil && selected == candidate {
				return selected, nil
			}
		}
	case "windows":
		localAppData := envGet(environment, "LOCALAPPDATA")
		if filepath.IsAbs(localAppData) {
			candidate := filepath.Join(localAppData, "Programs", "DockerDesktop", "resources", "bin", "docker.exe")
			if selected, err := exec.LookPath(candidate); err == nil && selected == candidate {
				return selected, nil
			}
		}

		for _, root := range []string{envGet(environment, "ProgramFiles"), envGet(environment, "ProgramW6432"), `C:\Program Files`} {
			if !filepath.IsAbs(root) {
				continue
			}
			candidate := filepath.Join(root, "Docker", "Docker", "resources", "bin", "docker.exe")
			if selected, err := exec.LookPath(candidate); err == nil && selected == candidate {
				return selected, nil
			}
		}
	}
	return "", &exec.Error{Name: "docker", Err: exec.ErrNotFound}
}
