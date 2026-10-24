// Package hostpython constructs Python commands shared by host scripts and package hooks.
package hostpython

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Command selects Python without involving a shell. An explicitly present override
// must be an absolute executable; an invalid override never falls back to PATH.
func Command(script string, environment []string) (*exec.Cmd, error) {
	return commandForPlatform(script, environment, runtime.GOOS, exec.LookPath)
}

func commandForPlatform(script string, environment []string, operatingSystem string, lookPath func(string) (string, error)) (*exec.Cmd, error) {
	selected := ""
	present := false
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		matches := key == "DOCKPIPE_HOST_PYTHON_BIN"
		if operatingSystem == "windows" {
			matches = strings.EqualFold(key, "DOCKPIPE_HOST_PYTHON_BIN")
		}
		if found && matches {
			selected = value
			present = true
		}
	}
	if present {
		if !filepath.IsAbs(selected) {
			return nil, fmt.Errorf("DOCKPIPE_HOST_PYTHON_BIN must name an absolute Python 3 executable")
		}
		executable, err := lookPath(selected)
		if err != nil {
			return nil, fmt.Errorf("DOCKPIPE_HOST_PYTHON_BIN is not executable: %w", err)
		}
		return exec.Command(executable, script), nil
	}
	candidates := []string{"python3"}
	if operatingSystem == "windows" {
		candidates = append(candidates, "python")
	}
	for _, candidate := range candidates {
		if executable, err := lookPath(candidate); err == nil {
			return exec.Command(executable, script), nil
		}
	}
	return nil, fmt.Errorf("a Python 3 interpreter is required for a Python host script; install it or set DOCKPIPE_HOST_PYTHON_BIN")
}
