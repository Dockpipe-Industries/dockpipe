package infrastructure

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
)

const publicPackageStateImportCohort = "public-package-state-v1"

// PackageStateStatus reports compatibility evidence for the public package-state surface.
// PackageOwnedImport means the package manifest owns exact mixed-cohort migration, so the generic
// whole-tree importer deliberately left the legacy scope untouched.
type PackageStateStatus struct {
	Dir                string
	ImportedLegacy     bool
	LegacyDiverged     bool
	PackageOwnedImport bool
}

var publicPackageStateImportTestHook func(string) error

// PreparePackageStateDir resolves public package state to collision-safe durable project/package
// storage. Unknown third-party scopes import their validated legacy tree whole. Maintained mixed
// packages declare package-owned migration aliases in package.yml and continue to select exact
// durable cohorts through their package code.
func PreparePackageStateDir(workdir, ownerID string) (PackageStateStatus, error) {
	manifest := strings.TrimSpace(os.Getenv("DOCKPIPE_PACKAGE_MANIFEST"))
	if manifest == "" {
		if root := strings.TrimSpace(os.Getenv("DOCKPIPE_PACKAGE_ROOT")); root != "" {
			manifest = filepath.Join(root, PackageManifestFilename)
		}
	}
	return PreparePackageStateDirWithManifests(workdir, ownerID, manifest)
}

// PreparePackageStateDirWithManifests accepts manifests already selected by validated package or
// workflow context. The engine understands only this generic declaration and exact owner IDs.
func PreparePackageStateDirWithManifests(workdir, ownerID string, manifests ...string) (PackageStateStatus, error) {
	ownerID, _, _, err := durableOwnerStorageIdentity(ownerID)
	if err != nil {
		return PackageStateStatus{}, err
	}
	packageOwned, err := packageOwnsCompatibilityImport(workdir, ownerID, manifests...)
	if err != nil {
		return PackageStateStatus{}, err
	}
	stateRoot, err := DurableStateRoot()
	if err != nil {
		return PackageStateStatus{}, err
	}
	location, err := durablePackageLocationAt(workdir, ownerID, stateRoot)
	if err != nil {
		return PackageStateStatus{}, err
	}
	lock, err := openPrivateLockFile(location.stateRoot, filepath.Join(location.projectRoot, ".package-"+location.ownerDigest+".lock"))
	if err != nil {
		return PackageStateStatus{}, fmt.Errorf("public package-state lock: %w", err)
	}
	defer lock.Close()
	if err := lockPrivateFile(lock); err != nil {
		return PackageStateStatus{}, fmt.Errorf("public package-state lock: %w", err)
	}
	defer unlockPrivateFile(lock)

	if packageOwned {
		if err := ensureDurablePackageLocation(location); err != nil {
			return PackageStateStatus{}, err
		}
		return PackageStateStatus{Dir: location.packageRoot, PackageOwnedImport: true}, nil
	}
	return prepareWholePublicPackageState(workdir, location)
}

func packageOwnsCompatibilityImport(workdir, ownerID string, explicitManifests ...string) (bool, error) {
	roots := WorkflowCompileRootsCached(workdir)
	seenRoots := map[string]bool{}
	seenManifests := map[string]bool{}
	owned := false
	inspectManifest := func(path string) error {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil
		}
		path, err := filepath.Abs(filepath.Clean(path))
		if err != nil || seenManifests[path] {
			return err
		}
		seenManifests[path] = true
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("package-state policy manifest %q is linked or not a regular file", path)
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil || !sameDurablePath(path, canonical) {
			return fmt.Errorf("package-state policy manifest %q has a linked or reparsed boundary", path)
		}
		manifest, err := domain.ParsePackageManifest(path)
		if err != nil {
			return err
		}
		if manifest.PackageState.CompatibilityImport != "package-owned" {
			return nil
		}
		for _, declared := range manifest.PackageState.OwnerIDs {
			if declared == ownerID {
				owned = true
				break
			}
		}
		return nil
	}
	for _, manifest := range explicitManifests {
		if err := inspectManifest(manifest); err != nil {
			return false, fmt.Errorf("discover explicit package-owned state policy: %w", err)
		}
	}
	for _, root := range roots {
		root, err := filepath.Abs(filepath.Clean(root))
		if err != nil || seenRoots[root] {
			continue
		}
		seenRoots[root] = true
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return false, err
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() || entry.Name() != PackageManifestFilename {
				return nil
			}
			return inspectManifest(path)
		})
		if err != nil {
			return false, fmt.Errorf("discover package-owned state policy: %w", err)
		}
	}
	return owned, nil
}

