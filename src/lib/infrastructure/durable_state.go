package infrastructure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	durableStateSchema      = 1
	durableProjectIndexFile = "projects.json"
	durableProjectMetaFile  = "project.json"
	durablePackageMetaFile  = "package.json"
)

type durableProjectIndex struct {
	Schema   int                    `json:"schema"`
	Projects []durableProjectRecord `json:"projects"`
}

type durableProjectRecord struct {
	ProjectID          string   `json:"project_id"`
	CanonicalPath      string   `json:"canonical_path"`
	PathAliases        []string `json:"path_aliases"`
	FilesystemIdentity string   `json:"filesystem_identity"`
}

type durableProjectMetadata struct {
	Schema             int      `json:"schema"`
	ProjectID          string   `json:"project_id"`
	CanonicalPath      string   `json:"canonical_path"`
	PathAliases        []string `json:"path_aliases"`
	FilesystemIdentity string   `json:"filesystem_identity"`
}

type durablePackageMetadata struct {
	Schema      int    `json:"schema"`
	OwnerID     string `json:"owner_id"`
	OwnerDigest string `json:"owner_digest"`
}

// DurableStateRoot returns the OS-appropriate per-user root for DockPipe state that must survive
// removal of a checkout's disposable bin/.dockpipe tree. It intentionally does not honor
// DOCKPIPE_GLOBAL_ROOT, which owns install/data semantics rather than project identity.
func DurableStateRoot() (string, error) {
	home := ""
	if runtime.GOOS != "windows" || strings.TrimSpace(os.Getenv("LOCALAPPDATA")) == "" {
		resolvedHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("durable DockPipe state root: %w", err)
		}
		home = resolvedHome
	}
	return durableStateRootFor(runtime.GOOS, map[string]string{
		"HOME":           home,
		"XDG_STATE_HOME": os.Getenv("XDG_STATE_HOME"),
		"LOCALAPPDATA":   os.Getenv("LOCALAPPDATA"),
	})
}

