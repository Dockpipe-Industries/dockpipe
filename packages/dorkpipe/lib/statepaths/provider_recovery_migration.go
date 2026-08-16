package statepaths

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

	"dockpipe/src/lib/infrastructure"
)

const (
	providerRecoveryMigrationSchema     = 1
	providerRecoveryMigrationCohort     = "dorkpipe-provider-recovery-authority"
	providerRecoveryDestinationName     = "provider-pools"
	providerRecoveryLockName            = ".provider-recovery-migration.lock"
	providerRecoveryPendingName         = ".provider-recovery-migration.pending.json"
	providerRecoveryProvenanceName      = ".provider-recovery-migration.json"
	providerRecoveryTemporaryNamePrefix = ".provider-recovery-import-"
	providerRecoveryManifestMaxBytes    = 64 << 20
)

type durableCohortMigrationSpec struct {
	label               string
	cohort              string
	destinationName     string
	lockName            string
	pendingName         string
	provenanceName      string
	temporaryNamePrefix string
	legacyContentRoot   func(string) string
	collectLegacy       func(string) (providerRecoveryObservedInventory, error)
	testHook            func(string) error
}

func providerRecoveryMigrationSpec() durableCohortMigrationSpec {
	return durableCohortMigrationSpec{
		label:               "provider recovery",
		cohort:              providerRecoveryMigrationCohort,
		destinationName:     providerRecoveryDestinationName,
		lockName:            providerRecoveryLockName,
		pendingName:         providerRecoveryPendingName,
		provenanceName:      providerRecoveryProvenanceName,
		temporaryNamePrefix: providerRecoveryTemporaryNamePrefix,
		legacyContentRoot: func(legacyRoot string) string {
			return filepath.Join(legacyRoot, providerRecoveryDestinationName)
		},
		collectLegacy: collectProviderRecoveryLegacy,
		testHook:      providerRecoveryMigrationTestHook,
	}
}

// ProviderRecoveryMigrationStatus reports compatibility evidence without making legacy bytes
// authoritative after the durable directory has been published.
type ProviderRecoveryMigrationStatus struct {
	DurableAuthoritative bool
	ImportedLegacy       bool
	LegacyDiverged       bool
}

type providerRecoveryInventoryEntry struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

type providerRecoveryMigrationManifest struct {
	Schema          int                              `json:"schema"`
	Cohort          string                           `json:"cohort"`
	Phase           string                           `json:"phase"`
	Source          string                           `json:"source"`
	SourcePath      string                           `json:"source_path,omitempty"`
	SourceIdentity  string                           `json:"source_identity,omitempty"`
	InventorySHA256 string                           `json:"inventory_sha256"`
	Inventory       []providerRecoveryInventoryEntry `json:"inventory"`
	SourceObjects   []providerRecoverySourceObject   `json:"source_objects"`
}

type providerRecoverySourceObject struct {
	Path     string `json:"path"`
	Identity string `json:"identity"`
}

type providerRecoveryObservedInventory struct {
	entries    []providerRecoveryInventoryEntry
	identities map[string]string
}

var providerRecoveryMigrationTestHook func(string) error

// PrepareProviderRecoveryAuthority resolves DorkPipe's collision-safe durable package owner and
// atomically imports only the provider recovery-authority cohort. The legacy tree is read-only.
func PrepareProviderRecoveryAuthority(workdir string) (string, ProviderRecoveryMigrationStatus, error) {
	return prepareDurableCohortAuthority(workdir, providerRecoveryMigrationSpec())
}

