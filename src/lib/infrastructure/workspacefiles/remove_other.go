//go:build !linux && !darwin && !windows

package workspacefiles

import (
	"errors"
	"os"
)

func removeFile(*os.Root, string) error {
	return errors.New("file-only removal is unavailable on this platform")
}
