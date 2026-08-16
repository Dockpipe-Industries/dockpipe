package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	durableImportSchema        = 1
	durableImportManifestName  = ".dockpipe-durable-import.json"
	durableImportPendingName   = ".dockpipe-durable-import.pending.json"
	durableImportManifestLimit = 16 << 20
)

// DurableImportMapping selects one optional legacy regular file or directory tree and maps it to
// a collision-safe durable cohort destination. Source and Destination are relative paths.
type DurableImportMapping struct {
	Source      string
	Destination string
	Tree        bool
}

// DurableCohortImportSpec describes one package-owned durable cohort. The generic importer owns
// copying and publication; the package caller remains responsible for the exact cohort selection.
type DurableCohortImportSpec struct {
	OwnerID     string
	Cohort      string
	InstanceID  string
	RunID       string
	LegacyRoot  string
	Mappings    []DurableImportMapping
	IgnorePaths []string
}

type DurableCohortImportStatus struct {
	DurableDir           string
	RuntimeDir           string
	DurableAuthoritative bool
	ImportedLegacy       bool
	LegacyDiverged       bool
}

type durableImportEntry struct {
	Path           string `json:"path"`
	SourcePath     string `json:"source_path"`
	Type           string `json:"type"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256,omitempty"`
	SourceIdentity string `json:"source_identity"`
}

type durableImportManifest struct {
	Schema          int                  `json:"schema"`
	Cohort          string               `json:"cohort"`
	InstanceID      string               `json:"instance_id"`
	Phase           string               `json:"phase"`
	Source          string               `json:"source"`
	SourcePath      string               `json:"source_path,omitempty"`
	SourceIdentity  string               `json:"source_identity,omitempty"`
	InventorySHA256 string               `json:"inventory_sha256"`
	Inventory       []durableImportEntry `json:"inventory"`
}

var durableCohortImportTestHook func(string) error

// PrepareDurableCohortImport atomically imports a package-selected legacy cohort into durable
// project/package state and returns a separate disposable per-run directory. Legacy bytes are
// never rewritten or removed.
func PrepareDurableCohortImport(workdir string, spec DurableCohortImportSpec) (DurableCohortImportStatus, error) {
	validated, err := validateDurableImportSpec(workdir, spec)
	if err != nil {
		return DurableCohortImportStatus{}, err
	}
	projectRoot, err := ProjectStateRoot(workdir)
	if err != nil {
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort project owner: %w", err)
	}
	packageCreationLock, err := openPrivateLockFile(projectRoot, filepath.Join(projectRoot, ".durable-cohort-package.lock"))
	if err != nil {
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort package lock: %w", err)
	}
	if err := lockPrivateFile(packageCreationLock); err != nil {
		packageCreationLock.Close()
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort package lock: %w", err)
	}
	packageRoot, err := ProjectPackageStateDir(workdir, validated.OwnerID)
	unlockPrivateFile(packageCreationLock)
	packageCreationLock.Close()
	if err != nil {
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort package owner: %w", err)
	}
	cohortRoot := filepath.Join(packageRoot, "cohorts", validated.cohortSegment, "instances")
	if err := ensurePrivateSubdirectory(packageRoot, cohortRoot); err != nil {
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort root: %w", err)
	}
	destination := filepath.Join(cohortRoot, validated.instanceSegment)
	runtimeDir, err := prepareDurableImportRuntimeDir(workdir, validated)
	if err != nil {
		return DurableCohortImportStatus{}, err
	}
	status := DurableCohortImportStatus{DurableDir: destination, RuntimeDir: runtimeDir}

	lockPath := filepath.Join(cohortRoot, "."+validated.instanceDigest+".lock")
	lock, err := openPrivateLockFile(packageRoot, lockPath)
	if err != nil {
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort lock: %w", err)
	}
	defer lock.Close()
	if err := lockPrivateFile(lock); err != nil {
		return DurableCohortImportStatus{}, fmt.Errorf("durable cohort lock: %w", err)
	}
	defer unlockPrivateFile(lock)

	if _, statErr := os.Lstat(destination); statErr == nil {
		manifest, err := validatePublishedDurableImport(packageRoot, destination, validated)
		if err != nil {
			return DurableCohortImportStatus{}, err
		}
		diverged, err := durableImportLegacyDiverged(workdir, manifest, validated)
		if err != nil {
			return DurableCohortImportStatus{}, err
		}
		status.DurableAuthoritative = true
		status.LegacyDiverged = diverged
		return status, nil
	} else if !os.IsNotExist(statErr) {
		return DurableCohortImportStatus{}, statErr
	}

	manifest, err := inspectDurableImportLegacy(workdir, validated)
	if err != nil {
		return DurableCohortImportStatus{}, err
	}
	if err := runDurableImportHook("after-source-inventory"); err != nil {
		return DurableCohortImportStatus{}, err
	}
	resumed, err := recoverDurableImportTemporary(packageRoot, cohortRoot, destination, manifest, validated)
	if err != nil {
		return DurableCohortImportStatus{}, err
	}
	if !resumed {
		if err := publishDurableImport(workdir, packageRoot, cohortRoot, destination, manifest, validated); err != nil {
			return DurableCohortImportStatus{}, err
		}
	}
	status.DurableAuthoritative = true
	status.ImportedLegacy = manifest.Source == "legacy" && len(manifest.Inventory) > 0
	return status, nil
}

