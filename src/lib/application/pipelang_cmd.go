package application

import pipelangcli "github.com/Dockpipe-Industries/pipelang/src/lib/application"

// cmdPipeLang preserves the Dockpipe CLI while the language owns its implementation.
func cmdPipeLang(args []string) error {
	return pipelangcli.Run(args)
}
