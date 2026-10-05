package application

import "dockpipe/src/lib/application/internal/doctorcmd"

func cmdDoctor(args []string) error {
	return doctorcmd.Run(args)
}
