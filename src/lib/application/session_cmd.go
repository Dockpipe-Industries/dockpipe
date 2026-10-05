package application

import "dockpipe/src/lib/application/internal/sessioncmd"

func cmdSession(args []string) error {
	return sessioncmd.Run(args)
}
