package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"dockpipe.vm/tools/internal/protocol"
)

const version = "0.7.0"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	showCapabilities := flag.Bool("capabilities", false, "print qualification capabilities")
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
	fmt.Printf("dockpipe-guest-agent %s: no arbitrary command surface; use the signed %s virtio-serial protocol\n", version, protocol.Version)
}
