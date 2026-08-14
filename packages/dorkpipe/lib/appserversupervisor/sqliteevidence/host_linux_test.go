//go:build linux

package sqliteevidence

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const linuxExt4Magic = 0xef53

type linuxMountInfo struct {
	ID           uint64
	ParentID     uint64
	Major        uint32
	Minor        uint32
	Root         string
	MountPoint   string
	MountOptions string
	Optional     []string
	FileSystem   string
	Source       string
	SuperOptions string
	Raw          string
}

type linuxPathFact struct {
	Path        string
	Kind        string
	Size        uint64
	Mode        uint16
	UID         uint32
	GID         uint32
	DeviceMajor uint32
	DeviceMinor uint32
	Inode       uint64
	MountID     uint64
	FSMagic     int64
}

type linuxQualification struct {
	FixtureRoot string
	Root        linuxPathFact
	Mount       linuxMountInfo
	UID         uint32
	GID         uint32
}

func selectedNativeVFS() string { return "unix" }

func collectAndProtectWindowsHost(string) (windowsHostFacts, error) {
	return windowsHostFacts{}, fmt.Errorf("Windows native evidence is unavailable on this host")
}

func requireWindowsPrivatePath(string) (string, error) {
	return "", fmt.Errorf("Windows DACL evidence is unavailable on this host")
}

func setWindowsPrivateDirectory(string) error {
	return fmt.Errorf("Windows DACL evidence is unavailable on this host")
}

func qualifyLinuxFixtureRoot(root string) (*linuxQualification, string, error) {
	return qualifyLinuxFixtureRootAtReviewedWholeDeviceMount(root, "")
}

func qualifyLinuxFixtureRootAtReviewedWholeDeviceMount(root, reviewedMountPoint string) (*linuxQualification, string, error) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return nil, "", fmt.Errorf("host is %s/%s, want linux/amd64", runtime.GOOS, runtime.GOARCH)
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return nil, "", fmt.Errorf("fixture root is not absolute: %q", root)
	}
	if err := requireReviewedWholeDeviceMountPoint(root, reviewedMountPoint); err != nil {
		return nil, "", err
	}
	if err := requireNoSymlinkComponents(root); err != nil {
		return nil, "", err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, "", fmt.Errorf("set fixture root mode: %w", err)
	}
	rootFact, err := readLinuxPathFact(root)
	if err != nil {
		return nil, "", err
	}
	if rootFact.Kind != "directory" || rootFact.Mode != 0o700 {
		return nil, "", fmt.Errorf("fixture root kind/mode = %s/%#o, want directory/0700", rootFact.Kind, rootFact.Mode)
	}
	uid, gid := uint32(os.Geteuid()), uint32(os.Getegid())
	if rootFact.UID != uid || rootFact.GID != gid {
		return nil, "", fmt.Errorf("fixture root owner = %d:%d, want effective %d:%d", rootFact.UID, rootFact.GID, uid, gid)
	}
	mounts, err := readLinuxMountInfo()
	if err != nil {
		return nil, "", err
	}
	mount, err := exactMountByID(mounts, rootFact.MountID)
	if err != nil {
		return nil, "", err
	}
	if err := requireQualifiedExt4MountAtReviewedMount(root, reviewedMountPoint, rootFact, mount, mounts); err != nil {
		return nil, "", err
	}
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return nil, "", fmt.Errorf("uname: %w", err)
	}
	release := unixField(uts.Release[:])
	version := unixField(uts.Version[:])
	major, minor, err := kernelMajorMinor(release)
	if err != nil || major < 5 || major == 5 && minor < 8 {
		return nil, "", fmt.Errorf("kernel release %q is below 5.8 or invalid: %v", release, err)
	}
	distribution, err := linuxDistribution()
	if err != nil {
		return nil, "", err
	}
	virtualization, err := linuxVirtualization()
	if err != nil {
		return nil, "", err
	}
	qualification := &linuxQualification{FixtureRoot: root, Root: rootFact, Mount: mount, UID: uid, GID: gid}
	evidence := fmt.Sprintf("distribution=%q kernel_release=%q kernel_build=%q arch=%s virtualization=%q go=%s fixture_root=%q source=%q mount_point=%q mount_id=%d device=%d:%d fs=%s fs_magic=%#x mount_options=%q super_options=%q mountinfo=%q uid=%d gid=%d root_mode=%#o rejected={bind,nested,overlay,fuse,network,removable,shared-host,drvfs,9p,tmpfs,symlink,cross-mount}",
		distribution, release, version, runtime.GOARCH, virtualization, runtime.Version(), root, mount.Source,
		mount.MountPoint, mount.ID, mount.Major, mount.Minor, mount.FileSystem, linuxExt4Magic,
		mount.MountOptions, mount.SuperOptions, mount.Raw, uid, gid, rootFact.Mode)
	return qualification, evidence, nil
}

