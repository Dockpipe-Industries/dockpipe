//go:build !linux

package infrastructure

import "os/exec"

func setRunHostProcAttrs(cmd *exec.Cmd) {}

func terminateRunHostProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
