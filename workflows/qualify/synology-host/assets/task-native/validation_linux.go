//go:build linux

package main

import (
	"strings"
	"syscall"

	"dockpipe/workflows/internal/synology/hostidentity"
)

func validReportedHost(report probeReport) bool {
	if !hostidentity.ValidHostname(report.Host.Hostname) || len(report.Host.Identity.Groups) > hostidentity.MaximumGroups || len(report.Host.Identity.Capabilities) > 5 || len(report.Docker.Containers) > maximumContainers {
		return false
	}
	if report.HTTPS.Available {
		if report.HTTPS.Outcome != "https_200" {
			return false
		}
	} else if report.HTTPS.Outcome != "https_unavailable" && report.HTTPS.Outcome != "unexpected_http_status" && report.HTTPS.Outcome != "request_unavailable" {
		return false
	}
	if len(report.HTTPS.Containers) != 0 {
		return false
	}
	if report.Docker.Available {
		if report.Docker.Outcome != "ok" {
			return false
		}
		for _, container := range report.Docker.Containers {
			if len(container.ID) != 64 || len(container.Name) == 0 || len(container.Name) > 255 || len(container.State) > 32 {
				return false
			}
			for _, character := range container.ID {
				if !strings.ContainsRune("0123456789abcdef", character) {
					return false
				}
			}
		}
	} else {
		if len(report.Docker.Containers) != 0 {
			return false
		}
		switch report.Docker.Outcome {
		case "docker_output_limit", "docker_cancelled", "docker_start_failed", "docker_exit_nonzero", "docker_process_failed", "invalid_or_oversized_inventory":
		default:
			return false
		}
	}
	return true
}

func (current *state) eligiblePeer(credentials *syscall.Ucred) bool {
	current.mutex.Lock()
	defer current.mutex.Unlock()
	current.expireLocked()
	return current.current != nil && current.result == nil && credentials != nil && credentials.Pid > 0 && credentials.Uid != 0 && credentials.Uid == current.account.UID && credentials.Gid == current.account.GID
}