type validatedDurableImportSpec struct {
	DurableCohortImportSpec
	cohortSegment    string
	instanceID       string
	instanceSegment  string
	instanceDigest   string
	runSegment       string
	legacySourcePath string
	ignore           map[string]bool
}

func validateDurableImportSpec(workdir string, spec DurableCohortImportSpec) (validatedDurableImportSpec, error) {
	if _, _, _, err := durableOwnerStorageIdentity(spec.OwnerID); err != nil {
		return validatedDurableImportSpec{}, err
	}
	cohort, cohortSegment, _, err := durableOwnerStorageIdentity(spec.Cohort)
	if err != nil {
		return validatedDurableImportSpec{}, fmt.Errorf("durable cohort identity: %w", err)
	}
	instanceID, instanceSegment, instanceDigest, err := durableImportStorageIdentity(spec.InstanceID, "instance")
	if err != nil {
		return validatedDurableImportSpec{}, err
	}
	_, runSegment, _, err := durableImportStorageIdentity(spec.RunID, "run")
	if err != nil {
		return validatedDurableImportSpec{}, err
	}
	legacyRoot, err := filepath.Abs(filepath.Clean(spec.LegacyRoot))
	if err != nil {
		return validatedDurableImportSpec{}, err
	}
	stateRoot, err := StateRoot(workdir)
	if err != nil {
		return validatedDurableImportSpec{}, err
	}
	stateRoot, err = filepath.Abs(filepath.Clean(stateRoot))
	if err != nil {
		return validatedDurableImportSpec{}, err
	}
	if !sameOrWithinDurablePath(stateRoot, legacyRoot) || sameDurablePath(stateRoot, legacyRoot) {
		legacyRoot, err = resolveLegacyPackageStateToken(workdir, legacyRoot)
		if err != nil {
			return validatedDurableImportSpec{}, err
		}
	}

	validated := validatedDurableImportSpec{
		DurableCohortImportSpec: spec,
		cohortSegment:           cohortSegment,
		instanceID:              instanceID,
		instanceSegment:         instanceSegment,
		instanceDigest:          instanceDigest,
		runSegment:              runSegment,
		ignore:                  map[string]bool{},
	}
	validated.Mappings = append([]DurableImportMapping(nil), spec.Mappings...)
	validated.IgnorePaths = append([]string(nil), spec.IgnorePaths...)
	legacySourcePath, err := filepath.Rel(stateRoot, legacyRoot)
	if err != nil {
		return validatedDurableImportSpec{}, err
	}
	validated.legacySourcePath = filepath.ToSlash(legacySourcePath)
	validated.Cohort = cohort
	validated.LegacyRoot = legacyRoot
	seenSource, seenDestination := map[string]bool{}, map[string]bool{}
	for index := range validated.Mappings {
		source, err := validateStateSuffixForOS("linux", filepath.ToSlash(validated.Mappings[index].Source))
		if err != nil {
			return validatedDurableImportSpec{}, fmt.Errorf("legacy mapping source: %w", err)
		}
		destination, err := validateStateSuffixForOS("linux", filepath.ToSlash(validated.Mappings[index].Destination))
		if err != nil {
			return validatedDurableImportSpec{}, fmt.Errorf("durable mapping destination: %w", err)
		}
		if seenSource[source] || seenDestination[destination] {
			return validatedDurableImportSpec{}, errors.New("durable import mappings contain a duplicate source or destination")
		}
		seenSource[source], seenDestination[destination] = true, true
		validated.Mappings[index].Source = source
		validated.Mappings[index].Destination = destination
	}
	for left := range validated.Mappings {
		for right := left + 1; right < len(validated.Mappings); right++ {
			leftSource := validated.Mappings[left].Source
			rightSource := validated.Mappings[right].Source
			leftDestination := validated.Mappings[left].Destination
			rightDestination := validated.Mappings[right].Destination
			if durableImportPathContains(leftSource, rightSource) || durableImportPathContains(rightSource, leftSource) || durableImportPathContains(leftDestination, rightDestination) || durableImportPathContains(rightDestination, leftDestination) {
				return validatedDurableImportSpec{}, errors.New("durable import mappings overlap")
			}
		}
	}
	for _, raw := range validated.IgnorePaths {
		path, err := validateStateSuffixForOS("linux", filepath.ToSlash(raw))
		if err != nil {
			return validatedDurableImportSpec{}, fmt.Errorf("legacy ignore path: %w", err)
		}
		validated.ignore[path] = true
	}
	return validated, nil
}

