//go:build !linux && !darwin && !windows

package mcpbundle

import (
	"fmt"
	"os"
)

func openRegular(path string) (*os.File, error) {
	return nil, fmt.Errorf("MCPB file input is unsupported on this operating system")
}
