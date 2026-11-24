// Command workerhost runs the Windows per-user worker without a console window.
package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"dockpipe/src/lib/infrastructure/process"

	"golang.org/x/sys/windows"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	if len(arguments) != 4 || arguments[0] != "--cli" || arguments[2] != "--state" {
		return 2
	}

	host, err := os.Executable()
	if err != nil {
		return 1
	}

	cli := arguments[1]
	state := arguments[3]
	if !filepath.IsAbs(cli) || !filepath.IsAbs(state) ||
		!strings.EqualFold(filepath.Dir(cli), filepath.Dir(host)) || strings.EqualFold(cli, host) {
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	command := exec.CommandContext(ctx, cli, "remote", "worker", "--state", state)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	if err := process.Run(command); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}

		return 1
	}

	return 0
}
