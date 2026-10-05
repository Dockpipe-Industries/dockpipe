//go:build !linux

package containedexec

import (
	"fmt"
	"os/exec"
	"time"
)

func CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	return nil, fmt.Errorf("generated compilation requires verified process-tree containment; this host has no implemented strategy")
}

type Measurement struct {
	Elapsed   time.Duration
	MaxRSSKiB int64
}

func Measure(cmd *exec.Cmd) ([]byte, Measurement, error) {
	out, err := CombinedOutput(cmd)
	return out, Measurement{}, err
}
