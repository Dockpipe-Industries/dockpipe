//go:build !linux && !darwin

package infrastructure

import (
	"os"
	"os/exec"
)

// The existing Windows process job owns descendants; inherited console streams
// do not use Unix foreground process groups.
func prepareDockerExecTerminal(_ *exec.Cmd, _ *os.File) (func() error, error) {
	return func() error { return nil }, nil
}
