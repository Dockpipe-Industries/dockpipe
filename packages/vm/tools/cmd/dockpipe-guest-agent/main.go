package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"dockpipe.vm/tools/internal/guest"
	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
)

const version = "1.0.0"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	showCapabilities := flag.Bool("capabilities", false, "print qualification capabilities")
	servePath := flag.String("serve-virtio-serial", "", "serve the signed qualification protocol on an exact virtio-serial device")
	configPath := flag.String("config", "", "read the hash-pinned guest-agent configuration")
	flag.Parse()
	if *showVersion {
		fmt.Println("dockpipe-guest-agent", version)
		return
	}
	if *showCapabilities {
		caps := []string{"identity/v1", "health/v1", "checkpoint/v1", "recovery/v1", "launch-hash-pinned/v1"}
		_ = json.NewEncoder(flag.CommandLine.Output()).Encode(caps)
		return
	}
	if *servePath != "" {
		if *configPath == "" {
			fatal("--config is required with --serve-virtio-serial")
		}
		executable, err := os.Executable()
		if err != nil {
			fatal(err.Error())
		}
		service, err := guest.LoadService(*configPath, executable, manifest.KernelBootIDSource)
		if err != nil {
			fatal(err.Error())
		}
		device, err := os.OpenFile(*servePath, os.O_RDWR, 0)
		if err != nil {
			fatal(err.Error())
		}
		defer device.Close()
		if err := service.Serve(device); err != nil {
			fatal(err.Error())
		}
		return
	}
	fmt.Printf("dockpipe-guest-agent %s: no arbitrary command surface; use the signed %s virtio-serial protocol\n", version, protocol.Version)
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "dockpipe-guest-agent:", message)
	os.Exit(2)
}