// resolveLegacyPackageStateToken lets maintained package importers keep accepting the public
// package-state path after that path becomes durable. The durable package metadata supplies only
// the exact legacy owner ID; discovery remains read-only and never links the two roots.
func resolveLegacyPackageStateToken(workdir, token string) (string, error) {
	stateRoot, err := DurableStateRoot()
	if err != nil {
		return "", err
	}
	stateRoot, err = filepath.Abs(filepath.Clean(stateRoot))
	if err != nil || !sameOrWithinDurablePath(stateRoot, token) || sameDurablePath(stateRoot, token) {
		return "", errors.New("legacy cohort root must be below the resolved project runtime root or equal a durable public package-state root")
	}
	if err := validatePrivatePath(token, true); err != nil {
		return "", fmt.Errorf("durable public package-state token: %w", err)
	}
	var metadata durablePackageMetadata
	if err := readPrivateJSON(stateRoot, filepath.Join(token, durablePackageMetaFile), &metadata); err != nil {
		return "", fmt.Errorf("durable public package-state token metadata: %w", err)
	}
	if metadata.Schema != durableStateSchema {
		return "", errors.New("durable public package-state token metadata schema is invalid")
	}
	location, err := durablePackageLocationAt(workdir, metadata.OwnerID, stateRoot)
	if err != nil {
		return "", err
	}
	if !sameDurablePath(location.packageRoot, token) || metadata.OwnerDigest != location.ownerDigest {
		return "", errors.New("durable public package-state token does not match this project and owner")
	}
	legacyRoot, _, err := DiscoverLegacyPackageState(workdir, metadata.OwnerID)
	if err != nil {
		return "", err
	}
	return legacyRoot, nil
}

func durableImportStorageIdentity(value, label string) (canonical, segment, digest string, err error) {
	canonical = strings.TrimSpace(value)
	if canonical == "" || !utf8.ValidString(canonical) || strings.IndexByte(canonical, 0) >= 0 {
		return "", "", "", fmt.Errorf("durable cohort %s identity is empty or invalid", label)
	}
	for _, character := range canonical {
		if character < 0x20 || character == 0x7f {
			return "", "", "", fmt.Errorf("durable cohort %s identity contains control characters", label)
		}
	}
	sum := sha256.Sum256([]byte(canonical))
	digest = hex.EncodeToString(sum[:])
	slug := durableOwnerSlug(filepath.Base(canonical))
	return canonical, slug + "-" + digest, digest, nil
}

