package application

import (
	"path/filepath"

	"dockpipe/src/lib/application/internal/remotecmd"
)

func cmdRemote(args []string) error {
	return remotecmd.Run(args, func(profilePath string) error {
		return checkWorkflowHostDependencies(nil, filepath.Dir(profilePath), "", &CliOpts{})
	})
}
