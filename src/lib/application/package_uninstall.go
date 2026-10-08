package application

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"dockpipe/src/lib/infrastructure"
)

func cmdPackageUninstall(args []string) error {
	flags := flag.NewFlagSet("package uninstall", flag.ContinueOnError)
	path := flags.String("path", "", "absolute optional-package archive path from package list --format json")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *path == "" || flags.NArg() != 0 {
		return fmt.Errorf("package uninstall requires --path <user package archive> and no positional arguments")
	}
	if err := infrastructure.UninstallUserPackage(*path); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Path string `json:"path"`
	}{*path})
}
