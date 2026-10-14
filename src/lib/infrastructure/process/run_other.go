//go:build !linux && !darwin && !windows

package process

import (
	"errors"
	"os/exec"
)

func run(command *exec.Cmd, started func()) error {
	return errors.New("process tree containment is unavailable on this platform")
}

func inheritedTree() bool { return false }
