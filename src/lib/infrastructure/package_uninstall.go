package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// userPackageArchivePath confines removal to individual optional-package archives.
// Project trees, installer-owned stores, core, state, and caches are never removed.
func userPackageArchivePath(store, filename string) (string, error) {
	root, err := filepath.EvalSymlinks(store)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(filename) {
		return "", fmt.Errorf("package path must be absolute")
	}
	archive, err := os.Lstat(filename)
	if err != nil {
		return "", err
	}
	if !archive.Mode().IsRegular() {
		return "", fmt.Errorf("package must be a regular archive, not a directory or symbolic link")
	}
	category, err := os.Lstat(filepath.Dir(filename))
	if err != nil {
		return "", err
	}
	if !category.IsDir() {
		return "", fmt.Errorf("package category must be a directory, not a symbolic link")
	}
	// Canonicalize both sides: Windows can spell the same ancestor using either
	// its short (8.3) name or its long name. Direct links were rejected above.
	canonical, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, canonical)
	if err != nil {
		return "", err
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) != 2 || (parts[0] != "workflows" && parts[0] != "resolvers") {
		return "", fmt.Errorf("only optional package archives in the user store can be uninstalled")
	}
	kind := strings.TrimSuffix(parts[0], "s")
	if !strings.HasPrefix(parts[1], "dockpipe-"+kind+"-") || !strings.HasSuffix(parts[1], ".tar.gz") {
		return "", fmt.Errorf("not a compiled package archive")
	}
	return relative, nil
}

// UninstallUserPackage removes one archive, retaining all package data and settings.
func UninstallUserPackage(filename string) error {
	store, err := GlobalPackagesRoot()
	if err != nil {
		return err
	}
	relative, err := userPackageArchivePath(store, filename)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(store)
	if err != nil {
		return err
	}
	defer root.Close()
	directory, err := root.Lstat(filepath.Dir(relative))
	if err != nil {
		return err
	}
	if !directory.IsDir() {
		return fmt.Errorf("package category must be a directory, not a symbolic link")
	}
	info, err := root.Lstat(relative)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("package must be a regular archive, not a directory or symbolic link")
	}
	file, err := root.Open(relative)
	if err != nil {
		return err
	}
	manifest, readErr := inventoryArchiveManifestReader(file, filename)
	closeErr := file.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if manifest.Kind != "workflow" && manifest.Kind != "resolver" {
		return fmt.Errorf("core packages cannot be uninstalled")
	}
	expected := filepath.Join(manifest.Kind+"s", "dockpipe-"+manifest.Kind+"-"+manifest.Name+"-"+manifest.Version+".tar.gz")
	if relative != expected {
		return fmt.Errorf("package identity does not match its archive path")
	}
	return root.Remove(relative)
}
