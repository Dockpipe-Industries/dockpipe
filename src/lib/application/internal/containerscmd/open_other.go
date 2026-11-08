//go:build !linux && !darwin

package containerscmd

import "os"

func openRequest(name string) (*os.File, error) {
	return os.Open(name)
}
