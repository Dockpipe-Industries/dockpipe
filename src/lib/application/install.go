package application

import "fmt"

func cmdInstall(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printInstallUsage()
		return nil
	}
	if args[0] == "core" {
		return fmt.Errorf("core installation is retired: the Dockpipe CLI is self-contained; select optional workflow and resolver packages through package stores")
	}
	return fmt.Errorf("unknown install target %q (try: dockpipe install --help)", args[0])
}

func printInstallUsage() {
	fmt.Print("Dockpipe CLI installation is self-contained. Optional workflow and resolver packages are supplied through configured package stores.\n")
}
