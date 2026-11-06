//go:build linux

package main

import (
	"crypto/tls"
	"errors"
	"os"

	"dockpipe/workflows/internal/synology/endpointtls"
	"dockpipe/workflows/internal/synology/hostidentity"
)

const endpointVar = "/var/packages/dockpipe-host-identity-qualification/var"
const privateEndpointDirectory = "endpoint-private"
const publicEndpointDirectory = "endpoint-public"

func initializeEndpoint(root string) error {
	return endpointtls.Initialize(root)
}
func loadEndpoint(root string) (*tls.Config, error) {
	return endpointtls.Load(root)
}
func endpointPaths(root string, create bool) (string, string, error) {
	return endpointtls.Paths(root, create)
}
func endpointFile(path string, mode os.FileMode, owner int) ([]byte, error) {
	return endpointtls.File(path, mode, owner)
}
func endpointDirectory(path string, mode os.FileMode, create bool) error {
	return endpointtls.Directory(path, mode, create)
}

func initializeFixedEndpoint() error {
	metadata, err := hostidentity.Read(schema)
	if err != nil || metadata.Identity.UID <= 0 || metadata.Identity.EUID != metadata.Identity.UID {
		return errors.New("endpoint initialization requires the ordinary package identity")
	}
	for _, name := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		if metadata.Identity.Capabilities[name] != noCapabilities {
			return errors.New("endpoint initialization must not have capabilities")
		}
	}
	return initializeEndpoint(endpointVar)
}