func prepareDurableImportRuntimeDir(workdir string, spec validatedDurableImportSpec) (string, error) {
	root, err := PackageRuntimeDir(workdir, spec.OwnerID)
	if err != nil {
		return "", err
	}
	runtimeDir := filepath.Join(root, "cohorts", spec.cohortSegment, "instances", spec.instanceSegment, "runs", spec.runSegment)
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		if err := ensurePrivateRoot(root); err != nil {
			return "", fmt.Errorf("durable cohort runtime owner: %w", err)
		}
	} else if err != nil {
		return "", err
	} else if err := validatePrivatePath(root, true); err != nil {
		return "", fmt.Errorf("durable cohort runtime owner: %w", err)
	}
	stateRoot, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	if err := validateExistingStateBoundary(stateRoot, root); err != nil {
		return "", fmt.Errorf("durable cohort runtime owner: %w", err)
	}
	if err := ensurePrivateImportSubdirectory(root, runtimeDir); err != nil {
		return "", fmt.Errorf("durable cohort runtime directory: %w", err)
	}
	if err := validatePrivatePath(runtimeDir, true); err != nil {
		return "", fmt.Errorf("durable cohort runtime directory: %w", err)
	}
	return runtimeDir, nil
}

func ensurePrivateImportSubdirectory(root, destination string) error {
	if !sameOrWithinDurablePath(root, destination) {
		return errors.New("private import directory escapes its root")
	}
	device, err := durableDeviceIdentity(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, destination)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if _, err := os.Lstat(current); os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o700); err != nil && !os.IsExist(err) {
				return err
			}
			if err := makePrivatePath(current, true); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := validatePrivatePath(current, true); err != nil {
			return err
		}
		if err := requireDurableImportDevice(current, device); err != nil {
			return err
		}
	}
	return nil
}

func durableImportPathContains(parent, child string) bool {
	rel, err := filepath.Rel(filepath.FromSlash(parent), filepath.FromSlash(child))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func publishDurableImport(workdir, packageRoot, cohortRoot, destination string, manifest durableImportManifest, spec validatedDurableImportSpec) error {
	temporary, err := os.MkdirTemp(cohortRoot, "."+spec.instanceDigest+".import-")
	if err != nil {
		return err
	}
	if err := makePrivatePath(temporary, true); err != nil {
		return err
	}
	pending := manifest
	pending.Phase = "copying"
	if err := writeDurableImportManifest(packageRoot, filepath.Join(temporary, durableImportPendingName), pending); err != nil {
		return err
	}
	if manifest.Source == "legacy" {
		for _, entry := range manifest.Inventory {
			if err := copyDurableImportEntry(temporary, spec.LegacyRoot, entry); err != nil {
				return err
			}
		}
	}
	if err := runDurableImportHook("after-copy"); err != nil {
		return err
	}
	current, err := inspectDurableImportLegacy(workdir, spec)
	if err != nil || !sameDurableImportSource(manifest, current) {
		return errors.New("legacy cohort source changed during migration")
	}
	observed, err := collectPublishedDurableImportTree(temporary, true)
	if err != nil || !sameDurableImportInventory(durableImportDestinationInventory(manifest.Inventory), observed) {
		return errors.New("durable cohort copied inventory does not match its legacy source")
	}
	if err := writeDurableImportManifest(packageRoot, filepath.Join(temporary, durableImportManifestName), manifest); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(temporary, durableImportPendingName)); err != nil {
		return err
	}
	if err := syncDurableImportDirectories(temporary); err != nil {
		return err
	}
	if err := runDurableImportHook("before-rename"); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("durable cohort destination was substituted before publication")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("publish durable cohort: %w", err)
	}
	if err := syncPrivateDirectory(cohortRoot); err != nil {
		return err
	}
	return runDurableImportHook("after-rename")
}

