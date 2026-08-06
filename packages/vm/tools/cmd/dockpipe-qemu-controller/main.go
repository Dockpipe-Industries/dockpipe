package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
	"dockpipe.vm/tools/internal/xdg"
)

const version = "0.8.0"

func main() {
	manifestPath := flag.String("validate-manifest", "", "validate an offline qualification manifest")
	planRuntime := flag.String("plan-runtime-dir", "", "print an inert QEMU argv plan for the validated manifest")
	provisioningPath := flag.String("plan-provisioning", "", "print the exact inert provisioning plan for one qualification instance")
	liveAuthorizationPath := flag.String("live-authorization", "", "validate a distinct short-lived authorization against the exact inert plan")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("dockpipe-qemu-controller", version)
		return
	}
	if *manifestPath == "" {
		fatal("only --version or --validate-manifest is supported; this binary never starts or powers a VM")
	}
	m, err := manifest.Load(*manifestPath)
	if err != nil {
		fatal(err.Error())
	}
	if *provisioningPath != "" {
		if *planRuntime != "" {
			fatal("QEMU argv and provisioning plans must be requested separately")
		}
		contract, err := provisioning.Load(*provisioningPath)
		if err != nil {
			fatal(err.Error())
		}
		home, err := os.UserHomeDir()
		if err != nil {
			fatal(err.Error())
		}
		paths, err := xdg.Resolve(home, map[string]string{
			"XDG_CACHE_HOME": os.Getenv("XDG_CACHE_HOME"), "XDG_STATE_HOME": os.Getenv("XDG_STATE_HOME"),
			"XDG_CONFIG_HOME": os.Getenv("XDG_CONFIG_HOME"), "XDG_RUNTIME_DIR": os.Getenv("XDG_RUNTIME_DIR"),
		})
		if err != nil {
			fatal(err.Error())
		}
		checkout, err := os.Getwd()
		if err != nil {
			fatal(err.Error())
		}
		plan, err := provisioning.BuildPlan(contract, paths, m, checkout, provisioning.OSImageInspector{})
		if err != nil {
			fatal(err.Error())
		}
		if *liveAuthorizationPath != "" {
			auth, err := provisioning.LoadAuthorization(*liveAuthorizationPath)
			if err != nil {
				fatal(err.Error())
			}
			plan, err = provisioning.AuthorizePlan(plan, contract, auth, time.Now())
			if err != nil {
				fatal(err.Error())
			}
		}
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			fatal(err.Error())
		}
		return
	}
	if *liveAuthorizationPath != "" {
		fatal("--live-authorization is valid only with --plan-provisioning")
	}
	if *planRuntime == "" {
		fmt.Println("qualification manifest valid; live execution unavailable")
		return
	}
	plan, err := manifest.PlanQEMU(m, *planRuntime)
	if err != nil {
		fatal(err.Error())
	}
	if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
		fatal(err.Error())
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "dockpipe-qemu-controller:", message)
	os.Exit(2)
}
