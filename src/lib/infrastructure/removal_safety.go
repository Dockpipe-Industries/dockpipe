package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DisposableRemovalTreeSummary is a deterministic logical-size inventory of a validated tree.
type DisposableRemovalTreeSummary struct {
	LogicalBytes int64
	Files        int64
}

// ValidateDisposableRemovalPath proves that target is a strict descendant of boundary and that
// every existing path component is a real directory on the boundary filesystem. Missing trailing
// components are allowed so callers can safely report an absent target as a no-op.
func ValidateDisposableRemovalPath(boundary, target string) error {
	return validateDisposableRemovalPath(boundary, target, durableDeviceIdentity)
}

func validateDisposableRemovalPath(boundary, target string, deviceIdentity func(string) (string, error)) error {
	boundary, err := filepath.Abs(filepath.Clean(boundary))
	if err != nil {
		return err
	}
	target, err = filepath.Abs(filepath.Clean(target))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(boundary, target)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("removal target must be a strict descendant of its boundary")
	}
	boundaryInfo, err := validateDisposableRemovalDirectory(boundary)
	if err != nil {
		return fmt.Errorf("removal boundary: %w", err)
	}
	boundaryDevice, err := deviceIdentity(boundary)
	if err != nil {
		return err
	}
	if durableFileInfoIsLinkOrReparse(boundaryInfo) {
		return errors.New("removal boundary is linked or reparsed")
	}
	current := boundary
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if !info.IsDir() || durableFileInfoIsLinkOrReparse(info) {
			return fmt.Errorf("removal target path %q is linked, reparsed, or not a directory", current)
		}
		device, deviceErr := deviceIdentity(current)
		if deviceErr != nil || device != boundaryDevice {
			return fmt.Errorf("removal target path %q crosses a filesystem boundary", current)
		}
	}
	return nil
}

func validateDisposableRemovalDirectory(path string) (os.FileInfo, error) {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	canonical, err = filepath.Abs(filepath.Clean(canonical))
	if err != nil || !sameDurablePath(path, canonical) {
		return nil, errors.New("path contains a filesystem link or reparse point")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || durableFileInfoIsLinkOrReparse(info) {
		return nil, errors.New("path is linked, reparsed, or not a directory")
	}
	return info, nil
}

// ValidateDisposableRemovalTree proves that an existing removal target is one real directory tree
// containing only regular files and directories on one filesystem. It rejects linked/reparsed roots,
// descendants, special files, and mount substitutions before a caller performs an authorized reset.
func ValidateDisposableRemovalTree(root string) error {
	_, err := InspectDisposableRemovalTree(root)
	return err
}

// InspectDisposableRemovalTree validates an existing disposable tree and returns its logical size.
func InspectDisposableRemovalTree(root string) (DisposableRemovalTreeSummary, error) {
	var summary DisposableRemovalTreeSummary
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return summary, err
	}
	rootInfo, err := validateDisposableRemovalDirectory(root)
	if err != nil {
		return summary, err
	}
	device, err := durableDeviceIdentity(root)
	if err != nil {
		return summary, err
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if durableFileInfoIsLinkOrReparse(info) {
			return fmt.Errorf("removal target entry %q is linked or reparsed", path)
		}
		if actual, err := durableDeviceIdentity(path); err != nil || actual != device {
			return fmt.Errorf("removal target entry %q crosses a filesystem boundary", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("removal target entry %q is not a regular file or directory", path)
		}
		if info.Mode().IsRegular() {
			summary.LogicalBytes += info.Size()
			summary.Files++
		}
		return nil
	})
	if err != nil {
		return DisposableRemovalTreeSummary{}, err
	}
	if rootInfo == nil {
		return DisposableRemovalTreeSummary{}, errors.New("removal target metadata is unavailable")
	}
	return summary, nil
}
