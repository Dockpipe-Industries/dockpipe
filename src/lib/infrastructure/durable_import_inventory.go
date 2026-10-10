package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func inspectDurableImportLegacy(workdir string, spec validatedDurableImportSpec) (durableImportManifest, error) {
	stateRoot, err := StateRoot(workdir)
	if err != nil {
		return durableImportManifest{}, err
	}
	stateRoot, err = filepath.Abs(filepath.Clean(stateRoot))
	if err != nil {
		return durableImportManifest{}, err
	}
	info, err := os.Lstat(spec.LegacyRoot)
	if os.IsNotExist(err) {
		return newDurableImportManifest("none", "", "", nil, spec), nil
	}
	if err != nil {
		return durableImportManifest{}, err
	}
	if !info.IsDir() || durableFileInfoIsLinkOrReparse(info) {
		return durableImportManifest{}, errors.New("legacy cohort root is linked, reparsed, or not a directory")
	}
	if err := validateExistingStateBoundary(stateRoot, spec.LegacyRoot); err != nil {
		return durableImportManifest{}, fmt.Errorf("legacy cohort boundary: %w", err)
	}
	rootIdentity, err := durableFileIdentity(spec.LegacyRoot)
	if err != nil {
		return durableImportManifest{}, err
	}
	entries, err := collectDurableImportEntries(spec)
	if err != nil {
		return durableImportManifest{}, err
	}
	return newDurableImportManifest("legacy", spec.legacySourcePath, rootIdentity, entries, spec), nil
}

func collectDurableImportEntries(spec validatedDurableImportSpec) ([]durableImportEntry, error) {
	device, err := durableDeviceIdentity(spec.LegacyRoot)
	if err != nil {
		return nil, err
	}
	entries := []durableImportEntry{}
	for _, mapping := range spec.Mappings {
		source := filepath.Join(spec.LegacyRoot, filepath.FromSlash(mapping.Source))
		info, err := os.Lstat(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if mapping.Tree {
			if !info.IsDir() || durableFileInfoIsLinkOrReparse(info) {
				return nil, fmt.Errorf("legacy cohort tree %q is linked, reparsed, or not a directory", mapping.Source)
			}
			if err := requireDurableImportDevice(source, device); err != nil {
				return nil, err
			}
			if err := collectDurableImportTree(&entries, spec, mapping, "", device); err != nil {
				return nil, err
			}
			continue
		}
		if !info.Mode().IsRegular() || durableFileInfoIsLinkOrReparse(info) {
			return nil, fmt.Errorf("legacy cohort file %q is linked, reparsed, or not regular", mapping.Source)
		}
		entry, err := readDurableImportFile(source, mapping.Source, mapping.Destination, device)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	for index := 1; index < len(entries); index++ {
		if entries[index-1].Path == entries[index].Path {
			return nil, errors.New("durable cohort inventory contains duplicate destination paths")
		}
	}
	return entries, nil
}

func collectDurableImportTree(entries *[]durableImportEntry, spec validatedDurableImportSpec, mapping DurableImportMapping, treeRel, device string) error {
	sourceRel := mapping.Source
	destinationRel := mapping.Destination
	if treeRel != "" {
		sourceRel = filepath.ToSlash(filepath.Join(mapping.Source, treeRel))
		destinationRel = filepath.ToSlash(filepath.Join(mapping.Destination, treeRel))
	}
	if spec.ignore[sourceRel] {
		return nil
	}
	source := filepath.Join(spec.LegacyRoot, filepath.FromSlash(sourceRel))
	info, err := os.Lstat(source)
	if err != nil || durableFileInfoIsLinkOrReparse(info) {
		return fmt.Errorf("legacy cohort tree entry %q is substituted", sourceRel)
	}
	if err := requireDurableImportDevice(source, device); err != nil {
		return err
	}
	identity, err := durableFileIdentity(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		*entries = append(*entries, durableImportEntry{Path: destinationRel, SourcePath: sourceRel, Type: "directory", SourceIdentity: identity})
		children, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, child := range children {
			childRel := child.Name()
			if treeRel != "" {
				childRel = filepath.Join(treeRel, child.Name())
			}
			if err := collectDurableImportTree(entries, spec, mapping, childRel, device); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("legacy cohort tree entry %q is not a regular file", sourceRel)
	}
	entry, err := readDurableImportFile(source, sourceRel, destinationRel, device)
	if err != nil {
		return err
	}
	*entries = append(*entries, entry)
	return nil
}

func readDurableImportFile(path, sourceRel, destinationRel, device string) (durableImportEntry, error) {
	if err := requireDurableImportDevice(path, device); err != nil {
		return durableImportEntry{}, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || durableFileInfoIsLinkOrReparse(before) {
		return durableImportEntry{}, fmt.Errorf("legacy cohort file %q is substituted", sourceRel)
	}
	identity, err := durableFileIdentity(path)
	if err != nil {
		return durableImportEntry{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return durableImportEntry{}, err
	}
	hash := sha256.New()
	size, readErr := io.Copy(hash, file)
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if readErr != nil || statErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(before, after) || size != opened.Size() {
		return durableImportEntry{}, fmt.Errorf("legacy cohort file %q changed while reading", sourceRel)
	}
	return durableImportEntry{Path: destinationRel, SourcePath: sourceRel, Type: "file", Size: size, SHA256: hex.EncodeToString(hash.Sum(nil)), SourceIdentity: identity}, nil
}

func requireDurableImportDevice(path, expected string) error {
	actual, err := durableDeviceIdentity(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("durable import path %q crosses a filesystem boundary", path)
	}
	return nil
}

func newDurableImportManifest(source, sourcePath, sourceIdentity string, entries []durableImportEntry, spec validatedDurableImportSpec) durableImportManifest {
	if entries == nil {
		entries = []durableImportEntry{}
	}
	copied := append([]durableImportEntry(nil), entries...)
	return durableImportManifest{Schema: durableImportSchema, Cohort: spec.Cohort, InstanceID: spec.instanceID, Phase: "authoritative", Source: source, SourcePath: sourcePath, SourceIdentity: sourceIdentity, InventorySHA256: durableImportInventoryDigest(copied), Inventory: copied}
}
