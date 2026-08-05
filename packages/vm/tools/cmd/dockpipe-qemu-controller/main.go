package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"dockpipe.vm/tools/internal/manifest"
)

const version = "0.7.0"

func main() {
	manifestPath := flag.String("validate-manifest", "", "validate an offline qualification manifest")
	planRuntime := flag.String("plan-runtime-dir", "", "print an inert QEMU argv plan for the validated manifest")
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
