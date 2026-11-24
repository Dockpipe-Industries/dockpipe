//go:build linux

package infrastructure

import (
	"os"
	"os/exec"
)

func waitContextHostScriptSignals(command *exec.Cmd, signals <-chan os.Signal) error {
	if command.SysProcAttr != nil && command.SysProcAttr.Setpgid && command.SysProcAttr.Pgid == 0 {
		return waitHostProcessGroup(command, signals)
	}
	return waitCommandWithSignals(command, signals)
}

func waitContextSourceHostScriptSignals(command *exec.Cmd, signals <-chan os.Signal, joinOutput, abortOutput func()) error {
	return waitHostProcessGroupWithCompletion(command, signals, observeHostProcessExit, joinOutput, abortOutput)
}
