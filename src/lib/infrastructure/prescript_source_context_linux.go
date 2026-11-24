//go:build linux

package infrastructure

import "os/exec"

func sourceHostUsesPrivateGroup(command *exec.Cmd) bool {
	return command.SysProcAttr != nil && command.SysProcAttr.Setpgid && command.SysProcAttr.Pgid == 0
}