func prepareWholePublicPackageState(workdir string, location durablePackageLocation) (PackageStateStatus, error) {
	manifest, legacyRoot, err := inspectWholePublicPackageLegacy(workdir, location)
	if err != nil {
		return PackageStateStatus{}, err
	}
	if _, statErr := os.Lstat(location.packageRoot); statErr == nil {
		if err := validateDurablePackageLocation(location); err != nil {
			return PackageStateStatus{}, err
		}
		published, err := validateWholePublicPackageState(location)
		if errors.Is(err, os.ErrNotExist) {
			contents, inspectErr := collectWholePublicPackagePublished(location.packageRoot)
			if inspectErr != nil {
				return PackageStateStatus{}, inspectErr
			}
			if len(contents) != 0 || manifest.Source == "legacy" {
				return PackageStateStatus{}, errors.New("durable package state exists without public compatibility provenance")
			}
			if err := writeDurableImportManifest(location.stateRoot, filepath.Join(location.packageRoot, durableImportManifestName), manifest); err != nil {
				return PackageStateStatus{}, err
			}
			published = manifest
		} else if err != nil {
			return PackageStateStatus{}, err
		}
		diverged := manifest.Source == "legacy" && !sameDurableImportSource(published, manifest)
		return PackageStateStatus{Dir: location.packageRoot, LegacyDiverged: diverged}, nil
	} else if !os.IsNotExist(statErr) {
		return PackageStateStatus{}, statErr
	}

	resumed, err := recoverWholePublicPackageTemporary(location, manifest)
	if err != nil {
		return PackageStateStatus{}, err
	}
	if !resumed {
		if err := publishWholePublicPackageState(workdir, location, legacyRoot, manifest); err != nil {
			return PackageStateStatus{}, err
		}
	}
	return PackageStateStatus{Dir: location.packageRoot, ImportedLegacy: manifest.Source == "legacy" && len(manifest.Inventory) != 0}, nil
}

func publicPackageImportSpec(location durablePackageLocation, legacyPath string) validatedDurableImportSpec {
	return validatedDurableImportSpec{
		DurableCohortImportSpec: DurableCohortImportSpec{OwnerID: location.ownerID, Cohort: publicPackageStateImportCohort},
		instanceID:              location.ownerID,
		legacySourcePath:        legacyPath,
	}
}

func inspectWholePublicPackageLegacy(workdir string, location durablePackageLocation) (durableImportManifest, string, error) {
	legacyRoot, found, err := DiscoverLegacyPackageState(workdir, location.ownerID)
	if err != nil {
		return durableImportManifest{}, "", fmt.Errorf("public package-state legacy discovery: %w", err)
	}
	legacyPath := filepath.ToSlash(filepath.Join(DockpipeDirRel, "packages", SanitizePackageStateScope(location.ownerID)))
	spec := publicPackageImportSpec(location, legacyPath)
	if !found {
		return newDurableImportManifest("none", "", "", nil, spec), legacyRoot, nil
	}
	identity, err := durableFileIdentity(legacyRoot)
	if err != nil {
		return durableImportManifest{}, "", err
	}
	entries, err := collectWholePublicPackageLegacy(legacyRoot)
	if err != nil {
		return durableImportManifest{}, "", err
	}
	return newDurableImportManifest("legacy", legacyPath, identity, entries, spec), legacyRoot, nil
}

