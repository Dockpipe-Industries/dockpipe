//go:build !linux && !darwin && !windows

package process

import (
	"errors"
	"os/exec"
)

func Run(command *exec.Cmd) error {
	return errors.New("process tree containment is unavailable on this platform")
}

func inheritedTree() bool { return false }
