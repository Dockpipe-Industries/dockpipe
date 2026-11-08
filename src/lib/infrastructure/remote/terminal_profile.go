package remote

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

// TerminalProfile is configured only by the worker owner. Terminal permission
// grants an interactive shell with the worker OS account's full authority; its
// starting directory is not a file sandbox. Environment is inherited/frozen at
// worker start, never accepted from the network or persisted in this profile.
type TerminalProfile struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

func (profile TerminalProfile) Validate() error {
	if !filepath.IsAbs(profile.Executable) || strings.ContainsRune(profile.Executable, 0) {
		return errors.New("terminal profile requires an absolute owner-selected shell")
	}
	for _, argument := range profile.Args {
		if strings.ContainsRune(argument, 0) {
			return errors.New("invalid terminal profile argument")
		}
	}
	return nil
}

func terminalSupported() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}
