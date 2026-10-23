package application

import (
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/engineversion"
)

// checkCLIProjectVersion runs before command dispatch can execute project code.
// Informational and installation commands remain available to repair a mismatch.
func checkCLIProjectVersion(argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	switch argv[0] {
	case "init", "install", "get", "core", "doctor", "__state", "--version", "version":
		return nil
	}
	if strings.HasPrefix(argv[0], "-") {
		_, options, err := ParseFlags("", argv)
		if err == nil && options.Help {
			return nil
		}
	}
	workdir := ""
	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		if arg == "--" {
			break
		}
		if (arg == "--help" || arg == "-h") && (index == 0 || !strings.HasPrefix(argv[index-1], "-")) {
			return nil
		}
		if arg == "--workdir" && index+1 < len(argv) {
			index++
			workdir = argv[index]
		} else if strings.HasPrefix(arg, "--workdir=") {
			workdir = strings.TrimPrefix(arg, "--workdir=")
		}
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	root, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return err
	}
	config, err := domain.LoadDockpipeProjectConfig(root)
	if err != nil {
		return err
	}
	return domain.CheckProjectDockpipeVersion(config, engineversion.Current())
}
