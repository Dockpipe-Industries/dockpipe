package application

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagecatalog"
)

func cmdPackageRemote(args []string, install bool) error {
	command := "catalog"
	if install {
		command = "install"
	}
	flags := flag.NewFlagSet("package "+command, flag.ContinueOnError)
	remote := flags.String("remote", "", "HTTPS latest, release, or store manifest URL (required)")
	kind := flags.String("kind", "", "package kind: core, workflow, resolver (install only)")
	name := flags.String("name", "", "package name (install only)")
	checksum := flags.String("sha256", "", "expected catalog checksum; rejects a changed selection (install only)")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || *remote == "" {
		return fmt.Errorf("package %s requires --remote <HTTPS manifest URL> and no positional arguments", command)
	}
	if install && (*kind == "" || *name == "") {
		return fmt.Errorf("package install requires --kind and --name")
	}
	if !install && (*kind != "" || *name != "" || *checksum != "") {
		return fmt.Errorf("--kind, --name, and --sha256 are install options")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	client := packagecatalog.NewClient()
	catalog, err := client.Load(ctx, *remote, runtime.GOOS+"-"+runtime.GOARCH)
	if err != nil {
		return err
	}
	if !install {
		return json.NewEncoder(os.Stdout).Encode(catalog)
	}
	for _, entry := range catalog.Packages {
		if entry.Kind != *kind || entry.Name != *name {
			continue
		}
		if *checksum != "" && !strings.EqualFold(*checksum, entry.SHA256) {
			return fmt.Errorf("selected package changed in the remote; refresh the catalog before installing")
		}
		root, err := infrastructure.GlobalPackagesRoot()
		if err != nil {
			return err
		}
		installed, err := client.Install(ctx, catalog, entry, root)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Path string `json:"path"`
		}{installed})
	}
	return fmt.Errorf("package %s/%s is not in this catalog", *kind, *name)
}
