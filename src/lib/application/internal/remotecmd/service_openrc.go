package remotecmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
)

var openRCVersion = regexp.MustCompile(`(?m)^rc-service \(OpenRC(?: \[[^\]\r\n]+\])?\) ([0-9]+)\.([0-9]+)\b`)

func openRCWorkerAvailable(ctx context.Context, flatpak bool, base string) bool {
	if runtime.GOOS != "linux" || flatpak || ctx.Err() != nil {
		return false
	}
	// Some versions of OpenRC source configuration paths using shell single
	// quotes. A custom base containing a quote cannot safely be used by them.
	if !filepath.IsAbs(base) || strings.ContainsRune(base, '\'') || !openRCDefaultRunlevelExists(base) {
		return false
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if !filepath.IsAbs(runtimeDir) || infrastructure.ValidatePrivatePath(runtimeDir, true) != nil {
		return false
	}
	// The login manager or owner must initialize the user session. Probe the
	// runlevel file without invoking OpenRC, which can update runtime state.
	currentRunlevel := filepath.Join(runtimeDir, "openrc", "softlevel")
	if info, err := os.Lstat(currentRunlevel); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return false
	}
	service, err := findOpenRCTool("rc-service")
	if err != nil {
		return false
	}
	interpreter, err := findOpenRCTool("openrc-run")
	if err != nil || strings.ContainsAny(interpreter, " \t\r\n") {
		return false
	}
	for _, tool := range []string{"rc-update", "supervise-daemon"} {
		if _, err := findOpenRCTool(tool); err != nil {
			return false
		}
	}

	// Version discovery does not initialize or start the user's OpenRC session.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	probe := exec.CommandContext(ctx, service, "--version")
	probe.WaitDelay = 250 * time.Millisecond
	version, err := probe.Output()
	if err != nil {
		return false
	}
	match := openRCVersion.FindStringSubmatch(string(version))
	if match == nil {
		return false
	}
	major, majorErr := strconv.Atoi(match[1])
	minor, minorErr := strconv.Atoi(match[2])
	return majorErr == nil && minorErr == nil && (major > 0 || minor >= 62)
}

func openRCDefaultRunlevelExists(base string) bool {
	info, err := os.Stat(filepath.Join(base, "rc", "runlevels", "default"))
	return err == nil && info.IsDir()
}

func findOpenRCTool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil && filepath.IsAbs(path) {
		return path, nil
	}
	// Unprivileged Alpine and Gentoo accounts need not include sbin in PATH.
	for _, directory := range []string{"/usr/sbin", "/sbin", "/usr/bin", "/bin"} {
		if path, err := exec.LookPath(filepath.Join(directory, name)); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("OpenRC tool %s is unavailable", name)
}

func openRCWorkerDefinition(base, name string, arguments []string, fingerprint string) (path, content string) {
	interpreter, _ := findOpenRCTool("openrc-run")
	supervisor, _ := findOpenRCTool("supervise-daemon")
	worker := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		worker = append(worker, quoteOpenRCWord(argument))
	}
	log := quoteOpenRCWord(filepath.Join(arguments[len(arguments)-1], "worker.log"))
	workerPath := quoteOpenRCWord("PATH=" + os.Getenv("PATH"))

	// Pass arguments directly. The default OpenRC start/stop helpers use shell
	// evaluation or splitting on configured paths, including paths with spaces.
	return filepath.Join(base, "rc", "init.d", name), fmt.Sprintf(`#!%s
# Dockpipe executable sha256: %s
description='Dockpipe remote worker'
supervisor=supervise-daemon

start() {
    ebegin "Starting ${RC_SVCNAME}"
    %s "$RC_SVCNAME" --start --respawn-delay 3 --respawn-max 5 --respawn-period 60 \
        --umask 077 --env %s --stdout %s --stderr %s -- %s
    eend $?
}

stop() {
    ebegin "Stopping ${RC_SVCNAME}"
    %s "$RC_SVCNAME" --stop
    eend $?
}
`, interpreter, fingerprint, quoteOpenRCWord(supervisor), workerPath, log, log, strings.Join(worker, " "), quoteOpenRCWord(supervisor))
}

func quoteOpenRCWord(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func activateOpenRCWorker(ctx context.Context, definition serviceDefinition, replace bool) error {
	service, err := findOpenRCTool("rc-service")
	if err != nil {
		return err
	}
	update, err := findOpenRCTool("rc-update")
	if err != nil {
		return err
	}
	rcRoot := filepath.Dir(filepath.Dir(definition.path))
	base := filepath.Dir(rcRoot)
	if !openRCDefaultRunlevelExists(base) {
		return fmt.Errorf("OpenRC user default runlevel is unavailable: %s", filepath.Join(rcRoot, "runlevels", "default"))
	}
	environment := append(os.Environ(), "XDG_CONFIG_HOME="+base)
	command := func(tool string, arguments ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, tool, append([]string{"--user"}, arguments...)...)
		cmd.Env = environment
		cmd.WaitDelay = 2 * time.Second
		cmd.Stderr = os.Stderr
		return cmd
	}
	resolved, err := command(service, "--resolve", definition.name).Output()
	if err != nil {
		return fmt.Errorf("resolve OpenRC user worker: %w", err)
	}
	if filepath.Clean(strings.TrimSpace(string(resolved))) != definition.path {
		return fmt.Errorf("OpenRC resolved a different user worker definition for %s", definition.name)
	}
	add := command(update, "add", definition.name, "default")
	add.Stdout = os.Stderr
	if err := add.Run(); err != nil {
		return fmt.Errorf("register OpenRC user worker in default runlevel: %w", err)
	}

	action := "start"
	if replace {
		action = "restart"
	}
	activate := command(service, "--nodeps", definition.name, action)
	activate.Stdout = os.Stderr
	if err := activate.Run(); err != nil {
		return fmt.Errorf("%s OpenRC user worker: %w", action, err)
	}
	return nil
}