func (qualification *linuxQualification) rootIdentity() (string, error) {
	fact, err := readLinuxPathFact(qualification.FixtureRoot)
	if err != nil {
		return "", err
	}
	if !sameLinuxIdentity(fact, qualification.Root) {
		return "", fmt.Errorf("fixture root identity changed: got=%s want=%s", linuxIdentity(fact), linuxIdentity(qualification.Root))
	}
	return linuxIdentity(fact), nil
}

func (qualification *linuxQualification) prepareEvidenceFile(name string) (string, error) {
	if name != "main" && name != "other" {
		return "", fmt.Errorf("invalid evidence directory %q", name)
	}
	directory := filepath.Join(qualification.FixtureRoot, name)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return "", fmt.Errorf("create %s evidence directory: %w", name, err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", fmt.Errorf("set %s evidence directory mode: %w", name, err)
	}
	if _, err := qualification.requirePath(directory, "directory", 0o700); err != nil {
		return "", err
	}
	databasePath := filepath.Join(directory, "aggregate.sqlite")
	file, err := os.OpenFile(databasePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return "", fmt.Errorf("pre-create private %s database: %w", name, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close private %s database: %w", name, err)
	}
	if _, err := qualification.requirePath(databasePath, "file", 0o600); err != nil {
		return "", err
	}
	if err := qualification.requireSiblings(directory, false); err != nil {
		return "", err
	}
	return databasePath, nil
}

func (qualification *linuxQualification) requirePath(path, wantKind string, wantMode uint16) (linuxPathFact, error) {
	path = filepath.Clean(path)
	if err := requireContainedPath(qualification.FixtureRoot, path); err != nil {
		return linuxPathFact{}, err
	}
	if err := requireNoSymlinkComponents(path); err != nil {
		return linuxPathFact{}, err
	}
	root, err := readLinuxPathFact(qualification.FixtureRoot)
	if err != nil {
		return linuxPathFact{}, err
	}
	if !sameLinuxIdentity(root, qualification.Root) {
		return linuxPathFact{}, fmt.Errorf("fixture root was substituted: got=%s want=%s", linuxIdentity(root), linuxIdentity(qualification.Root))
	}
	fact, err := readLinuxPathFact(path)
	if err != nil {
		return linuxPathFact{}, err
	}
	if fact.Kind != wantKind || fact.Mode != wantMode {
		return linuxPathFact{}, fmt.Errorf("%s kind/mode = %s/%#o, want %s/%#o", path, fact.Kind, fact.Mode, wantKind, wantMode)
	}
	if fact.UID != qualification.UID || fact.GID != qualification.GID {
		return linuxPathFact{}, fmt.Errorf("%s owner = %d:%d, want effective %d:%d", path, fact.UID, fact.GID, qualification.UID, qualification.GID)
	}
	if fact.MountID != qualification.Root.MountID || fact.DeviceMajor != qualification.Root.DeviceMajor || fact.DeviceMinor != qualification.Root.DeviceMinor || fact.FSMagic != linuxExt4Magic {
		return linuxPathFact{}, fmt.Errorf("%s storage identity = mount=%d device=%d:%d magic=%#x, want mount=%d device=%d:%d magic=%#x", path, fact.MountID, fact.DeviceMajor, fact.DeviceMinor, fact.FSMagic, qualification.Root.MountID, qualification.Root.DeviceMajor, qualification.Root.DeviceMinor, linuxExt4Magic)
	}
	mounts, err := readLinuxMountInfo()
	if err != nil {
		return linuxPathFact{}, err
	}
	mount, err := exactMountByID(mounts, fact.MountID)
	if err != nil {
		return linuxPathFact{}, err
	}
	if mount.Raw != qualification.Mount.Raw {
		return linuxPathFact{}, fmt.Errorf("mountinfo row changed: got=%q want=%q", mount.Raw, qualification.Mount.Raw)
	}
	return fact, nil
}

func (qualification *linuxQualification) requireJournal(journalPath string) error {
	_, err := qualification.requirePath(journalPath, "file", 0o600)
	if err != nil {
		return err
	}
	return qualification.requireSiblings(filepath.Dir(journalPath), true)
}

func (qualification *linuxQualification) requireSiblings(directory string, requireJournal bool) error {
	if _, err := qualification.requirePath(directory, "directory", 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read database siblings: %w", err)
	}
	seenMain := false
	seenJournal := false
	for _, entry := range entries {
		switch entry.Name() {
		case "aggregate.sqlite":
			seenMain = true
		case "aggregate.sqlite-journal":
			seenJournal = true
		default:
			return fmt.Errorf("unexpected database sibling %q", entry.Name())
		}
		if _, err := qualification.requirePath(filepath.Join(directory, entry.Name()), "file", 0o600); err != nil {
			return err
		}
	}
	if !seenMain || requireJournal && !seenJournal {
		return fmt.Errorf("database siblings main=%t journal=%t require_journal=%t", seenMain, seenJournal, requireJournal)
	}
	return nil
}

func (qualification *linuxQualification) stableTreeHash() (string, error) {
	first, err := qualification.treeHash()
	if err != nil {
		return "", err
	}
	second, err := qualification.treeHash()
	if err != nil {
		return "", err
	}
	if first != second {
		return "", fmt.Errorf("metadata tree changed while quiescent: first=%s second=%s", first, second)
	}
	return first, nil
}

func (qualification *linuxQualification) treeHash() (string, error) {
	rows := make([]string, 0, 7)
	err := filepath.WalkDir(qualification.FixtureRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		kind := "file"
		mode := uint16(0o600)
		if entry.IsDir() {
			kind = "directory"
			mode = 0o700
		}
		fact, err := qualification.requirePath(path, kind, mode)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(qualification.FixtureRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative != "." {
			parts := strings.Split(relative, "/")
			if len(parts) > 2 || parts[0] != "main" && parts[0] != "other" || len(parts) == 2 && parts[1] != "aggregate.sqlite" && parts[1] != "aggregate.sqlite-journal" {
				return fmt.Errorf("unexpected metadata-tree path %q", relative)
			}
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\t%d\t%#o\t%d\t%d\t%d:%d\t%d\t%d\t%s\t%#x\t%s\t%s", relative, fact.Kind, fact.Size, fact.Mode, fact.UID, fact.GID, fact.DeviceMajor, fact.DeviceMinor, fact.Inode, fact.MountID, qualification.Mount.FileSystem, fact.FSMagic, qualification.Mount.Source, qualification.Mount.MountPoint))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rows)
	digest := sha256Bytes([]byte(strings.Join(rows, "\n") + "\n"))
	return hex.EncodeToString(digest), nil
}

func (qualification *linuxQualification) validateChildPath(root, databasePath, rootIdentity string) error {
	if root != qualification.FixtureRoot {
		return fmt.Errorf("child root = %q, want %q", root, qualification.FixtureRoot)
	}
	identity, err := qualification.rootIdentity()
	if err != nil {
		return err
	}
	if rootIdentity == "" || rootIdentity != identity {
		return fmt.Errorf("child root identity = %q, want %q", identity, rootIdentity)
	}
	if filepath.Base(databasePath) != "aggregate.sqlite" {
		return fmt.Errorf("child database basename = %q", filepath.Base(databasePath))
	}
	parent := filepath.Base(filepath.Dir(databasePath))
	if parent != "main" && parent != "other" {
		return fmt.Errorf("child database session directory = %q", parent)
	}
	_, err = qualification.requirePath(databasePath, "file", 0o600)
	return err
}

func readLinuxPathFact(path string) (linuxPathFact, error) {
	var stat unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_NO_AUTOMOUNT|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BASIC_STATS|unix.STATX_MNT_ID, &stat); err != nil {
		return linuxPathFact{}, fmt.Errorf("statx %s: %w", path, err)
	}
	if stat.Mask&unix.STATX_MNT_ID == 0 {
		return linuxPathFact{}, fmt.Errorf("statx %s omitted STATX_MNT_ID: mask=%#x", path, stat.Mask)
	}
	kind := ""
	flags := unix.O_PATH | unix.O_CLOEXEC | unix.O_NOFOLLOW
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		kind = "directory"
		flags |= unix.O_DIRECTORY
	case unix.S_IFREG:
		kind = "file"
	default:
		return linuxPathFact{}, fmt.Errorf("%s has unsupported type bits %#o", path, stat.Mode&unix.S_IFMT)
	}
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return linuxPathFact{}, fmt.Errorf("open metadata-only handle for %s: %w", path, err)
	}
	defer unix.Close(fd)
	var fileSystem unix.Statfs_t
	if err := unix.Fstatfs(fd, &fileSystem); err != nil {
		return linuxPathFact{}, fmt.Errorf("fstatfs %s: %w", path, err)
	}
	return linuxPathFact{Path: path, Kind: kind, Size: stat.Size, Mode: stat.Mode & 0o7777, UID: stat.Uid, GID: stat.Gid, DeviceMajor: stat.Dev_major, DeviceMinor: stat.Dev_minor, Inode: stat.Ino, MountID: stat.Mnt_id, FSMagic: fileSystem.Type}, nil
}

func readLinuxMountInfo() ([]linuxMountInfo, error) {
	payload, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("read mountinfo: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	mounts := make([]linuxMountInfo, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed mountinfo row %q", line)
		}
		left := strings.Fields(parts[0])
		right := strings.Fields(parts[1])
		if len(left) < 6 || len(right) < 3 {
			return nil, fmt.Errorf("short mountinfo row %q", line)
		}
		id, err := strconv.ParseUint(left[0], 10, 64)
		if err != nil {
			return nil, err
		}
		parent, err := strconv.ParseUint(left[1], 10, 64)
		if err != nil {
			return nil, err
		}
		device := strings.Split(left[2], ":")
		if len(device) != 2 {
			return nil, fmt.Errorf("invalid mountinfo device %q", left[2])
		}
		major, err := strconv.ParseUint(device[0], 10, 32)
		if err != nil {
			return nil, err
		}
		minor, err := strconv.ParseUint(device[1], 10, 32)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, linuxMountInfo{ID: id, ParentID: parent, Major: uint32(major), Minor: uint32(minor), Root: mountInfoUnescape(left[3]), MountPoint: mountInfoUnescape(left[4]), MountOptions: left[5], Optional: append([]string(nil), left[6:]...), FileSystem: right[0], Source: mountInfoUnescape(right[1]), SuperOptions: strings.Join(right[2:], " "), Raw: line})
	}
	return mounts, nil
}

