package remotecmd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"dockpipe/src/lib/infrastructure/packageplatform"
	"dockpipe/src/lib/infrastructure/process"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"golang.org/x/sys/unix"
)

const (
	desktopWorkerRegistrationPoll = time.Second
	desktopWorkerRestartDelay     = 10 * time.Second
	desktopWorkerShutdownGrace    = 10 * time.Second
	desktopWorkerReadyFD          = 3
	desktopWorkerReady            = '1'
)

func desktopWorkerDefinition(base, home, root, name string, arguments []string, fingerprint string, flatpak bool) (string, string, []string, error) {
	if flatpak {
		return "", "", nil, errors.New("worker registration from Flatpak requires a host systemd user manager; use the native Dockpipe installation for XDG desktop autostart")
	}

	if !filepath.IsAbs(base) {
		base = filepath.Join(home, ".config")
	}
	path := filepath.Join(base, "autostart", name+".desktop")
	if len(arguments) == 0 || !filepath.IsAbs(arguments[0]) || strings.Contains(arguments[0], "=") || !filepath.IsAbs(path) {
		return "", "", nil, errors.New("desktop worker requires absolute XDG registration and executable paths; the executable path must exclude =")
	}

	launchArguments := []string{arguments[0], "remote", "service-worker-desktop", "--state", root, "--request", path, "--expected-digest", fingerprint}
	command, err := desktopWorkerExec(launchArguments)
	if err != nil {
		return "", "", nil, err
	}

	content := "# Dockpipe executable sha256: " + fingerprint + "\n" +
		"[Desktop Entry]\nType=Application\nName=Dockpipe Remote Worker\nTerminal=false\nExec=" + command + "\n"
	return path, content, launchArguments, nil
}

