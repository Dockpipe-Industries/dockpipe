package workspacefiles

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func entryMetadataUnavailable(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