func prepareDurableCohortAuthority(workdir string, spec durableCohortMigrationSpec) (string, ProviderRecoveryMigrationStatus, error) {
	packageRoot, err := infrastructure.ProjectPackageStateDir(workdir, dorkpipeScope)
	if err != nil {
		return "", ProviderRecoveryMigrationStatus{}, fmt.Errorf("%s durable owner: %w", spec.label, err)
	}
	lock, err := openProviderRecoveryMigrationLock(packageRoot, spec)
	if err != nil {
		return "", ProviderRecoveryMigrationStatus{}, fmt.Errorf("%s migration lock: %w", spec.label, err)
	}
	defer closeProviderRecoveryMigrationLock(lock)

	destination := filepath.Join(packageRoot, spec.destinationName)
	if _, statErr := os.Lstat(destination); statErr == nil {
		manifest, err := validateProviderRecoveryDestination(packageRoot, destination, spec)
		if err != nil {
			return "", ProviderRecoveryMigrationStatus{}, err
		}
		diverged, err := providerRecoveryLegacyDiverged(workdir, manifest, spec)
		if err != nil {
			return "", ProviderRecoveryMigrationStatus{}, err
		}
		return destination, ProviderRecoveryMigrationStatus{DurableAuthoritative: true, LegacyDiverged: diverged}, nil
	} else if !os.IsNotExist(statErr) {
		return "", ProviderRecoveryMigrationStatus{}, fmt.Errorf("provider recovery durable authority: %w", statErr)
	}

	source, legacyRoot, observed, err := inspectProviderRecoveryLegacy(workdir, spec)
	if err != nil {
		return "", ProviderRecoveryMigrationStatus{}, err
	}
	if err := runProviderRecoveryMigrationHook("after-source-inventory", spec); err != nil {
		return "", ProviderRecoveryMigrationStatus{}, err
	}

	resumed, err := recoverProviderRecoveryTemporary(packageRoot, destination, source, spec)
	if err != nil {
		return "", ProviderRecoveryMigrationStatus{}, err
	}
	if resumed {
		if err := runProviderRecoveryMigrationHook("after-rename", spec); err != nil {
			return "", ProviderRecoveryMigrationStatus{}, err
		}
		return destination, ProviderRecoveryMigrationStatus{DurableAuthoritative: true, ImportedLegacy: source.Source == "legacy"}, nil
	}

	if err := importProviderRecoveryAuthority(packageRoot, destination, legacyRoot, source, observed, spec); err != nil {
		return "", ProviderRecoveryMigrationStatus{}, err
	}
	return destination, ProviderRecoveryMigrationStatus{DurableAuthoritative: true, ImportedLegacy: source.Source == "legacy"}, nil
}

func openProviderRecoveryMigrationLock(packageRoot string, spec durableCohortMigrationSpec) (*os.File, error) {
	path := filepath.Join(packageRoot, spec.lockName)
	for attempt := 0; attempt < 2; attempt++ {
		before, err := os.Lstat(path)
		if os.IsNotExist(err) {
			file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
			if os.IsExist(createErr) {
				continue
			}
			if createErr != nil {
				return nil, createErr
			}
			if err := migrationMakePrivatePath(path, false); err != nil {
				file.Close()
				return nil, err
			}
			if err := migrationLockFile(file); err != nil {
				file.Close()
				return nil, err
			}
			if err := validateLockedMigrationFile(path, file); err != nil {
				migrationUnlockFile(file)
				file.Close()
				return nil, err
			}
			return file, nil
		}
		if err != nil {
			return nil, err
		}
		if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("provider recovery migration lock is linked or not regular")
		}
		if err := migrationMakePrivatePath(path, false); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		after, err := file.Stat()
		if err != nil || !os.SameFile(before, after) {
			file.Close()
			return nil, errors.New("provider recovery migration lock changed while opening")
		}
		if err := migrationLockFile(file); err != nil {
			file.Close()
			return nil, err
		}
		if err := validateLockedMigrationFile(path, file); err != nil {
			migrationUnlockFile(file)
			file.Close()
			return nil, err
		}
		return file, nil
	}
	return nil, errors.New("provider recovery migration lock changed during creation")
}

func validateLockedMigrationFile(path string, file *os.File) error {
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return errors.New("provider recovery migration lock is not regular after locking")
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, current) {
		return errors.New("provider recovery migration lock was substituted while locking")
	}
	return nil
}

func closeProviderRecoveryMigrationLock(file *os.File) {
	if file == nil {
		return
	}
	migrationUnlockFile(file)
	_ = file.Close()
}