func collectWholePublicPackageLegacy(root string) ([]durableImportEntry, error) {
	device, err := durableDeviceIdentity(root)
	if err != nil {
		return nil, err
	}
	entries := []durableImportEntry{}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == durablePackageMetaFile || rel == durableImportManifestName || rel == durableImportPendingName {
			return fmt.Errorf("legacy package state contains reserved path %q", rel)
		}
		if durableFileInfoIsLinkOrReparse(info) {
			return fmt.Errorf("legacy package-state entry %q is linked or reparsed", rel)
		}
		if err := requireDurableImportDevice(path, device); err != nil {
			return err
		}
		identity, err := durableFileIdentity(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			entries = append(entries, durableImportEntry{Path: rel, SourcePath: rel, Type: "directory", SourceIdentity: identity})
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("legacy package-state entry %q is not a regular file", rel)
		}
		entry, err := readDurableImportFile(path, rel, rel, device)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func publishWholePublicPackageState(workdir string, location durablePackageLocation, legacyRoot string, manifest durableImportManifest) error {
	packagesRoot := filepath.Dir(location.packageRoot)
	if err := ensurePrivateSubdirectory(location.stateRoot, packagesRoot); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(packagesRoot, "."+location.ownerDigest+".public-import-")
	if err != nil {
		return err
	}
	if err := makePrivatePath(temporary, true); err != nil {
		return err
	}
	temporaryLocation := location
	temporaryLocation.packageRoot = temporary
	if err := validateOrWriteDurablePackageMetadata(temporaryLocation, true); err != nil {
		return err
	}
	pending := manifest
	pending.Phase = "copying"
	if err := writeDurableImportManifest(location.stateRoot, filepath.Join(temporary, durableImportPendingName), pending); err != nil {
		return err
	}
	for _, entry := range manifest.Inventory {
		if err := copyDurableImportEntry(temporary, legacyRoot, entry); err != nil {
			return err
		}
	}
	if err := runPublicPackageStateImportHook("after-copy"); err != nil {
		return err
	}
	current, _, err := inspectWholePublicPackageLegacy(workdir, location)
	if err != nil || !sameDurableImportSource(manifest, current) {
		return errors.New("legacy package-state source changed during migration")
	}
	observed, err := collectWholePublicPackagePublished(temporary)
	if err != nil || !sameDurableImportInventory(durableImportDestinationInventory(manifest.Inventory), observed) {
		return errors.New("durable package-state inventory does not match its legacy source")
	}
	if err := writeDurableImportManifest(location.stateRoot, filepath.Join(temporary, durableImportManifestName), manifest); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(temporary, durableImportPendingName)); err != nil {
		return err
	}
	if err := syncDurableImportDirectories(temporary); err != nil {
		return err
	}
	if err := runPublicPackageStateImportHook("before-rename"); err != nil {
		return err
	}
	if _, err := os.Lstat(location.packageRoot); err == nil {
		return errors.New("durable package-state destination was substituted before publication")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temporary, location.packageRoot); err != nil {
		return fmt.Errorf("publish durable package state: %w", err)
	}
	if err := syncPrivateDirectory(packagesRoot); err != nil {
		return err
	}
	return runPublicPackageStateImportHook("after-rename")
}

