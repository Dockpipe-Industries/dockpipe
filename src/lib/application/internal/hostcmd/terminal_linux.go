package hostcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"dockpipe/src/lib/infrastructure/nativehost"
	"golang.org/x/term"
)

func relayTerminal(ctx context.Context, client *nativehost.Client, cwd string) error {
	rows, columns := uint16(24), uint16(80)
	resize := func() {
		width, height, err := term.GetSize(int(os.Stdin.Fd()))
		if err == nil && width > 0 && width <= 65535 && height > 0 && height <= 65535 {
			rows, columns = uint16(height), uint16(width)
		}
	}
	resize()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	terminal, err := client.OpenTerminal(ctx, nativehost.TerminalOptions{CWD: cwd, Rows: rows, Columns: columns})
	if err != nil {
		return err
	}
	defer terminal.Close()
	if term.IsTerminal(int(os.Stdin.Fd())) {
		state, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		defer term.Restore(int(os.Stdin.Fd()), state)
	}
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	defer signal.Stop(changes)
	go func() {
		for {
			select {
			case <-changes:
				resize()
				terminal.Resize(rows, columns)
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		io.Copy(terminal, os.Stdin)
	}()
	exit, err := terminal.CopyOutput(os.Stdout)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("native host terminal exited with status %d", exit)
	}
	return nil
}