func desktopWorkerExec(arguments []string) (string, error) {
	// The desktop entry string escapes are decoded before double-quoted Exec arguments.
	quoteArgument := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`, "%", "%%")
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		if !utf8.ValidString(argument) || strings.ContainsFunc(argument, unicode.IsControl) {
			return "", errors.New("desktop worker command arguments must be UTF-8 and exclude control characters")
		}

		escaped := strings.ReplaceAll(quoteArgument.Replace(argument), `\`, `\\`)
		quoted[index] = `"` + escaped + `"`
	}

	return strings.Join(quoted, " "), nil
}

func requireDesktopWorkerSession() error {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return errors.New("XDG worker autostart requires a graphical desktop session")
	}
	if os.Getenv("XDG_CURRENT_DESKTOP") == "" && os.Getenv("DESKTOP_SESSION") == "" && os.Getenv("SESSION_MANAGER") == "" {
		return errors.New("XDG worker autostart requires a desktop environment that launches XDG autostart applications")
	}
	return nil
}

func activateDesktopWorker(ctx context.Context, definition serviceDefinition) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireDesktopWorkerSession(); err != nil {
		return err
	}
	if len(definition.args) == 0 {
		return errors.New("missing desktop worker supervisor command")
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer reader.Close()
	defer writer.Close()

	arguments := append(slices.Clone(definition.args[1:]), "--stdio")
	command := exec.Command(definition.args[0], arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.ExtraFiles = []*os.File{writer}
	if err := command.Start(); err != nil {
		return err
	}
	_ = writer.Close()

	stopCancellation := context.AfterFunc(ctx, func() { _ = reader.Close() })
	defer stopCancellation()
	var acknowledgement [1]byte
	_, err = io.ReadFull(reader, acknowledgement[:])
	if err != nil || acknowledgement[0] != desktopWorkerReady {
		_ = command.Process.Kill()
		_ = command.Wait()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("desktop worker supervisor failed to confirm its registration and ownership")
	}

	return command.Process.Release()
}

func superviseDesktopWorker(ctx context.Context, root, path, fingerprint string, stdio bool) error {
	if packageplatform.IsFlatpak() {
		return errors.New("XDG desktop autostart requires a native installation")
	}
	if err := requireDesktopWorkerSession(); err != nil {
		return err
	}

	executablePath, err := process.Executable()
	if err != nil {
		return err
	}
	executable, err := os.Open(executablePath)
	if err != nil {
		return err
	}
	defer executable.Close()

	// Retain the inspected file through every worker spawn, including during an
	// atomic package update that might replace the installed path.
	executableHandle := fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), executable.Fd())
	installedFingerprint, err := executableFingerprint(executableHandle)
	if err != nil {
		return err
	}
	if fingerprint != installedFingerprint {
		return errors.New("installed desktop worker executable differs from its reviewed registration")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	name := remoteServiceName(root, "worker")
	expectedPath, content, _, err := desktopWorkerDefinition(os.Getenv("XDG_CONFIG_HOME"), home, root, name, []string{executablePath}, fingerprint, false)
	if err != nil {
		return err
	}
	if expectedPath != path || !desktopWorkerRegistrationMatches(path, content) {
		return errors.New("desktop worker registration changed; review and replace it with remote service --role worker --replace")
	}
	sessionEnded := desktopWorkerSessionEnd()
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}

	owner, err := claimDesktopWorker(ctx, root, path, content)
	if err != nil {
		return err
	}
	if owner != nil {
		defer owner.Close()
	}
	if sessionEnded() {
		return errors.New("graphical session ended before the desktop worker could start")
	}
	if stdio {
		acknowledgement := os.NewFile(desktopWorkerReadyFD, "desktop-worker-readiness")
		if _, err := acknowledgement.Write([]byte{desktopWorkerReady}); err != nil {
			_ = acknowledgement.Close()
			return err
		}
		_ = acknowledgement.Close()
	}
	if owner == nil {
		// Another supervisor already owns this identical registration.
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go watchDesktopWorkerRegistration(workerCtx, cancel, path, content, sessionEnded)
	return runDesktopWorkerLoop(workerCtx, executablePath, executableHandle, root, desktopWorkerRestartDelay)
}

func claimDesktopWorker(ctx context.Context, root, path, content string) (*os.File, error) {
	lockPath := filepath.Join(root, "worker-desktop-supervisor.lock")
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), lockPath)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		_ = file.Close()
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o077 != 0 || stat.Nlink != 1 || int64(stat.Uid) != int64(os.Getuid()) {
		_ = file.Close()
		return nil, errors.New("desktop worker supervisor lock must be a private user-owned regular file")
	}

	token := sha256.Sum256([]byte(content))
	waitCtx, cancel := context.WithTimeout(ctx, 2*desktopWorkerShutdownGrace)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for waitCtx.Err() == nil {
		if !desktopWorkerRegistrationMatches(path, content) {
			_ = file.Close()
			return nil, errors.New("desktop worker registration changed while acquiring ownership")
		}

		err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			_, err := file.WriteAt(token[:], 0)
			if err != nil {
				_ = file.Close()
				return nil, err
			}
			return file, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			_ = file.Close()
			return nil, err
		}
		var activeToken [sha256.Size]byte
		if _, err := file.ReadAt(activeToken[:], 0); err == nil && activeToken == token {
			_ = file.Close()
			return nil, nil
		}

		select {
		case <-ticker.C:
		case <-waitCtx.Done():
		}
	}

	_ = file.Close()
	return nil, fmt.Errorf("waiting for the previous desktop worker supervisor to stop: %w", waitCtx.Err())
}

func desktopWorkerRegistrationMatches(path, content string) bool {
	current, err := readServiceDefinition(path)
	return err == nil && string(current) == content
}

func watchDesktopWorkerRegistration(ctx context.Context, cancel context.CancelFunc, path, content string, sessionEnded func() bool) {
	ticker := time.NewTicker(desktopWorkerRegistrationPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sessionEnded() || !desktopWorkerRegistrationMatches(path, content) {
				cancel()
				return
			}
		}
	}
}

func desktopWorkerSessionEnd() func() bool {
	// XSMP identifies the current Xfce (or other X11) session manager, unlike
	// DISPLAY, whose X server can also serve a subsequent login.
	var candidates []string
	for _, address := range strings.Split(os.Getenv("SESSION_MANAGER"), ",") {
		transport, endpoint, ok := strings.Cut(address, "/")
		if !ok || (transport != "local" && transport != "unix") {
			continue
		}
		_, path, ok := strings.Cut(endpoint, ":")
		if ok && filepath.IsAbs(path) {
			candidates = append(candidates, path)
		}
	}
	if display := os.Getenv("WAYLAND_DISPLAY"); display != "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if !filepath.IsAbs(display) && filepath.IsAbs(runtimeDir) {
			display = filepath.Join(runtimeDir, display)
		}
		if filepath.IsAbs(display) {
			candidates = append(candidates, display)
		}
	}

	for _, path := range candidates {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}
		return func() bool {
			current, err := os.Lstat(path)
			return err != nil || !os.SameFile(info, current)
		}
	}

	// Some sessions have no stable local socket; still honor session-manager
	// termination signals and explicit registration removal or replacement.
	return func() bool { return false }
}

func runDesktopWorkerLoop(ctx context.Context, executable, executableHandle, root string, restartDelay time.Duration) error {
	// Linux delivers Pdeathsig when the thread that started the child exits.
	// Keep that thread alive until the owned child has exited and been reaped.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	verified, err := os.Stat(executableHandle)
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		installed, err := os.Stat(executable)
		if err != nil || !os.SameFile(verified, installed) || verified.Size() != installed.Size() || verified.ModTime() != installed.ModTime() {
			return errors.New("desktop worker executable changed; review and replace its service registration")
		}

		command := exec.CommandContext(ctx, executableHandle, "remote", "worker", "--state", root)
		command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Cancel = func() error { return command.Process.Signal(syscall.SIGTERM) }
		command.WaitDelay = desktopWorkerShutdownGrace
		if err := command.Run(); err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "Dockpipe desktop worker stopped:", err)
		}

		timer := time.NewTimer(restartDelay)
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
	return nil
}
