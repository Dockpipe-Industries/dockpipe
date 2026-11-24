//go:build !linux && !darwin

package infrastructure

import (
	"fmt"
	"os"
)

func openControlRegular(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("control artifact must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		file.Close()
		return nil, fmt.Errorf("control artifact changed during admission")
	}
	return file, nil
}

func workflowControlSignals() []os.Signal { return []os.Signal{os.Interrupt} }
