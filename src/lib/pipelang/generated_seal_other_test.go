//go:build !linux

package pipelang

import (
	"fmt"
	"os"
)

func sealGeneratedExecutable(path, expected string) (*os.File, error) {
	return nil, fmt.Errorf("native bundle prototype requires Linux executable sealing")
}