func inspectProviderRecoveryLegacy(workdir string, spec durableCohortMigrationSpec) (providerRecoveryMigrationManifest, string, providerRecoveryObservedInventory, error) {
	legacyRoot, found, err := infrastructure.DiscoverLegacyPackageState(workdir, dorkpipeScope)
	if err != nil {
		return providerRecoveryMigrationManifest{}, "", providerRecoveryObservedInventory{}, fmt.Errorf("%s legacy discovery: %w", spec.label, err)
	}
	if !found {
		empty := []providerRecoveryInventoryEntry{}
		return newProviderRecoveryManifest("none", "", "", empty, nil, spec), "", providerRecoveryObservedInventory{entries: empty, identities: map[string]string{}}, nil
	}
	identity, err := migrationFileIdentity(legacyRoot)
	if err != nil {
		return providerRecoveryMigrationManifest{}, "", providerRecoveryObservedInventory{}, fmt.Errorf("provider recovery legacy identity: %w", err)
	}
	observed, err := spec.collectLegacy(legacyRoot)
	if err != nil {
		return providerRecoveryMigrationManifest{}, "", providerRecoveryObservedInventory{}, err
	}
	return newProviderRecoveryManifest("legacy", filepath.ToSlash(filepath.Join(infrastructure.DockpipeDirRel, "packages", infrastructure.SanitizePackageStateScope(dorkpipeScope))), identity, observed.entries, observed.identities, spec), legacyRoot, observed, nil
}

func newProviderRecoveryManifest(source, sourcePath, sourceIdentity string, inventory []providerRecoveryInventoryEntry, identities map[string]string, spec durableCohortMigrationSpec) providerRecoveryMigrationManifest {
	if inventory == nil {
		inventory = []providerRecoveryInventoryEntry{}
	}
	copiedInventory := make([]providerRecoveryInventoryEntry, len(inventory))
	copy(copiedInventory, inventory)
	objects := make([]providerRecoverySourceObject, 0, len(copiedInventory))
	for _, entry := range copiedInventory {
		objects = append(objects, providerRecoverySourceObject{Path: entry.Path, Identity: identities[entry.Path]})
	}
	return providerRecoveryMigrationManifest{
		Schema:          providerRecoveryMigrationSchema,
		Cohort:          spec.cohort,
		Phase:           "authoritative",
		Source:          source,
		SourcePath:      sourcePath,
		SourceIdentity:  sourceIdentity,
		InventorySHA256: providerRecoveryInventoryDigest(inventory),
		Inventory:       copiedInventory,
		SourceObjects:   objects,
	}
}

func collectProviderRecoveryLegacy(legacyRoot string) (providerRecoveryObservedInventory, error) {
	rootDevice, err := migrationDeviceIdentity(legacyRoot)
	if err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	providerRoot := filepath.Join(legacyRoot, providerRecoveryDestinationName)
	info, err := os.Lstat(providerRoot)
	if os.IsNotExist(err) {
		return providerRecoveryObservedInventory{entries: []providerRecoveryInventoryEntry{}, identities: map[string]string{}}, nil
	}
	if err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return providerRecoveryObservedInventory{}, errors.New("provider recovery legacy root is linked or not a directory")
	}
	if err := requireMigrationDevice(providerRoot, rootDevice); err != nil {
		return providerRecoveryObservedInventory{}, err
	}

	collector := providerRecoveryCollector{root: providerRoot, device: rootDevice, identities: map[string]string{}}
	entries, err := os.ReadDir(providerRoot)
	if err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "sessions.json":
			if err := collector.addRegular("sessions.json"); err != nil {
				return providerRecoveryObservedInventory{}, err
			}
		case "session-adapters":
			if err := collector.addFlatDirectory("session-adapters", func(name string) bool { return strings.HasSuffix(name, ".json") }); err != nil {
				return providerRecoveryObservedInventory{}, err
			}
		case "app-server":
			if err := collector.addAppServer(); err != nil {
				return providerRecoveryObservedInventory{}, err
			}
		case "leases", "scratch":
			// Explicitly disposable provider-pool state remains in the checkout.
		default:
			return providerRecoveryObservedInventory{}, fmt.Errorf("provider recovery legacy root contains unclassified entry %q", entry.Name())
		}
	}
	collector.sort()
	return providerRecoveryObservedInventory{entries: collector.entries, identities: collector.identities}, nil
}

type providerRecoveryCollector struct {
	root       string
	device     string
	entries    []providerRecoveryInventoryEntry
	identities map[string]string
}

func (c *providerRecoveryCollector) addAppServer() error {
	if err := c.addDirectory("app-server"); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(c.root, "app-server"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		rel := filepath.Join("app-server", entry.Name())
		switch entry.Name() {
		case "sessions":
			if err := c.addFlatDirectory(rel, func(name string) bool {
				return strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".json.lock")
			}); err != nil {
				return err
			}
		case "snapshots", "audit", "aggregates":
			if err := c.addTree(rel); err != nil {
				return err
			}
		default:
			return fmt.Errorf("provider recovery App Server state contains unclassified entry %q", entry.Name())
		}
	}
	return nil
}

