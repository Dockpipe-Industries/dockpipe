package terminalcmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"dockpipe/src/lib/infrastructure/localterminal"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

func Run(args []string) error {
	if len(args) != 1 || args[0] != "--stdio" {
		return errors.New("usage: dockpipe terminal --stdio")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	target, identity := nativeinvocation.Local()
	return localterminal.Stream(ctx, os.Stdin, os.Stdout, terminalsession.Authority{Target: target, Identity: identity}, localterminal.InstalledProvider)
}
