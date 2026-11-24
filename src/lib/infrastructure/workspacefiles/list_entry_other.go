//go:build !windows

package workspacefiles

import (
	"errors"
	"os"
)

func entryMetadataUnavailable(err error) bool {
	return errors.Is(err, os.ErrPermission)
}