func (c *providerRecoveryCollector) addFlatDirectory(rel string, accept func(string) bool) error {
	if err := c.addDirectory(rel); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(c.root, rel))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !accept(entry.Name()) {
			return fmt.Errorf("provider recovery directory %q contains unexpected entry %q", filepath.ToSlash(rel), entry.Name())
		}
		if err := c.addRegular(filepath.Join(rel, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (c *providerRecoveryCollector) addTree(rel string) error {
	if err := c.addDirectory(rel); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(c.root, rel))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(rel, entry.Name())
		info, err := os.Lstat(filepath.Join(c.root, child))
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("provider recovery source %q is linked", filepath.ToSlash(child))
		}
		if info.IsDir() {
			if err := c.addTree(child); err != nil {
				return err
			}
			continue
		}
		if err := c.addRegular(child); err != nil {
			return err
		}
	}
	return nil
}

func (c *providerRecoveryCollector) addDirectory(rel string) error {
	path := filepath.Join(c.root, rel)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("provider recovery source %q is linked or not a directory", filepath.ToSlash(rel))
	}
	if err := requireMigrationDevice(path, c.device); err != nil {
		return err
	}
	identity, err := migrationFileIdentity(path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	c.entries = append(c.entries, providerRecoveryInventoryEntry{Path: rel, Type: "directory", Size: 0})
	c.identities[rel] = identity
	return nil
}

func (c *providerRecoveryCollector) addRegular(rel string) error {
	path := filepath.Join(c.root, rel)
	entry, identity, err := inspectProviderRecoveryRegular(path, rel, c.device)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	entry.Path = rel
	c.entries = append(c.entries, entry)
	c.identities[rel] = identity
	return nil
}

func (c *providerRecoveryCollector) sort() {
	sort.Slice(c.entries, func(i, j int) bool { return c.entries[i].Path < c.entries[j].Path })
}

func inspectProviderRecoveryRegular(path, rel, device string) (providerRecoveryInventoryEntry, string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return providerRecoveryInventoryEntry{}, "", err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return providerRecoveryInventoryEntry{}, "", fmt.Errorf("provider recovery source %q is not a regular file", filepath.ToSlash(rel))
	}
	if err := requireMigrationDevice(path, device); err != nil {
		return providerRecoveryInventoryEntry{}, "", err
	}
	identity, err := migrationFileIdentity(path)
	if err != nil {
		return providerRecoveryInventoryEntry{}, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return providerRecoveryInventoryEntry{}, "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return providerRecoveryInventoryEntry{}, "", fmt.Errorf("provider recovery source %q changed while opening", filepath.ToSlash(rel))
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return providerRecoveryInventoryEntry{}, "", err
	}
	final, err := file.Stat()
	if err != nil || !os.SameFile(opened, final) || size != final.Size() {
		return providerRecoveryInventoryEntry{}, "", fmt.Errorf("provider recovery source %q changed while reading", filepath.ToSlash(rel))
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return providerRecoveryInventoryEntry{}, "", fmt.Errorf("provider recovery source %q was substituted", filepath.ToSlash(rel))
	}
	return providerRecoveryInventoryEntry{Type: "file", Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))}, identity, nil
}

func requireMigrationDevice(path, expected string) error {
	actual, err := migrationDeviceIdentity(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("provider recovery path %q crosses a filesystem boundary", path)
	}
	return nil
}