func exactMountByID(mounts []linuxMountInfo, id uint64) (linuxMountInfo, error) {
	var found []linuxMountInfo
	for _, mount := range mounts {
		if mount.ID == id {
			found = append(found, mount)
		}
	}
	if len(found) != 1 {
		return linuxMountInfo{}, fmt.Errorf("mount ID %d has %d mountinfo rows, want 1", id, len(found))
	}
	return found[0], nil
}

func requireQualifiedExt4Mount(root string, rootFact linuxPathFact, mount linuxMountInfo, mounts []linuxMountInfo) error {
	return requireQualifiedExt4MountAtReviewedMount(root, "", rootFact, mount, mounts)
}

func requireQualifiedExt4MountAtReviewedMount(root, reviewedMountPoint string, rootFact linuxPathFact, mount linuxMountInfo, mounts []linuxMountInfo) error {
	return requireQualifiedExt4MountAtReviewedMountWithProbes(root, reviewedMountPoint, rootFact, mount, mounts, func(source string) (os.FileMode, error) {
		sourceInfo, err := os.Stat(source)
		return modeOrZero(sourceInfo), err
	}, linuxBlockRemovable)
}

func requireQualifiedExt4MountWithProbes(root string, rootFact linuxPathFact, mount linuxMountInfo, mounts []linuxMountInfo, statSource func(string) (os.FileMode, error), blockRemovable func(string) (bool, error)) error {
	return requireQualifiedExt4MountAtReviewedMountWithProbes(root, "", rootFact, mount, mounts, statSource, blockRemovable)
}

