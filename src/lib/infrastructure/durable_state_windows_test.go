//go:build windows

package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsUnlinkedPathsAcceptShortNamesAndRejectJunctions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "long directory name")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	longPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	pathPointer, err := windows.UTF16PtrFromString(longPath)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	length, err := windows.GetShortPathName(pathPointer, &buffer[0], uint32(len(buffer)))
	if err != nil || length == 0 || length >= uint32(len(buffer)) {
		t.Fatalf("short path resolution: length=%d, %v", length, err)
	}
	shortPath := windows.UTF16ToString(buffer[:length])
	if _, err := ValidatePackageStateOverride(longPath, shortPath, filepath.Join(t.TempDir(), "resolved")); err == nil || !strings.Contains(err.Error(), "outside the checkout") {
		t.Fatalf("short-name alias bypassed checkout exclusion: %v", err)
	}
	for _, candidate := range []string{longPath, shortPath} {
		if err := ValidateUnlinkedPath(candidate); err != nil {
			t.Fatalf("ordinary path %q rejected: %v", candidate, err)
		}
		if _, err := InspectDisposableRemovalTree(candidate); err != nil {
			t.Fatal(err)
		}
	}
	if strings.EqualFold(shortPath, longPath) {
		t.Log("filesystem did not assign a distinct short name; ordinary-path assertions still ran")
	}
	junction := filepath.Join(t.TempDir(), "junction")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, longPath).CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v: %s", err, output)
	}
	marker := filepath.Join(longPath, "marker")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{junction, filepath.Join(junction, "marker")} {
		if err := ValidateUnlinkedPath(candidate); err == nil {
			t.Fatalf("junction boundary accepted: %s", candidate)
		}
	}
	if _, err := InspectDisposableRemovalTree(junction); err == nil {
		t.Fatal("junction accepted for removal")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "preserve" {
		t.Fatalf("junction target changed: %q, %v", raw, err)
	}
}

func TestDurableStateWindowsDACLIsProtectedAndOwnerOnly(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(base, "state")
	packageRoot, err := projectPackageStateDirAt(project, "example.tools/provider-pool", stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	userSID, err := currentWindowsUserSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{stateRoot, packageRoot, filepath.Join(packageRoot, durablePackageMetaFile)} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validatePrivatePath(path, info.IsDir()); err != nil {
			t.Fatalf("newly created private path %q failed validation: %v", path, err)
		}
		descriptor, err := windows.GetNamedSecurityInfo(
			path,
			windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
		)
		if err != nil {
			t.Fatal(err)
		}
		owner, _, err := descriptor.Owner()
		if err != nil || owner == nil || !owner.Equals(userSID) {
			t.Fatalf("path %q is not owned by the current user", path)
		}
		dacl, _, err := descriptor.DACL()
		if err != nil {
			t.Fatalf("path %q has no valid DACL: %v", path, err)
		}
		if dacl == nil || dacl.AceCount != 2 {
			t.Fatalf("path %q DACL has %v entries, want only current user and Local System", path, dacl)
		}
		control, _, err := descriptor.Control()
		if err != nil {
			t.Fatal(err)
		}
		if control&windows.SE_DACL_PROTECTED == 0 {
			t.Fatalf("path %q DACL still inherits broad grants", path)
		}
	}
}

func TestDurableStateWindowsDACLRejectsPartialControl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-state")
	if err := os.WriteFile(path, []byte("state"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := makePrivatePath(path, false); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	ace.Mask = windows.FILE_GENERIC_READ
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if err := validatePrivatePath(path, false); err == nil {
		t.Fatal("partial-control DACL unexpectedly accepted")
	}
}

func TestPreparePrivateDirectoryRefusesExistingPartialDACL(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := PreparePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	ace.Mask = windows.FILE_GENERIC_READ
	if err := windows.SetNamedSecurityInfo(root, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := makePrivatePath(root, true); err != nil {
			t.Error(err)
		}
	})
	if err := PreparePrivateDirectory(root); err == nil {
		t.Fatal("existing partial-control directory was accepted or repaired")
	}
	if err := validatePrivatePath(root, true); err == nil {
		t.Fatal("rejected directory permissions were changed")
	}
}
