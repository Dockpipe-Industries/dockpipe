//go:build !linux

package executor

import (
	"context"
	"fmt"
)

func NewGate3LinuxRunner(Gate3RunnerConfig) (*Gate3LinuxRunner, error) {
	return nil, fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}

type Gate3LinuxRunner struct{}

func (*Gate3LinuxRunner) Boot(context.Context, int) error {
	return fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}
func (*Gate3LinuxRunner) Recover(context.Context, Gate3Trial) error {
	return fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}
func (*Gate3LinuxRunner) Checkpoint(context.Context, Gate3Trial) error {
	return fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}
func (*Gate3LinuxRunner) HardPower(context.Context, Gate3Trial) error {
	return fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}
func (*Gate3LinuxRunner) ControlledShutdown(context.Context) error {
	return fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}
func (*Gate3LinuxRunner) Preserve(context.Context) error {
	return fmt.Errorf("Linux Gate 3 runner is unavailable on this platform")
}

var _ Gate3Runner = (*Gate3LinuxRunner)(nil)
