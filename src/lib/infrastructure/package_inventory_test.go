package infrastructure

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeInventoryPackage(t *testing.T, root, name string) string {
	t.Helper()
	directory := filepath.Join(root, "workflows")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "dockpipe-workflow-"+name+"-1.2.3.tar.gz")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	archive := tar.NewWriter(gz)
	body := "schema: 1\nkind: workflow\nname: " + name + "\nversion: 1.2.3\n"
	if err := archive.WriteHeader(&tar.Header{Name: "workflows/" + name + "/package.yml", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestInstalledPackageInventoryIncludesRuntimeStores(t *testing.T) {
	project := t.TempDir()
	global := t.TempDir()
	system := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", system)
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	projectStore, err := PackagesRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	writeInventoryPackage(t, projectStore, "project-tool")
	userArchive := writeInventoryPackage(t, filepath.Join(global, "packages"), "user-tool")
	writeInventoryPackage(t, filepath.Join(system, "packages"), "system-tool")
	inventory, err := ListInstalledPackages(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Packages) != 3 || len(inventory.Warnings) != 0 {
		t.Fatalf("inventory: %+v", inventory)
	}
	sources := map[string]string{}
	for _, entry := range inventory.Packages {
		sources[entry.Name] = entry.Source
	}
	if sources["project-tool"] != "Project" || sources["user-tool"] != "User" || sources["system-tool"] != "System" {
		t.Fatalf("sources: %v", sources)
	}
	resolved, err := FindLatestWorkflowTarball(project, "user-tool")
	if err != nil || resolved != userArchive {
		t.Fatalf("installed user package is not runtime-visible: %s, %v", resolved, err)
	}
}

func TestInstalledPackageInventoryReportsCorruption(t *testing.T) {
	project := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	root, err := PackagesRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	filename := writeInventoryPackage(t, root, "broken")
	if err := os.WriteFile(filename, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	inventory, err := ListInstalledPackages(project)
	if err != nil || len(inventory.Packages) != 0 || len(inventory.Warnings) != 1 {
		t.Fatalf("corruption was hidden: %+v, %v", inventory, err)
	}
}
