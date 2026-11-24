//go:build !windows

package localterminal

import "os/exec"

func configureProviderProcess(*exec.Cmd) {}