func importProviderRecoveryAuthority(packageRoot, destination, legacyRoot string, source providerRecoveryMigrationManifest, observed providerRecoveryObservedInventory, spec durableCohortMigrationSpec) error {
	temporary, err := os.MkdirTemp(packageRoot, spec.temporaryNamePrefix)
	if err != nil {
		return err
	}
	if err := migrationMakePrivatePath(temporary, true); err != nil {
		return err
	}
	pending := source
	pending.Phase = "copying"
	if err := writeProviderRecoveryManifest(filepath.Join(temporary, spec.pendingName), pending, spec); err != nil {
		return err
	}
	if source.Source == "legacy" {
		for _, entry := range source.Inventory {
			if err := copyProviderRecoveryEntry(legacyRoot, temporary, entry, spec); err != nil {
				return err
			}
		}
	}
	if err := runProviderRecoveryMigrationHook("after-copy", spec); err != nil {
		return err
	}
	current, currentRoot, currentObserved, err := inspectProviderRecoveryLegacyFromExpected(legacyRoot, source, spec)
	if err != nil {
		return err
	}
	if !sameProviderRecoverySource(source, current) || currentRoot != legacyRoot || !sameProviderRecoveryObserved(observed, currentObserved) {
		return errors.New("provider recovery legacy source changed during migration")
	}
	temporaryInventory, err := collectProviderRecoveryTree(temporary, true, spec)
	if err != nil {
		return err
	}
	if !sameProviderRecoveryInventory(source.Inventory, temporaryInventory.entries) {
		return errors.New("provider recovery copied inventory does not match its legacy source")
	}
	if err := writeProviderRecoveryManifest(filepath.Join(temporary, spec.provenanceName), source, spec); err != nil {
		return err
	}
	if err := migrationSyncDirectory(temporary); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(temporary, spec.pendingName)); err != nil {
		return err
	}
	if err := syncProviderRecoveryTreeDirectories(temporary); err != nil {
		return err
	}
	if err := runProviderRecoveryMigrationHook("before-rename", spec); err != nil {
		return err
	}
	if err := requireProviderRecoveryDestinationAbsent(destination); err != nil {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("publish provider recovery durable authority: %w", err)
	}
	if err := migrationSyncDirectory(packageRoot); err != nil {
		return err
	}
	if err := runProviderRecoveryMigrationHook("after-rename", spec); err != nil {
		return err
	}
	return nil
}

func inspectProviderRecoveryLegacyFromExpected(legacyRoot string, expected providerRecoveryMigrationManifest, spec durableCohortMigrationSpec) (providerRecoveryMigrationManifest, string, providerRecoveryObservedInventory, error) {
	if expected.Source == "none" {
		if legacyRoot != "" {
			return providerRecoveryMigrationManifest{}, "", providerRecoveryObservedInventory{}, errors.New("provider recovery legacy source identity is inconsistent")
		}
		empty := []providerRecoveryInventoryEntry{}
		return newProviderRecoveryManifest("none", "", "", empty, nil, spec), "", providerRecoveryObservedInventory{entries: empty, identities: map[string]string{}}, nil
	}
	identity, err := migrationFileIdentity(legacyRoot)
	if err != nil {
		return providerRecoveryMigrationManifest{}, "", providerRecoveryObservedInventory{}, err
	}
	observed, err := spec.collectLegacy(legacyRoot)
	if err != nil {
		return providerRecoveryMigrationManifest{}, "", providerRecoveryObservedInventory{}, err
	}
	return newProviderRecoveryManifest("legacy", expected.SourcePath, identity, observed.entries, observed.identities, spec), legacyRoot, observed, nil
}

func copyProviderRecoveryEntry(legacyRoot, temporary string, entry providerRecoveryInventoryEntry, spec durableCohortMigrationSpec) error {
	destination := filepath.Join(temporary, filepath.FromSlash(entry.Path))
	if entry.Type == "directory" {
		if err := os.Mkdir(destination, 0o700); err != nil && !os.IsExist(err) {
			return err
		}
		return migrationMakePrivatePath(destination, true)
	}
	if entry.Type != "file" {
		return fmt.Errorf("provider recovery inventory type %q is invalid", entry.Type)
	}
	if err := runProviderRecoveryMigrationHook("copy-file:"+entry.Path, spec); err != nil {
		return err
	}
	source := filepath.Join(spec.legacyContentRoot(legacyRoot), filepath.FromSlash(entry.Path))
	before, err := os.Lstat(source)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("provider recovery source %q changed before copy", entry.Path)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return fmt.Errorf("provider recovery source %q changed while opening for copy", entry.Path)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
	if err := migrationMakePrivatePath(destination, false); err != nil {
		return err
	}
	final, err := input.Stat()
	after, afterErr := os.Lstat(source)
	if err != nil || afterErr != nil || !os.SameFile(opened, final) || !os.SameFile(before, after) || written != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("provider recovery source %q changed while copying", entry.Path)
	}
	return nil
}

