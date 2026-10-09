package remotecmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"dockpipe/src/lib/infrastructure/packageplatform"
)

func installService(ctx context.Context, root, role string) error {
	if role != "broker" && role != "worker" {
		return errors.New("service role must be broker or worker")
	}
	config := "operator.json"
	command := "serve"
	if role == "worker" {
		config, command = "worker.json", "worker"
	}
	if _, err := os.Stat(filepath.Join(root, config)); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(root))
	name := fmt.Sprintf("com.dockpipe.remote.%s.%x", role, digest[:6])
	arguments := []string{executable, "remote", command, "--state", root}
	flatpak := packageplatform.IsFlatpak()
	if flatpak {
		applicationID, err := packageplatform.ApplicationID()
		if err != nil {
			return err
		}
		arguments = flatpakServiceArguments(applicationID, command, root)
	}
	var path, content string
	switch runtime.GOOS {
	case "darwin":
		path = filepath.Join(home, "Library", "LaunchAgents", name+".plist")
		content = launchAgent(name, arguments, filepath.Join(root, role+".log"))
	case "linux":
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		path = filepath.Join(base, "systemd", "user", name+".service")
		content = systemdUnit(arguments)
		if flatpak {
			// /app and the sandbox PATH do not exist in the host user manager.
			content = systemdUnitWithPath(arguments, "")
		}
	default:
		return errors.New("remote user services currently support Linux and macOS")
	}
	if _, err := os.Lstat(path); err == nil {
		previous, err := os.ReadFile(path)
		if err != nil || string(previous) != content {
			return errors.New("existing service differs; inspect it before replacing")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, writeErr := file.WriteString(content)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	} else if !os.IsExist(err) {
		return err
	}
	run := func(tool string, args ...string) error {
		if flatpak {
			args = append([]string{"--host", "--watch-bus", tool}, args...)
			tool = "flatpak-spawn"
		}
		process := exec.CommandContext(ctx, tool, args...)
		process.Stdout, process.Stderr = os.Stderr, os.Stderr
		return process.Run()
	}
	if runtime.GOOS == "darwin" {
		domain := "gui/" + strconv.Itoa(os.Getuid())
		// An existing loaded service is a valid retry after setup lost its reply.
		if exec.CommandContext(ctx, "launchctl", "print", domain+"/"+name).Run() == nil {
			return nil
		}
		return run("launchctl", "bootstrap", domain, path)
	}
	if flatpak {
		// The unit lives in the app's host-visible XDG config directory. Link it
		// explicitly rather than writing through Flatpak's redirected ~/.config.
		if err := run("systemctl", "--user", "link", path); err != nil {
			return err
		}
	}
	if err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	return run("systemctl", "--user", "enable", "--now", name+".service")
}

func launchAgent(name string, arguments []string, log string) string {
	escape := func(value string) string {
		var buffer bytes.Buffer
		_ = xml.EscapeText(&buffer, []byte(value))
		return buffer.String()
	}
	var values strings.Builder
	for _, argument := range arguments {
		fmt.Fprintf(&values, "<string>%s</string>\n", escape(argument))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array>%s</array>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string></dict>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><true/>
<key>ThrottleInterval</key><integer>10</integer>
<key>Umask</key><integer>63</integer>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, escape(name), values.String(), escape(os.Getenv("PATH")), escape(log), escape(log))
}

func systemdUnit(arguments []string) string {
	return systemdUnitWithPath(arguments, os.Getenv("PATH"))
}

func flatpakServiceArguments(applicationID, command, root string) []string {
	return []string{"/usr/bin/flatpak", "run", "--command=dockpipe", applicationID, "remote", command, "--state", root}
}

func systemdUnitWithPath(arguments []string, environmentPath string) string {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		argument = strings.ReplaceAll(argument, "%", "%%")
		argument = strings.ReplaceAll(argument, "$", "$$")
		quoted[index] = strconv.Quote(argument)
	}
	environment := ""
	if environmentPath != "" {
		environmentPath = strings.ReplaceAll(environmentPath, "%", "%%")
		environment = "Environment=" + strconv.Quote("PATH="+environmentPath) + "\n"
	}
	return "[Unit]\nDescription=Dockpipe remote node\nAfter=network-online.target\n\n[Service]\nType=simple\nExecStart=" + strings.Join(quoted, " ") + "\n" + environment + "Restart=on-failure\nRestartSec=10\nKillMode=control-group\nUMask=0077\n\n[Install]\nWantedBy=default.target\n"
}