func durableStateRootFor(goos string, env map[string]string) (string, error) {
	home := strings.TrimSpace(env["HOME"])
	switch goos {
	case "windows":
		base := strings.TrimSpace(env["LOCALAPPDATA"])
		if base == "" {
			if home == "" {
				return "", errors.New("durable DockPipe state root: user home is unavailable")
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		if !durablePathIsAbsoluteForOS(goos, base) {
			return "", errors.New("durable DockPipe state root: LOCALAPPDATA or user home must be absolute")
		}
		return filepath.Join(filepath.Clean(base), "dockpipe", "state"), nil
	case "darwin":
		if home == "" {
			return "", errors.New("durable DockPipe state root: user home is unavailable")
		}
		if !durablePathIsAbsoluteForOS(goos, home) {
			return "", errors.New("durable DockPipe state root: user home must be absolute")
		}
		return filepath.Join(home, "Library", "Application Support", "dockpipe", "state"), nil
	default:
		base := strings.TrimSpace(env["XDG_STATE_HOME"])
		if base == "" {
			if home == "" {
				return "", errors.New("durable DockPipe state root: user home is unavailable")
			}
			base = filepath.Join(home, ".local", "state")
		}
		if !durablePathIsAbsoluteForOS(goos, base) {
			return "", errors.New("durable DockPipe state root: XDG_STATE_HOME or user home must be absolute")
		}
		return filepath.Join(filepath.Clean(base), "dockpipe"), nil
	}
}

func durablePathIsAbsoluteForOS(goos, path string) bool {
	if goos != "windows" {
		return strings.HasPrefix(path, "/")
	}
	path = strings.ReplaceAll(path, "/", `\`)
	return strings.HasPrefix(path, `\\`) || (len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && path[2] == '\\')
}

// ProjectStateRoot resolves or creates the owner-only durable root for a stable project identity.
// Identity follows a checkout through a same-filesystem rename and symlink aliases, while a copy,
// clone, or cross-filesystem move receives a new random 128-bit project ID.
func ProjectStateRoot(workdir string) (string, error) {
	root, err := DurableStateRoot()
	if err != nil {
		return "", err
	}
	return projectStateRootAt(workdir, root)
}

func projectStateRootAt(workdir, stateRoot string) (string, error) {
	canonicalPath, filesystemIdentity, err := durableProjectIdentity(workdir)
	if err != nil {
		return "", err
	}
	stateRoot, err = filepath.Abs(filepath.Clean(stateRoot))
	if err != nil {
		return "", fmt.Errorf("durable state root: %w", err)
	}
	if err := ensurePrivateRoot(stateRoot); err != nil {
		return "", fmt.Errorf("durable state root: %w", err)
	}
	projectsRoot := filepath.Join(stateRoot, "projects")
	if err := ensurePrivateSubdirectory(stateRoot, projectsRoot); err != nil {
		return "", fmt.Errorf("durable projects root: %w", err)
	}

	lockPath := filepath.Join(stateRoot, ".project-identity.lock")
	lockFile, err := openPrivateLockFile(stateRoot, lockPath)
	if err != nil {
		return "", fmt.Errorf("durable project identity lock: %w", err)
	}
	defer lockFile.Close()
	if err := lockPrivateFile(lockFile); err != nil {
		return "", fmt.Errorf("durable project identity lock: %w", err)
	}
	defer unlockPrivateFile(lockFile)

	indexPath := filepath.Join(stateRoot, durableProjectIndexFile)
	index, err := readDurableProjectIndex(stateRoot, indexPath)
	if err != nil {
		return "", err
	}
	recordIndex := -1
	for i := range index.Projects {
		if index.Projects[i].FilesystemIdentity == filesystemIdentity {
			if recordIndex >= 0 {
				return "", fmt.Errorf("durable project index contains duplicate filesystem identity %q", filesystemIdentity)
			}
			recordIndex = i
		}
	}

	changed := false
	if recordIndex < 0 {
		projectID, idErr := newDurableProjectID(projectsRoot)
		if idErr != nil {
			return "", idErr
		}
		index.Projects = append(index.Projects, durableProjectRecord{
			ProjectID:          projectID,
			CanonicalPath:      canonicalPath,
			PathAliases:        []string{canonicalPath},
			FilesystemIdentity: filesystemIdentity,
		})
		recordIndex = len(index.Projects) - 1
		changed = true
	} else {
		record := &index.Projects[recordIndex]
		if !sameDurablePath(record.CanonicalPath, canonicalPath) {
			record.CanonicalPath = canonicalPath
			changed = true
		}
		if addDurablePathAlias(&record.PathAliases, canonicalPath) {
			changed = true
		}
	}

	// A copied checkout can occupy a former path while retaining a different filesystem identity.
	// Keep that path bound to only the current identity; the old identity remains discoverable by
	// its filesystem identity and any other historical alias.
	for i := range index.Projects {
		if i == recordIndex {
			continue
		}
		aliases := removeDurablePathAlias(index.Projects[i].PathAliases, canonicalPath)
		if len(aliases) != len(index.Projects[i].PathAliases) {
			index.Projects[i].PathAliases = aliases
			changed = true
		}
	}

	record := &index.Projects[recordIndex]
	if err := validateDurableProjectRecord(*record); err != nil {
		return "", err
	}
	projectID := record.ProjectID
	sort.Slice(index.Projects, func(i, j int) bool {
		return index.Projects[i].ProjectID < index.Projects[j].ProjectID
	})
	for i := range index.Projects {
		if index.Projects[i].ProjectID == projectID {
			recordIndex = i
			break
		}
	}
	record = &index.Projects[recordIndex]

	projectRoot := filepath.Join(projectsRoot, record.ProjectID)
	if err := ensurePrivateSubdirectory(stateRoot, projectRoot); err != nil {
		return "", fmt.Errorf("durable project directory: %w", err)
	}
	if err := ensurePrivateSubdirectory(stateRoot, filepath.Join(projectRoot, "packages")); err != nil {
		return "", fmt.Errorf("durable project packages directory: %w", err)
	}
	metadata := durableProjectMetadata{
		Schema:             durableStateSchema,
		ProjectID:          record.ProjectID,
		CanonicalPath:      record.CanonicalPath,
		PathAliases:        append([]string(nil), record.PathAliases...),
		FilesystemIdentity: record.FilesystemIdentity,
	}
	metadataPath := filepath.Join(projectRoot, durableProjectMetaFile)
	if err := validateExistingProjectMetadata(stateRoot, metadataPath, *record); err != nil {
		return "", err
	}
	metadataChanged, err := durableJSONDiffers(stateRoot, metadataPath, metadata)
	if err != nil {
		return "", err
	}
	if metadataChanged {
		if err := writePrivateJSONAtomic(stateRoot, metadataPath, metadata); err != nil {
			return "", fmt.Errorf("write durable project metadata: %w", err)
		}
	}
	if changed || !durablePathExists(indexPath) {
		if err := writePrivateJSONAtomic(stateRoot, indexPath, index); err != nil {
			return "", fmt.Errorf("write durable project index: %w", err)
		}
	}
	return projectRoot, nil
}

// ProjectPackageStateDir resolves and creates an owner-only durable package directory. The path
// uses an informational slug plus the full SHA-256 of the validated, case-preserving owner ID, so
// names that collide under the legacy sanitizer cannot share durable bytes.
func ProjectPackageStateDir(workdir, ownerID string) (string, error) {
	root, err := DurableStateRoot()
	if err != nil {
		return "", err
	}
	return projectPackageStateDirAt(workdir, ownerID, root)
}

func projectPackageStateDirAt(workdir, ownerID, stateRoot string) (string, error) {
	location, err := durablePackageLocationAt(workdir, ownerID, stateRoot)
	if err != nil {
		return "", err
	}
	lock, err := openPrivateLockFile(location.stateRoot, filepath.Join(location.projectRoot, ".package-"+location.ownerDigest+".lock"))
	if err != nil {
		return "", fmt.Errorf("durable package lock: %w", err)
	}
	defer lock.Close()
	if err := lockPrivateFile(lock); err != nil {
		return "", fmt.Errorf("durable package lock: %w", err)
	}
	defer unlockPrivateFile(lock)
	if err := ensureDurablePackageLocation(location); err != nil {
		return "", err
	}
	return location.packageRoot, nil
}

type durablePackageLocation struct {
	stateRoot   string
	projectRoot string
	packageRoot string
	ownerID     string
	ownerDigest string
	metadata    durablePackageMetadata
}

func durablePackageLocationAt(workdir, ownerID, stateRoot string) (durablePackageLocation, error) {
	ownerID, segment, digest, err := durableOwnerStorageIdentity(ownerID)
	if err != nil {
		return durablePackageLocation{}, err
	}
	projectRoot, err := projectStateRootAt(workdir, stateRoot)
	if err != nil {
		return durablePackageLocation{}, err
	}
	return durablePackageLocation{
		stateRoot:   filepath.Clean(stateRoot),
		projectRoot: projectRoot,
		packageRoot: filepath.Join(projectRoot, "packages", segment),
		ownerID:     ownerID,
		ownerDigest: digest,
		metadata:    durablePackageMetadata{Schema: durableStateSchema, OwnerID: ownerID, OwnerDigest: digest},
	}, nil
}

func ensureDurablePackageLocation(location durablePackageLocation) error {
	if err := ensurePrivateSubdirectory(location.stateRoot, location.packageRoot); err != nil {
		return fmt.Errorf("durable package directory: %w", err)
	}
	return validateOrWriteDurablePackageMetadata(location, true)
}

func validateDurablePackageLocation(location durablePackageLocation) error {
	if err := validateExistingStateBoundary(location.stateRoot, location.packageRoot); err != nil {
		return err
	}
	if err := validatePrivatePath(location.packageRoot, true); err != nil {
		return fmt.Errorf("durable package directory: %w", err)
	}
	return validateOrWriteDurablePackageMetadata(location, false)
}

func validateOrWriteDurablePackageMetadata(location durablePackageLocation, allowCreate bool) error {
	metadataPath := filepath.Join(location.packageRoot, durablePackageMetaFile)
	differs, err := durableJSONDiffers(location.stateRoot, metadataPath, location.metadata)
	if err != nil {
		return err
	}
	if !differs {
		return nil
	}
	if durablePathExists(metadataPath) {
		return errors.New("durable package metadata does not match its storage identity")
	}
	if !allowCreate {
		return errors.New("durable package metadata is missing")
	}
	if err := writePrivateJSONAtomic(location.stateRoot, metadataPath, location.metadata); err != nil {
		return fmt.Errorf("write durable package metadata: %w", err)
	}
	return nil
}

// PackageRuntimeDir returns a collision-safe package runtime path inside the checkout's disposable
// bin/.dockpipe tree. It does not create the directory and does not affect durable project state.
func PackageRuntimeDir(workdir, ownerID string) (string, error) {
	_, segment, _, err := durableOwnerStorageIdentity(ownerID)
	if err != nil {
		return "", err
	}
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "packages-runtime", segment), nil
}

// PreparePackageRuntimeDir creates and validates an owner-only package runtime directory beneath
// an existing checkout state root. It never changes the checkout state-root permissions and never
// falls back to durable package state.
func PreparePackageRuntimeDir(workdir, ownerID string) (string, error) {
	runtimeDir, err := PackageRuntimeDir(workdir, ownerID)
	if err != nil {
		return "", err
	}
	stateRoot, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(stateRoot)
	if err != nil {
		return "", fmt.Errorf("package runtime state root: %w", err)
	}
	if durableFileInfoIsLinkOrReparse(rootInfo) || !rootInfo.IsDir() {
		return "", errors.New("package runtime state root is linked, reparsed, or not a directory")
	}
	rootDevice, err := durableDeviceIdentity(stateRoot)
	if err != nil {
		return "", err
	}
	current := stateRoot
	for _, part := range []string{"packages-runtime", filepath.Base(runtimeDir)} {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !os.IsExist(mkdirErr) {
				return "", mkdirErr
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil {
			return "", statErr
		}
		if durableFileInfoIsLinkOrReparse(info) || !info.IsDir() {
			return "", fmt.Errorf("package runtime component %q is linked, reparsed, or not a directory", current)
		}
		device, deviceErr := durableDeviceIdentity(current)
		if deviceErr != nil || device != rootDevice {
			return "", fmt.Errorf("package runtime component %q crosses a filesystem boundary", current)
		}
		if err := makePrivatePath(current, true); err != nil {
			return "", err
		}
	}
	return runtimeDir, nil
}

// PreparePrivateStateSubdirectory creates one validated owner-only directory below an already
// private state root. It is used by package-owned durable/runtime layouts that need nested bind
// mount sources without allowing links, reparse points, or filesystem substitutions.
func PreparePrivateStateSubdirectory(root, suffix string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	if err := validatePrivatePath(root, true); err != nil {
		return "", fmt.Errorf("private state root: %w", err)
	}
	destination, err := JoinStatePath(root, suffix)
	if err != nil {
		return "", err
	}
	if err := ensurePrivateSubdirectory(root, destination); err != nil {
		return "", err
	}
	if err := validateExistingStateBoundary(root, destination); err != nil {
		return "", err
	}
	if err := validatePrivatePath(destination, true); err != nil {
		return "", err
	}
	return destination, nil
}

// ValidatePackageStateOverride accepts the resolved durable path or an independently existing,
// owner-only directory outside the checkout and disposable DockPipe runtime tree. It never creates
// or repairs an override and rejects links/reparse points in the resolved path.
func ValidatePackageStateOverride(workdir, candidate, resolved string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", errors.New("package-state override is empty")
	}
	candidate = HostPathForGit(candidate)
	if !filepath.IsAbs(candidate) {
		return "", errors.New("package-state override must be absolute")
	}
	candidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(filepath.Clean(resolved))
	if err != nil {
		return "", err
	}
	if sameDurablePath(candidate, resolved) {
		return resolved, nil
	}
	workdir, err = absHostWorkdir(workdir)
	if err != nil {
		return "", err
	}
	stateRoot, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	if sameOrWithinDurablePath(workdir, candidate) || sameOrWithinDurablePath(stateRoot, candidate) {
		return "", errors.New("package-state override must be outside the checkout and disposable state")
	}
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	canonical, err = filepath.Abs(filepath.Clean(canonical))
	if err != nil || !sameDurablePath(candidate, canonical) {
		return "", errors.New("package-state override contains a filesystem link or reparse point")
	}
	if err := validatePrivatePath(candidate, true); err != nil {
		return "", fmt.Errorf("package-state override is not owner-controlled: %w", err)
	}
	return candidate, nil
}

// DiscoverLegacyPackageState performs a read-only lookup of the pre-durable package-state path.
// It never creates the checkout state root or durable metadata and rejects linked, reparsed, or
// mount-substituted legacy boundaries.
func DiscoverLegacyPackageState(workdir, ownerID string) (path string, found bool, err error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "", false, errors.New("legacy package owner ID is empty")
	}
	if !utf8.ValidString(ownerID) || strings.IndexByte(ownerID, 0) >= 0 {
		return "", false, errors.New("legacy package owner ID contains invalid Unicode or NUL")
	}
	root, err := StateRoot(workdir)
	if err != nil {
		return "", false, err
	}
	path = filepath.Join(root, "packages", SanitizePackageStateScope(ownerID))
	info, statErr := os.Lstat(path)
	if os.IsNotExist(statErr) {
		return path, false, nil
	}
	if statErr != nil {
		return "", false, statErr
	}
	if !info.IsDir() || durableFileInfoIsLinkOrReparse(info) {
		return "", false, errors.New("legacy package state is linked, reparsed, or not a directory")
	}
	if err := validateExistingStateBoundary(root, path); err != nil {
		return "", false, fmt.Errorf("legacy package state: %w", err)
	}
	return path, true, nil
}

// JoinStatePath validates a suffix as data and joins it beneath an existing or future selected
// state root. Existing components are checked without following links or filesystem substitutions.
func JoinStatePath(root, suffix string) (string, error) {
	rel, err := validateStateSuffixForOS(runtime.GOOS, suffix)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.FromSlash(rel))
	if !sameOrWithinDurablePath(root, candidate) {
		return "", errors.New("state suffix escapes the selected root")
	}
	if _, statErr := os.Lstat(root); statErr == nil {
		if err := validateExistingStateBoundary(root, candidate); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	return candidate, nil
}

func validateStateSuffixForOS(goos, suffix string) (string, error) {
	if suffix == "" || suffix != strings.TrimSpace(suffix) || !utf8.ValidString(suffix) || strings.IndexByte(suffix, 0) >= 0 {
		return "", errors.New("state suffix is empty, untrimmed, or invalid Unicode")
	}
	if goos == "windows" {
		suffix = strings.ReplaceAll(suffix, `\`, "/")
	} else if strings.Contains(suffix, `\`) {
		return "", errors.New("state suffix contains a foreign path separator")
	}
	if strings.HasPrefix(suffix, "/") || filepath.IsAbs(suffix) || filepath.VolumeName(suffix) != "" {
		return "", errors.New("state suffix must be relative")
	}
	parts := strings.Split(suffix, "/")
	for _, part := range parts {
		if err := validateStatePathComponent(part); err != nil {
			return "", err
		}
	}
	canonical := strings.Join(parts, "/")
	if canonical != filepath.ToSlash(filepath.Clean(filepath.FromSlash(canonical))) {
		return "", errors.New("state suffix is not canonical")
	}
	return canonical, nil
}

func validateStatePathComponent(part string) error {
	if part == "" || part == "." || part == ".." {
		return errors.New("state suffix contains an empty, current, or parent component")
	}
	if strings.ContainsAny(part, `<>:"|?*`) || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
		return fmt.Errorf("state suffix component %q is unsafe on supported filesystems", part)
	}
	for _, r := range part {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("state suffix component %q contains a control character", part)
		}
	}
	base := part
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	upper := strings.ToUpper(base)
	if upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" ||
		(len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9') {
		return fmt.Errorf("state suffix component %q is a reserved device name", part)
	}
	return nil
}

func durableOwnerStorageIdentity(ownerID string) (canonical, segment, digest string, err error) {
	canonical = strings.TrimSpace(ownerID)
	if canonical == "" {
		return "", "", "", errors.New("durable owner ID is empty")
	}
	if canonical == "default" {
		return "", "", "", errors.New("durable owner ID uses the reserved default scope")
	}
	if !utf8.ValidString(canonical) || strings.IndexByte(canonical, 0) >= 0 {
		return "", "", "", errors.New("durable owner ID contains invalid Unicode or NUL")
	}
	for _, r := range canonical {
		// Durable owner IDs are currently constrained to the manifest-compatible ASCII identity
		// alphabet. Every accepted value is therefore already NFC without adding a normalization
		// dependency to the generic engine.
		if r > 0x7f || r < 0x20 || r == 0x7f {
			return "", "", "", errors.New("durable owner ID must use printable ASCII")
		}
	}
	sum := sha256.Sum256([]byte(canonical))
	digest = hex.EncodeToString(sum[:])
	slug := durableOwnerSlug(canonical)
	return canonical, slug + "-" + digest, digest, nil
}

func durableOwnerSlug(ownerID string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(ownerID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
		if b.Len() >= 48 {
			break
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "owner"
	}
	return slug
}

func durableProjectIdentity(workdir string) (canonicalPath, identity string, err error) {
	abs, err := absHostWorkdir(workdir)
	if err != nil {
		return "", "", err
	}
	canonicalPath, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "", fmt.Errorf("resolve project checkout: %w", err)
	}
	canonicalPath, err = filepath.Abs(filepath.Clean(canonicalPath))
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(canonicalPath)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", errors.New("project checkout is not a directory")
	}
	identity, err = durableFileIdentity(canonicalPath)
	if err != nil {
		return "", "", fmt.Errorf("project filesystem identity: %w", err)
	}
	return canonicalPath, identity, nil
}

func newDurableProjectID(projectsRoot string) (string, error) {
	for attempt := 0; attempt < 16; attempt++ {
		raw := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, raw); err != nil {
			return "", fmt.Errorf("generate durable project ID: %w", err)
		}
		id := hex.EncodeToString(raw)
		if _, err := os.Lstat(filepath.Join(projectsRoot, id)); os.IsNotExist(err) {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not allocate a collision-free durable project ID")
}
