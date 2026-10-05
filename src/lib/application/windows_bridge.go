package application

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/application/internal/wslbridge"
)

// EnvUseWSLBridge, when set to "1", makes dockpipe.exe forward all commands
// (except "windows …") into WSL. The default is native Windows execution so Git,
// Docker Desktop, and paths stay on one side.
const EnvUseWSLBridge = "DOCKPIPE_USE_WSL_BRIDGE"

var windowsGetwdFn = os.Getwd

// UseWSLBridge reports whether the Windows host binary should forward into WSL.
func UseWSLBridge() bool {
	return os.Getenv(EnvUseWSLBridge) == "1"
}

// TryWindowsWSLBridge runs dockpipe inside WSL when invoked from Windows.
// It returns handled=false for subcommands that must stay on the host (e.g. "windows").
// The current Windows working directory is mapped with wslpath and used as cwd in WSL.
func TryWindowsWSLBridge(argv []string, stdin io.Reader, stdout, stderr io.Writer) (handled bool, exitCode int) {
	if windowsGoosFn() != "windows" {
		return false, 0
	}
	if !UseWSLBridge() {
		return false, 0
	}
	if len(argv) > 0 && argv[0] == "windows" {
		return false, 0
	}
	distro, err := resolveBridgeDistro()
	if err != nil {
		fmt.Fprintf(stderr, "[dockpipe] %v\n", err)
		return true, 1
	}
	winWd, err := windowsGetwdFn()
	if err != nil {
		fmt.Fprintf(stderr, "[dockpipe] get working directory: %v\n", err)
		return true, 1
	}
	winWd, err = filepath.Abs(winWd)
	if err != nil {
		fmt.Fprintf(stderr, "[dockpipe] abs working directory: %v\n", err)
		return true, 1
	}
	wslWd := winPathToWSL(distro, winWd)
	fmt.Fprintf(stderr, "[dockpipe] Windows bridge: distro=%q cwd=%s -> %s\n", distro, winWd, wslWd)

	translator := wslbridge.New(func(path string) string {
		return winPathToWSL(distro, path)
	}, windowsGetwdFn)
	script := translator.ForwardScript(wslWd, argv)
	cmd := windowsExecCommandFn("wsl.exe", "-d", distro, "--", "bash", "-lc", script)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "DOCKPIPE_WINDOWS_BRIDGE=1")
	err = cmd.Run()
	if err == nil {
		return true, 0
	}
	if x, ok := err.(*exec.ExitError); ok {
		return true, x.ExitCode()
	}
	fmt.Fprintf(stderr, "[dockpipe] wsl: %v\n", err)
	return true, 1
}

func resolveBridgeDistro() (string, error) {
	cfg, err := loadWindowsConfig()
	if err != nil {
		return "", err
	}
	if cfg != "" {
		return cfg, nil
	}
	distros, err := listWSLDistros()
	if err != nil {
		return "", fmt.Errorf("WSL: %w (run `dockpipe windows setup` to pick a distro)", err)
	}
	if len(distros) == 0 {
		return "", fmt.Errorf("no WSL distros found; install one with `wsl --install -d Alpine` (or Ubuntu) then `dockpipe windows setup`")
	}
	d := distros[0]
	fmt.Fprintf(windowsStderr, "[dockpipe] No %%APPDATA%%\\dockpipe\\windows-config.env; using first distro %q (run `dockpipe windows setup` to pin)\n", d)
	return d, nil
}

func winPathToWSL(distro, winPath string) string {
	cmd := windowsExecCommandFn("wsl.exe", "-d", distro, "wslpath", "-u", winPath)
	out, err := cmd.Output()
	if err == nil {
		return strings.ReplaceAll(strings.TrimSpace(string(out)), `\`, `/`)
	}
	return wslbridge.PathToWSLFallback(winPath)
}