func copyDurableImportEntry(temporary, legacyRoot string, entry durableImportEntry) error {
	destination := filepath.Join(temporary, filepath.FromSlash(entry.Path))
	if entry.Type == "directory" {
		return ensurePrivateSubdirectory(temporary, destination)
	}
	if err := ensurePrivateSubdirectory(temporary, filepath.Dir(destination)); err != nil {
		return err
	}
	source := filepath.Join(legacyRoot, filepath.FromSlash(entry.SourcePath))
	before, err := os.Lstat(source)
	if err != nil || !before.Mode().IsRegular() || durableFileInfoIsLinkOrReparse(before) {
		return fmt.Errorf("legacy cohort file %q changed before copy", entry.SourcePath)
	}
	identity, err := durableFileIdentity(source)
	if err != nil || identity != entry.SourceIdentity {
		return fmt.Errorf("legacy cohort file %q was substituted before copy", entry.SourcePath)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return fmt.Errorf("legacy cohort file %q changed while opening", entry.SourcePath)
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(output, hash), input)
	if copyErr == nil {
		copyErr = output.Sync()
	}
	if closeErr := output.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}
	if err := makePrivatePath(destination, false); err != nil {
		return err
	}
	final, finalErr := input.Stat()
	after, err := os.Lstat(source)
	if err != nil || finalErr != nil || !os.SameFile(opened, final) || !os.SameFile(before, after) || written != entry.Size || written != final.Size() || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("legacy cohort file %q changed while copying", entry.SourcePath)
	}
	return nil
}

func recoverDurableImportTemporary(packageRoot, cohortRoot, destination string, source durableImportManifest, spec validatedDurableImportSpec) (bool, error) {
	entries, err := os.ReadDir(cohortRoot)
	if err != nil {
		return false, err
	}
	prefix := "." + spec.instanceDigest + ".import-"
	temporary := ""
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			if temporary != "" {
				return false, errors.New("multiple durable cohort temporaries require manual review")
			}
			temporary = filepath.Join(cohortRoot, entry.Name())
		}
	}
	if temporary == "" {
		return false, nil
	}
	if err := validatePrivatePath(temporary, true); err != nil {
		return false, fmt.Errorf("durable cohort temporary is unsafe: %w", err)
	}
	if err := validateExistingStateBoundary(packageRoot, temporary); err != nil {
		return false, err
	}
	pending, pendingErr := readDurableImportManifest(packageRoot, filepath.Join(temporary, durableImportPendingName), "copying", spec)
	ready, readyErr := readDurableImportManifest(packageRoot, filepath.Join(temporary, durableImportManifestName), "authoritative", spec)
	if pendingErr != nil && !errors.Is(pendingErr, os.ErrNotExist) {
		return false, fmt.Errorf("durable cohort pending manifest is malformed: %w", pendingErr)
	}
	if readyErr != nil && !errors.Is(readyErr, os.ErrNotExist) {
		return false, fmt.Errorf("durable cohort ready manifest is malformed: %w", readyErr)
	}
	if errors.Is(pendingErr, os.ErrNotExist) && errors.Is(readyErr, os.ErrNotExist) {
		return false, errors.New("durable cohort temporary has no byte-proving manifest")
	}
	for _, candidate := range []durableImportManifest{pending, ready} {
		if candidate.Schema != 0 && !sameDurableImportSource(source, candidate) {
			return false, errors.New("durable cohort temporary source is no longer authoritative")
		}
	}
	observed, err := collectPublishedDurableImportTree(temporary, true)
	if err != nil {
		return false, err
	}
	if ready.Schema != 0 {
		if !sameDurableImportInventory(durableImportDestinationInventory(source.Inventory), observed) {
			return false, errors.New("durable cohort ready temporary does not match its source")
		}
		if pending.Schema != 0 {
			if err := os.Remove(filepath.Join(temporary, durableImportPendingName)); err != nil {
				return false, err
			}
		}
		if _, err := os.Lstat(destination); err == nil {
			return false, errors.New("durable cohort destination was substituted before recovery")
		} else if !os.IsNotExist(err) {
			return false, err
		}
		if err := os.Rename(temporary, destination); err != nil {
			return false, err
		}
		if err := syncPrivateDirectory(cohortRoot); err != nil {
			return false, err
		}
		return true, nil
	}
	if !durableImportInventorySubset(observed, source.Inventory) {
		return false, errors.New("durable cohort incomplete temporary is not a byte-proven source subset")
	}
	if err := os.RemoveAll(temporary); err != nil {
		return false, err
	}
	if err := syncPrivateDirectory(cohortRoot); err != nil {
		return false, err
	}
	return false, nil
}