func recoverProviderRecoveryTemporary(packageRoot, destination string, source providerRecoveryMigrationManifest, spec durableCohortMigrationSpec) (bool, error) {
	entries, err := os.ReadDir(packageRoot)
	if err != nil {
		return false, err
	}
	var temporary string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), spec.temporaryNamePrefix) {
			if temporary != "" {
				return false, errors.New("multiple provider recovery migration temporaries require manual review")
			}
			temporary = filepath.Join(packageRoot, entry.Name())
		}
	}
	if temporary == "" {
		return false, nil
	}
	info, err := os.Lstat(temporary)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, errors.New("provider recovery migration temporary is substituted")
	}
	if err := migrationMakePrivatePath(temporary, true); err != nil {
		return false, err
	}
	packageDevice, err := migrationDeviceIdentity(packageRoot)
	if err != nil {
		return false, err
	}
	if err := requireMigrationDevice(temporary, packageDevice); err != nil {
		return false, err
	}

	pending, pendingErr := readProviderRecoveryManifestForSpec(filepath.Join(temporary, spec.pendingName), "copying", spec)
	final, finalErr := readProviderRecoveryManifestForSpec(filepath.Join(temporary, spec.provenanceName), "authoritative", spec)
	if pendingErr != nil && !errors.Is(pendingErr, os.ErrNotExist) {
		return false, fmt.Errorf("provider recovery pending migration is malformed: %w", pendingErr)
	}
	if finalErr != nil && !errors.Is(finalErr, os.ErrNotExist) {
		return false, fmt.Errorf("provider recovery ready migration is malformed: %w", finalErr)
	}
	if errors.Is(pendingErr, os.ErrNotExist) && errors.Is(finalErr, os.ErrNotExist) {
		return false, errors.New("provider recovery migration temporary has no byte-proving manifest")
	}
	for _, manifest := range []providerRecoveryMigrationManifest{pending, final} {
		if manifest.Schema != 0 && !sameProviderRecoverySource(source, manifest) {
			return false, errors.New("provider recovery migration temporary source is no longer authoritative")
		}
	}
	temporaryInventory, err := collectProviderRecoveryTree(temporary, true, spec)
	if err != nil {
		return false, err
	}
	if final.Schema != 0 {
		if !sameProviderRecoveryInventory(source.Inventory, temporaryInventory.entries) {
			return false, errors.New("provider recovery ready temporary does not match its source inventory")
		}
		if pending.Schema != 0 {
			if err := os.Remove(filepath.Join(temporary, spec.pendingName)); err != nil {
				return false, err
			}
		}
		if err := syncProviderRecoveryTreeDirectories(temporary); err != nil {
			return false, err
		}
		if err := requireProviderRecoveryDestinationAbsent(destination); err != nil {
			return false, err
		}
		if err := os.Rename(temporary, destination); err != nil {
			return false, err
		}
		if err := migrationSyncDirectory(packageRoot); err != nil {
			return false, err
		}
		return true, nil
	}
	if !providerRecoveryInventorySubset(temporaryInventory.entries, source.Inventory) {
		return false, errors.New("provider recovery incomplete temporary is not a byte-proven source subset")
	}
	if err := os.RemoveAll(temporary); err != nil {
		return false, err
	}
	if err := migrationSyncDirectory(packageRoot); err != nil {
		return false, err
	}
	return false, nil
}

