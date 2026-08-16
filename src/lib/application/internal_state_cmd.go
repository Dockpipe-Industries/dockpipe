package application

import "dockpipe/src/lib/application/internal/internalstatecmd"

func cmdInternalState(args []string) error {
	return internalstatecmd.Run(args)
}
