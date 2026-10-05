package application

import "dockpipe/src/lib/application/internal/resultcmd"

func cmdResult(args []string) error {
	return resultcmd.Run(args)
}
