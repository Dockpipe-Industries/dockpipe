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
	operationErrors := make(chan error, 2)
	failOperation := func(err error) {
		select {
		case operationErrors <- err:
		default:
		}
		cancel()
	}
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	defer signal.Stop(changes)
	go func() {
		for {
			select {
			case <-changes:
				resize()
				if err := terminal.Resize(rows, columns); err != nil {
					failOperation(err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		if _, err := io.Copy(terminal, os.Stdin); err != nil {
			failOperation(err)
		}
	}()
	exit, err := terminal.CopyOutput(os.Stdout)
	select {
	case operationErr := <-operationErrors:
		return operationErr
	default:
	}
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("native host terminal exited with status %d", exit)
	}
	return nil
}