func recoverWholePublicPackageTemporary(location durablePackageLocation, source durableImportManifest) (bool, error) {
	packagesRoot := filepath.Dir(location.packageRoot)
	if _, err := os.Lstat(packagesRoot); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(packagesRoot)
	if err != nil {
		return false, err
	}
	prefix := "." + location.ownerDigest + ".public-import-"
	temporary := ""
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			if temporary != "" {
				return false, errors.New("multiple public package-state temporaries require manual review")
			}
			temporary = filepath.Join(packagesRoot, entry.Name())
		}
	}
	if temporary == "" {
		return false, nil
	}
	if err := validatePrivatePath(temporary, true); err != nil {
		return false, fmt.Errorf("public package-state temporary is unsafe: %w", err)
	}
	if err := validateExistingStateBoundary(location.stateRoot, temporary); err != nil {
		return false, err
	}
	temporaryLocation := location
	temporaryLocation.packageRoot = temporary
	if err := validateDurablePackageLocation(temporaryLocation); err != nil {
		return false, err
	}
	spec := publicPackageImportSpec(location, source.SourcePath)
	pending, pendingErr := readDurableImportManifest(location.stateRoot, filepath.Join(temporary, durableImportPendingName), "copying", spec)
	ready, readyErr := readDurableImportManifest(location.stateRoot, filepath.Join(temporary, durableImportManifestName), "authoritative", spec)
	if pendingErr != nil && !errors.Is(pendingErr, os.ErrNotExist) {
		return false, fmt.Errorf("public package-state pending manifest is malformed: %w", pendingErr)
	}
	if readyErr != nil && !errors.Is(readyErr, os.ErrNotExist) {
		return false, fmt.Errorf("public package-state ready manifest is malformed: %w", readyErr)
	}
	if errors.Is(pendingErr, os.ErrNotExist) && errors.Is(readyErr, os.ErrNotExist) {
		return false, errors.New("public package-state temporary has no byte-proving manifest")
	}
	for _, candidate := range []durableImportManifest{pending, ready} {
		if candidate.Schema != 0 && !sameDurableImportSource(source, candidate) {
			return false, errors.New("public package-state temporary source is no longer authoritative")
		}
	}
	observed, err := collectWholePublicPackagePublished(temporary)
	if err != nil {
		return false, err
	}
	if ready.Schema != 0 {
		if !sameDurableImportInventory(durableImportDestinationInventory(source.Inventory), observed) {
			return false, errors.New("public package-state ready temporary does not match its source")
		}
		if pending.Schema != 0 {
			if err := os.Remove(filepath.Join(temporary, durableImportPendingName)); err != nil {
				return false, err
			}
		}
		if _, err := os.Lstat(location.packageRoot); err == nil {
			return false, errors.New("public package-state destination was substituted before recovery")
		} else if !os.IsNotExist(err) {
			return false, err
		}
		if err := os.Rename(temporary, location.packageRoot); err != nil {
			return false, err
		}
		if err := syncPrivateDirectory(packagesRoot); err != nil {
			return false, err
		}
		return true, nil
	}
	if !durableImportInventorySubset(observed, source.Inventory) {
		return false, errors.New("public package-state incomplete temporary is not a byte-proven source subset")
	}
	if err := os.RemoveAll(temporary); err != nil {
		return false, err
	}
	if err := syncPrivateDirectory(packagesRoot); err != nil {
		return false, err
	}
	return false, nil
}

func validateWholePublicPackageState(location durablePackageLocation) (durableImportManifest, error) {
	manifestPath := filepath.Join(location.packageRoot, durableImportManifestName)
	spec := publicPackageImportSpec(location, filepath.ToSlash(filepath.Join(DockpipeDirRel, "packages", SanitizePackageStateScope(location.ownerID))))
	manifest, err := readDurableImportManifest(location.stateRoot, manifestPath, "authoritative", spec)
	if err != nil {
		return durableImportManifest{}, err
	}
	observed, err := collectWholePublicPackagePublished(location.packageRoot)
	if err != nil {
		return durableImportManifest{}, err
	}
	if !sameDurableImportInventory(durableImportDestinationInventory(manifest.Inventory), observed) {
		return durableImportManifest{}, errors.New("durable public package-state inventory does not match provenance")
	}
	return manifest, nil
}

func collectWholePublicPackagePublished(root string) ([]durableImportEntry, error) {
	entries, err := collectPublishedDurableImportTree(root, true)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, entry := range entries {
		if entry.Path == durablePackageMetaFile {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func runPublicPackageStateImportHook(stage string) error {
	if publicPackageStateImportTestHook == nil {
		return nil
	}
	return publicPackageStateImportTestHook(stage)
}
