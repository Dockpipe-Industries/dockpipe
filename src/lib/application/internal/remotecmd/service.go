package remotecmd

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func installService(ctx context.Context, root, role string) error {
	return installServiceWithReplacement(ctx, root, role, false)
}

func installServiceWithReplacement(ctx context.Context, root, role string, replace bool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if role != "broker" && role != "worker" {
		return errors.New("service role must be broker or worker")
	}
	if role == "broker" {
		if err := requireLocalBroker(root); err != nil {
			return err
		}
	}
	config := "operator.json"
	if role == "worker" {
		config = "worker.json"
	}
	if _, err := os.Stat(filepath.Join(root, config)); err != nil {
		return err
	}
	definition, err := prepareService(ctx, root, role)
	if err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "service.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	if _, _, err := readServiceForReplacement(definition, replace); err != nil {
		return err
	}
	pendingPath := filepath.Join(root, role+"-service-update.json")
	if !replace {
		if _, err := os.Lstat(pendingPath); err == nil {
			return fmt.Errorf("previous service activation did not finish; retry remote service --role %s --replace", role)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := remoteio.WritePrivate(pendingPath, map[string]bool{"pending": true}); err != nil {
		return err
	}
	if err := publishServiceDefinition(ctx, definition, replace); err != nil {
		return err
	}
	if err := activateService(ctx, definition, replace); err != nil {
		return fmt.Errorf("service activation failed; registration is preserved; retry remote service --role %s --replace: %w", role, err)
	}
	return os.Remove(pendingPath)
}

func activateService(ctx context.Context, definition serviceDefinition, replace bool) error {
	return activateServiceForPlatform(ctx, definition, replace, runtime.GOOS)
}

func activateServiceForPlatform(ctx context.Context, definition serviceDefinition, replace bool, platform string) error {
	path, name, flatpak := definition.path, definition.name, definition.flatpak
	run := func(tool string, args ...string) error {
		if flatpak {
			args = append([]string{"--host", "--watch-bus", tool}, args...)
			tool = "flatpak-spawn"
		}
		process := exec.CommandContext(ctx, tool, args...)
		process.Stdout, process.Stderr = os.Stderr, os.Stderr
		process.WaitDelay = 2 * time.Second
		return process.Run()
	}
	if platform == "darwin" {
		domain := "gui/" + strconv.Itoa(os.Getuid())
		// An existing loaded service is a valid retry after setup lost its reply.
		if exec.CommandContext(ctx, "launchctl", "print", domain+"/"+name).Run() == nil {
			if !replace {
				return nil
			}
			if err := run("launchctl", "bootout", domain+"/"+name); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
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
	if replace {
		if err := run("systemctl", "--user", "enable", name+".service"); err != nil {
			return err
		}
		return run("systemctl", "--user", "restart", name+".service")
	}
	return run("systemctl", "--user", "enable", "--now", name+".service")
}

func launchAgent(name string, arguments []string, log string, bundleIdentifier string) string {
	escape := func(value string) string {
		var buffer bytes.Buffer
		_ = xml.EscapeText(&buffer, []byte(value))
		return buffer.String()
	}
	var values strings.Builder
	for _, argument := range arguments {
		fmt.Fprintf(&values, "<string>%s</string>\n", escape(argument))
	}
	association := ""
	if bundleIdentifier != "" {
		association = "<key>AssociatedBundleIdentifiers</key><array><string>" + escape(bundleIdentifier) + "</string></array>\n"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array>%s</array>
%s<key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string></dict>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><true/>
<key>ThrottleInterval</key><integer>10</integer>
<key>Umask</key><integer>63</integer>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, escape(name), values.String(), association, escape(os.Getenv("PATH")), escape(log), escape(log))
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
