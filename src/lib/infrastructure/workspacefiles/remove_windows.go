//go:build windows

package workspacefiles

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func removeFile(root *os.Root, name string) error {
	parent, err := root.Open(".")
	if err != nil {
		return err
	}
	defer parent.Close()
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: windows.Handle(parent.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var status windows.IO_STATUS_BLOCK
	var handle windows.Handle
	// Resolve a single leaf relative to the pinned directory. Never reconstruct
	// an absolute path or fall back to deleting a replacement directory.
	err = windows.NtCreateFile(&handle, windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		&attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	deleteFile := byte(1)
	return windows.NtSetInformationFile(handle, &status, &deleteFile, 1, windows.FileDispositionInformation)
}
