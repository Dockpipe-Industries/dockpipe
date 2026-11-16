//go:build !linux && !windows

package infrastructure

import "os/exec"

func waitHostScriptWithSignalForward(cmd *exec.Cmd) error {
	return waitCommandWithSignalForward(cmd)
}
