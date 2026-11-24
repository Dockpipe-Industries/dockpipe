//go:build !linux && !windows

package infrastructure

import (
	"os"
	"os/exec"
)

func waitContextHostScriptSignals(command *exec.Cmd, signals <-chan os.Signal) error {
	return waitCommandWithSignals(command, signals)
}

func waitContextSourceHostScriptSignals(command *exec.Cmd, signals <-chan os.Signal, joinOutput, abortOutput func()) error {
	return waitCommandWithSignals(command, signals)
}
