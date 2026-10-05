//go:build windows

package statepaths

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func migrationFileIdentity(path string) (string, error) {
	info, err := migrationWindowsFileInformation(path)
	if err != nil {
		return "", err
	}
	fileID := uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	return fmt.Sprintf("windows:%08x:%016x", info.VolumeSerialNumber, fileID), nil
}

func migrationDeviceIdentity(path string) (string, error) {
	info, err := migrationWindowsFileInformation(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("windows:%08x", info.VolumeSerialNumber), nil
}

func migrationWindowsFileInformation(path string) (*windows.ByHandleFileInformation, error) {
	pathp, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pathp, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, errors.New("provider recovery path is a Windows reparse point")
	}
	return &info, nil
}

func migrationMakePrivatePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("provider recovery private path is a Windows reparse point")
	}
	if directory && !info.IsDir() {
		return errors.New("provider recovery private directory is not a directory")
	}
	if !directory && !info.Mode().IsRegular() {
		return errors.New("provider recovery private file is not regular")
	}
	userSID, err := migrationCurrentWindowsUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	ownerSID, _, err := descriptor.Owner()
	if err != nil || ownerSID == nil || !ownerSID.Equals(userSID) {
		return errors.New("provider recovery private path is not owned by the current user")
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if directory {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entries := []windows.EXPLICIT_ACCESS{
		{AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.SET_ACCESS, Inheritance: inheritance, Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(userSID)}},
		{AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.SET_ACCESS, Inheritance: inheritance, Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(systemSID)}},
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

func migrationCurrentWindowsUserSID() (*windows.SID, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid.Copy()
}

func migrationLockFile(file *os.File) error {
	return windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, new(windows.Overlapped))
}

func migrationUnlockFile(file *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, new(windows.Overlapped))
}

func migrationSyncDirectory(string) error { return nil }
