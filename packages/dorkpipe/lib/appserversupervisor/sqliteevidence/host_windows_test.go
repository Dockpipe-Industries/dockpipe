//go:build windows

package sqliteevidence

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const fileAllAccess windows.ACCESS_MASK = 0x001f01ff

func selectedNativeVFS() string { return "win32" }

func collectAndProtectWindowsHost(root string) (windowsHostFacts, error) {
	if err := setWindowsPrivateDirectory(root); err != nil {
		return windowsHostFacts{}, fmt.Errorf("protect fixture root: %w", err)
	}
	rootUTF16, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return windowsHostFacts{}, fmt.Errorf("encode fixture root: %w", err)
	}
	volumePathBuffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(rootUTF16, &volumePathBuffer[0], uint32(len(volumePathBuffer))); err != nil {
		return windowsHostFacts{}, fmt.Errorf("resolve volume path: %w", err)
	}
	volumePath := windows.UTF16ToString(volumePathBuffer)
	volumePathUTF16, err := windows.UTF16PtrFromString(volumePath)
	if err != nil {
		return windowsHostFacts{}, fmt.Errorf("encode volume path: %w", err)
	}
	if driveType := windows.GetDriveType(volumePathUTF16); driveType != windows.DRIVE_FIXED {
		return windowsHostFacts{}, fmt.Errorf("fixture volume drive type = %d, want fixed (%d)", driveType, windows.DRIVE_FIXED)
	}

	volumeLabel := make([]uint16, windows.MAX_PATH+1)
	fileSystemName := make([]uint16, windows.MAX_PATH+1)
	var serialNumber, maximumComponentLength, fileSystemFlags uint32
	if err := windows.GetVolumeInformation(
		volumePathUTF16,
		&volumeLabel[0], uint32(len(volumeLabel)),
		&serialNumber,
		&maximumComponentLength,
		&fileSystemFlags,
		&fileSystemName[0], uint32(len(fileSystemName)),
	); err != nil {
		return windowsHostFacts{}, fmt.Errorf("read volume information: %w", err)
	}
	fsName := windows.UTF16ToString(fileSystemName)
	if fsName != "NTFS" {
		return windowsHostFacts{}, fmt.Errorf("fixture filesystem = %q, want NTFS", fsName)
	}
	volumeNameBuffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeNameForVolumeMountPoint(volumePathUTF16, &volumeNameBuffer[0], uint32(len(volumeNameBuffer))); err != nil {
		return windowsHostFacts{}, fmt.Errorf("read volume identity: %w", err)
	}
	volumeGUID := windows.UTF16ToString(volumeNameBuffer)

	version := windows.RtlGetVersion()
	ntfsVersion := "unavailable"
	if major, minor, ok := queryNTFSVersion(volumePath); ok {
		ntfsVersion = fmt.Sprintf("%d.%d", major, minor)
	}
	protection, err := requireWindowsPrivatePath(root)
	if err != nil {
		return windowsHostFacts{}, fmt.Errorf("verify fixture root protection: %w", err)
	}
	return windowsHostFacts{
		WindowsBuild: fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber),
		Architecture: runtime.GOARCH,
		DriveType:    "fixed",
		FileSystem:   fsName,
		NTFSVersion:  ntfsVersion,
		VolumeID:     fmt.Sprintf("guid=%s serial=%08x label=%q", volumeGUID, serialNumber, windows.UTF16ToString(volumeLabel)),
		GoVersion:    runtime.Version(),
		Protection:   protection,
	}, nil
}

func queryNTFSVersion(volumePath string) (uint16, uint16, bool) {
	volumePath = strings.TrimRight(volumePath, `\/`)
	if len(volumePath) != 2 || volumePath[1] != ':' {
		return 0, 0, false
	}
	devicePath, err := windows.UTF16PtrFromString(`\\.\` + volumePath)
	if err != nil {
		return 0, 0, false
	}
	handle, err := windows.CreateFile(
		devicePath,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, 0, false
	}
	defer windows.CloseHandle(handle)
	buffer := make([]byte, 160)
	var returned uint32
	if err := windows.DeviceIoControl(handle, windows.FSCTL_GET_NTFS_VOLUME_DATA, nil, 0, &buffer[0], uint32(len(buffer)), &returned, nil); err != nil {
		return 0, 0, false
	}
	// NTFS_VOLUME_DATA_BUFFER is 96 bytes. When supported, the appended
	// NTFS_EXTENDED_VOLUME_DATA begins with ByteCount, MajorVersion, MinorVersion.
	if returned < 104 || binary.LittleEndian.Uint32(buffer[96:100]) < 8 {
		return 0, 0, false
	}
	return binary.LittleEndian.Uint16(buffer[100:102]), binary.LittleEndian.Uint16(buffer[102:104]), true
}

func setWindowsPrivateDirectory(path string) error {
	userSID, systemSID, err := windowsEvidenceSIDs()
	if err != nil {
		return err
	}
	sddl := fmt.Sprintf("D:P(A;OICI;FA;;;%s)(A;OICI;FA;;;%s)", userSID.String(), systemSID.String())
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("build restricted DACL: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read restricted DACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		return fmt.Errorf("set restricted DACL: %w", err)
	}
	return nil
}

func requireWindowsPrivatePath(path string) (string, error) {
	userSID, systemSID, err := windowsEvidenceSIDs()
	if err != nil {
		return "", err
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return "", fmt.Errorf("read security descriptor for %s: %w", filepath.Base(path), err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return "", fmt.Errorf("read owner for %s: %w", filepath.Base(path), err)
	}
	if !owner.Equals(userSID) {
		return "", fmt.Errorf("owner for %s = %s, want current user %s", filepath.Base(path), owner.String(), userSID.String())
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return "", fmt.Errorf("read DACL for %s: %w", filepath.Base(path), err)
	}
	if dacl == nil || dacl.AceCount != 2 {
		return "", fmt.Errorf("DACL ACE count for %s = %d, want exactly 2", filepath.Base(path), aceCount(dacl))
	}
	foundUser := false
	foundSystem := false
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return "", fmt.Errorf("read DACL ACE %d for %s: %w", i, filepath.Base(path), err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return "", fmt.Errorf("DACL ACE %d for %s has type %d, want allow", i, filepath.Base(path), ace.Header.AceType)
		}
		if ace.Mask != fileAllAccess {
			return "", fmt.Errorf("DACL ACE %d for %s has mask %#x, want %#x", i, filepath.Base(path), ace.Mask, fileAllAccess)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		switch {
		case sid.Equals(userSID):
			foundUser = true
		case sid.Equals(systemSID):
			foundSystem = true
		default:
			return "", fmt.Errorf("DACL ACE %d for %s grants unexpected SID %s", i, filepath.Base(path), sid.String())
		}
	}
	if !foundUser || !foundSystem {
		return "", fmt.Errorf("DACL for %s missing current-user or SYSTEM full control", filepath.Base(path))
	}
	return fmt.Sprintf("owner=%s trustees=[%s,%s] access=full", userSID.String(), userSID.String(), systemSID.String()), nil
}

func aceCount(dacl *windows.ACL) uint16 {
	if dacl == nil {
		return 0
	}
	return dacl.AceCount
}

func windowsEvidenceSIDs() (*windows.SID, *windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, nil, fmt.Errorf("read current user SID: %w", err)
	}
	userSID, err := user.User.Sid.Copy()
	if err != nil {
		return nil, nil, fmt.Errorf("copy current user SID: %w", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, nil, fmt.Errorf("create SYSTEM SID: %w", err)
	}
	return userSID, systemSID, nil
}
