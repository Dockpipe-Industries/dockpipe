package application

import "dockpipe/src/lib/application/internal/clonecmd"

func cmdClone(args []string) error {
	return clonecmd.Run(args)
}
