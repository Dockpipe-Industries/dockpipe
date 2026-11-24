//go:build !linux && !windows

package infrastructure

import "os/exec"

func sourceHostUsesPrivateGroup(*exec.Cmd) bool {
	return false
}
