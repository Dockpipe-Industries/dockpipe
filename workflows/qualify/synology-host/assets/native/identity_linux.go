//go:build linux

package main

import (
	"io"
	"syscall"

	"dockpipe/workflows/internal/synology/hostidentity"
)

const schema = "dockpipe.synology_host_qualification.v1"
const maximumGroups = hostidentity.MaximumGroups
const maximumStatusBytes = hostidentity.MaximumStatusBytes

type identity = hostidentity.Identity
type status = hostidentity.Status

func currentStatus() (status, error) {
	return hostidentity.Read(schema)
}
func validHostname(hostname string) bool {
	return hostidentity.ValidHostname(hostname)
}
func parseCapabilities(reader io.Reader) (map[string]string, error) {
	return hostidentity.ParseCapabilities(reader)
}
func noNewPrivilegesResult(value uintptr, errno syscall.Errno) (*bool, error) {
	return hostidentity.NoNewPrivilegesResult(value, errno)
}