func requireQualifiedExt4MountAtReviewedMountWithProbes(root, reviewedMountPoint string, rootFact linuxPathFact, mount linuxMountInfo, mounts []linuxMountInfo, statSource func(string) (os.FileMode, error), blockRemovable func(string) (bool, error)) error {
	if err := requireReviewedWholeDeviceMountPoint(root, reviewedMountPoint); err != nil {
		return err
	}
	if rootFact.FSMagic != linuxExt4Magic || mount.FileSystem != "ext4" {
		return fmt.Errorf("fixture filesystem magic/type = %#x/%q, want %#x/ext4", rootFact.FSMagic, mount.FileSystem, linuxExt4Magic)
	}
	if mount.ID != rootFact.MountID || mount.Major != rootFact.DeviceMajor || mount.Minor != rootFact.DeviceMinor {
		return fmt.Errorf("fixture mount identity = mount=%d device=%d:%d, want mount=%d device=%d:%d", mount.ID, mount.Major, mount.Minor, rootFact.MountID, rootFact.DeviceMajor, rootFact.DeviceMinor)
	}
	if mount.Root != "/" {
		return fmt.Errorf("fixture mount root = %q, want whole-device root /", mount.Root)
	}
	if reviewedMountPoint != "" && mount.MountPoint != reviewedMountPoint {
		return fmt.Errorf("fixture mount point = %q, want exact reviewed whole-device mount %q", mount.MountPoint, reviewedMountPoint)
	}
	if reviewedMountPoint == "" && mount.MountPoint != "/" && mount.MountPoint != root {
		return fmt.Errorf("fixture mount root/point = %q/%q, want whole-device root / mounted at / or fixture root %q", mount.Root, mount.MountPoint, root)
	}
	if !optionContains(mount.MountOptions, "rw") || !optionContains(mount.SuperOptions, "rw") || optionContains(mount.MountOptions, "ro") {
		return fmt.Errorf("fixture mount is not unambiguously read-write: mount=%q super=%q", mount.MountOptions, mount.SuperOptions)
	}
	if !strings.HasPrefix(mount.Source, "/dev/") {
		return fmt.Errorf("fixture source %q is not a local block device", mount.Source)
	}
	sourceMode, err := statSource(mount.Source)
	if err != nil || sourceMode&os.ModeDevice == 0 {
		return fmt.Errorf("fixture source %q is not an accessible block device: mode=%v err=%v", mount.Source, sourceMode, err)
	}
	removable, err := blockRemovable(filepath.Base(mount.Source))
	if err != nil || removable {
		return fmt.Errorf("fixture source removable=%t err=%v", removable, err)
	}
	for _, candidate := range mounts {
		if candidate.ID == mount.ID {
			continue
		}
		point := filepath.Clean(candidate.MountPoint)
		if (isPathWithin(point, root) && point != "/") || isPathWithin(root, point) {
			return fmt.Errorf("fixture path crosses or contains nested mount %q: %q", point, candidate.Raw)
		}
	}
	return nil
}