func validatePublishedDurableImport(packageRoot, destination string, spec validatedDurableImportSpec) (durableImportManifest, error) {
	if err := validateExistingStateBoundary(packageRoot, destination); err != nil {
		return durableImportManifest{}, err
	}
	if _, err := collectPublishedDurableImportTree(destination, true); err != nil {
		return durableImportManifest{}, fmt.Errorf("durable cohort authority is unsafe: %w", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, durableImportPendingName)); err == nil {
		return durableImportManifest{}, errors.New("durable cohort authority contains an incomplete marker")
	} else if !os.IsNotExist(err) {
		return durableImportManifest{}, err
	}
	manifest, err := readDurableImportManifest(packageRoot, filepath.Join(destination, durableImportManifestName), "authoritative", spec)
	if err != nil {
		return durableImportManifest{}, fmt.Errorf("durable cohort provenance: %w", err)
	}
	return manifest, nil
}

func collectPublishedDurableImportTree(root string, private bool) ([]durableImportEntry, error) {
	if private {
		if err := validatePrivatePath(root, true); err != nil {
			return nil, err
		}
	}
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
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if rel == durableImportManifestName || rel == durableImportPendingName {
			return nil
		}
		if durableFileInfoIsLinkOrReparse(info) {
			return fmt.Errorf("durable cohort entry %q is linked or reparsed", rel)
		}
		if err := requireDurableImportDevice(path, device); err != nil {
			return err
		}
		if info.IsDir() {
			if private {
				if err := validatePrivatePath(path, true); err != nil {
					return err
				}
			}
			entries = append(entries, durableImportEntry{Path: rel, Type: "directory"})
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("durable cohort entry %q is not regular", rel)
		}
		if private {
			if err := validatePrivatePath(path, false); err != nil {
				return err
			}
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || size != info.Size() {
			return fmt.Errorf("durable cohort entry %q changed while reading", rel)
		}
		entries = append(entries, durableImportEntry{Path: rel, Type: "file", Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func durableImportLegacyDiverged(workdir string, manifest durableImportManifest, spec validatedDurableImportSpec) (bool, error) {
	current, err := inspectDurableImportLegacy(workdir, spec)
	if err != nil {
		return false, err
	}
	if current.Source == "none" {
		return false, nil
	}
	return !sameDurableImportSource(manifest, current), nil
}

func writeDurableImportManifest(root, path string, manifest durableImportManifest) error {
	raw, err := encodeDurableImportManifest(manifest)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := makePrivatePath(path, false); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func readDurableImportManifest(root, path, phase string, spec validatedDurableImportSpec) (durableImportManifest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return durableImportManifest{}, err
	}
	if !info.Mode().IsRegular() || durableFileInfoIsLinkOrReparse(info) || info.Size() <= 0 || info.Size() > durableImportManifestLimit {
		return durableImportManifest{}, errors.New("durable import manifest is linked, special, empty, or oversized")
	}
	if err := validateExistingStateBoundary(root, path); err != nil {
		return durableImportManifest{}, err
	}
	if err := validatePrivatePath(path, false); err != nil {
		return durableImportManifest{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return durableImportManifest{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return durableImportManifest{}, errors.New("durable import manifest changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, durableImportManifestLimit+1))
	if err != nil || len(raw) > durableImportManifestLimit {
		return durableImportManifest{}, errors.New("durable import manifest read is invalid")
	}
	final, err := file.Stat()
	after, afterErr := os.Lstat(path)
	if err != nil || afterErr != nil || !os.SameFile(opened, final) || !os.SameFile(info, after) || int64(len(raw)) != final.Size() {
		return durableImportManifest{}, errors.New("durable import manifest changed while reading")
	}
	var manifest durableImportManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return durableImportManifest{}, errors.New("durable import manifest JSON is malformed")
	}
	if err := validateDurableImportManifest(manifest, phase, spec); err != nil {
		return durableImportManifest{}, err
	}
	canonical, err := encodeDurableImportManifest(manifest)
	if err != nil || !bytes.Equal(raw, canonical) {
		return durableImportManifest{}, errors.New("durable import manifest is not canonical")
	}
	return manifest, nil
}

func validateDurableImportManifest(manifest durableImportManifest, phase string, spec validatedDurableImportSpec) error {
	if manifest.Schema != durableImportSchema || manifest.Cohort != spec.Cohort || manifest.InstanceID != spec.instanceID || manifest.Phase != phase {
		return errors.New("durable import manifest identity is invalid")
	}
	if manifest.Source != "none" && manifest.Source != "legacy" {
		return errors.New("durable import manifest source is invalid")
	}
	if manifest.Source == "none" && (manifest.SourcePath != "" || manifest.SourceIdentity != "" || len(manifest.Inventory) != 0) {
		return errors.New("durable import empty source is inconsistent")
	}
	if manifest.Source == "legacy" && (manifest.SourcePath == "" || manifest.SourceIdentity == "") {
		return errors.New("durable import legacy source is incomplete")
	}
	if manifest.Source == "legacy" && manifest.SourcePath != spec.legacySourcePath {
		return errors.New("durable import legacy source path is invalid")
	}
	if !sort.SliceIsSorted(manifest.Inventory, func(i, j int) bool { return manifest.Inventory[i].Path < manifest.Inventory[j].Path }) {
		return errors.New("durable import inventory is not sorted")
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Inventory {
		if entry.Path == "" || entry.SourcePath == "" || seen[entry.Path] || entry.Path != filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry.Path))) || entry.SourcePath != filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry.SourcePath))) {
			return errors.New("durable import inventory path is invalid")
		}
		seen[entry.Path] = true
		if entry.SourceIdentity == "" {
			return errors.New("durable import source identity is missing")
		}
		if entry.Type == "directory" {
			if entry.Size != 0 || entry.SHA256 != "" {
				return errors.New("durable import directory inventory is invalid")
			}
		} else if entry.Type != "file" || entry.Size < 0 || len(entry.SHA256) != sha256.Size*2 {
			return errors.New("durable import file inventory is invalid")
		} else if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return errors.New("durable import file hash is invalid")
		}
	}
	if manifest.InventorySHA256 != durableImportInventoryDigest(manifest.Inventory) {
		return errors.New("durable import inventory digest is invalid")
	}
	return nil
}

func encodeDurableImportManifest(manifest durableImportManifest) ([]byte, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func durableImportInventoryDigest(entries []durableImportEntry) string {
	raw, _ := json.Marshal(entries)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func sameDurableImportSource(left, right durableImportManifest) bool {
	return left.Source == right.Source && left.SourcePath == right.SourcePath && left.SourceIdentity == right.SourceIdentity && left.InventorySHA256 == right.InventorySHA256 && sameDurableImportInventory(left.Inventory, right.Inventory)
}

func sameDurableImportInventory(left, right []durableImportEntry) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func durableImportInventorySubset(subset, complete []durableImportEntry) bool {
	wanted := map[string]durableImportEntry{}
	for _, entry := range durableImportDestinationInventory(complete) {
		wanted[entry.Path] = entry
	}
	for _, entry := range subset {
		if wanted[entry.Path] != entry {
			return false
		}
	}
	return true
}

func durableImportDestinationInventory(source []durableImportEntry) []durableImportEntry {
	byPath := map[string]durableImportEntry{}
	for _, entry := range source {
		entry.SourcePath = ""
		entry.SourceIdentity = ""
		byPath[entry.Path] = entry
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(entry.Path)))
		for parent != "." && parent != "" {
			if _, found := byPath[parent]; !found {
				byPath[parent] = durableImportEntry{Path: parent, Type: "directory"}
			}
			next := filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent)))
			if next == parent {
				break
			}
			parent = next
		}
	}
	destination := make([]durableImportEntry, 0, len(byPath))
	for _, entry := range byPath {
		destination = append(destination, entry)
	}
	sort.Slice(destination, func(i, j int) bool { return destination[i].Path < destination[j].Path })
	return destination
}

func syncDurableImportDirectories(root string) error {
	directories := []string{root}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path != root && info.IsDir() {
			directories = append(directories, path)
		}
		return nil
	}); err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := syncPrivateDirectory(directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func runDurableImportHook(stage string) error {
	if durableCohortImportTestHook == nil {
		return nil
	}
	return durableCohortImportTestHook(stage)
}
