package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallUserPackagePreservesOtherPackagesAndData(t *testing.T) {
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	store := filepath.Join(global, "packages")
	selected := writeInventoryPackage(t, store, "selected")
	other := writeInventoryPackage(t, store, "other")
	data := filepath.Join(global, "settings.json")
	if err := os.WriteFile(data, []byte("user settings"), 0o600); err != nil {
		t.Fatal(err)
	}
	inventory, err := ListInstalledPackages(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range inventory.Packages {
		if entry.Source == "User" && !entry.Removable {
			t.Fatalf("user archive was not removable: %+v", entry)
		}
	}
	if err := UninstallUserPackage(selected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(selected); !os.IsNotExist(err) {
		t.Fatalf("selected archive remains: %v", err)
	}
	for _, filename := range []string{other, data} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUninstallRejectsProtectedAndUnsafePaths(t *testing.T) {
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	store := filepath.Join(global, "packages")
	selected := writeInventoryPackage(t, store, "selected")
	outside := writeInventoryPackage(t, t.TempDir(), "system")
	core := filepath.Join(store, "core", "dockpipe-core-1.2.3.tar.gz")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("core"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{outside, core, "relative.tar.gz", filepath.Dir(selected)} {
		if err := UninstallUserPackage(filename); err == nil {
			t.Fatalf("accepted protected path %q", filename)
		}
	}
	mismatched := filepath.Join(filepath.Dir(selected), "dockpipe-workflow-different-1.2.3.tar.gz")
	bytes, err := os.ReadFile(selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mismatched, bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UninstallUserPackage(mismatched); err == nil {
		t.Fatal("accepted mismatched package identity")
	}
	for _, filename := range []string{selected, outside, core, mismatched} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUninstallRejectsSymlinkArchiveAndCategory(t *testing.T) {
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	store := filepath.Join(global, "packages")
	outsideStore := t.TempDir()
	outside := writeInventoryPackage(t, outsideStore, "external")
	category := filepath.Join(store, "workflows")
	if err := os.MkdirAll(category, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(category, filepath.Base(outside))
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := UninstallUserPackage(link); err == nil {
		t.Fatal("accepted symlink archive")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(category); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), category); err != nil {
		t.Fatal(err)
	}
	if err := UninstallUserPackage(link); err == nil {
		t.Fatal("accepted symlink category")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}