func requireReviewedWholeDeviceMountPoint(root, reviewedMountPoint string) error {
	if reviewedMountPoint == "" {
		return nil
	}
	cleanMountPoint := filepath.Clean(reviewedMountPoint)
	if !filepath.IsAbs(reviewedMountPoint) || cleanMountPoint != reviewedMountPoint || cleanMountPoint == "/" || cleanMountPoint == root || !isPathWithin(cleanMountPoint, root) {
		return fmt.Errorf("fixture root %q is not strictly beneath exact reviewed whole-device mount %q", root, reviewedMountPoint)
	}
	return nil
}

func linuxBlockRemovable(blockName string) (bool, error) {
	classPath := filepath.Join("/sys/class/block", blockName)
	realPath, err := filepath.EvalSymlinks(classPath)
	if err != nil {
		return false, err
	}
	deviceName := blockName
	if _, err := os.Stat(filepath.Join(classPath, "partition")); err == nil {
		deviceName = filepath.Base(filepath.Dir(realPath))
	}
	payload, err := os.ReadFile(filepath.Join("/sys/class/block", deviceName, "removable"))
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(string(payload))
	if value != "0" && value != "1" {
		return false, fmt.Errorf("invalid removable value %q", value)
	}
	return value == "1", nil
}

func requireNoSymlinkComponents(path string) error {
	path = filepath.Clean(path)
	current := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("lstat path component %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component %s is a symlink", current)
		}
	}
	return nil
}

