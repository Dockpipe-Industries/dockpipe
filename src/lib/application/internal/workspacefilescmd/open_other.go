//go:build !linux && !darwin

package workspacefilescmd

import "os"

func openRequest(name string) (*os.File, error) { return os.Open(name) }
