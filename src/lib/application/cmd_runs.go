package application

import "dockpipe/src/lib/application/internal/runscmd"

func cmdRuns(args []string) error {
	return runscmd.Run(args)
}