func requireContainedPath(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path %q is not beneath fixture root %q", path, root)
	}
	return nil
}

func linuxDistribution() (string, error) {
	payload, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("read os-release: %w", err)
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(payload), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = strings.Trim(value, `"`)
		}
	}
	if values["ID"] != "pop" || values["VERSION_ID"] == "" || values["PRETTY_NAME"] == "" {
		return "", fmt.Errorf("unsupported or incomplete distribution identity: ID=%q VERSION_ID=%q PRETTY_NAME=%q", values["ID"], values["VERSION_ID"], values["PRETTY_NAME"])
	}
	return values["PRETTY_NAME"], nil
}

func linuxVirtualization() (string, error) {
	command := exec.Command("systemd-detect-virt", "--vm")
	output, err := command.Output()
	value := strings.TrimSpace(string(output))
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 && (value == "" || value == "none") {
			return "bare-metal(systemd-detect-virt=none)", nil
		}
		return "", fmt.Errorf("detect virtualization: %w output=%q", err, value)
	}
	if value == "none" {
		return "bare-metal(systemd-detect-virt=none)", nil
	}
	return "virtual-machine(" + value + ")", nil
}

func kernelMajorMinor(release string) (int, int, error) {
	version := strings.SplitN(release, "-", 2)[0]
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid kernel release %q", release)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minor, err := strconv.Atoi(parts[1])
	return major, minor, err
}

func unixField(value []byte) string {
	if index := strings.IndexByte(string(value), 0); index >= 0 {
		value = value[:index]
	}
	return string(value)
}

func mountInfoUnescape(value string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(value)
}

func optionContains(options, want string) bool {
	for _, option := range strings.Split(options, ",") {
		if option == want {
			return true
		}
	}
	return false
}

func isPathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func sameLinuxIdentity(first, second linuxPathFact) bool {
	return first.MountID == second.MountID && first.DeviceMajor == second.DeviceMajor && first.DeviceMinor == second.DeviceMinor && first.Inode == second.Inode && first.Kind == second.Kind
}

func linuxIdentity(fact linuxPathFact) string {
	return fmt.Sprintf("mount=%d device=%d:%d inode=%d kind=%s", fact.MountID, fact.DeviceMajor, fact.DeviceMinor, fact.Inode, fact.Kind)
}

func modeOrZero(info os.FileInfo) os.FileMode {
	if info == nil {
		return 0
	}
	return info.Mode()
}
