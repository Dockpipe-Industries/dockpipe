// Package machinecmd owns local machine inspection, without a remote shell or
// transport-specific configuration.
package machinecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/telemetry"
)

const usage = `dockpipe machine — inspect the machine running this CLI.

Usage:
  dockpipe machine snapshot [--system] [--docker] [--json]

Without category flags, snapshot includes system/process and Docker inventory.
It is read-only, uses existing account permissions, and emits at most 1 MiB of
JSON. It excludes environment values, command arguments and application logs.
Unavailable categories are reported explicitly. Collection has an 8-second limit.
`

func Run(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return run(ctx, args, os.Stdout, infrastructure.ReadMachineSnapshot)
}

func run(ctx context.Context, args []string, output io.Writer,
	collect func(context.Context, bool, bool) (telemetry.MachineSnapshot, error),
) error {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")) {
		_, err := io.WriteString(output, usage)
		return err
	}
	if args[0] != "snapshot" {
		return fmt.Errorf("dockpipe machine: unknown subcommand %q", args[0])
	}
	system, docker := false, false
	for _, argument := range args[1:] {
		switch argument {
		case "--system":
			system = true
		case "--docker":
			docker = true
		case "--json":
		case "--help", "-h":
			_, err := io.WriteString(output, usage)
			return err
		default:
			return fmt.Errorf("dockpipe machine snapshot: unexpected argument %q", argument)
		}
	}
	if !system && !docker {
		system, docker = true, true
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot, err := collect(ctx, system, docker)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := infrastructure.FitMachineSnapshot(&snapshot); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	written, err := output.Write(append(data, '\n'))
	if err == nil && written != len(data)+1 {
		return io.ErrShortWrite
	}
	return err
}
