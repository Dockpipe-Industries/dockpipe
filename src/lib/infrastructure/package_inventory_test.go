package infrastructure

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
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
	config := "name: " + name + "\nrun: echo ok\n"
	if err := archive.WriteHeader(&tar.Header{Name: "workflows/" + name + "/config.yml", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(config))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(config)); err != nil {
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

func writeZstdInventoryPackage(t *testing.T, root, name string) string {
	t.Helper()
	gzipPath := writeInventoryPackage(t, root, name)
	input, err := os.Open(gzipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	tarBytes, err := io.ReadAll(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	zstdPath := strings.TrimSuffix(gzipPath, ".gz") + ".zst"
	output, err := os.Create(zstdPath)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.Write(tarBytes); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gzipPath); err != nil {
		t.Fatal(err)
	}
	return zstdPath
}

func TestInstalledPackageInventoryAndUninstallZstd(t *testing.T) {
	project := t.TempDir()
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", t.TempDir())
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	selected := writeZstdInventoryPackage(t, filepath.Join(global, "packages"), "zstd-tool")
	inventory, err := ListInstalledPackages(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Packages) != 1 || len(inventory.Warnings) != 0 {
		t.Fatalf("unexpected inventory: %+v", inventory)
	}
	entry := inventory.Packages[0]
	selectedInfo, err := os.Stat(selected)
	if err != nil {
		t.Fatal(err)
	}
	entryInfo, err := os.Stat(entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "zstd-tool" || !os.SameFile(selectedInfo, entryInfo) || !entry.Removable {
		t.Fatalf("zstd package identity: %+v", entry)
	}
	if err := UninstallUserPackage(entry.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(selected); !os.IsNotExist(err) {
		t.Fatalf("zstd archive remains: %v", err)
	}
}

func TestWorkflowNameDiscoveryDeduplicatesArchiveCodecs(t *testing.T) {
	root := t.TempDir()
	gzipPath := writeInventoryPackage(t, root, "same-tool")
	gzipBytes, err := os.ReadFile(gzipPath)
	if err != nil {
		t.Fatal(err)
	}
	writeZstdInventoryPackage(t, root, "same-tool")
	if err := os.WriteFile(gzipPath, gzipBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := listWorkflowNamesFromTarballDir(filepath.Dir(gzipPath))
	if err != nil || len(names) != 1 || names[0] != "same-tool" {
		t.Fatalf("mixed archive workflow names: %v, %v", names, err)
	}
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
		info, err := os.Stat(entry.Path)
		if err != nil {
			t.Fatal(err)
		}
		if entry.SizeBytes <= 0 || entry.SizeBytes != info.Size() {
			t.Fatalf("inventory archive size does not match stored file: %+v", entry)
		}
		sources[entry.Name] = entry.Source
		if entry.Removable != (entry.Source == "User") {
			t.Fatalf("unexpected removal permission: %+v", entry)
		}
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
