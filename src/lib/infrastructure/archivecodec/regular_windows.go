//go:build windows

package archivecodec

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func openRegular(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	kind, kindErr := windows.GetFileType(handle)
	infoErr := windows.GetFileInformationByHandle(handle, &info)
	if kindErr != nil || infoErr != nil || kind != windows.FILE_TYPE_DISK ||
		info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DEVICE) != 0 {
		windows.CloseHandle(handle)
		return nil, errors.New("archive decoder input must be a regular non-reparse disk file")
	}
	return os.NewFile(uintptr(handle), path), nil
}
