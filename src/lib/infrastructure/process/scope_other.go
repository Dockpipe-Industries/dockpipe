//go:build !linux

package process

import (
	"context"
	"errors"
	"os/exec"
)

func createScope(string, string) (ScopeReference, error) {
	return ScopeReference{}, errors.New("durable process recovery requires Linux delegated cgroup v2")
}

func attachScope(_ *exec.Cmd, reference ScopeReference) (func(), error) {
	if reference.Mode == "host-reboot" {
		return func() {}, nil
	}
	return nil, errors.New("durable process recovery requires Linux delegated cgroup v2")
}

func StopScope(_ context.Context, reference ScopeReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if reference.Mode == "host-reboot" && reference.Boot != "" {
		boot, err := HostBootIdentity()
		if err != nil {
			return err
		}
		if boot != reference.Boot {
			return nil
		}
		return ErrRebootRequired
	}
	return errors.New("durable process recovery requires Linux delegated cgroup v2")
}
