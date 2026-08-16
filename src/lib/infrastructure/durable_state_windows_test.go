//go:build windows

package infrastructure

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

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
