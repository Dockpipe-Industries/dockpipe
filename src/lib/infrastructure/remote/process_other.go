//go:build !linux && !darwin

package remote

import "os/exec"

func ContainProcess(command *exec.Cmd) {}
func CleanupProcess(command *exec.Cmd) {}