func requireProviderRecoveryDestinationAbsent(destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("provider recovery durable destination was substituted before publication")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateProviderRecoveryDestination(packageRoot, destination string, spec durableCohortMigrationSpec) (providerRecoveryMigrationManifest, error) {
	if err := migrationMakePrivatePath(destination, true); err != nil {
		return providerRecoveryMigrationManifest{}, fmt.Errorf("provider recovery durable authority is unsafe: %w", err)
	}
	packageDevice, err := migrationDeviceIdentity(packageRoot)
	if err != nil {
		return providerRecoveryMigrationManifest{}, err
	}
	if err := requireMigrationDevice(destination, packageDevice); err != nil {
		return providerRecoveryMigrationManifest{}, fmt.Errorf("provider recovery durable authority is unsafe: %w", err)
	}
	if _, err := collectProviderRecoveryTree(destination, true, spec); err != nil {
		return providerRecoveryMigrationManifest{}, fmt.Errorf("provider recovery durable authority is unsafe: %w", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, spec.pendingName)); err == nil {
		return providerRecoveryMigrationManifest{}, errors.New("provider recovery durable authority contains an incomplete migration marker")
	} else if !os.IsNotExist(err) {
		return providerRecoveryMigrationManifest{}, err
	}
	manifest, err := readProviderRecoveryManifestForSpec(filepath.Join(destination, spec.provenanceName), "authoritative", spec)
	if err != nil {
		return providerRecoveryMigrationManifest{}, fmt.Errorf("provider recovery durable provenance: %w", err)
	}
	if !sameOrWithin(packageRoot, destination) {
		return providerRecoveryMigrationManifest{}, errors.New("provider recovery durable authority escapes its package owner")
	}
	return manifest, nil
}

func providerRecoveryLegacyDiverged(workdir string, manifest providerRecoveryMigrationManifest, spec durableCohortMigrationSpec) (bool, error) {
	current, _, _, err := inspectProviderRecoveryLegacy(workdir, spec)
	if err != nil {
		return false, err
	}
	if current.Source == "none" {
		return false, nil
	}
	return !sameProviderRecoverySource(manifest, current), nil
}

func collectProviderRecoveryTree(root string, private bool, spec durableCohortMigrationSpec) (providerRecoveryObservedInventory, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return providerRecoveryObservedInventory{}, errors.New("provider recovery tree root is linked or not a directory")
	}
	if private {
		if err := migrationMakePrivatePath(root, true); err != nil {
			return providerRecoveryObservedInventory{}, err
		}
	}
	device, err := migrationDeviceIdentity(root)
	if err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	collector := providerRecoveryCollector{root: root, device: device, identities: map[string]string{}}
	var walk func(string) error
	walk = func(rel string) error {
		entries, err := os.ReadDir(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			child := filepath.Join(rel, entry.Name())
			if rel == "" && (entry.Name() == spec.pendingName || entry.Name() == spec.provenanceName) {
				continue
			}
			info, err := os.Lstat(filepath.Join(root, child))
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("provider recovery tree %q is substituted", filepath.ToSlash(child))
			}
			if info.IsDir() {
				if private {
					if err := migrationMakePrivatePath(filepath.Join(root, child), true); err != nil {
						return err
					}
				}
				if err := collector.addDirectory(child); err != nil {
					return err
				}
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			if private {
				if err := migrationMakePrivatePath(filepath.Join(root, child), false); err != nil {
					return err
				}
			}
			if err := collector.addRegular(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	collector.sort()
	return providerRecoveryObservedInventory{entries: collector.entries, identities: collector.identities}, nil
}

func writeProviderRecoveryManifest(path string, manifest providerRecoveryMigrationManifest, spec durableCohortMigrationSpec) error {
	if err := validateProviderRecoveryManifest(manifest, manifest.Phase, spec); err != nil {
		return err
	}
	raw, err := encodeProviderRecoveryManifest(manifest)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := migrationMakePrivatePath(path, false); err != nil {
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

func readProviderRecoveryManifest(path, phase string) (providerRecoveryMigrationManifest, error) {
	return readProviderRecoveryManifestForSpec(path, phase, providerRecoveryMigrationSpec())
}

func readProviderRecoveryManifestForSpec(path, phase string, spec durableCohortMigrationSpec) (providerRecoveryMigrationManifest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return providerRecoveryMigrationManifest{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > providerRecoveryManifestMaxBytes {
		return providerRecoveryMigrationManifest{}, errors.New("migration manifest is linked, special, empty, or oversized")
	}
	if err := migrationMakePrivatePath(path, false); err != nil {
		return providerRecoveryMigrationManifest{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return providerRecoveryMigrationManifest{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return providerRecoveryMigrationManifest{}, errors.New("migration manifest changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, providerRecoveryManifestMaxBytes+1))
	if err != nil || len(raw) > providerRecoveryManifestMaxBytes {
		return providerRecoveryMigrationManifest{}, errors.New("migration manifest read is invalid")
	}
	var manifest providerRecoveryMigrationManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return providerRecoveryMigrationManifest{}, errors.New("migration manifest JSON is malformed")
	}
	if err := validateProviderRecoveryManifest(manifest, phase, spec); err != nil {
		return providerRecoveryMigrationManifest{}, err
	}
	canonical, err := encodeProviderRecoveryManifest(manifest)
	if err != nil || !bytes.Equal(raw, canonical) {
		return providerRecoveryMigrationManifest{}, errors.New("migration manifest is not canonical")
	}
	return manifest, nil
}

func validateProviderRecoveryManifest(manifest providerRecoveryMigrationManifest, phase string, spec durableCohortMigrationSpec) error {
	if manifest.Schema != providerRecoveryMigrationSchema || manifest.Cohort != spec.cohort || manifest.Phase != phase {
		return errors.New("migration manifest schema, cohort, or phase is invalid")
	}
	if manifest.Source != "none" && manifest.Source != "legacy" {
		return errors.New("migration manifest source is invalid")
	}
	if manifest.Source == "none" && (manifest.SourcePath != "" || manifest.SourceIdentity != "" || len(manifest.Inventory) != 0 || len(manifest.SourceObjects) != 0) {
		return errors.New("migration manifest empty source is inconsistent")
	}
	if manifest.Source == "legacy" && (manifest.SourcePath == "" || manifest.SourceIdentity == "") {
		return errors.New("migration manifest legacy source is incomplete")
	}
	legacySourcePath := filepath.ToSlash(filepath.Join(infrastructure.DockpipeDirRel, "packages", infrastructure.SanitizePackageStateScope(dorkpipeScope)))
	if manifest.Source == "legacy" && manifest.SourcePath != legacySourcePath {
		return errors.New("migration manifest legacy source path is invalid")
	}
	if !sort.SliceIsSorted(manifest.Inventory, func(i, j int) bool { return manifest.Inventory[i].Path < manifest.Inventory[j].Path }) {
		return errors.New("migration manifest inventory is not sorted")
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Inventory {
		if entry.Path == "" || entry.Path != filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry.Path))) || strings.HasPrefix(entry.Path, "../") || filepath.IsAbs(filepath.FromSlash(entry.Path)) || seen[entry.Path] {
			return errors.New("migration manifest inventory path is invalid")
		}
		seen[entry.Path] = true
		if entry.Type == "directory" {
			if entry.Size != 0 || entry.SHA256 != "" {
				return errors.New("migration manifest directory entry is invalid")
			}
		} else if entry.Type != "file" || entry.Size < 0 || len(entry.SHA256) != sha256.Size*2 {
			return errors.New("migration manifest file entry is invalid")
		} else if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return errors.New("migration manifest file hash is invalid")
		}
	}
	if manifest.InventorySHA256 != providerRecoveryInventoryDigest(manifest.Inventory) {
		return errors.New("migration manifest inventory digest is invalid")
	}
	if manifest.Source == "legacy" {
		if len(manifest.SourceObjects) != len(manifest.Inventory) {
			return errors.New("migration manifest source identity inventory is incomplete")
		}
		for index, object := range manifest.SourceObjects {
			if object.Path != manifest.Inventory[index].Path || object.Identity == "" {
				return errors.New("migration manifest source object identity is invalid")
			}
		}
	}
	return nil
}

func encodeProviderRecoveryManifest(manifest providerRecoveryMigrationManifest) ([]byte, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func providerRecoveryInventoryDigest(entries []providerRecoveryInventoryEntry) string {
	raw, _ := json.Marshal(entries)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func sameProviderRecoverySource(left, right providerRecoveryMigrationManifest) bool {
	return left.Source == right.Source && left.SourcePath == right.SourcePath && left.SourceIdentity == right.SourceIdentity && left.InventorySHA256 == right.InventorySHA256 && sameProviderRecoveryInventory(left.Inventory, right.Inventory) && sameProviderRecoverySourceObjects(left.SourceObjects, right.SourceObjects)
}

func sameProviderRecoverySourceObjects(left, right []providerRecoverySourceObject) bool {
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

func sameProviderRecoveryInventory(left, right []providerRecoveryInventoryEntry) bool {
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

func sameProviderRecoveryObserved(left, right providerRecoveryObservedInventory) bool {
	if !sameProviderRecoveryInventory(left.entries, right.entries) || len(left.identities) != len(right.identities) {
		return false
	}
	for path, identity := range left.identities {
		if right.identities[path] != identity {
			return false
		}
	}
	return true
}

func providerRecoveryInventorySubset(subset, complete []providerRecoveryInventoryEntry) bool {
	want := make(map[string]providerRecoveryInventoryEntry, len(complete))
	for _, entry := range complete {
		want[entry.Path] = entry
	}
	for _, entry := range subset {
		if expected, found := want[entry.Path]; !found || expected != entry {
			return false
		}
	}
	return true
}

func runProviderRecoveryMigrationHook(stage string, spec durableCohortMigrationSpec) error {
	if spec.testHook == nil {
		return nil
	}
	return spec.testHook(stage)
}

func syncProviderRecoveryTreeDirectories(root string) error {
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
		if err := migrationSyncDirectory(directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func sameOrWithin(root, candidate string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
